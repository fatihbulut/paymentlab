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
//
// MİMARİ:
// - Issuer'a N adet persistent TCP connection (connection pool)
// - Her request'e unique correlation ID (corrID) atanır
// - corrID TPDU header'a gömülür (ISO message'a dokunulmaz)
// - Response geldiğinde corrID ile eşleştirilir ve doğru channel'a route edilir
//
// NEDEN MULTIPLEXING?
// - Tek connection: sıralı işlem → yavaş (head-of-line blocking)
// - Her request için yeni connection: overhead çok yüksek
// - Connection pool + multiplexing: hem hızlı hem verimli
//
// NEDEN sync.Map?
// - Concurrent read/write güvenli (map + mutex'ten daha performanslı)
// - Key: corrID (uint32), Value: response channel (chan []byte)
type AcquirerSwitch struct {
	issuerAddr          string
	connectionPool      []net.Conn     // TCP connection pool (örn: 50 connection)
	writeMutexes        []sync.Mutex   // Her connection için ayrı mutex (byte interleaving önleme)
	poolSize            int            // Pool'daki connection sayısı
	switchTimeout       time.Duration  // Issuer'dan response bekleme timeout'u
	poolMutex           sync.Mutex     // Pool'a erişim mutex'i (connection ekleme/çıkarma)
	currentConnIndex    int            // Round-robin için index (hangi connection sırada)
	pendingTransactions sync.Map       // Bekleyen transaction'lar: corrID → response channel
	corrIDCounter       atomic.Uint32  // Unique corrID üreteci (atomic: race-free)
	shutdownChan        chan struct{}  // Shutdown sinyali
	wg                  sync.WaitGroup // Goroutine'lerin bitmesini beklemek için
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
//
// AKIŞ:
// 1. Unique corrID üret (atomic counter)
// 2. Response channel oluştur (buffered, size=1)
// 3. corrID → channel mapping'i sync.Map'e kaydet
// 4. Request'i issuer'a gönder (TPDU'ya corrID göm)
// 5. Response'u bekle (timeout ile)
// 6. Cleanup: corrID'yi sync.Map'ten sil
//
// ÖNEMLİ: Channel'ı KAPATMA!
// - close(responseChan) → panic riski (listener hâlâ yazıyor olabilir)
// - GC otomatik temizler (her iki goroutine de referansı bırakınca)
func (s *AcquirerSwitch) HandleTerminalRequest(ctx context.Context, rawISO []byte) ([]byte, error) {
	// 1. Unique corrID üret (atomic: thread-safe)
	corrID := s.corrIDCounter.Add(1)

	// 2. Bu transaction için response channel oluştur
	// Buffered (size=1): listener non-blocking send yapabilsin
	responseChan := make(chan []byte, 1)

	// 3. corrID → channel mapping'i kaydet
	// corrID unique olduğu için collision riski yok
	s.pendingTransactions.Store(corrID, responseChan)
	defer s.pendingTransactions.Delete(corrID) // Fonksiyon bitince temizle
	// NOT: close(responseChan) YAPMA! Listener concurrent yazıyor olabilir → panic
	// GC otomatik temizler (her iki taraf da referansı bırakınca)

	// 4. Request'i issuer'a gönder (TPDU'ya corrID göm)
	if err := s.sendToIssuer(rawISO, corrID); err != nil {
		return nil, fmt.Errorf("failed to send to issuer: %w", err)
	}

	// 5. Response'u bekle (3 farklı senaryo)
	// NewTimer kullan (time.After yerine): Stop() ile erken GC mümkün
	timer := time.NewTimer(s.switchTimeout)
	defer timer.Stop() // Memory leak önleme
	select {
	case response := <-responseChan:
		// SENARYO 1: Response geldi (normal case)
		return response, nil
	case <-timer.C:
		// SENARYO 2: Timeout (issuer yavaş veya cevap vermiyor)
		// corrID sync.Map'te kalır, late response gelirse "no pending request" log'u görülür
		return nil, fmt.Errorf("transaction timeout after %s", s.switchTimeout)
	case <-ctx.Done():
		// SENARYO 3: Client bağlantıyı kesti (HTTP request cancelled)
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

		// corrID'yi TPDU'dan çıkar (bytes 1-4)
		// Zero-allocation: ISO message parse etmeye gerek yok
		if len(packet) < 5 {
			// Packet çok kısa, corrupt olabilir
			continue
		}
		corrID := binary.BigEndian.Uint32(packet[1:5]) // TPDU bytes 1-4: corrID
		isoMessage := packet[5:]                       // TPDU'yu at, sadece ISO message

		// corrID ile bekleyen channel'ı bul ve response'u route et
		if value, ok := s.pendingTransactions.Load(corrID); ok {
			// Bekleyen request var!
			responseChan := value.(chan []byte)
			select {
			case responseChan <- isoMessage:
				// Başarılı! Response channel'a yazıldı, HandleTerminalRequest alacak
			default:
				// Channel full (olmamalı, buffered size=1)
				log.Printf("acquirer: response dropped for corrID %d (channel full)", corrID)
			}
		} else {
			// Bekleyen request yok!
			// Sebepler:
			// 1. Timeout oldu, corrID sync.Map'ten silindi
			// 2. Late response (issuer çok yavaş cevap verdi)
			log.Printf("acquirer: no pending request for corrID %d (late response or timeout)", corrID)
		}
	}
}

// sendToIssuer sends message with TPDU wrapper using round-robin load balancing
// with retry logic: 3 attempts with 100ms delay between retries.
// corrID is embedded in TPDU bytes 1-4 for correlation on response.
//
// TPDU FORMAT:
// [2-byte Length] + [5-byte TPDU] + [ISO Message]
//
//	└─ TPDU: [0x60] + [4-byte corrID]
//
// ROUND-ROBIN LOAD BALANCING:
// - currentConnIndex: sıradaki connection index'i
// - Her send'de +1 artır, poolSize'a ulaşınca 0'a dön
// - Tüm connection'lar eşit yük alır
//
// RETRY LOGIC:
// - 3 deneme, aralarında 100ms bekleme
// - Connection yoksa veya write error → retry
// - 3 deneme de başarısız → error döndür
func (s *AcquirerSwitch) sendToIssuer(rawISO []byte, corrID uint32) error {
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		// Round-robin: sıradaki connection'ı al
		s.poolMutex.Lock()
		connIndex := s.currentConnIndex                            // Mevcut index
		s.currentConnIndex = (s.currentConnIndex + 1) % s.poolSize // Bir sonraki için +1
		conn := s.connectionPool[connIndex]                        // Connection'ı al
		s.poolMutex.Unlock()

		if conn == nil {
			// Connection henüz kurulmamış veya kopmuş
			lastErr = errors.New("no connection to issuer")
			// Listener goroutine yeniden bağlanıyor olabilir, bekle ve tekrar dene
			if attempt < 2 {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return lastErr
		}

		// TPDU oluştur: [0x60] + [4-byte corrID]
		// 0x60: TPDU identifier (ISO8583 standart)
		// corrID: Response routing için (bytes 1-4)
		tpdu := [5]byte{0x60}                         // İlk byte: 0x60
		binary.BigEndian.PutUint32(tpdu[1:5], corrID) // Bytes 1-4: corrID (big-endian)
		totalLen := len(tpdu) + len(rawISO)           // TPDU + ISO message uzunluğu

		// Packet oluştur: [2-byte length prefix] + [TPDU] + [ISO]
		// Length prefix: TCP stream'de packet sınırlarını belirlemek için
		packet := make([]byte, 2+totalLen)
		binary.BigEndian.PutUint16(packet[:2], uint16(totalLen)) // İlk 2 byte: uzunluk
		copy(packet[2:], tpdu[:])                                // TPDU'yu kopyala
		copy(packet[2+len(tpdu):], rawISO)                       // ISO message'ı kopyala

		// Packet'i gönder (connection başına mutex: byte interleaving önleme)
		// ÖNEMLİ: Aynı connection'a concurrent write → bytes karışır → corrupt packet
		// Mutex: aynı anda sadece 1 goroutine yazabilir
		s.writeMutexes[connIndex].Lock()
		_, err := conn.Write(packet) // TCP write (blocking)
		s.writeMutexes[connIndex].Unlock()
		if err == nil {
			// Başarılı!
			if attempt > 0 {
				// İlk denemede başarısız olmuştuk, log'la
				log.Printf("acquirer: sendToIssuer succeeded on attempt %d/3", attempt+1)
			}
			return nil // Success!
		}

		// Write error (connection kopmuş olabilir)
		lastErr = err
		log.Printf("acquirer: sendToIssuer attempt %d/3 failed: %v", attempt+1, err)

		// Son deneme değilse bekle ve tekrar dene
		// Listener goroutine connection'ı yeniden kuruyor olabilir
		if attempt < 2 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// 3 deneme de başarısız
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
