package issuer

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"runtime"
	"time"

	"iso-parser-service/internal/transport"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

	tracer := otel.Tracer("issuer-tracer")

	for {
		start := time.Now()

		// TCP üzerinden context propagation yapılamaz, bu yüzden yeni bir span başlatıyoruz
		// ancak bu span acquirer'dan gelen trace'in child'ı olacak şekilde tasarlanabilir
		ctx, span := tracer.Start(context.Background(), "issuer.handle_message")
		span.SetAttributes(
			attribute.String("remote.addr", remote),
			attribute.String("protocol", "tcp"),
			attribute.String("service", "issuer"),
		)

		hexReq, err := transport.ReadFrame(conn)
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

		hexResp, respMsg, err := svc.HandleHex(hexReq)
		if err != nil {
			log.Printf("issuer: handle error for %s: %v", remote, err)
			span.RecordError(err)
			span.SetStatus(codes.Error, "handle message error")
			span.End()
			return
		}

		if respMsg != nil {
			span.SetAttributes(
				attribute.String("iso.mti", respMsg.MTI),
				attribute.String("iso.response_code", respMsg.RespCode),
				attribute.String("transaction.status", "processed"),
			)
			if respMsg.STAN != "" {
				span.SetAttributes(attribute.String("iso.stan", respMsg.STAN))
			}

			// Record metrics
			duration := time.Since(start)
			span.SetAttributes(
				attribute.Float64("transaction.duration_ms", duration.Seconds()*1000),
			)

			// Sadece kritik sağlık metrikleri
			recordIssuerHealthMetrics()
		}

		span.SetAttributes(attribute.Int("response.length", len(hexResp)))

		if err := transport.WriteFrame(conn, hexResp); err != nil {
			log.Printf("issuer: write error to %s: %v", remote, err)
			span.RecordError(err)
			span.SetStatus(codes.Error, "write frame error")
			span.End()
			return
		}

		span.SetStatus(codes.Ok, "success")
		span.End()
		ctx.Done()
	}
}

// recordIssuerHealthMetrics records only critical health metrics for issuer
func recordIssuerHealthMetrics() {
	// Aktif İş Parçacığı (Goroutine) Sayısı
	var goroutines = runtime.NumGoroutine()

	// Bellek Kullanımı (RSS) - Linux/Unix sistemleri için
	var rss int64
	// Basit RSS ölçümü - production'da daha gelişmiş yöntem kullanılabilir
	rss = estimateIssuerRSSMemory()

	fmt.Printf("HEALTH_METRIC: service=issuer goroutines=%d rss_bytes=%d\n", goroutines, rss)
}

// estimateIssuerRSSMemory basit RSS tahmini yapar
func estimateIssuerRSSMemory() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// Sys değerinden heap tahmini çıkararak basit RSS tahmini
	return int64(m.Sys) - int64(m.HeapSys)
}
