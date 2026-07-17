package acquirer

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// ConcurrencyLimiter enforces bounded in-flight + bounded queue wait.
// Requests beyond the limit are rejected with 429 (backpressure) instead of letting
// the system pile up infinite in-flight work and timing out later.
//
// Limit is calculated via Little's Law from observed system metrics:
//
//	L = λ × W
//	where λ = throughput (req/s), W = latency (s)
//
// Example: 700 req/s @ 10ms → L = 7, use 10 for safety margin.
//
// ÖNEMLİ KAVRAMLAR:
// - inflight: Şu anda işlenen (downstream'e gönderilmiş) request sayısı
// - queued: Slot bekleyen (henüz işlenmeye başlamamış) request sayısı
// - limit: Aynı anda maksimum kaç request işlenebilir (Little's Law'dan hesaplanır)
// - maxQueue: Maksimum kaç request kuyrukta bekleyebilir
// - queueWaitTimeout: Bir request kuyrukta en fazla ne kadar bekleyebilir
type ConcurrencyLimiter struct {
	inflight int64 // atomic: şu anda işlenen request sayısı
	queued   int64 // atomic: kuyrukta bekleyen request sayısı

	limit            int64         // maksimum eşzamanlı işlem sayısı (static)
	maxQueue         int64         // maksimum kuyruk boyutu
	queueWaitTimeout time.Duration // kuyrukta maksimum bekleme süresi

	rejectedTotal atomic.Uint64 // toplam reddedilen request sayısı (metrik)
}

// NewConcurrencyLimiter creates a limiter from env vars.
//
// - CONCURRENT_LIMIT: max in-flight (default 200)
// - MAX_QUEUE: max number of queued waiters (default 500)
// - QUEUE_WAIT_TIMEOUT_MS: max time to wait for a slot before rejecting (default 50ms)
func NewConcurrencyLimiter() *ConcurrencyLimiter {
	limit := envInt("CONCURRENT_LIMIT", 200)
	maxQueue := envInt("MAX_QUEUE", 500)
	queueWaitTimeout := envDurationMS("QUEUE_WAIT_TIMEOUT_MS", 50) * time.Millisecond

	if limit < 1 {
		limit = 1
	}
	if maxQueue < 0 {
		maxQueue = 0
	}
	if queueWaitTimeout < 0 {
		queueWaitTimeout = 0
	}

	log.Printf("acquirer: limiter concurrent_limit=%d max_queue=%d queue_wait_timeout=%s", limit, maxQueue, queueWaitTimeout)
	return &ConcurrencyLimiter{
		limit:            int64(limit),
		maxQueue:         int64(maxQueue),
		queueWaitTimeout: queueWaitTimeout,
	}
}

// tryAcquire atomik olarak bir slot almaya çalışır.
// Compare-And-Swap (CAS) döngüsü kullanır: race condition'dan korunmak için.
//
// Mantık:
// 1. Mevcut inflight değerini oku
// 2. Eğer limit'e ulaşılmışsa → false döndür (slot yok)
// 3. CAS ile inflight'ı +1 artırmaya çalış
// 4. CAS başarısızsa (başka goroutine araya girdiyse) → tekrar dene (for loop)
// 5. CAS başarılıysa → true döndür (slot alındı)
//
// ÖNEMLİ: Bu fonksiyon lock-free'dir, mutex kullanmaz. Yüksek concurrency'de
// performanslıdır çünkü sadece atomic CPU instruction kullanır.
func (cl *ConcurrencyLimiter) tryAcquire() bool {
	for {
		current := atomic.LoadInt64(&cl.inflight) // 1. Mevcut değeri oku
		if current >= cl.limit {                  // 2. Limit kontrolü
			return false // Slot yok, hemen red
		}
		// 3. CAS: "current" hâlâ geçerliyse +1 yap
		if atomic.CompareAndSwapInt64(&cl.inflight, current, current+1) {
			return true // Başarılı, slot alındı
		}
		// CAS başarısız (race), tekrar dene
	}
}

func (cl *ConcurrencyLimiter) release() {
	atomic.AddInt64(&cl.inflight, -1)
}

// Middleware enforces the concurrency limit.
//
// AKIŞ:
//  1. Hemen slot almayı dene (tryAcquire)
//     → Başarılıysa: işlemi yap, slot'u geri ver (defer release)
//  2. Slot yoksa: kuyruk kontrolü
//     → maxQueue=0 ise: hemen 429 döndür (queue disabled)
//     → maxQueue>0 ise: kuyruğa gir
//  3. Kuyrukta:
//     → Sürekli tryAcquire dene (1ms polling)
//     → Timeout olursa: 429 döndür
//     → Context cancel olursa: 408 döndür
//     → Slot bulunursa: işlemi yap
func (cl *ConcurrencyLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startWait := time.Now()

		// FAST PATH: Hemen slot var mı?
		if cl.tryAcquire() {
			c.Set("queue_wait_ms", float64(0)) // Kuyruk bekleme yok
			defer cl.release()                 // İşlem bitince slot'u geri ver
			c.Next()                           // Handler'ı çalıştır
			return
		}

		// SLOW PATH: Slot yok, kuyruk kontrolü
		// Eğer kuyruk devre dışıysa (maxQueue=0) → hemen red
		if cl.maxQueue == 0 {
			cl.rejectedTotal.Add(1)      // Metrik: reddedilen sayısını artır
			c.Header("Retry-After", "1") // Client'a 1 saniye sonra tekrar denemesini söyle
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":  "overloaded",
				"active": atomic.LoadInt64(&cl.inflight), // Kaç request işleniyor
				"limit":  cl.limit,                       // Limit neydi
			})
			return
		}

		// Kuyruğa girmeyi dene (atomik olarak queued sayacını +1 artır)
		queuedNow := atomic.AddInt64(&cl.queued, 1)
		if queuedNow > cl.maxQueue {
			// Kuyruk dolu! Geri çık
			atomic.AddInt64(&cl.queued, -1) // Sayacı geri al
			cl.rejectedTotal.Add(1)
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":     "overloaded",
				"reason":    "queue_full",
				"queued":    atomic.LoadInt64(&cl.queued),
				"max_queue": cl.maxQueue,
			})
			return
		}
		// Kuyruktan çıkarken sayacı azalt (defer: fonksiyon bitince çalışır)
		defer atomic.AddInt64(&cl.queued, -1)

		// Timeout timer'ı başlat (eğer timeout ayarlanmışsa)
		var timer *time.Timer
		if cl.queueWaitTimeout > 0 {
			timer = time.NewTimer(cl.queueWaitTimeout)
			defer timer.Stop() // Fonksiyon bitince timer'ı temizle (memory leak önleme)
		}

		// POLLING LOOP: Sürekli slot aramaya devam et
		for {
			// Her iterasyonda slot almayı dene
			if cl.tryAcquire() {
				// Slot bulundu! Ne kadar bekledik?
				c.Set("queue_wait_ms", float64(time.Since(startWait).Milliseconds()))
				defer cl.release() // İşlem bitince slot'u geri ver
				c.Next()           // Handler'ı çalıştır
				return
			}

			// Context kontrolü: Client bağlantıyı kesti mi?
			select {
			case <-c.Request.Context().Done():
				// Client timeout oldu veya bağlantıyı kapattı
				cl.rejectedTotal.Add(1)
				c.AbortWithStatusJSON(http.StatusRequestTimeout, gin.H{"error": "cancelled"})
				return
			default:
				// Context hâlâ aktif, devam et
			}

			// Timeout kontrolü: Çok uzun süre bekledik mi?
			if timer != nil {
				select {
				case <-timer.C:
					// Timeout! Kuyrukta çok bekledik, red et
					cl.rejectedTotal.Add(1)
					c.Header("Retry-After", "1")
					c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
						"error":     "overloaded",
						"reason":    "queue_wait_timeout",
						"waited_ms": time.Since(startWait).Milliseconds(),
					})
					return
				default:
					// Henüz timeout olmadı, devam et
				}
			}

			// 1ms bekle, sonra tekrar dene (busy-wait yerine CPU'ya nefes aldır)
			// ÖNEMLİ: Bu polling stratejisi basit ama etkili. Alternatif: condition variable
			// ama o daha karmaşık ve bu use case için gerekli değil.
			time.Sleep(1 * time.Millisecond)
		}
	}
}

func (cl *ConcurrencyLimiter) Active() int64      { return atomic.LoadInt64(&cl.inflight) }
func (cl *ConcurrencyLimiter) Queued() int64      { return atomic.LoadInt64(&cl.queued) }
func (cl *ConcurrencyLimiter) Limit() int64       { return cl.limit }
func (cl *ConcurrencyLimiter) MaxConcurrent() int { return int(cl.limit) }
func (cl *ConcurrencyLimiter) MaxQueue() int      { return int(cl.maxQueue) }
func (cl *ConcurrencyLimiter) RejectedTotal() uint64 {
	return cl.rejectedTotal.Load()
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func envDurationMS(key string, defMS int) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return time.Duration(n)
		}
	}
	return time.Duration(defMS)
}
