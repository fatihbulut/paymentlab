package issuer

import (
	"context"
	"io"
	"log"
	"net"

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

	tracer := otel.Tracer("issuer")

	for {
		ctx, span := tracer.Start(context.Background(), "issuer.handle_message")
		span.SetAttributes(
			attribute.String("remote.addr", remote),
			attribute.String("protocol", "tcp"),
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
			)
			if respMsg.STAN != "" {
				span.SetAttributes(attribute.String("iso.stan", respMsg.STAN))
			}
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
