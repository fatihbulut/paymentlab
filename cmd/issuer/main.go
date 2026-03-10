package main

import (
	"context"
	"log"
	"os"

	"iso-parser-service/internal/issuer"
	"iso-parser-service/internal/otel"
)

func main() {
	tp, err := otel.InitTracer()
	if err != nil {
		log.Fatal(err)
	}
	defer tp.Shutdown(context.Background())

	addr := os.Getenv("ISSUER_ADDR")
	if addr == "" {
		addr = "localhost:5001"
	}
	svc := issuer.NewService()
	if err := issuer.ServeTCP(addr, svc); err != nil {
		log.Fatalf("issuer: listen error on %s: %v", addr, err)
	}
}
