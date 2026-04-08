package main

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port string

	// Latency budget: request processing should finish within this window.
	RequestTimeout time.Duration

	// Backpressure limits.
	InFlightLimit int
	QueueSize     int
	WorkerCount   int
	MaxBodyBytes  int64
	LogRequests   bool

	// Server-level timeouts.
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

func loadConfig() Config {
	cfg := Config{
		Port:           envString("PORT", "8080"),
		RequestTimeout: envDurationMS("REQUEST_TIMEOUT_MS", 1800) * time.Millisecond,

		InFlightLimit: envInt("INFLIGHT_LIMIT", 200),
		QueueSize:     envInt("QUEUE_SIZE", 500),
		WorkerCount:   envInt("WORKER_COUNT", 100),
		MaxBodyBytes:  int64(envInt("MAX_BODY_BYTES", 1<<20)), // 1MiB default
		LogRequests:   envBool("LOG_REQUESTS", false),

		ReadHeaderTimeout: envDurationMS("READ_HEADER_TIMEOUT_MS", 5000) * time.Millisecond,
		ReadTimeout:       envDurationMS("READ_TIMEOUT_MS", 10000) * time.Millisecond,
		WriteTimeout:      envDurationMS("WRITE_TIMEOUT_MS", 2500) * time.Millisecond,
		IdleTimeout:       envDurationMS("IDLE_TIMEOUT_MS", 60000) * time.Millisecond,
	}

	if cfg.InFlightLimit < 1 {
		cfg.InFlightLimit = 1
	}
	if cfg.QueueSize < 1 {
		cfg.QueueSize = 1
	}
	if cfg.WorkerCount < 1 {
		cfg.WorkerCount = 1
	}
	if cfg.MaxBodyBytes < 1024 {
		cfg.MaxBodyBytes = 1024
	}
	if cfg.RequestTimeout < 100*time.Millisecond {
		cfg.RequestTimeout = 100 * time.Millisecond
	}

	return cfg
}

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envDurationMS(key string, defMS int) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return time.Duration(defMS)
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return time.Duration(defMS)
	}
	return time.Duration(n)
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "TRUE", "True", "yes", "YES", "y", "Y", "on", "ON":
		return true
	case "0", "false", "FALSE", "False", "no", "NO", "n", "N", "off", "OFF":
		return false
	default:
		return def
	}
}
