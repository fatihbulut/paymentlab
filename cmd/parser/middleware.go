package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type InFlightLimiter struct {
	tokens chan struct{}
}

func NewInFlightLimiter(limit int) *InFlightLimiter {
	return &InFlightLimiter{tokens: make(chan struct{}, limit)}
}

func (l *InFlightLimiter) TryAcquire() bool {
	select {
	case l.tokens <- struct{}{}:
		return true
	default:
		return false
	}
}

func (l *InFlightLimiter) Release() {
	select {
	case <-l.tokens:
	default:
		// should never happen
	}
}

func maxBodyBytesMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes > 0 && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

func requestTimeoutMiddleware(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d <= 0 {
			c.Next()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

func inFlightMiddleware(limiter *InFlightLimiter, metrics *Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter != nil && !limiter.TryAcquire() {
			if metrics != nil {
				metrics.IncRejectedInFlight()
			}
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "overloaded",
			})
			return
		}
		if metrics != nil {
			metrics.IncInFlight()
		}
		start := time.Now()

		defer func() {
			if limiter != nil {
				limiter.Release()
			}
			if metrics != nil {
				metrics.DecInFlight()
				route := c.FullPath()
				metrics.Observe(route, c.Writer.Status(), time.Since(start))
			}
		}()

		c.Next()
	}
}
