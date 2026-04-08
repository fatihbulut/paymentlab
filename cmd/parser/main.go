package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

type ParseRequest struct {
	RawHex string `json:"raw_hex" binding:"required"`
}

func main() {
	cfg := loadConfig()

	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	metrics := NewMetrics()
	limiter := NewInFlightLimiter(cfg.InFlightLimit)
	wp := NewWorkerPool(cfg.QueueSize, cfg.WorkerCount, metrics)

	router := gin.New()
	router.Use(gin.Recovery())
	if cfg.LogRequests {
		router.Use(gin.Logger())
	}
	router.Use(maxBodyBytesMiddleware(cfg.MaxBodyBytes))
	router.Use(requestTimeoutMiddleware(cfg.RequestTimeout))
	router.Use(inFlightMiddleware(limiter, metrics))

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/metrics", gin.WrapF(metrics.ServePrometheus))

	router.POST("/v1/parse", func(c *gin.Context) {
		var req ParseRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("[HATA] JSON Bind Hatası: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz JSON veya eksik raw_hex"})
			return
		}

		if cfg.LogRequests {
			log.Printf("Gelen ISO Mesajı: %s", req.RawHex)
		}

		replyCh := make(chan jobResult, 1)
		if ok := wp.TrySubmit(job{kind: jobParse, ctx: c.Request.Context(), rawHex: req.RawHex, replyCh: replyCh}); !ok {
			metrics.IncRejectedQueue()
			c.Header("Retry-After", "1")
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "overloaded"})
			return
		}

		select {
		case res := <-replyCh:
			if res.err != nil {
				log.Printf("[HATA] parse failed: %v", res.err)
			}
			c.JSON(res.status, res.body)
		case <-c.Request.Context().Done():
			metrics.IncTimeout()
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "timeout"})
		}
	})

	router.POST("/v1/pack", func(c *gin.Context) {
		var incomingData ISOMessage

		if err := c.ShouldBindJSON(&incomingData); err != nil {
			log.Printf("[HATA] JSON Bind Hatası: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz veri formatı"})
			return
		}

		replyCh := make(chan jobResult, 1)
		if ok := wp.TrySubmit(job{kind: jobPack, ctx: c.Request.Context(), msg: incomingData, replyCh: replyCh}); !ok {
			metrics.IncRejectedQueue()
			c.Header("Retry-After", "1")
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "overloaded"})
			return
		}

		select {
		case res := <-replyCh:
			if res.err != nil {
				log.Printf("[HATA] pack failed: %v", res.err)
			}
			c.JSON(res.status, res.body)
		case <-c.Request.Context().Done():
			metrics.IncTimeout()
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "timeout"})
		}
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		BaseContext: func(net.Listener) context.Context {
			return context.Background()
		},
	}

	fmt.Printf(
		"iso-parser %s portunda hazır. request_timeout=%s inflight_limit=%d queue_size=%d workers=%d\n",
		cfg.Port,
		cfg.RequestTimeout,
		cfg.InFlightLimit,
		cfg.QueueSize,
		cfg.WorkerCount,
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		log.Printf("shutdown signal: %v", sig)
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)

}
