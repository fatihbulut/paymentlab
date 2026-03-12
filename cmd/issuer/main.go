package main

import (
	"context"
	"log"
	"os"

	"iso-parser-service/internal/issuer"
	"iso-parser-service/internal/otel"
)

func main() {
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

	addr := os.Getenv("ISSUER_ADDR")
	if addr == "" {
		addr = "localhost:5001"
	}
	svc := issuer.NewService()
	if err := issuer.ServeTCP(addr, svc); err != nil {
		log.Fatalf("issuer: listen error on %s: %v", addr, err)
	}
}
