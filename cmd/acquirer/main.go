package main

import (
	"context"
	"log"
	"os"

	"iso-parser-service/internal/acquirer"
	"iso-parser-service/internal/iso"
	"iso-parser-service/internal/otel"
)

func main() {
	// Load ISO spec from JSON (Single Source of Truth)
	if err := iso.InitSpec("web/spec.json"); err != nil {
		log.Fatalf("failed to load ISO spec: %v", err)
	}
	log.Println("ISO spec loaded from web/spec.json")

	ctx := context.Background()

	shutdown, err := otel.InitOTel(ctx, "acquirer")
	if err != nil {
		log.Fatalf("failed to initialize OpenTelemetry: %v", err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Printf("failed to shutdown OpenTelemetry: %v", err)
		}
	}()

	httpPort := os.Getenv("ACQUIRER_PORT")
	if httpPort == "" {
		httpPort = "8081"
	}

	issuerAddr := os.Getenv("ISSUER_ADDR")
	if issuerAddr == "" {
		issuerAddr = "localhost:5001"
	}

	client := acquirer.NewTCPIssuerClient(issuerAddr)
	server := acquirer.NewHTTPServer(client)

	addr := ":" + httpPort
	log.Printf("acquirer: HTTP dinliyor %s, issuer=%s", addr, issuerAddr)
	if err := server.Router().Run(addr); err != nil {
		log.Fatalf("acquirer: HTTP başlatma hatası: %v", err)
	}
}
