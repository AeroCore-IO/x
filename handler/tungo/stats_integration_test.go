package tungo

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestIntegration_UDPTrafficReporter demonstrates the complete integration
// of the TrafficStatsReporter with UDP traffic metering.
func TestIntegration_UDPTrafficReporter(t *testing.T) {
	// Clean up after test
	defer func() {
		statsReportersMu.Lock()
		statsReporters = make(map[string]TrafficStatsReporter)
		statsReportersMu.Unlock()
	}()

	// Create a reporter
	reporter := &IntegrationReporter{}
	guid := "integration-test-guid"

	// Register it
	RegisterStatsReporter(guid, reporter)
	defer UnregisterStatsReporter(guid)

	// Simulate UDP packet events
	dispatchOnConnectionStart(guid, "udp", "10.0.0.1:12345", "8.8.8.8:53")

	// Simulate RX packets (client -> server)
	dispatchOnPacket(guid, "udp", "rx", "10.0.0.1:12345", "8.8.8.8:53", 512)
	dispatchOnPacket(guid, "udp", "rx", "10.0.0.1:12345", "8.8.8.8:53", 256)

	// Simulate TX packets (server -> client)
	dispatchOnPacket(guid, "udp", "tx", "8.8.8.8:53", "10.0.0.1:12345", 1024)
	dispatchOnPacket(guid, "udp", "tx", "8.8.8.8:53", "10.0.0.1:12345", 2048)

	dispatchOnConnectionEnd(guid, "udp", "10.0.0.1:12345", "8.8.8.8:53")

	// Verify stats
	if reporter.connectionStarts.Load() != 1 {
		t.Errorf("expected 1 connection start, got %d", reporter.connectionStarts.Load())
	}
	if reporter.connectionEnds.Load() != 1 {
		t.Errorf("expected 1 connection end, got %d", reporter.connectionEnds.Load())
	}
	if reporter.rxPackets.Load() != 2 {
		t.Errorf("expected 2 rx packets, got %d", reporter.rxPackets.Load())
	}
	if reporter.txPackets.Load() != 2 {
		t.Errorf("expected 2 tx packets, got %d", reporter.txPackets.Load())
	}
	if reporter.rxBytes.Load() != 768 { // 512 + 256
		t.Errorf("expected 768 rx bytes, got %d", reporter.rxBytes.Load())
	}
	if reporter.txBytes.Load() != 3072 { // 1024 + 2048
		t.Errorf("expected 3072 tx bytes, got %d", reporter.txBytes.Load())
	}
}

// IntegrationReporter is a sample implementation for integration testing
type IntegrationReporter struct {
	rxBytes          atomic.Uint64
	txBytes          atomic.Uint64
	rxPackets        atomic.Uint64
	txPackets        atomic.Uint64
	connectionStarts atomic.Uint64
	connectionEnds   atomic.Uint64
}

func (r *IntegrationReporter) OnPacket(protocol, direction, srcAddr, dstAddr string, byteSize int) {
	if direction == "rx" {
		r.rxBytes.Add(uint64(byteSize))
		r.rxPackets.Add(1)
	} else if direction == "tx" {
		r.txBytes.Add(uint64(byteSize))
		r.txPackets.Add(1)
	}
}

func (r *IntegrationReporter) OnConnectionStart(protocol, srcAddr, dstAddr string) {
	r.connectionStarts.Add(1)
}

func (r *IntegrationReporter) OnConnectionEnd(protocol, srcAddr, dstAddr string) {
	r.connectionEnds.Add(1)
}

// TestWingLikeUsage demonstrates how Wing would use the reporter
func TestWingLikeUsage(t *testing.T) {
	// Clean up after test
	defer func() {
		statsReportersMu.Lock()
		statsReporters = make(map[string]TrafficStatsReporter)
		statsReportersMu.Unlock()
	}()

	// Wing initialization: create and register reporter
	wingReporter := &WingTrafficReporter{
		sessions: make(map[string]*SessionStats),
	}
	wingGUID := "wing-service-instance-abc123"
	RegisterStatsReporter(wingGUID, wingReporter)

	// Simulate multiple concurrent UDP sessions
	var wg sync.WaitGroup
	numSessions := 5
	wg.Add(numSessions)

	for i := 0; i < numSessions; i++ {
		go func(sessionID int) {
			defer wg.Done()

			srcAddr := "10.0.0.1:1000"
			dstAddr := "8.8.8.8:53"

			dispatchOnConnectionStart(wingGUID, "udp", srcAddr, dstAddr)
			for j := 0; j < 10; j++ {
				dispatchOnPacket(wingGUID, "udp", "rx", srcAddr, dstAddr, 100)
				dispatchOnPacket(wingGUID, "udp", "tx", dstAddr, srcAddr, 200)
			}
			dispatchOnConnectionEnd(wingGUID, "udp", srcAddr, dstAddr)
		}(i)
	}

	wg.Wait()

	// Verify Wing collected stats
	totalRX, totalTX := wingReporter.GetTotalBytes()
	expectedRX := uint64(numSessions * 10 * 100) // 5 sessions * 10 packets * 100 bytes
	expectedTX := uint64(numSessions * 10 * 200) // 5 sessions * 10 packets * 200 bytes

	if totalRX != expectedRX {
		t.Errorf("expected %d total RX bytes, got %d", expectedRX, totalRX)
	}
	if totalTX != expectedTX {
		t.Errorf("expected %d total TX bytes, got %d", expectedTX, totalTX)
	}

	// Wing shutdown: unregister reporter
	UnregisterStatsReporter(wingGUID)
}

// WingTrafficReporter is a Wing-like implementation that tracks per-session stats
type WingTrafficReporter struct {
	mu       sync.RWMutex
	sessions map[string]*SessionStats
	totalRX  atomic.Uint64
	totalTX  atomic.Uint64
}

type SessionStats struct {
	rxBytes atomic.Uint64
	txBytes atomic.Uint64
}

func (w *WingTrafficReporter) OnPacket(protocol, direction, srcAddr, dstAddr string, byteSize int) {
	sessionKey := srcAddr + "->" + dstAddr

	w.mu.RLock()
	session, exists := w.sessions[sessionKey]
	w.mu.RUnlock()

	if !exists {
		w.mu.Lock()
		session, exists = w.sessions[sessionKey]
		if !exists {
			session = &SessionStats{}
			w.sessions[sessionKey] = session
		}
		w.mu.Unlock()
	}

	if direction == "rx" {
		session.rxBytes.Add(uint64(byteSize))
		w.totalRX.Add(uint64(byteSize))
	} else if direction == "tx" {
		session.txBytes.Add(uint64(byteSize))
		w.totalTX.Add(uint64(byteSize))
	}
}

func (w *WingTrafficReporter) OnConnectionStart(protocol, srcAddr, dstAddr string) {
	// Wing could use this to track active sessions
}

func (w *WingTrafficReporter) OnConnectionEnd(protocol, srcAddr, dstAddr string) {
	// Wing could use this to clean up session state
}

func (w *WingTrafficReporter) GetTotalBytes() (rx, tx uint64) {
	return w.totalRX.Load(), w.totalTX.Load()
}
