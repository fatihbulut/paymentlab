package acquirer

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

// ConcurrencyLimiter enforces a static in-flight request limit.
// No queue, no adaptive logic — requests beyond the limit are rejected with 429.
//
// Limit is calculated via Little's Law from observed system metrics:
//
//	L = λ × W
//	where λ = throughput (req/s), W = latency (s)
//
// Example: 700 req/s @ 10ms → L = 7, use 10 for safety margin.
type ConcurrencyLimiter struct {
	inflight int64
	limit    int64
}

// NewConcurrencyLimiter creates a static limiter from CONCURRENT_LIMIT env var.
// Default: 10 (based on observed 700 req/s @ 10ms RTT).
func NewConcurrencyLimiter() *ConcurrencyLimiter {
	limit := envInt("CONCURRENT_LIMIT", 10)
	log.Printf("acquirer: concurrency limiter limit=%d (static)", limit)
	return &ConcurrencyLimiter{limit: int64(limit)}
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
		if !cl.tryAcquire() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":  "capacity limit reached",
				"active": atomic.LoadInt64(&cl.inflight),
				"limit":  cl.limit,
			})
			return
		}
		defer cl.release()
		c.Next()
	}
}

func (cl *ConcurrencyLimiter) Active() int64      { return atomic.LoadInt64(&cl.inflight) }
func (cl *ConcurrencyLimiter) Queued() int64      { return 0 }
func (cl *ConcurrencyLimiter) Limit() int64       { return cl.limit }
func (cl *ConcurrencyLimiter) MaxConcurrent() int { return int(cl.limit) }
func (cl *ConcurrencyLimiter) MaxQueue() int      { return 0 }

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
