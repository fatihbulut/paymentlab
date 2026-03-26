package main

import (
	"context"
	"log"
	"os"

	"iso-parser-service/internal/acquirer"
	"iso-parser-service/internal/iso"
)

func main() {
	// Load ISO spec from JSON (Single Source of Truth)
	if err := iso.InitSpec("web/spec.json"); err != nil {
		log.Fatalf("failed to load ISO spec: %v", err)
	}
	log.Println("ISO spec loaded from web/spec.json")

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
	ctx := context.Background()
	if err := switchInstance.Start(ctx); err != nil {
		log.Fatalf("failed to start switch: %v", err)
	}
	defer switchInstance.Stop(ctx)

	// Create legacy client (for compatibility)
	client := acquirer.NewTCPIssuerClient(issuerAddr)

	// Create HTTP server with both client and switch
	server := acquirer.NewHTTPServer(client, switchInstance)

	addr := ":" + httpPort
	log.Printf("acquirer: HTTP listening on %s, issuer=%s (TPDU-enabled)", addr, issuerAddr)
	if err := server.Router().Run(addr); err != nil {
		log.Fatalf("acquirer: failed to start HTTP server: %v", err)
	}
}
