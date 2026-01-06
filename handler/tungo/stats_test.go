package tungo

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// normalizeConnID creates a normalized connection identifier
// This matches the pattern used in real implementations
func normalizeConnID(srcAddr, dstAddr, protocol string) string {
	return fmt.Sprintf("%s->%s/%s", srcAddr, dstAddr, strings.ToLower(protocol))
}

// realisticStatsReporter is a realistic test implementation that demonstrates
// production usage patterns including per-connection metering and metadata tracking
type realisticStatsReporter struct {
	metersMu sync.RWMutex
	meters   map[string]*connectionMeter

	// Track lifecycle events
	startsCount atomic.Uint64
	endsCount   atomic.Uint64
}

// connectionMeter tracks statistics for a single connection
type connectionMeter struct {
	connID   string
	protocol string
	srcAddr  string
	dstAddr  string

	// Traffic counters - matches real usage with atomic operations
	rxBytes atomic.Int64
	txBytes atomic.Int64

	// Optional metadata - similar to Wing's traffic.Metadata
	metaMu   sync.RWMutex
	metadata map[string]interface{}
}

func newRealisticStatsReporter() *realisticStatsReporter {
	return &realisticStatsReporter{
		meters: make(map[string]*connectionMeter),
	}
}

func (r *realisticStatsReporter) OnPacket(protocol, direction, srcAddr, dstAddr string, byteSize int) {
	connID := normalizeConnID(srcAddr, dstAddr, protocol)
	meter := r.getOrCreateMeter(connID, protocol, srcAddr, dstAddr)

	// Record bytes based on direction - matches Wing's implementation
	if direction == "rx" {
		meter.rxBytes.Add(int64(byteSize))
	} else if direction == "tx" {
		meter.txBytes.Add(int64(byteSize))
	}
}

func (r *realisticStatsReporter) OnConnectionStart(protocol, srcAddr, dstAddr string) {
	r.startsCount.Add(1)
	connID := normalizeConnID(srcAddr, dstAddr, protocol)

	// Pre-create meter on connection start
	r.getOrCreateMeter(connID, protocol, srcAddr, dstAddr)
}

func (r *realisticStatsReporter) OnConnectionEnd(protocol, srcAddr, dstAddr string) {
	r.endsCount.Add(1)
	connID := normalizeConnID(srcAddr, dstAddr, protocol)

	r.metersMu.Lock()
	delete(r.meters, connID)
	r.metersMu.Unlock()
}

// getOrCreateMeter implements the double-check locking pattern
// used in real implementations for thread-safe meter creation
func (r *realisticStatsReporter) getOrCreateMeter(connID, protocol, srcAddr, dstAddr string) *connectionMeter {
	// Fast path: check with read lock
	r.metersMu.RLock()
	if meter, exists := r.meters[connID]; exists {
		r.metersMu.RUnlock()
		return meter
	}
	r.metersMu.RUnlock()

	// Slow path: create new meter with write lock
	r.metersMu.Lock()
	defer r.metersMu.Unlock()

	// Double-check after acquiring write lock
	if meter, exists := r.meters[connID]; exists {
		return meter
	}

	meter := &connectionMeter{
		connID:   connID,
		protocol: protocol,
		srcAddr:  srcAddr,
		dstAddr:  dstAddr,
		metadata: make(map[string]interface{}),
	}
	r.meters[connID] = meter
	return meter
}

// Helper methods for testing
func (r *realisticStatsReporter) getMeter(connID string) *connectionMeter {
	r.metersMu.RLock()
	defer r.metersMu.RUnlock()
	return r.meters[connID]
}

func (r *realisticStatsReporter) getMeterCount() int {
	r.metersMu.RLock()
	defer r.metersMu.RUnlock()
	return len(r.meters)
}

// Simple mock for basic registry tests
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

// TestRealisticUsage_PerConnectionMetering demonstrates realistic usage pattern
// matching actual implementations like Wing's StatsReporter
func TestRealisticUsage_PerConnectionMetering(t *testing.T) {
	defer func() {
		statsReportersMu.Lock()
		statsReporters = make(map[string]TrafficStatsReporter)
		statsReportersMu.Unlock()
	}()

	reporter := newRealisticStatsReporter()
	guid := "realistic-test-guid"
	RegisterStatsReporter(guid, reporter)
	defer UnregisterStatsReporter(guid)

	// Simulate connection lifecycle
	srcAddr := "10.0.0.1:12345"
	dstAddr := "8.8.8.8:53"
	protocol := "udp"

	// Start connection
	dispatchOnConnectionStart(guid, protocol, srcAddr, dstAddr)

	// Verify connection was started
	if reporter.startsCount.Load() != 1 {
		t.Errorf("expected 1 connection start, got %d", reporter.startsCount.Load())
	}

	// Simulate bidirectional traffic
	dispatchOnPacket(guid, protocol, "rx", srcAddr, dstAddr, 512)
	dispatchOnPacket(guid, protocol, "rx", srcAddr, dstAddr, 256)
	dispatchOnPacket(guid, protocol, "tx", dstAddr, srcAddr, 1024)
	dispatchOnPacket(guid, protocol, "tx", dstAddr, srcAddr, 2048)

	// Verify per-connection metering
	connID := normalizeConnID(srcAddr, dstAddr, protocol)
	meter := reporter.getMeter(connID)
	if meter == nil {
		t.Fatal("expected meter to exist for connection")
	}

	if meter.rxBytes.Load() != 768 { // 512 + 256
		t.Errorf("expected 768 rx bytes, got %d", meter.rxBytes.Load())
	}
	if meter.txBytes.Load() != 3072 { // 1024 + 2048
		t.Errorf("expected 3072 tx bytes, got %d", meter.txBytes.Load())
	}

	// End connection
	dispatchOnConnectionEnd(guid, protocol, srcAddr, dstAddr)

	// Verify connection was ended and meter was cleaned up
	if reporter.endsCount.Load() != 1 {
		t.Errorf("expected 1 connection end, got %d", reporter.endsCount.Load())
	}
	if reporter.getMeter(connID) != nil {
		t.Error("expected meter to be cleaned up after connection end")
	}
}

// TestRealisticUsage_MultipleConnections tests handling multiple concurrent connections
func TestRealisticUsage_MultipleConnections(t *testing.T) {
	defer func() {
		statsReportersMu.Lock()
		statsReporters = make(map[string]TrafficStatsReporter)
		statsReportersMu.Unlock()
	}()

	reporter := newRealisticStatsReporter()
	guid := "multi-conn-test-guid"
	RegisterStatsReporter(guid, reporter)
	defer UnregisterStatsReporter(guid)

	// Simulate multiple concurrent connections
	connections := []struct {
		srcAddr string
		dstAddr string
	}{
		{"10.0.0.1:12345", "8.8.8.8:53"},
		{"10.0.0.2:23456", "1.1.1.1:53"},
		{"10.0.0.3:34567", "8.8.4.4:53"},
	}

	// Start all connections
	for _, conn := range connections {
		dispatchOnConnectionStart(guid, "udp", conn.srcAddr, conn.dstAddr)
		// Send some traffic
		dispatchOnPacket(guid, "udp", "rx", conn.srcAddr, conn.dstAddr, 100)
		dispatchOnPacket(guid, "udp", "tx", conn.dstAddr, conn.srcAddr, 200)
	}

	// Verify all connections have meters
	if reporter.getMeterCount() != 3 {
		t.Errorf("expected 3 meters, got %d", reporter.getMeterCount())
	}

	// Verify each meter has correct stats
	for _, conn := range connections {
		connID := normalizeConnID(conn.srcAddr, conn.dstAddr, "udp")
		meter := reporter.getMeter(connID)
		if meter == nil {
			t.Errorf("expected meter for %s", connID)
			continue
		}
		if meter.rxBytes.Load() != 100 {
			t.Errorf("connection %s: expected 100 rx bytes, got %d", connID, meter.rxBytes.Load())
		}
		if meter.txBytes.Load() != 200 {
			t.Errorf("connection %s: expected 200 tx bytes, got %d", connID, meter.txBytes.Load())
		}
	}

	// End all connections
	for _, conn := range connections {
		dispatchOnConnectionEnd(guid, "udp", conn.srcAddr, conn.dstAddr)
	}

	// Verify all meters cleaned up
	if reporter.getMeterCount() != 0 {
		t.Errorf("expected 0 meters after cleanup, got %d", reporter.getMeterCount())
	}
}

// TestRealisticUsage_GetOrCreatePattern tests the double-check locking pattern
// used in real implementations for thread-safe meter creation
func TestRealisticUsage_GetOrCreatePattern(t *testing.T) {
	defer func() {
		statsReportersMu.Lock()
		statsReporters = make(map[string]TrafficStatsReporter)
		statsReportersMu.Unlock()
	}()

	reporter := newRealisticStatsReporter()
	guid := "get-or-create-test-guid"
	RegisterStatsReporter(guid, reporter)
	defer UnregisterStatsReporter(guid)

	srcAddr := "10.0.0.1:12345"
	dstAddr := "8.8.8.8:53"

	// Simulate concurrent packet arrivals before OnConnectionStart
	// This tests the getOrCreate pattern works correctly
	var wg sync.WaitGroup
	numGoroutines := 10
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			// All goroutines try to send packets concurrently
			dispatchOnPacket(guid, "udp", "rx", srcAddr, dstAddr, 100)
		}()
	}

	wg.Wait()

	// Verify only one meter was created
	if reporter.getMeterCount() != 1 {
		t.Errorf("expected 1 meter, got %d", reporter.getMeterCount())
	}

	// Verify all packets were counted
	connID := normalizeConnID(srcAddr, dstAddr, "udp")
	meter := reporter.getMeter(connID)
	if meter == nil {
		t.Fatal("expected meter to exist")
	}
	if meter.rxBytes.Load() != 1000 { // 10 goroutines * 100 bytes
		t.Errorf("expected 1000 rx bytes, got %d", meter.rxBytes.Load())
	}
}

// TestRealisticUsage_ConnectionIDNormalization tests the connection ID pattern
func TestRealisticUsage_ConnectionIDNormalization(t *testing.T) {
	tests := []struct {
		name     string
		srcAddr  string
		dstAddr  string
		protocol string
		expected string
	}{
		{
			name:     "UDP DNS query",
			srcAddr:  "10.0.0.1:12345",
			dstAddr:  "8.8.8.8:53",
			protocol: "udp",
			expected: "10.0.0.1:12345->8.8.8.8:53/udp",
		},
		{
			name:     "UDP uppercase protocol",
			srcAddr:  "10.0.0.1:12345",
			dstAddr:  "8.8.8.8:53",
			protocol: "UDP",
			expected: "10.0.0.1:12345->8.8.8.8:53/udp",
		},
		{
			name:     "Different ports",
			srcAddr:  "192.168.1.1:54321",
			dstAddr:  "1.1.1.1:443",
			protocol: "udp",
			expected: "192.168.1.1:54321->1.1.1.1:443/udp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connID := normalizeConnID(tt.srcAddr, tt.dstAddr, tt.protocol)
			if connID != tt.expected {
				t.Errorf("expected connID %s, got %s", tt.expected, connID)
			}
		})
	}
}
