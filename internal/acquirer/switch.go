package acquirer

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// AcquirerSwitch manages asynchronous ISO8583 message routing with multiplexing
type AcquirerSwitch struct {
	issuerAddr          string
	connectionPool      []net.Conn    // Connection pool for load balancing
	writeMutexes        []sync.Mutex  // Per-connection write mutexes
	poolSize            int           // Number of connections in pool
	switchTimeout       time.Duration // Timeout for issuer response
	poolMutex           sync.Mutex    // Mutex for pool access
	currentConnIndex    int           // Round-robin index
	pendingTransactions sync.Map      // Key: uint32 (corrID), Value: chan []byte
	corrIDCounter       atomic.Uint32 // Unique correlation ID generator
	shutdownChan        chan struct{}
	wg                  sync.WaitGroup
}

// NewAcquirerSwitch creates a new switch instance with connection pooling
func NewAcquirerSwitch(issuerAddr string) *AcquirerSwitch {
	poolSize := 40 // default for local dev
	if v := os.Getenv("ACQUIRER_TCP_POOL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			poolSize = n
		}
	}
	// Switch timeout should be slightly below HTTP request timeout to avoid
	// doing work after the caller has given up.
	switchTimeoutMS := 1600
	if v := os.Getenv("SWITCH_TIMEOUT_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			switchTimeoutMS = n
		}
	} else if v := os.Getenv("SWITCH_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			switchTimeoutMS = n * 1000
		}
	}
	return &AcquirerSwitch{
		issuerAddr:     issuerAddr,
		poolSize:       poolSize,
		switchTimeout:  time.Duration(switchTimeoutMS) * time.Millisecond,
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
// Embeds a unique correlation ID in the TPDU header — no ISO re-parsing needed.
func (s *AcquirerSwitch) HandleTerminalRequest(ctx context.Context, rawISO []byte) ([]byte, error) {
	// Generate unique correlation ID (embedded in TPDU, not in ISO message)
	corrID := s.corrIDCounter.Add(1)

	// Create response channel for this transaction
	responseChan := make(chan []byte, 1)

	// Store channel — corrID guarantees no collision
	s.pendingTransactions.Store(corrID, responseChan)
	defer s.pendingTransactions.Delete(corrID)
	// NOTE: Do NOT close(responseChan) — causes panic if listener sends concurrently.
	// Channel will be GC'd after both goroutines drop references.

	// Send to issuer with TPDU wrapper (corrID embedded in TPDU bytes 1-4)
	if err := s.sendToIssuer(rawISO, corrID); err != nil {
		return nil, fmt.Errorf("failed to send to issuer: %w", err)
	}

	// Wait for response with timeout (use NewTimer to allow early GC via Stop)
	timer := time.NewTimer(s.switchTimeout)
	defer timer.Stop()
	select {
	case response := <-responseChan:
		return response, nil
	case <-timer.C:
		return nil, fmt.Errorf("transaction timeout after %s", s.switchTimeout)
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

		// Extract correlation ID from TPDU bytes 1-4 (zero-alloc, no ISO parsing)
		if len(packet) < 5 {
			continue
		}
		corrID := binary.BigEndian.Uint32(packet[1:5])
		isoMessage := packet[5:] // strip TPDU

		// Route to waiting channel
		if value, ok := s.pendingTransactions.Load(corrID); ok {
			responseChan := value.(chan []byte)
			select {
			case responseChan <- isoMessage:
				// Successfully routed response
			default:
				log.Printf("acquirer: response dropped for corrID %d (channel full)", corrID)
			}
		} else {
			log.Printf("acquirer: no pending request for corrID %d (late response or timeout)", corrID)
		}
	}
}

// sendToIssuer sends message with TPDU wrapper using round-robin load balancing
// with retry logic: 3 attempts with 100ms delay between retries.
// corrID is embedded in TPDU bytes 1-4 for correlation on response.
func (s *AcquirerSwitch) sendToIssuer(rawISO []byte, corrID uint32) error {
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
		// Bytes 1-4 carry the correlation ID for response routing
		tpdu := [5]byte{0x60}
		binary.BigEndian.PutUint32(tpdu[1:5], corrID)
		totalLen := len(tpdu) + len(rawISO)

		// Create packet: [2-byte length] + [TPDU] + [ISO]
		packet := make([]byte, 2+totalLen)
		binary.BigEndian.PutUint16(packet[:2], uint16(totalLen))
		copy(packet[2:], tpdu[:])
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
