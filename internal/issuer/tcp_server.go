package issuer

import (
	"io"
	"log"
	"net"

	"iso-parser-service/internal/transport"
)

// ServeTCP starts a TCP server on the given address and uses the provided
// Service to handle ISO8583 hex requests.
func ServeTCP(addr string, svc *Service) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	log.Printf("issuer: listening on %s", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("issuer: accept error: %v", err)
			continue
		}
		go handleConn(conn, svc)
	}
}

func handleConn(conn net.Conn, svc *Service) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()
	log.Printf("issuer: accepted connection from %s", remote)

	for {
		hexReq, err := transport.ReadFrame(conn)
		if err != nil {
			if err == io.EOF {
				log.Printf("issuer: connection from %s closed", remote)
			} else {
				log.Printf("issuer: read error from %s: %v", remote, err)
			}
			return
		}

		log.Printf("issuer: received hex request from %s: %s", remote, hexReq)

		hexResp, _, err := svc.HandleHex(hexReq)
		if err != nil {
			log.Printf("issuer: handle error for %s: %v", remote, err)
			return
		}

		log.Printf("issuer: sending hex response to %s: %s", remote, hexResp)

		if err := transport.WriteFrame(conn, hexResp); err != nil {
			log.Printf("issuer: write error to %s: %v", remote, err)
			return
		}
	}
}
