package main

import (
	"context"
	"log"
	"os"

	"iso-parser-service/internal/acquirer"
	"iso-parser-service/internal/config"
	"iso-parser-service/internal/iso"
	"iso-parser-service/internal/store"
	storepostgres "iso-parser-service/internal/store/postgres"
)

func main() {
	// Load ISO spec from JSON (Single Source of Truth)
	if err := iso.InitSpec("/home/ubuntu/issuer-service/web/spec.json"); err != nil {
		log.Fatalf("failed to load ISO spec: %v", err)
	}
	log.Println("ISO spec loaded from web/spec.json")

	cfg := config.FromEnv()
	ctx := context.Background()
	var appStore store.Store

	if cfg.DatabaseURL != "" {
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
	if err := server.Router().Run(addr); err != nil {
		log.Fatalf("acquirer: failed to start HTTP server: %v", err)
	}
}
