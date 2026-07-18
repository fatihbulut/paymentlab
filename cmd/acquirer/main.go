package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"iso-parser-service/internal/acquirer"
	"iso-parser-service/internal/config"
	"iso-parser-service/internal/iso"
	"iso-parser-service/internal/otel"
	"iso-parser-service/internal/store"
	storepostgres "iso-parser-service/internal/store/postgres"
)

func main() {
	ctx := context.Background()

	// Initialize OpenTelemetry (metrics + traces)
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		shutdown, err := otel.InitOTel(ctx, "acquirer")
		if err != nil {
			log.Printf("warning: failed to init OpenTelemetry: %v", err)
		} else {
			defer shutdown(ctx)
			log.Println("OpenTelemetry initialized")
		}
	}

	// Load ISO spec from JSON (Single Source of Truth)
	if err := iso.InitSpec("web/spec.json"); err != nil {
		log.Fatalf("failed to load ISO spec: %v", err)
	}
	log.Println("ISO spec loaded from web/spec.json")

	cfg := config.FromEnv()
	var appStore store.Store

	issuerDBURL := os.Getenv("ISSUER_DATABASE_URL")

	if cfg.DatabaseURL != "" && issuerDBURL != "" {
		// Split-DB mode: acquirer DB for audit logs, issuer DB for card CRUD
		pgStore, err := storepostgres.NewAcquirerStore(ctx, cfg.DatabaseURL, issuerDBURL)
		if err != nil {
			log.Fatalf("failed to init acquirer store: %v", err)
		}
		defer pgStore.Close()
		appStore = pgStore

		if err := storepostgres.MigrateUp(ctx, pgStore.Pool(), "migrations/acquirer"); err != nil {
			log.Fatalf("failed to run acquirer migrations: %v", err)
		}
		log.Println("acquirer postgres migrations applied")
	} else if cfg.DatabaseURL != "" {
		// Single-DB fallback (local dev without split DBs)
		pgStore, err := storepostgres.New(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("failed to init postgres store: %v", err)
		}
		defer pgStore.Close()
		appStore = pgStore

		if err := storepostgres.MigrateUp(ctx, pgStore.Pool(), "migrations"); err != nil {
			log.Fatalf("failed to run migrations: %v", err)
		}
		log.Println("postgres migrations applied")
	} else {
		log.Fatal("acquirer: DATABASE_URL not set - database is required")
	}

	httpPort := os.Getenv("ACQUIRER_PORT")
	if httpPort == "" {
		httpPort = "8081"
	}

	issuerAddr := os.Getenv("ISSUER_ADDR")
	if issuerAddr == "" {
		issuerAddr = "localhost:5001"
	}

	// Create switch instance for TPDU-based communication
	switchInstance := acquirer.NewAcquirerSwitch(issuerAddr)

	// Start the switch
	if err := switchInstance.Start(ctx); err != nil {
		log.Fatalf("failed to start switch: %v", err)
	}
	defer switchInstance.Stop(ctx)

	server := acquirer.NewHTTPServer(switchInstance, appStore)

	addr := ":" + httpPort
	log.Printf("acquirer: HTTP listening on %s, issuer=%s (TPDU-enabled)", addr, issuerAddr)

	// Custom listener with larger backlog for high concurrency
	lc := net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		log.Fatalf("acquirer: failed to listen: %v", err)
	}

	// Custom HTTP server with proper timeouts
	requestTimeoutSec := 2
	if v := os.Getenv("ACQUIRER_REQUEST_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			requestTimeoutSec = n
		}
	}

	httpServer := &http.Server{
		Handler:           server.Router(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      time.Duration(requestTimeoutSec+1) * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	if err := httpServer.Serve(ln); err != nil {
		log.Fatalf("acquirer: failed to start HTTP server: %v", err)
	}
}
