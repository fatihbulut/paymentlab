package acquirer

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"iso-parser-service/internal/transport"
)

type IssuerClient interface {
	SendAndReceive(hexReq string) (string, error)
	Close() error
}

type TCPIssuerClient struct {
	addr     string
	pool     chan *net.Conn
	mu       sync.Mutex
	maxConns int
	timeout  time.Duration
}

func NewTCPIssuerClient(addr string) *TCPIssuerClient {
	maxConns := 5
	pool := make(chan *net.Conn, maxConns)
	
	client := &TCPIssuerClient{
		addr:     addr,
		pool:     pool,
		maxConns: maxConns,
		timeout:  30 * time.Second,
	}
	
	return client
}

func (c *TCPIssuerClient) getConnection() (*net.Conn, error) {
	select {
	case conn := <-c.pool:
		if conn != nil {
			return conn, nil
		}
	default:
	}
	
	conn, err := net.DialTimeout("tcp", c.addr, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("acquirer: dial issuer at %s: %w", c.addr, err)
	}
	
	return &conn, nil
}

func (c *TCPIssuerClient) releaseConnection(conn *net.Conn) {
	if conn == nil {
		return
	}
	
	select {
	case c.pool <- conn:
	default:
		(*conn).Close()
	}
}

func (c *TCPIssuerClient) SendAndReceive(hexReq string) (string, error) {
	connPtr, err := c.getConnection()
	if err != nil {
		return "", err
	}
	
	conn := *connPtr
	defer c.releaseConnection(connPtr)
	
	if err := conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return "", fmt.Errorf("acquirer: set deadline: %w", err)
	}

	log.Printf("acquirer: connected to issuer at %s", c.addr)

	if err := transport.WriteFrame(conn, hexReq); err != nil {
		return "", fmt.Errorf("acquirer: write frame: %w", err)
	}

	hexResp, err := transport.ReadFrame(conn)
	if err != nil {
		return "", fmt.Errorf("acquirer: read frame: %w", err)
	}

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
