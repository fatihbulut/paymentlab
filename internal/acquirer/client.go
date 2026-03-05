package acquirer

import (
	"fmt"
	"net"

	"iso-parser-service/internal/transport"
)

type IssuerClient interface {
	SendAndReceive(hexReq string) (string, error)
}

type TCPIssuerClient struct {
	addr string
}

func NewTCPIssuerClient(addr string) *TCPIssuerClient {
	return &TCPIssuerClient{addr: addr}
}

func (c *TCPIssuerClient) SendAndReceive(hexReq string) (string, error) {
	conn, err := net.Dial("tcp", c.addr)
	if err != nil {
		return "", fmt.Errorf("acquirer: dial issuer at %s: %w", c.addr, err)
	}
	defer conn.Close()

	if err := transport.WriteFrame(conn, hexReq); err != nil {
		return "", fmt.Errorf("acquirer: write frame: %w", err)
	}

	hexResp, err := transport.ReadFrame(conn)
	if err != nil {
		return "", fmt.Errorf("acquirer: read frame: %w", err)
	}

	return hexResp, nil
}
