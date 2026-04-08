package acquirer

import (
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// AdaptiveLimiter implements TCP Vegas-style congestion control for HTTP requests.
// Instead of a fixed concurrency limit, it dynamically adjusts based on
// observed downstream latency (switch_rtt).
//
// When downstream is fast → limit increases → more throughput
// When downstream slows  → limit decreases → prevents congestion collapse
type AdaptiveLimiter struct {
	inflight int64 // atomic: requests currently being processed
	limit    int64 // atomic: current dynamic concurrency limit
	queued   int64 // atomic: requests waiting in queue

	notify chan struct{} // signals slot availability to waiting goroutines

	minLimit        int
	maxLimit        int
	maxQueue        int
	queueTimeout    time.Duration
	sampleSize      int // samples per adjustment window
	healthThreshold int // consecutive healthy windows required to increase

	// AIMD multi-signal controller
	mu sync.Mutex

	// RTT signal (EWMA + decaying baseline)
	rttEWMA   float64
	minRTT    float64
	minRTTAge int // samples since minRTT last updated; triggers recalibration

	// Failure rate signal (EWMA + derivative)
	totalInWindow  int
	failedInWindow int
	failRateEWMA   float64
	prevFailRate   float64

	// Anti-oscillation state
	healthyStreak int
	probeCount    int
}

// NewAdaptiveLimiter creates an AIMD multi-signal adaptive limiter from environment variables.
func NewAdaptiveLimiter() *AdaptiveLimiter {
	minLimit := envInt("MIN_CONCURRENT", 5)
	maxLimit := envInt("MAX_CONCURRENT", 100)
	initial := envInt("INITIAL_CONCURRENT", 10)
	maxQueue := envInt("MAX_QUEUE", 200)
	queueTimeoutSec := envInt("QUEUE_TIMEOUT_SEC", 5)
	sampleSize := envInt("LIMITER_SAMPLE_SIZE", 50)
	healthThreshold := envInt("LIMITER_HEALTH_THRESHOLD", 3)

	al := &AdaptiveLimiter{
		notify:          make(chan struct{}, maxLimit),
		minLimit:        minLimit,
		maxLimit:        maxLimit,
		maxQueue:        maxQueue,
		queueTimeout:    time.Duration(queueTimeoutSec) * time.Second,
		sampleSize:      sampleSize,
		healthThreshold: healthThreshold,
	}
	atomic.StoreInt64(&al.limit, int64(initial))

	log.Printf("acquirer: adaptive limiter min=%d initial=%d max=%d queue=%d sample=%d health=%d",
		minLimit, initial, maxLimit, maxQueue, sampleSize, healthThreshold)

	return al
}

// tryAcquire atomically increments inflight if below the current dynamic limit.
func (al *AdaptiveLimiter) tryAcquire() bool {
	for {
		current := atomic.LoadInt64(&al.inflight)
		if current >= atomic.LoadInt64(&al.limit) {
			return false
		}
		if atomic.CompareAndSwapInt64(&al.inflight, current, current+1) {
			return true
		}
	}
}

// release decrements inflight and notifies one waiting goroutine.
func (al *AdaptiveLimiter) release() {
	atomic.AddInt64(&al.inflight, -1)
	select {
	case al.notify <- struct{}{}:
	default:
	}
}

// RecordRTT records a successful downstream call duration.
// Feeds RTT into EWMA and triggers periodic AIMD adjustment.
func (al *AdaptiveLimiter) RecordRTT(rttMs float64) {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.rttEWMA == 0 {
		al.rttEWMA = rttMs
		al.minRTT = rttMs
	} else {
		al.rttEWMA = 0.2*rttMs + 0.8*al.rttEWMA

		// Decay minRTT to prevent stale non-stationary baseline.
		// After 300 samples without a new minimum, recalibrate to 90% of current EWMA.
		al.minRTTAge++
		if al.minRTTAge > 300 {
			al.minRTT = al.rttEWMA * 0.9
			al.minRTTAge = 0
		}
		if rttMs < al.minRTT {
			al.minRTT = rttMs
			al.minRTTAge = 0
		}
	}

	al.totalInWindow++
	al.probeCount++
	if al.probeCount >= al.sampleSize {
		al.probeCount = 0
		al.adjustLimit()
	}
}

// RecordTimeout records a downstream timeout/error.
// Counted in the failure window; triggers adjustment at next sample boundary.
func (al *AdaptiveLimiter) RecordTimeout() {
	al.mu.Lock()
	defer al.mu.Unlock()

	al.failedInWindow++
	al.totalInWindow++
	al.probeCount++
	if al.probeCount >= al.sampleSize {
		al.probeCount = 0
		al.adjustLimit()
	}
}

// adjustLimit implements AIMD congestion control with multi-signal feedback.
// Must be called under al.mu.
//
// Degradation signals (any triggers Multiplicative Decrease):
//  1. RTT ratio   — rttEWMA / minRTT > 1.5 (latency inflated >50% above baseline)
//  2. Failure rate — EWMA of window failure fraction > 5%
//  3. Failure rate derivative — rising failure rate (>2% worsening per window)
//
// Recovery (Additive Increase):
//
//	Requires healthThreshold consecutive healthy windows (anti-oscillation hysteresis).
func (al *AdaptiveLimiter) adjustLimit() {
	current := atomic.LoadInt64(&al.limit)

	// --- Signal 1: RTT ratio ---
	rttRatio := 1.0
	if al.minRTT > 0 && al.rttEWMA > 0 {
		rttRatio = al.rttEWMA / al.minRTT
	}

	// --- Signal 2: Failure rate + derivative ---
	failRate := 0.0
	if al.totalInWindow > 0 {
		failRate = float64(al.failedInWindow) / float64(al.totalInWindow)
	}
	al.prevFailRate = al.failRateEWMA
	al.failRateEWMA = 0.3*failRate + 0.7*al.failRateEWMA
	failRateDelta := al.failRateEWMA - al.prevFailRate

	// Reset window for next period
	al.totalInWindow = 0
	al.failedInWindow = 0

	// --- Degradation detection (multi-signal OR) ---
	const (
		rttThreshold       = 1.5  // RTT inflated >50% above decaying baseline
		failRateThreshold  = 0.05 // >5% failure rate (EWMA)
		failDeltaThreshold = 0.02 // error rate rising >2% absolute per window
	)

	degraded := rttRatio > rttThreshold ||
		al.failRateEWMA > failRateThreshold ||
		failRateDelta > failDeltaThreshold

	if degraded {
		// Multiplicative Decrease: fast reaction
		newLimit := int64(float64(current) * 0.75)
		if newLimit < int64(al.minLimit) {
			newLimit = int64(al.minLimit)
		}
		atomic.StoreInt64(&al.limit, newLimit)
		al.healthyStreak = 0
		log.Printf("acquirer: adaptive ↓ limit=%d→%d rtt=%.1fx fail=%.1f%% Δfail=%.1f%%",
			current, newLimit, rttRatio, al.failRateEWMA*100, failRateDelta*100)
		return
	}

	// --- Recovery: require sustained health before increasing ---
	al.healthyStreak++
	if al.healthyStreak < al.healthThreshold {
		return // wait for more consecutive healthy windows
	}
	al.healthyStreak = 0

	// Additive Increase: slow, conservative recovery
	newLimit := current + 1
	if newLimit > int64(al.maxLimit) {
		newLimit = int64(al.maxLimit)
	}
	atomic.StoreInt64(&al.limit, newLimit)
	log.Printf("acquirer: adaptive ↑ limit=%d→%d rtt=%.1fx fail=%.1f%%",
		current, newLimit, rttRatio, al.failRateEWMA*100)
}

// Middleware returns a Gin middleware with adaptive concurrency control.
func (al *AdaptiveLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		arrival := time.Now()

		// Fast path: try to acquire immediately
		if al.tryAcquire() {
			defer al.release()
			c.Set("queue_wait_ms", float64(0))
			c.Next()
			return
		}

		// Gradual load shedding based on queue pressure
		currentQueued := atomic.LoadInt64(&al.queued)
		if al.maxQueue > 0 {
			fillPct := float64(currentQueued) / float64(al.maxQueue) * 100
			var rejectPct int
			if fillPct > 95 {
				rejectPct = 90
			} else if fillPct > 90 {
				rejectPct = 60
			} else if fillPct > 80 {
				rejectPct = 20
			}
			if rejectPct > 0 && rand.Intn(100) < rejectPct {
				log.Printf("acquirer: load shedding — queue %.0f%% full, rejecting", fillPct)
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error":       "server busy — load shedding",
					"retry_after": 1,
				})
				return
			}
		}

		// Enter queue
		queued := atomic.AddInt64(&al.queued, 1)
		if queued > int64(al.maxQueue) {
			atomic.AddInt64(&al.queued, -1)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error":       "server busy — queue full",
				"retry_after": 1,
			})
			return
		}

		// Wait for a slot notification with timeout
		timer := time.NewTimer(al.queueTimeout)
		defer timer.Stop()

		for {
			select {
			case <-al.notify:
				if al.tryAcquire() {
					atomic.AddInt64(&al.queued, -1)
					defer al.release()
					c.Set("queue_wait_ms", float64(time.Since(arrival).Milliseconds()))
					c.Next()
					return
				}

			case <-timer.C:
				atomic.AddInt64(&al.queued, -1)
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
					"error":       "server busy — queue timeout",
					"retry_after": 5,
				})
				return

			case <-c.Request.Context().Done():
				atomic.AddInt64(&al.queued, -1)
				c.Abort()
				return
			}
		}
	}
}

// Active returns the number of requests currently being processed.
func (al *AdaptiveLimiter) Active() int64 {
	return atomic.LoadInt64(&al.inflight)
}

// Queued returns the number of requests waiting in queue.
func (al *AdaptiveLimiter) Queued() int64 {
	return atomic.LoadInt64(&al.queued)
}

// Limit returns the current dynamic concurrency limit.
func (al *AdaptiveLimiter) Limit() int64 {
	return atomic.LoadInt64(&al.limit)
}

// MaxConcurrent returns the configured max (ceiling) limit.
func (al *AdaptiveLimiter) MaxConcurrent() int {
	return al.maxLimit
}

// MaxQueue returns the configured max queue size.
func (al *AdaptiveLimiter) MaxQueue() int {
	return al.maxQueue
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
