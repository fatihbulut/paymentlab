package acquirer

import (
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

type mockConn struct {
	writeCount int
	failCount  int
	failErr    error
}

func (m *mockConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func (m *mockConn) Write(b []byte) (n int, err error) {
	m.writeCount++
	if m.failCount > 0 {
		m.failCount--
		return 0, m.failErr
	}
	return len(b), nil
}

func TestSendToIssuer_RetrySuccessOnThirdAttempt(t *testing.T) {
	// 2 kez başarısız, 3. denemede başarılı
	mockConn := &mockConn{
		failCount: 2,
		failErr:   errors.New("connection refused"),
	}

	s := &AcquirerSwitch{
		connectionPool:   []net.Conn{mockConn},
		writeMutexes:     make([]sync.Mutex, 1),
		poolSize:         1,
		currentConnIndex: 0,
	}

	err := s.sendToIssuer([]byte("test"), 1)

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if mockConn.writeCount != 3 {
		t.Fatalf("expected 3 write attempts, got %d", mockConn.writeCount)
	}
}

func TestSendToIssuer_AllAttemptsFail(t *testing.T) {
	// Hepsi başarısız
	mockConn := &mockConn{
		failCount: 100, // Her zaman fail
		failErr:   errors.New("connection refused"),
	}

	s := &AcquirerSwitch{
		connectionPool:   []net.Conn{mockConn},
		writeMutexes:     make([]sync.Mutex, 1),
		poolSize:         1,
		currentConnIndex: 0,
	}

	err := s.sendToIssuer([]byte("test"), 1)

	if err == nil {
		t.Fatal("expected error after 3 attempts, got nil")
	}

	if mockConn.writeCount != 3 {
		t.Fatalf("expected 3 write attempts, got %d", mockConn.writeCount)
	}

	if err.Error() != "write failed after 3 attempts: connection refused" {
		t.Fatalf("unexpected error message: %v", err)
	}
}
