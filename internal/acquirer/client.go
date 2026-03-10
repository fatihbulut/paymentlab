package acquirer

import (
	"context"
	"fmt"
	"net"
	"time"

	"iso-parser-service/internal/transport"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type IssuerClient interface {
	SendAndReceive(ctx context.Context, hexReq string) (string, error)
	Close() error
}

type TCPIssuerClient struct {
	addr     string
	pool     chan *net.Conn
	maxConns int
	timeout  time.Duration
}

func NewTCPIssuerClient(addr string) *TCPIssuerClient {
	maxConns := 10
	pool := make(chan *net.Conn, maxConns)

	client := &TCPIssuerClient{
		addr:     addr,
		pool:     pool,
		maxConns: maxConns,
		timeout:  10 * time.Second,
	}

	return client
}

func (c *TCPIssuerClient) getConnection() (*net.Conn, error) {
	select {
	case conn := <-c.pool:
		if conn != nil && c.isHealthy(conn) {
			return conn, nil
		}
		if conn != nil {
			(*conn).Close()
		}
	default:
	}

	conn, err := net.DialTimeout("tcp", c.addr, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("acquirer: dial issuer at %s: %w", c.addr, err)
	}

	return &conn, nil
}

func (c *TCPIssuerClient) isHealthy(conn *net.Conn) bool {
	if conn == nil {
		return false
	}
	(*conn).SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	one := make([]byte, 1)
	_, err := (*conn).Read(one)
	(*conn).SetReadDeadline(time.Time{})

	if err == nil {
		return false
	}

	netErr, ok := err.(net.Error)
	return ok && netErr.Timeout()
}

func (c *TCPIssuerClient) releaseConnection(conn *net.Conn, hasError bool) {
	if conn == nil {
		return
	}

	if hasError {
		(*conn).Close()
		return
	}

	select {
	case c.pool <- conn:
	default:
		(*conn).Close()
	}
}

func (c *TCPIssuerClient) SendAndReceive(ctx context.Context, hexReq string) (string, error) {
	tracer := otel.Tracer("acquirer")
	ctx, span := tracer.Start(ctx, "tcp.send_to_issuer")
	defer span.End()

	span.SetAttributes(
		attribute.String("issuer.addr", c.addr),
		attribute.Int("request.length", len(hexReq)),
	)

	connPtr, err := c.getConnection()
	if err != nil {
		hasError := true
		span.RecordError(err)
		span.SetStatus(codes.Error, "connection failed")
		c.releaseConnection(connPtr, hasError)
		return "", err
	}

	conn := *connPtr
	hasError := false
	defer func() {
		c.releaseConnection(connPtr, hasError)
	}()

	if err := conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		hasError = true
		span.RecordError(err)
		span.SetStatus(codes.Error, "set deadline failed")
		return "", fmt.Errorf("acquirer: set deadline: %w", err)
	}

	if err := transport.WriteFrame(conn, hexReq); err != nil {
		hasError = true
		span.RecordError(err)
		span.SetStatus(codes.Error, "write frame failed")
		return "", fmt.Errorf("acquirer: write frame: %w", err)
	}

	hexResp, err := transport.ReadFrame(conn)
	if err != nil {
		hasError = true
		span.RecordError(err)
		span.SetStatus(codes.Error, "read frame failed")
		return "", fmt.Errorf("acquirer: read frame: %w", err)
	}

	span.SetAttributes(attribute.Int("response.length", len(hexResp)))
	span.SetStatus(codes.Ok, "success")

	return hexResp, nil
}

func (c *TCPIssuerClient) Close() error {
	close(c.pool)
	for conn := range c.pool {
		if conn != nil {
			(*conn).Close()
		}
	}
	return nil
}
