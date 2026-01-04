// Package tungo provides UDP traffic metering through the TrafficStatsReporter interface.
//
// # Overview
//
// The tungo handler supports registering a TrafficStatsReporter to receive callbacks
// when UDP packets are sent or received. This enables external callers (such as Wing)
// to implement traffic metering and statistics collection.
//
// # Usage Example
//
// Here's how to implement and register a TrafficStatsReporter:
//
//	package main
//
//	import (
//		"fmt"
//		"sync/atomic"
//		"github.com/go-gost/x/handler/tungo"
//	)
//
//	// MyTrafficReporter implements tungo.TrafficStatsReporter
//	type MyTrafficReporter struct {
//		rxBytes atomic.Uint64
//		txBytes atomic.Uint64
//		rxPackets atomic.Uint64
//		txPackets atomic.Uint64
//	}
//
//	func (r *MyTrafficReporter) OnPacket(protocol, direction, srcAddr, dstAddr string, byteSize int) {
//		if direction == "rx" {
//			r.rxBytes.Add(uint64(byteSize))
//			r.rxPackets.Add(1)
//		} else if direction == "tx" {
//			r.txBytes.Add(uint64(byteSize))
//			r.txPackets.Add(1)
//		}
//		fmt.Printf("[%s] %s: %s -> %s (%d bytes)\n", protocol, direction, srcAddr, dstAddr, byteSize)
//	}
//
//	func (r *MyTrafficReporter) OnConnectionStart(protocol, srcAddr, dstAddr string) {
//		fmt.Printf("[%s] Connection started: %s -> %s\n", protocol, srcAddr, dstAddr)
//	}
//
//	func (r *MyTrafficReporter) OnConnectionEnd(protocol, srcAddr, dstAddr string) {
//		fmt.Printf("[%s] Connection ended: %s -> %s\n", protocol, srcAddr, dstAddr)
//	}
//
//	func (r *MyTrafficReporter) GetStats() (rxBytes, txBytes, rxPackets, txPackets uint64) {
//		return r.rxBytes.Load(), r.txBytes.Load(), r.rxPackets.Load(), r.txPackets.Load()
//	}
//
//	func main() {
//		// Create a reporter instance
//		reporter := &MyTrafficReporter{}
//
//		// Register it with a unique GUID
//		guid := "my-service-instance-123"
//		tungo.RegisterStatsReporter(guid, reporter)
//
//		// When configuring the tungo handler, set the statsGUID in metadata:
//		// In YAML config:
//		//   handler:
//		//     type: tungo
//		//     metadata:
//		//       statsGUID: "my-service-instance-123"
//		//
//		// Or programmatically via metadata map
//
//		// ... run your application ...
//
//		// Periodically read stats
//		rx, tx, rxPkts, txPkts := reporter.GetStats()
//		fmt.Printf("Stats: RX=%d bytes (%d pkts), TX=%d bytes (%d pkts)\n", rx, rxPkts, tx, txPkts)
//
//		// Unregister when done
//		defer tungo.UnregisterStatsReporter(guid)
//	}
//
// # Integration with Wing
//
// For Wing integration, the typical flow is:
//
//  1. Wing creates a custom TrafficStatsReporter implementation during initialization
//  2. Wing registers the reporter with RegisterStatsReporter using a unique GUID
//  3. Wing passes the same GUID to the tungo handler via metadata (statsGUID field)
//  4. The tungo handler will automatically call the reporter's methods for UDP traffic
//  5. Wing can unregister the reporter during shutdown with UnregisterStatsReporter
//
// # Protocol and Direction Conventions
//
// - protocol: Currently always "udp" for UDP traffic
// - direction:
//   - "rx": Received packets (from client/remote to destination)
//   - "tx": Transmitted packets (from destination back to client/remote)
// - srcAddr/dstAddr: Always in "ip:port" format (e.g., "10.0.0.1:12345")
//
// # Performance Considerations
//
// When no reporter is registered for a GUID, the dispatcher functions perform
// a fast map lookup and return immediately with minimal overhead. The OnPacket
// callback is invoked for every UDP packet, so implementations should be efficient
// and avoid blocking operations.
//
// For high-throughput scenarios, consider:
// - Using atomic operations for counters (as shown in the example)
// - Avoiding mutex locks in the hot path
// - Buffering/batching statistics if writing to external systems
//
// # Thread Safety
//
// All registration functions (RegisterStatsReporter, UnregisterStatsReporter,
// getStatsReporter) are thread-safe and can be called concurrently from multiple
// goroutines. Your TrafficStatsReporter implementation should also be thread-safe,
// as callbacks may be invoked from multiple goroutines handling different UDP flows.

package tungo
