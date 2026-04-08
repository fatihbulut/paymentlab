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
type ConcurrencyLimiter struct {
	inflight int64
	queued   int64

	limit            int64
	maxQueue         int64
	queueWaitTimeout time.Duration

	rejectedTotal atomic.Uint64
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

func (cl *ConcurrencyLimiter) tryAcquire() bool {
	for {
		current := atomic.LoadInt64(&cl.inflight)
		if current >= cl.limit {
			return false
		}
		if atomic.CompareAndSwapInt64(&cl.inflight, current, current+1) {
			return true
		}
	}
}

func (cl *ConcurrencyLimiter) release() {
	atomic.AddInt64(&cl.inflight, -1)
}

// Middleware enforces the concurrency limit.
func (cl *ConcurrencyLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startWait := time.Now()

		if cl.tryAcquire() {
			c.Set("queue_wait_ms", float64(0))
			defer cl.release()
			c.Next()
			return
		}

		// No immediate slot. Optionally allow a small bounded wait queue.
		if cl.maxQueue == 0 {
			cl.rejectedTotal.Add(1)
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":  "overloaded",
				"active": atomic.LoadInt64(&cl.inflight),
				"limit":  cl.limit,
			})
			return
		}

		queuedNow := atomic.AddInt64(&cl.queued, 1)
		if queuedNow > cl.maxQueue {
			atomic.AddInt64(&cl.queued, -1)
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
		defer atomic.AddInt64(&cl.queued, -1)

		var timer *time.Timer
		if cl.queueWaitTimeout > 0 {
			timer = time.NewTimer(cl.queueWaitTimeout)
			defer timer.Stop()
		}

		for {
			if cl.tryAcquire() {
				c.Set("queue_wait_ms", float64(time.Since(startWait).Milliseconds()))
				defer cl.release()
				c.Next()
				return
			}

			select {
			case <-c.Request.Context().Done():
				cl.rejectedTotal.Add(1)
				c.AbortWithStatusJSON(http.StatusRequestTimeout, gin.H{"error": "cancelled"})
				return
			default:
			}

			if timer != nil {
				select {
				case <-timer.C:
					cl.rejectedTotal.Add(1)
					c.Header("Retry-After", "1")
					c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
						"error":     "overloaded",
						"reason":    "queue_wait_timeout",
						"waited_ms": time.Since(startWait).Milliseconds(),
					})
					return
				default:
				}
			}

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
