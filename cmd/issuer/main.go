package main

import (
	"log"
	"os"

	"iso-parser-service/internal/issuer"
)

func main() {
	addr := os.Getenv("ISSUER_ADDR")
	if addr == "" {
		addr = "localhost:5001"
	}
	//trigger 2
	svc := issuer.NewService()
	if err := issuer.ServeTCP(addr, svc); err != nil {
		log.Fatalf("issuer: listen error on %s: %v", addr, err)
	}
}
