package tungo

import (
	"sync"
)

// TrafficStatsReporter defines the interface for reporting UDP traffic statistics.
// Implementations can be registered by GUID to receive callbacks when UDP packets
// are sent or received.
type TrafficStatsReporter interface {
	// OnPacket is called when a UDP packet is sent or received.
	// Parameters:
	//   - protocol: the protocol (e.g., "udp")
	//   - direction: "rx" for received packets, "tx" for transmitted packets
	//   - srcAddr: source address in "ip:port" format
	//   - dstAddr: destination address in "ip:port" format
	//   - byteSize: number of bytes in the packet
	OnPacket(protocol, direction, srcAddr, dstAddr string, byteSize int)

	// OnConnectionStart is called when a UDP connection/session starts.
	// Optional: implementations may leave this empty if not needed.
	OnConnectionStart(protocol, srcAddr, dstAddr string)

	// OnConnectionEnd is called when a UDP connection/session ends.
	// Optional: implementations may leave this empty if not needed.
	OnConnectionEnd(protocol, srcAddr, dstAddr string)
}

var (
	// Global registry of stats reporters by GUID
	statsReporters   = make(map[string]TrafficStatsReporter)
	statsReportersMu sync.RWMutex
)

// RegisterStatsReporter registers a TrafficStatsReporter for the given GUID.
// If a reporter is already registered for this GUID, it will be replaced.
func RegisterStatsReporter(guid string, reporter TrafficStatsReporter) {
	if guid == "" || reporter == nil {
		return
	}
	statsReportersMu.Lock()
	statsReporters[guid] = reporter
	statsReportersMu.Unlock()
}

// UnregisterStatsReporter removes the TrafficStatsReporter for the given GUID.
func UnregisterStatsReporter(guid string) {
	if guid == "" {
		return
	}
	statsReportersMu.Lock()
	delete(statsReporters, guid)
	statsReportersMu.Unlock()
}

// getStatsReporter retrieves the TrafficStatsReporter for the given GUID.
// Returns nil if no reporter is registered for this GUID.
func getStatsReporter(guid string) TrafficStatsReporter {
	if guid == "" {
		return nil
	}
	statsReportersMu.RLock()
	reporter := statsReporters[guid]
	statsReportersMu.RUnlock()
	return reporter
}

// dispatchOnPacket triggers the OnPacket callback for the reporter registered
// with the given GUID. If no reporter is registered, this is a fast no-op.
//
// Parameters:
//   - guid: the GUID to look up the reporter
//   - protocol: the protocol (e.g., "udp")
//   - direction: "rx" for received, "tx" for transmitted
//   - srcAddr: source address in "ip:port" format
//   - dstAddr: destination address in "ip:port" format
//   - byteSize: number of bytes
func dispatchOnPacket(guid, protocol, direction, srcAddr, dstAddr string, byteSize int) {
	reporter := getStatsReporter(guid)
	if reporter != nil {
		reporter.OnPacket(protocol, direction, srcAddr, dstAddr, byteSize)
	}
}

// dispatchOnConnectionStart triggers the OnConnectionStart callback for the
// reporter registered with the given GUID. If no reporter is registered,
// this is a fast no-op.
func dispatchOnConnectionStart(guid, protocol, srcAddr, dstAddr string) {
	reporter := getStatsReporter(guid)
	if reporter != nil {
		reporter.OnConnectionStart(protocol, srcAddr, dstAddr)
	}
}

// dispatchOnConnectionEnd triggers the OnConnectionEnd callback for the
// reporter registered with the given GUID. If no reporter is registered,
// this is a fast no-op.
func dispatchOnConnectionEnd(guid, protocol, srcAddr, dstAddr string) {
	reporter := getStatsReporter(guid)
	if reporter != nil {
		reporter.OnConnectionEnd(protocol, srcAddr, dstAddr)
	}
}
