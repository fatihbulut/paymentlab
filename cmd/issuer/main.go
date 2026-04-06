package main

import (
	"context"
	"log"
	"os"

	"iso-parser-service/internal/config"
	"iso-parser-service/internal/iso"
	"iso-parser-service/internal/issuer"
	"iso-parser-service/internal/otel"
	"iso-parser-service/internal/store"
	storepostgres "iso-parser-service/internal/store/postgres"
)

func main() {
	ctx := context.Background()

	// Initialize OpenTelemetry (metrics + traces)
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		shutdown, err := otel.InitOTel(ctx, "issuer")
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

	if cfg.DatabaseURL != "" {
		pgStore, err := storepostgres.New(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("failed to init postgres store: %v", err)
		}
		defer pgStore.Close()
		appStore = pgStore

		if err := storepostgres.MigrateUp(ctx, pgStore.Pool(), "migrations/issuer"); err != nil {
			log.Fatalf("failed to run issuer migrations: %v", err)
		}
		log.Println("issuer postgres migrations applied")
	}

	listenAddr := os.Getenv("ISSUER_LISTEN")
	if listenAddr == "" {
		listenAddr = "0.0.0.0:5001"
	}
	svc := issuer.NewService(appStore)
	log.Printf("issuer: TCP listening on %s", listenAddr)
	if err := issuer.ServeTCP(listenAddr, svc); err != nil {
		log.Fatalf("issuer: listen error on %s: %v", listenAddr, err)
	}
}
