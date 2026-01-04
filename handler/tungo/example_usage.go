// Example: Basic usage of TrafficStatsReporter for UDP traffic metering
//
// This example demonstrates how to implement a simple traffic reporter
// and register it with the tungo handler. This is NOT compiled as part
// of the package - it's just for documentation purposes.
//
//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-gost/x/handler/tungo"
)

// SimpleTrafficReporter implements tungo.TrafficStatsReporter to track
// UDP traffic statistics using atomic counters.
type SimpleTrafficReporter struct {
	rxBytes   atomic.Uint64
	txBytes   atomic.Uint64
	rxPackets atomic.Uint64
	txPackets atomic.Uint64
	sessions  atomic.Uint64
}

// OnPacket is called for each UDP packet sent or received.
func (r *SimpleTrafficReporter) OnPacket(protocol, direction, srcAddr, dstAddr string, byteSize int) {
	if direction == "rx" {
		r.rxBytes.Add(uint64(byteSize))
		r.rxPackets.Add(1)
		fmt.Printf("[RX] %s -> %s: %d bytes\n", srcAddr, dstAddr, byteSize)
	} else if direction == "tx" {
		r.txBytes.Add(uint64(byteSize))
		r.txPackets.Add(1)
		fmt.Printf("[TX] %s -> %s: %d bytes\n", srcAddr, dstAddr, byteSize)
	}
}

// OnConnectionStart is called when a UDP session begins.
func (r *SimpleTrafficReporter) OnConnectionStart(protocol, srcAddr, dstAddr string) {
	r.sessions.Add(1)
	fmt.Printf("[SESSION START] %s: %s -> %s\n", protocol, srcAddr, dstAddr)
}

// OnConnectionEnd is called when a UDP session ends.
func (r *SimpleTrafficReporter) OnConnectionEnd(protocol, srcAddr, dstAddr string) {
	fmt.Printf("[SESSION END] %s: %s -> %s\n", protocol, srcAddr, dstAddr)
}

// PrintStats outputs the current statistics.
func (r *SimpleTrafficReporter) PrintStats() {
	fmt.Printf("\n=== Traffic Statistics ===\n")
	fmt.Printf("RX: %d bytes in %d packets\n", r.rxBytes.Load(), r.rxPackets.Load())
	fmt.Printf("TX: %d bytes in %d packets\n", r.txBytes.Load(), r.txPackets.Load())
	fmt.Printf("Total Sessions: %d\n", r.sessions.Load())
	fmt.Printf("=========================\n\n")
}

func main() {
	// Step 1: Create a reporter instance
	reporter := &SimpleTrafficReporter{}

	// Step 2: Register it with a unique GUID
	// This GUID must match the one configured in the handler metadata
	guid := "example-service-guid-12345"
	tungo.RegisterStatsReporter(guid, reporter)
	defer tungo.UnregisterStatsReporter(guid)

	fmt.Printf("TrafficStatsReporter registered with GUID: %s\n", guid)
	fmt.Printf("Configure your tungo handler with: statsGUID: \"%s\"\n\n", guid)

	// Step 3: Configure the handler with matching GUID in metadata
	// In YAML config:
	//   handler:
	//     type: tungo
	//     metadata:
	//       statsGUID: "example-service-guid-12345"
	//
	// Or programmatically:
	//   md := metadata.NewMetadata(map[string]any{
	//       "statsGUID": "example-service-guid-12345",
	//   })
	//   handler.Init(md)

	// Step 4: Simulate periodic stats reporting
	// In a real application, this would run concurrently with the handler
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	fmt.Println("Waiting for UDP traffic... (In real usage, the handler would be running)")
	fmt.Println("Press Ctrl+C to exit\n")

	// Simulate some traffic for demonstration
	go func() {
		time.Sleep(1 * time.Second)
		// In reality, these would be called by the tungo handler
		tungo.RegisterStatsReporter(guid, reporter) // Ensure it's registered
		reporter.OnConnectionStart("udp", "10.0.0.1:12345", "8.8.8.8:53")
		reporter.OnPacket("udp", "rx", "10.0.0.1:12345", "8.8.8.8:53", 512)
		reporter.OnPacket("udp", "tx", "8.8.8.8:53", "10.0.0.1:12345", 1024)
		reporter.OnConnectionEnd("udp", "10.0.0.1:12345", "8.8.8.8:53")
	}()

	// Print stats periodically
	for i := 0; i < 3; i++ {
		<-ticker.C
		reporter.PrintStats()
	}

	fmt.Println("Example completed. In production, keep the service running.")
}
