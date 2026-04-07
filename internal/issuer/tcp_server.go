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
	"sync"
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

	workerAcquireTimeoutSec := 2
	if v := os.Getenv("WORKER_ACQUIRE_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			workerAcquireTimeoutSec = n
		}
	}
	requestTimeoutSec := 3
	if v := os.Getenv("REQUEST_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			requestTimeoutSec = n
		}
	}
	workerAcquireTimeout := time.Duration(workerAcquireTimeoutSec) * time.Second
	requestTimeout := time.Duration(requestTimeoutSec) * time.Second

	log.Printf("issuer: listening on %s (workers: %d, acquire_timeout: %s, request_timeout: %s)",
		addr, workerPoolSize, workerAcquireTimeout, requestTimeout)

	// Shared worker pool across ALL connections (prevents goroutine explosion)
	workerPool := make(chan struct{}, workerPoolSize)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("issuer: accept error: %v", err)
			continue
		}
		go handleConn(conn, svc, workerPool, workerAcquireTimeout, requestTimeout)
	}
}

func handleConn(conn net.Conn, svc *Service, workerPool chan struct{}, workerAcquireTimeout, requestTimeout time.Duration) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()

	// Write mutex to prevent concurrent writes to the same TCP connection
	var writeMu sync.Mutex

	// Context for cancellation (prevents goroutine leaks)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		start := time.Now()

		tpduHeader, hexReq, err := readTPDUFrame(conn)
		if err != nil {
			if err != io.EOF {
				log.Printf("issuer: read error from %s: %v", remote, err)
			}
			return
		}

		// Acquire worker slot with timeout (bounded admission)
		workerWaitStart := time.Now()
		acquireTimer := time.NewTimer(workerAcquireTimeout)
		var acquired bool
		select {
		case workerPool <- struct{}{}:
			acquired = true
		case <-acquireTimer.C:
			log.Printf("issuer: worker pool saturated, dropping request from %s (waited %dms)",
				remote, time.Since(workerWaitStart).Milliseconds())
		case <-ctx.Done():
			acquireTimer.Stop()
			return
		}
		acquireTimer.Stop()
		if !acquired {
			continue
		}
		workerWaitMs := time.Since(workerWaitStart).Milliseconds()

		// Per-request context with deadline (prevents wasted work after timeout)
		reqCtx, reqCancel := context.WithTimeout(ctx, requestTimeout)

		// Process request in goroutine for concurrent handling
		go func(req string, tpdu []byte, startTime time.Time, wWaitMs int64, reqCtx context.Context, reqCancel context.CancelFunc) {
			defer reqCancel()
			defer func() { <-workerPool }() // Release worker slot

			// Check if context is cancelled
			select {
			case <-reqCtx.Done():
				return
			default:
			}

			hexResp, respMsg, err := svc.HandleHex(reqCtx, req)
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

			writeMu.Lock()
			writeStart := time.Now()
			err = writeTPDUFrame(conn, hexResp, tpdu)
			writeMs := time.Since(writeStart).Milliseconds()
			writeMu.Unlock()
			if err != nil {
				log.Printf("issuer: write error to %s: %v", remote, err)
				return
			}

			log.Printf("issuer: txn worker_wait=%dms handle=%dms write=%dms total=%dms",
				wWaitMs,
				time.Since(startTime).Milliseconds()-wWaitMs-writeMs,
				writeMs,
				time.Since(startTime).Milliseconds(),
			)
		}(hexReq, tpduHeader, start, workerWaitMs, reqCtx, reqCancel)
	}
}

// readTPDUFrame reads TPDU-wrapped frames from acquirer switch.
// Returns the 5-byte TPDU header (for echo-back) and the ISO hex string.
func readTPDUFrame(conn net.Conn) ([]byte, string, error) {
	// Read 2-byte length prefix
	lengthBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, lengthBytes); err != nil {
		return nil, "", err
	}

	length := binary.BigEndian.Uint16(lengthBytes)
	if length == 0 || length > 8192 {
		return nil, "", fmt.Errorf("invalid packet length: %d", length)
	}

	// Read packet data
	packet := make([]byte, length)
	if _, err := io.ReadFull(conn, packet); err != nil {
		return nil, "", err
	}

	// Strip TPDU (first 5 bytes)
	if len(packet) < 5 {
		return nil, "", fmt.Errorf("packet too short for TPDU")
	}

	tpduHeader := make([]byte, 5)
	copy(tpduHeader, packet[:5])
	isoData := packet[5:]
	return tpduHeader, hex.EncodeToString(isoData), nil
}

// writeTPDUFrame writes TPDU-wrapped response, echoing back the request's TPDU header
// so the acquirer can extract the correlation ID.
func writeTPDUFrame(conn net.Conn, hexResp string, tpduHeader []byte) error {
	// Convert hex string to bytes
	respBytes, err := hex.DecodeString(hexResp)
	if err != nil {
		return fmt.Errorf("decode response hex failed: %w", err)
	}

	// Echo back the request's TPDU header (carries correlation ID in bytes 1-4)
	totalLen := len(tpduHeader) + len(respBytes)

	// Create packet: [2-byte length] + [TPDU] + [ISO]
	packet := make([]byte, 2+totalLen)
	binary.BigEndian.PutUint16(packet[:2], uint16(totalLen))
	copy(packet[2:], tpduHeader)
	copy(packet[2+len(tpduHeader):], respBytes)

	// Send packet
	_, err = conn.Write(packet)
	return err
}
