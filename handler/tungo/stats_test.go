package tungo

import (
	"sync"
	"testing"
)

// mockStatsReporter is a test implementation of TrafficStatsReporter
type mockStatsReporter struct {
	mu               sync.Mutex
	packets          []packetEvent
	connectionsStart []connectionEvent
	connectionsEnd   []connectionEvent
}

type packetEvent struct {
	protocol  string
	direction string
	srcAddr   string
	dstAddr   string
	byteSize  int
}

type connectionEvent struct {
	protocol string
	srcAddr  string
	dstAddr  string
}

func (m *mockStatsReporter) OnPacket(protocol, direction, srcAddr, dstAddr string, byteSize int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.packets = append(m.packets, packetEvent{
		protocol:  protocol,
		direction: direction,
		srcAddr:   srcAddr,
		dstAddr:   dstAddr,
		byteSize:  byteSize,
	})
}

func (m *mockStatsReporter) OnConnectionStart(protocol, srcAddr, dstAddr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectionsStart = append(m.connectionsStart, connectionEvent{
		protocol: protocol,
		srcAddr:  srcAddr,
		dstAddr:  dstAddr,
	})
}

func (m *mockStatsReporter) OnConnectionEnd(protocol, srcAddr, dstAddr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectionsEnd = append(m.connectionsEnd, connectionEvent{
		protocol: protocol,
		srcAddr:  srcAddr,
		dstAddr:  dstAddr,
	})
}

func (m *mockStatsReporter) getPacketCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.packets)
}

func (m *mockStatsReporter) getLastPacket() *packetEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.packets) == 0 {
		return nil
	}
	return &m.packets[len(m.packets)-1]
}

func TestRegisterStatsReporter(t *testing.T) {
	// Clean up after test
	defer func() {
		statsReportersMu.Lock()
		statsReporters = make(map[string]TrafficStatsReporter)
		statsReportersMu.Unlock()
	}()

	mock := &mockStatsReporter{}
	guid := "test-guid-123"

	// Register reporter
	RegisterStatsReporter(guid, mock)

	// Verify it was registered
	reporter := getStatsReporter(guid)
	if reporter == nil {
		t.Fatal("expected reporter to be registered")
	}
	if reporter != mock {
		t.Fatal("expected to get back the same reporter instance")
	}
}

func TestUnregisterStatsReporter(t *testing.T) {
	// Clean up after test
	defer func() {
		statsReportersMu.Lock()
		statsReporters = make(map[string]TrafficStatsReporter)
		statsReportersMu.Unlock()
	}()

	mock := &mockStatsReporter{}
	guid := "test-guid-456"

	// Register and then unregister
	RegisterStatsReporter(guid, mock)
	UnregisterStatsReporter(guid)

	// Verify it was unregistered
	reporter := getStatsReporter(guid)
	if reporter != nil {
		t.Fatal("expected reporter to be unregistered")
	}
}

func TestGetStatsReporter_NotRegistered(t *testing.T) {
	// Clean up after test
	defer func() {
		statsReportersMu.Lock()
		statsReporters = make(map[string]TrafficStatsReporter)
		statsReportersMu.Unlock()
	}()

	// Get non-existent reporter
	reporter := getStatsReporter("non-existent-guid")
	if reporter != nil {
		t.Fatal("expected nil for non-existent guid")
	}
}

func TestGetStatsReporter_EmptyGUID(t *testing.T) {
	reporter := getStatsReporter("")
	if reporter != nil {
		t.Fatal("expected nil for empty guid")
	}
}

func TestDispatchOnPacket_WithReporter(t *testing.T) {
	// Clean up after test
	defer func() {
		statsReportersMu.Lock()
		statsReporters = make(map[string]TrafficStatsReporter)
		statsReportersMu.Unlock()
	}()

	mock := &mockStatsReporter{}
	guid := "test-guid-packet"
	RegisterStatsReporter(guid, mock)

	// Dispatch packet event
	dispatchOnPacket(guid, "udp", "rx", "10.0.0.1:12345", "8.8.8.8:53", 512)

	// Verify callback was invoked
	if mock.getPacketCount() != 1 {
		t.Fatalf("expected 1 packet event, got %d", mock.getPacketCount())
	}

	pkt := mock.getLastPacket()
	if pkt.protocol != "udp" {
		t.Errorf("expected protocol 'udp', got '%s'", pkt.protocol)
	}
	if pkt.direction != "rx" {
		t.Errorf("expected direction 'rx', got '%s'", pkt.direction)
	}
	if pkt.srcAddr != "10.0.0.1:12345" {
		t.Errorf("expected srcAddr '10.0.0.1:12345', got '%s'", pkt.srcAddr)
	}
	if pkt.dstAddr != "8.8.8.8:53" {
		t.Errorf("expected dstAddr '8.8.8.8:53', got '%s'", pkt.dstAddr)
	}
	if pkt.byteSize != 512 {
		t.Errorf("expected byteSize 512, got %d", pkt.byteSize)
	}
}

func TestDispatchOnPacket_NoReporter(t *testing.T) {
	// Clean up after test
	defer func() {
		statsReportersMu.Lock()
		statsReporters = make(map[string]TrafficStatsReporter)
		statsReportersMu.Unlock()
	}()

	// Dispatch without registered reporter - should not panic
	dispatchOnPacket("non-existent-guid", "udp", "tx", "10.0.0.1:12345", "8.8.8.8:53", 1024)
	// Test passes if no panic occurred
}

func TestDispatchOnConnectionStart(t *testing.T) {
	// Clean up after test
	defer func() {
		statsReportersMu.Lock()
		statsReporters = make(map[string]TrafficStatsReporter)
		statsReportersMu.Unlock()
	}()

	mock := &mockStatsReporter{}
	guid := "test-guid-conn-start"
	RegisterStatsReporter(guid, mock)

	// Dispatch connection start event
	dispatchOnConnectionStart(guid, "udp", "10.0.0.1:12345", "8.8.8.8:53")

	// Verify callback was invoked
	mock.mu.Lock()
	count := len(mock.connectionsStart)
	mock.mu.Unlock()

	if count != 1 {
		t.Fatalf("expected 1 connection start event, got %d", count)
	}
}

func TestDispatchOnConnectionEnd(t *testing.T) {
	// Clean up after test
	defer func() {
		statsReportersMu.Lock()
		statsReporters = make(map[string]TrafficStatsReporter)
		statsReportersMu.Unlock()
	}()

	mock := &mockStatsReporter{}
	guid := "test-guid-conn-end"
	RegisterStatsReporter(guid, mock)

	// Dispatch connection end event
	dispatchOnConnectionEnd(guid, "udp", "10.0.0.1:12345", "8.8.8.8:53")

	// Verify callback was invoked
	mock.mu.Lock()
	count := len(mock.connectionsEnd)
	mock.mu.Unlock()

	if count != 1 {
		t.Fatalf("expected 1 connection end event, got %d", count)
	}
}

func TestConcurrentAccess(t *testing.T) {
	// Clean up after test
	defer func() {
		statsReportersMu.Lock()
		statsReporters = make(map[string]TrafficStatsReporter)
		statsReportersMu.Unlock()
	}()

	const numGoroutines = 10
	const numOps = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			guid := "test-guid-concurrent"
			mock := &mockStatsReporter{}

			for j := 0; j < numOps; j++ {
				// Register
				RegisterStatsReporter(guid, mock)
				// Get
				_ = getStatsReporter(guid)
				// Dispatch
				dispatchOnPacket(guid, "udp", "rx", "10.0.0.1:1", "8.8.8.8:53", 100)
				// Unregister
				UnregisterStatsReporter(guid)
			}
		}(i)
	}

	wg.Wait()
	// Test passes if no race conditions or panics occurred
}

func TestRegisterNilReporter(t *testing.T) {
	// Clean up after test
	defer func() {
		statsReportersMu.Lock()
		statsReporters = make(map[string]TrafficStatsReporter)
		statsReportersMu.Unlock()
	}()

	// Should not panic or register anything
	RegisterStatsReporter("test-guid", nil)

	reporter := getStatsReporter("test-guid")
	if reporter != nil {
		t.Fatal("expected nil reporter to not be registered")
	}
}

func TestRegisterEmptyGUID(t *testing.T) {
	// Clean up after test
	defer func() {
		statsReportersMu.Lock()
		statsReporters = make(map[string]TrafficStatsReporter)
		statsReportersMu.Unlock()
	}()

	mock := &mockStatsReporter{}
	// Should not register with empty GUID
	RegisterStatsReporter("", mock)

	// Verify map is still empty
	statsReportersMu.RLock()
	count := len(statsReporters)
	statsReportersMu.RUnlock()

	if count != 0 {
		t.Fatalf("expected 0 reporters, got %d", count)
	}
}
