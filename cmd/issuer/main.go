package main

import (
	"context"
	"log"
	"os"

	"iso-parser-service/internal/iso"
	"iso-parser-service/internal/issuer"
	"iso-parser-service/internal/otel"
)

func main() {
	// Load ISO spec from JSON (Single Source of Truth)
	if err := iso.InitSpec("web/spec.json"); err != nil {
		log.Fatalf("failed to load ISO spec: %v", err)
	}
	log.Println("ISO spec loaded from web/spec.json")

	ctx := context.Background()

	shutdown, err := otel.InitOTel(ctx, "issuer")
	if err != nil {
		log.Fatalf("failed to initialize OpenTelemetry: %v", err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Printf("failed to shutdown OpenTelemetry: %v", err)
		}
	}()

	listenAddr := os.Getenv("ISSUER_ADDR")
	if listenAddr == "" {
		listenAddr = "localhost:5001"
	}
	svc := issuer.NewService()
	log.Printf("issuer: TCP dinliyor %s", listenAddr)
	if err := issuer.ServeTCP(listenAddr, svc); err != nil {
		log.Fatalf("issuer: listen error on %s: %v", listenAddr, err)
	}
}
