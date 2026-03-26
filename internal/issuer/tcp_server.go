package issuer

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"runtime"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ServeTCP starts a TCP server on the given address and uses the provided
// Service to handle ISO8583 hex requests with worker pool for concurrent processing.
func ServeTCP(addr string, svc *Service) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	log.Printf("issuer: listening on %s (worker pool: 50)", addr)

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

	// Worker pool for concurrent request processing (50 workers)
	workerPool := make(chan struct{}, 50)

	// Context for cancellation (prevents goroutine leaks)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tracer := otel.Tracer("issuer-tracer")

	for {
		start := time.Now()

		// Start new span for each message (context propagation not available over raw TCP)
		_, span := tracer.Start(ctx, "issuer.handle_message")
		span.SetAttributes(
			attribute.String("remote.addr", remote),
			attribute.String("protocol", "tcp"),
			attribute.String("service", "issuer"),
		)

		hexReq, err := readTPDUFrame(conn)
		if err != nil {
			if err != io.EOF {
				log.Printf("issuer: read error from %s: %v", remote, err)
				span.RecordError(err)
				span.SetStatus(codes.Error, "read frame error")
			}
			span.End()
			return
		}

		span.SetAttributes(attribute.Int("request.length", len(hexReq)))

		// Acquire worker slot (blocks if pool is full)
		workerPool <- struct{}{}

		// Process request in goroutine for concurrent handling
		go func(req string, s trace.Span, startTime time.Time, reqCtx context.Context) {
			defer func() { <-workerPool }() // Release worker slot
			defer s.End()

			// Check if context is cancelled
			select {
			case <-reqCtx.Done():
				return
			default:
			}

			hexResp, respMsg, err := svc.HandleHex(req)
			if err != nil {
				log.Printf("issuer: handle error for %s: %v", remote, err)
				s.RecordError(err)
				s.SetStatus(codes.Error, "handle message error")
				return
			}

			if respMsg != nil {
				s.SetAttributes(
					attribute.String("iso.mti", respMsg.MTI),
					attribute.String("iso.response_code", respMsg.RespCode),
					attribute.String("transaction.status", "processed"),
				)
				if respMsg.STAN != "" {
					s.SetAttributes(attribute.String("iso.stan", respMsg.STAN))
				}

				// Record metrics
				duration := time.Since(startTime)
				s.SetAttributes(
					attribute.Float64("transaction.duration_ms", duration.Seconds()*1000),
				)

				// Record critical health metrics
				recordIssuerHealthMetrics()
			}

			s.SetAttributes(attribute.Int("response.length", len(hexResp)))

			// Check context before write
			select {
			case <-reqCtx.Done():
				return
			default:
			}

			if err := writeTPDUFrame(conn, hexResp); err != nil {
				log.Printf("issuer: write error to %s: %v", remote, err)
				s.RecordError(err)
				s.SetStatus(codes.Error, "write frame error")
				return
			}

			s.SetStatus(codes.Ok, "success")
		}(hexReq, span, start, ctx)
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

// recordIssuerHealthMetrics records critical health metrics for monitoring
func recordIssuerHealthMetrics() {
	goroutines := runtime.NumGoroutine()
	rss := estimateIssuerRSSMemory()
	fmt.Printf("HEALTH_METRIC: service=issuer goroutines=%d rss_bytes=%d\n", goroutines, rss)
}

// estimateIssuerRSSMemory provides a simple RSS memory estimation
func estimateIssuerRSSMemory() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Sys) - int64(m.HeapSys)
}
