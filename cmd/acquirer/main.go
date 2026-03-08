package main

import (
	"log"
	"os"

	"iso-parser-service/internal/acquirer"
)

func main() {

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
