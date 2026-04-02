package acquirer

import (
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// ConcurrencyLimiter provides two-stage admission control:
// 1. Processing slots — max concurrent requests being actively handled
// 2. Queue slots — max requests waiting for a processing slot
// Requests that exceed both limits get immediate 503.
type ConcurrencyLimiter struct {
	processing   chan struct{}  // bounded processing slots
	queueTimeout time.Duration // max time to wait in queue

	// Metrics (atomic for lock-free reads)
	activeCount int64
	queuedCount int64
	maxConc     int
	maxQueue    int
}

// NewConcurrencyLimiter creates a limiter from environment variables.
// Defaults are tuned for local dev (high capacity).
func NewConcurrencyLimiter() *ConcurrencyLimiter {
	maxConcurrent := envInt("MAX_CONCURRENT", 500)
	maxQueue := envInt("MAX_QUEUE", 100000)
	queueTimeoutSec := envInt("QUEUE_TIMEOUT_SEC", 60)

	return &ConcurrencyLimiter{
		processing:   make(chan struct{}, maxConcurrent),
		queueTimeout: time.Duration(queueTimeoutSec) * time.Second,
		maxConc:      maxConcurrent,
		maxQueue:     maxQueue,
	}
}

// Middleware returns a Gin middleware that enforces concurrency limits.
func (cl *ConcurrencyLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Fast path: try to acquire a processing slot without blocking
		select {
		case cl.processing <- struct{}{}:
			atomic.AddInt64(&cl.activeCount, 1)
			defer func() {
				<-cl.processing
				atomic.AddInt64(&cl.activeCount, -1)
			}()
			c.Next()
			return
		default:
			// All processing slots busy — enter queue
		}

		// Check queue capacity (atomic increment, rollback if over limit)
		queued := atomic.AddInt64(&cl.queuedCount, 1)
		if queued > int64(cl.maxQueue) {
			atomic.AddInt64(&cl.queuedCount, -1)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error":       "server busy — queue full",
				"retry_after": 1,
			})
			return
		}

		// Wait for a processing slot with timeout
		timer := time.NewTimer(cl.queueTimeout)
		defer timer.Stop()

		select {
		case cl.processing <- struct{}{}:
			atomic.AddInt64(&cl.queuedCount, -1)
			atomic.AddInt64(&cl.activeCount, 1)
			defer func() {
				<-cl.processing
				atomic.AddInt64(&cl.activeCount, -1)
			}()
			c.Next()

		case <-timer.C:
			atomic.AddInt64(&cl.queuedCount, -1)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error":       "server busy — queue timeout",
				"retry_after": 5,
			})

		case <-c.Request.Context().Done():
			atomic.AddInt64(&cl.queuedCount, -1)
			c.Abort()
		}
	}
}

// Active returns the number of requests currently being processed.
func (cl *ConcurrencyLimiter) Active() int64 {
	return atomic.LoadInt64(&cl.activeCount)
}

// Queued returns the number of requests waiting in queue.
func (cl *ConcurrencyLimiter) Queued() int64 {
	return atomic.LoadInt64(&cl.queuedCount)
}

// MaxConcurrent returns the configured max concurrent processing slots.
func (cl *ConcurrencyLimiter) MaxConcurrent() int {
	return cl.maxConc
}

// MaxQueue returns the configured max queue size.
func (cl *ConcurrencyLimiter) MaxQueue() int {
	return cl.maxQueue
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
