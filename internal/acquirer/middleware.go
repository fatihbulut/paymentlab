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

	minLimit     int
	maxLimit     int
	maxQueue     int
	queueTimeout time.Duration

	// Vegas RTT controller
	mu         sync.Mutex
	rttEWMA    float64 // exponentially weighted moving average
	minRTT     float64 // baseline (no-congestion) RTT in ms
	probeCount int
	sampleSize int // adjust limit every N completions
}

// NewAdaptiveLimiter creates a Vegas-style adaptive limiter from environment variables.
func NewAdaptiveLimiter() *AdaptiveLimiter {
	minLimit := envInt("MIN_CONCURRENT", 5)
	maxLimit := envInt("MAX_CONCURRENT", 100)
	initial := envInt("INITIAL_CONCURRENT", 10)
	maxQueue := envInt("MAX_QUEUE", 200)
	queueTimeoutSec := envInt("QUEUE_TIMEOUT_SEC", 5)
	sampleSize := envInt("LIMITER_SAMPLE_SIZE", 20)

	al := &AdaptiveLimiter{
		notify:       make(chan struct{}, maxLimit),
		minLimit:     minLimit,
		maxLimit:     maxLimit,
		maxQueue:     maxQueue,
		queueTimeout: time.Duration(queueTimeoutSec) * time.Second,
		sampleSize:   sampleSize,
	}
	atomic.StoreInt64(&al.limit, int64(initial))

	log.Printf("acquirer: adaptive limiter min=%d initial=%d max=%d queue=%d sample=%d",
		minLimit, initial, maxLimit, maxQueue, sampleSize)

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

// RecordRTT feeds an observed round-trip time (ms) into the Vegas controller.
// Called by handleTransaction after each successful switch call.
func (al *AdaptiveLimiter) RecordRTT(rttMs float64) {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.rttEWMA == 0 {
		al.rttEWMA = rttMs
		al.minRTT = rttMs
	} else {
		al.rttEWMA = 0.2*rttMs + 0.8*al.rttEWMA
		if rttMs < al.minRTT {
			al.minRTT = rttMs
		}
	}

	al.probeCount++
	if al.probeCount < al.sampleSize {
		return
	}
	al.probeCount = 0
	al.adjustLimit()
}

// RecordTimeout signals extreme congestion — halve the limit immediately.
func (al *AdaptiveLimiter) RecordTimeout() {
	al.mu.Lock()
	defer al.mu.Unlock()

	current := atomic.LoadInt64(&al.limit)
	newLimit := int64(float64(current) * 0.5)
	if newLimit < int64(al.minLimit) {
		newLimit = int64(al.minLimit)
	}
	atomic.StoreInt64(&al.limit, newLimit)
	al.probeCount = 0

	log.Printf("acquirer: adaptive TIMEOUT limit=%d→%d", current, newLimit)
}

// adjustLimit implements the Vegas algorithm. Must be called under al.mu.
//
// Vegas estimates a "virtual queue" from RTT inflation:
//
//	expected_throughput = limit / minRTT
//	actual_throughput   = limit / rttEWMA
//	queue_estimate      = limit × (1 − minRTT/rttEWMA)
//
// If queue_estimate < alpha → room to grow → increase limit
// If queue_estimate > beta  → congested    → decrease limit
func (al *AdaptiveLimiter) adjustLimit() {
	if al.minRTT <= 0 || al.rttEWMA <= 0 {
		return
	}

	current := atomic.LoadInt64(&al.limit)
	queueEstimate := float64(current) * (1.0 - al.minRTT/al.rttEWMA)

	var newLimit int64
	const alpha = 3.0 // below alpha: room to grow
	const beta = 6.0  // above beta: congested

	if queueEstimate < alpha {
		newLimit = current + 1 // additive increase
	} else if queueEstimate > beta {
		newLimit = current - 1 // additive decrease
	} else {
		newLimit = current // stable zone — hold
	}

	if newLimit < int64(al.minLimit) {
		newLimit = int64(al.minLimit)
	}
	if newLimit > int64(al.maxLimit) {
		newLimit = int64(al.maxLimit)
	}
	atomic.StoreInt64(&al.limit, newLimit)

	log.Printf("acquirer: adaptive limit=%d q_est=%.1f rtt=%.0fms min_rtt=%.0fms inflight=%d",
		newLimit, queueEstimate, al.rttEWMA, al.minRTT, atomic.LoadInt64(&al.inflight))
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
