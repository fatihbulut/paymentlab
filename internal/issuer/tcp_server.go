package issuer

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"iso-parser-service/internal/otel"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

// ServeTCP starts a TCP server on the given address and uses the provided
// Service to handle ISO8583 hex requests with worker pool for concurrent processing.
func ServeTCP(addr string, svc *Service) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	// Get worker pool size from environment (default: 1000)
	workerPoolSize := 1000
	if poolEnv := os.Getenv("ISSUER_WORKER_POOL"); poolEnv != "" {
		if size, err := strconv.Atoi(poolEnv); err == nil && size > 0 {
			workerPoolSize = size
		}
	}

	log.Printf("issuer: listening on %s (worker pool: %d)", addr, workerPoolSize)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("issuer: accept error: %v", err)
			continue
		}
		go handleConn(conn, svc, workerPoolSize)
	}
}

func handleConn(conn net.Conn, svc *Service, workerPoolSize int) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()

	// Worker pool for concurrent request processing
	workerPool := make(chan struct{}, workerPoolSize)

	// Context for cancellation (prevents goroutine leaks)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		start := time.Now()

		hexReq, err := readTPDUFrame(conn)
		if err != nil {
			if err != io.EOF {
				log.Printf("issuer: read error from %s: %v", remote, err)
			}
			return
		}

		// Acquire worker slot (blocks if pool is full)
		workerPool <- struct{}{}

		// Process request in goroutine for concurrent handling
		go func(req string, startTime time.Time, reqCtx context.Context) {
			defer func() { <-workerPool }() // Release worker slot

			// Check if context is cancelled
			select {
			case <-reqCtx.Done():
				return
			default:
			}

			hexResp, respMsg, err := svc.HandleHex(req)
			if err != nil {
				log.Printf("issuer: handle error for %s: %v", remote, err)
				return
			}

			// Record metrics for monitoring
			if respMsg != nil {
				duration := time.Since(startTime)
				otel.RecordTransactionWithService(reqCtx, "issuer", respMsg.MTI, respMsg.RespCode, duration)
			}

			// Check context before write
			select {
			case <-reqCtx.Done():
				return
			default:
			}

			if err := writeTPDUFrame(conn, hexResp); err != nil {
				log.Printf("issuer: write error to %s: %v", remote, err)
				return
			}
		}(hexReq, start, ctx)
	}
}

// readTPDUFrame reads TPDU-wrapped frames from acquirer switch
func readTPDUFrame(conn net.Conn) (string, error) {
	// Read 2-byte length prefix
	lengthBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, lengthBytes); err != nil {
		return "", err
	}

	length := binary.BigEndian.Uint16(lengthBytes)
	if length == 0 || length > 8192 {
		return "", fmt.Errorf("invalid packet length: %d", length)
	}

	// Read packet data
	packet := make([]byte, length)
	if _, err := io.ReadFull(conn, packet); err != nil {
		return "", err
	}

	// Strip TPDU (first 5 bytes)
	if len(packet) < 5 {
		return "", fmt.Errorf("packet too short for TPDU")
	}

	isoData := packet[5:]
	return fmt.Sprintf("%x", isoData), nil
}

// writeTPDUFrame writes TPDU-wrapped response
func writeTPDUFrame(conn net.Conn, hexResp string) error {
	// Convert hex string to bytes
	respBytes, err := hex.DecodeString(hexResp)
	if err != nil {
		return fmt.Errorf("decode response hex failed: %w", err)
	}

	// Create TPDU wrapper: [2-byte Length] + [5-byte TPDU] + Response
	tpdu := []byte{0x60, 0x00, 0x01, 0x00, 0x00}
	totalLen := len(tpdu) + len(respBytes)

	// Create packet
	packet := make([]byte, 2+totalLen)
	binary.BigEndian.PutUint16(packet[:2], uint16(totalLen))
	copy(packet[2:], tpdu)
	copy(packet[2+len(tpdu):], respBytes)

	// Send packet
	_, err = conn.Write(packet)
	return err
}
