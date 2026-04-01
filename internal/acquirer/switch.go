package acquirer

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"iso-parser-service/internal/iso"
)

// AcquirerSwitch manages asynchronous ISO8583 message routing with multiplexing
type AcquirerSwitch struct {
	issuerAddr          string
	connectionPool      []net.Conn    // Connection pool for load balancing
	writeMutexes        []sync.Mutex  // Per-connection write mutexes
	poolSize            int           // Number of connections in pool
	poolMutex           sync.Mutex    // Mutex for pool access
	currentConnIndex    int           // Round-robin index
	pendingTransactions sync.Map      // Key: STAN (string), Value: chan []byte
	stanCounter         atomic.Uint64 // Unique STAN generator
	shutdownChan        chan struct{}
	wg                  sync.WaitGroup
}

// NewAcquirerSwitch creates a new switch instance with connection pooling
func NewAcquirerSwitch(issuerAddr string) *AcquirerSwitch {
	poolSize := 20 // 20 connections for load balancing under high concurrency
	return &AcquirerSwitch{
		issuerAddr:     issuerAddr,
		poolSize:       poolSize,
		connectionPool: make([]net.Conn, poolSize),
		writeMutexes:   make([]sync.Mutex, poolSize),
		shutdownChan:   make(chan struct{}),
	}
}

// Start initializes the switch and starts the issuer listeners.
// Blocks until at least one connection to issuer is established or timeout.
func (s *AcquirerSwitch) Start(ctx context.Context) error {
	// Start listener goroutine for each connection in pool
	for i := 0; i < s.poolSize; i++ {
		s.wg.Add(1)
		go s.issuerListener(ctx, i)
	}

	// Wait until at least one connection is ready (max 10s)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		s.poolMutex.Lock()
		ready := 0
		for i := 0; i < s.poolSize; i++ {
			if s.connectionPool[i] != nil {
				ready++
			}
		}
		s.poolMutex.Unlock()
		if ready > 0 {
			log.Printf("acquirer: switch ready with %d/%d issuer connections", ready, s.poolSize)
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	log.Printf("acquirer: WARNING — no issuer connections established after 10s, starting anyway")
	return nil
}

// Stop gracefully shuts down the switch
func (s *AcquirerSwitch) Stop(ctx context.Context) error {
	close(s.shutdownChan)
	s.wg.Wait()
	s.closeAllConnections()
	return nil
}

// HandleTerminalRequest processes a terminal request with multiplexing.
// Assigns a unique STAN to prevent collision, then correlates the response.
func (s *AcquirerSwitch) HandleTerminalRequest(ctx context.Context, rawISO []byte) ([]byte, error) {
	// Generate unique STAN for issuer-facing message (prevents collision)
	seq := s.stanCounter.Add(1)
	uniqueSTAN := fmt.Sprintf("%06d", (seq%999999)+1)

	// Rewrite STAN in the ISO message
	rewrittenISO, err := s.rewriteSTAN(rawISO, uniqueSTAN)
	if err != nil {
		return nil, fmt.Errorf("failed to rewrite STAN: %w", err)
	}

	// Create response channel for this transaction
	responseChan := make(chan []byte, 1)

	// Store channel — uniqueSTAN guarantees no collision
	s.pendingTransactions.Store(uniqueSTAN, responseChan)
	defer s.pendingTransactions.Delete(uniqueSTAN)
	// NOTE: Do NOT close(responseChan) — causes panic if listener sends concurrently.
	// Channel will be GC'd after both goroutines drop references.

	// Send to issuer with TPDU wrapper
	if err := s.sendToIssuer(rewrittenISO); err != nil {
		return nil, fmt.Errorf("failed to send to issuer: %w", err)
	}

	// Wait for response with timeout
	select {
	case response := <-responseChan:
		return response, nil
	case <-time.After(15 * time.Second):
		return nil, errors.New("transaction timeout after 15 seconds")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// issuerListener maintains persistent connection and routes responses
func (s *AcquirerSwitch) issuerListener(ctx context.Context, poolIndex int) {
	defer s.wg.Done()

	for {
		select {
		case <-s.shutdownChan:
			return
		case <-ctx.Done():
			return
		default:
		}

		// Ensure connection is established
		if !s.ensureConnection(poolIndex) {
			time.Sleep(2 * time.Second)
			continue
		}

		// Listen for responses
		if err := s.listenForResponses(ctx, poolIndex); err != nil {
			s.closeConnection(poolIndex)
			time.Sleep(2 * time.Second)
		}
	}
}

// ensureConnection establishes connection to issuer if not connected
func (s *AcquirerSwitch) ensureConnection(poolIndex int) bool {
	s.poolMutex.Lock()
	defer s.poolMutex.Unlock()

	// Check if connection already exists
	if s.connectionPool[poolIndex] != nil {
		return true
	}

	// Establish new connection
	conn, err := net.DialTimeout("tcp", s.issuerAddr, 5*time.Second)
	if err != nil {
		return false
	}

	// Enable TCP_NODELAY for low latency
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
	}

	s.connectionPool[poolIndex] = conn
	return true
}

// listenForResponses reads incoming packets and routes them
func (s *AcquirerSwitch) listenForResponses(ctx context.Context, poolIndex int) error {
	s.poolMutex.Lock()
	conn := s.connectionPool[poolIndex]
	s.poolMutex.Unlock()

	if conn == nil {
		return errors.New("no connection available")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.shutdownChan:
			return nil
		default:
		}

		// Set read deadline
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Read packet with TPDU
		packet, err := s.readTPDUPacket(conn)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // Normal timeout, keep listening
			}
			return fmt.Errorf("read packet failed: %w", err)
		}

		// Strip TPDU and get ISO message
		isoMessage, err := s.stripTPDU(packet)
		if err != nil {
			return fmt.Errorf("strip TPDU failed: %w", err)
		}

		// Extract STAN from response
		stan, err := s.extractSTAN(isoMessage)
		if err != nil {
			continue // Skip invalid responses
		}

		// Route to waiting channel (recover protects against rare send-after-delete)
		if value, ok := s.pendingTransactions.Load(stan); ok {
			responseChan := value.(chan []byte)
			select {
			case responseChan <- isoMessage:
				// Successfully routed response
			default:
				log.Printf("acquirer: response dropped for STAN %s (channel full)", stan)
			}
		} else {
			log.Printf("acquirer: no pending request for STAN %s (late response or timeout)", stan)
		}
	}
}

// sendToIssuer sends message with TPDU wrapper using round-robin load balancing
// with retry logic: 3 attempts with 100ms delay between retries
func (s *AcquirerSwitch) sendToIssuer(rawISO []byte) error {
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		// Get next connection from pool (round-robin)
		s.poolMutex.Lock()
		connIndex := s.currentConnIndex
		s.currentConnIndex = (s.currentConnIndex + 1) % s.poolSize
		conn := s.connectionPool[connIndex]
		s.poolMutex.Unlock()

		if conn == nil {
			lastErr = errors.New("no connection to issuer")
			// Connection yoksa yeniden dene, belki listener yeniden bağlanmıştır
			if attempt < 2 {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return lastErr
		}

		// Create TPDU wrapper: [2-byte Length] + [5-byte TPDU] + ISO Message
		tpdu := []byte{0x60, 0x00, 0x01, 0x00, 0x00}
		totalLen := len(tpdu) + len(rawISO)

		// Create packet: [2-byte length] + [TPDU] + [ISO]
		packet := make([]byte, 2+totalLen)
		binary.BigEndian.PutUint16(packet[:2], uint16(totalLen))
		copy(packet[2:], tpdu)
		copy(packet[2+len(tpdu):], rawISO)

		// Send packet (synchronized per connection to prevent byte interleaving)
		s.writeMutexes[connIndex].Lock()
		_, err := conn.Write(packet)
		s.writeMutexes[connIndex].Unlock()
		if err == nil {
			if attempt > 0 {
				log.Printf("acquirer: sendToIssuer succeeded on attempt %d/3", attempt+1)
			}
			return nil // Success!
		}

		lastErr = err
		log.Printf("acquirer: sendToIssuer attempt %d/3 failed: %v", attempt+1, err)

		// Son deneme değilse bekle ve tekrar dene
		if attempt < 2 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return fmt.Errorf("write failed after 3 attempts: %w", lastErr)
}

// readTPDUPacket reads a packet with 2-byte length prefix
func (s *AcquirerSwitch) readTPDUPacket(conn net.Conn) ([]byte, error) {
	// Read 2-byte length (use ReadFull to prevent partial reads)
	lengthBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, lengthBytes); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint16(lengthBytes)
	if length == 0 || length > 8192 {
		return nil, fmt.Errorf("invalid packet length: %d", length)
	}

	// Read packet data (use ReadFull to prevent partial reads)
	packet := make([]byte, length)
	if _, err := io.ReadFull(conn, packet); err != nil {
		return nil, err
	}

	return packet, nil
}

// stripTPDU removes TPDU header from packet
func (s *AcquirerSwitch) stripTPDU(packet []byte) ([]byte, error) {
	if len(packet) < 5 {
		return nil, errors.New("packet too short for TPDU")
	}
	return packet[5:], nil
}

// extractSTAN extracts Field 11 (STAN) from ISO message
func (s *AcquirerSwitch) extractSTAN(rawISO []byte) (string, error) {
	// Parse ISO message
	message, err := iso.ParseHexToMessage(fmt.Sprintf("%x", rawISO))
	if err != nil {
		return "", fmt.Errorf("parse ISO message failed: %w", err)
	}

	if message.STAN == "" {
		return "", errors.New("STAN field not found in message")
	}

	return message.STAN, nil
}

// rewriteSTAN replaces the STAN (Field 11) in the ISO message with a new value.
// Returns the modified raw ISO bytes.
func (s *AcquirerSwitch) rewriteSTAN(rawISO []byte, newSTAN string) ([]byte, error) {
	hexStr := fmt.Sprintf("%x", rawISO)
	msg, err := iso.ParseHexToMessage(hexStr)
	if err != nil {
		return nil, fmt.Errorf("parse for STAN rewrite: %w", err)
	}

	msg.STAN = newSTAN

	newHex, err := iso.PackMessageToHex(msg)
	if err != nil {
		return nil, fmt.Errorf("pack after STAN rewrite: %w", err)
	}

	newRaw, err := hex.DecodeString(newHex)
	if err != nil {
		return nil, fmt.Errorf("hex decode after STAN rewrite: %w", err)
	}

	return newRaw, nil
}

// closeConnection safely closes a specific connection in the pool
func (s *AcquirerSwitch) closeConnection(poolIndex int) {
	s.poolMutex.Lock()
	defer s.poolMutex.Unlock()

	if s.connectionPool[poolIndex] != nil {
		s.connectionPool[poolIndex].Close()
		s.connectionPool[poolIndex] = nil
	}
}

// closeAllConnections safely closes all connections in the pool
func (s *AcquirerSwitch) closeAllConnections() {
	s.poolMutex.Lock()
	defer s.poolMutex.Unlock()

	for i := 0; i < s.poolSize; i++ {
		if s.connectionPool[i] != nil {
			s.connectionPool[i].Close()
			s.connectionPool[i] = nil
		}
	}
}
