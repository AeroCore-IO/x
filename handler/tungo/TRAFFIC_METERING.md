# UDP Traffic Metering with TrafficStatsReporter

This document explains how to use the `TrafficStatsReporter` interface in the `tungo` handler to implement UDP traffic metering for Wing and other applications.

## Overview

The `tungo` package provides a `TrafficStatsReporter` interface that allows external applications (such as Wing) to receive callbacks when UDP packets are sent or received. This enables real-time traffic monitoring and statistics collection.

## Quick Start

### 1. Implement the TrafficStatsReporter Interface

Create a struct that implements the three required methods:

```go
import "github.com/go-gost/x/handler/tungo"

type MyReporter struct {
    // Your fields here
}

func (r *MyReporter) OnPacket(protocol, direction, srcAddr, dstAddr string, byteSize int) {
    // Called for every UDP packet
    // direction: "rx" (received) or "tx" (transmitted)
    // protocol: "udp"
    // srcAddr/dstAddr: in "ip:port" format
}

func (r *MyReporter) OnConnectionStart(protocol, srcAddr, dstAddr string) {
    // Called when a UDP session starts (optional to implement)
}

func (r *MyReporter) OnConnectionEnd(protocol, srcAddr, dstAddr string) {
    // Called when a UDP session ends (optional to implement)
}
```

### 2. Register Your Reporter

During application initialization (e.g., Wing startup):

```go
reporter := &MyReporter{}
guid := "my-unique-service-guid"  // Use a unique identifier for your service instance

tungo.RegisterStatsReporter(guid, reporter)
defer tungo.UnregisterStatsReporter(guid)  // Cleanup on shutdown
```

### 3. Configure the Handler

Pass the GUID to the tungo handler via metadata. There are multiple ways to do this:

#### Option A: Via YAML Configuration

```yaml
handler:
  type: tungo
  metadata:
    statsGUID: "my-unique-service-guid"
```

#### Option B: Programmatically

```go
import (
    "github.com/go-gost/core/metadata"
    "github.com/go-gost/x/handler/tungo"
)

md := metadata.NewMetadata(map[string]any{
    "statsGUID": "my-unique-service-guid",
})

handler := tungo.NewHandler(/* options */)
handler.Init(md)
```

## Complete Example

See the [stats_integration_test.go](stats_integration_test.go) file for complete working examples, including:

- Basic traffic metering
- Per-session statistics tracking
- Thread-safe implementations using atomic operations

## API Reference

### TrafficStatsReporter Interface

```go
type TrafficStatsReporter interface {
    OnPacket(protocol, direction, srcAddr, dstAddr string, byteSize int)
    OnConnectionStart(protocol, srcAddr, dstAddr string)
    OnConnectionEnd(protocol, srcAddr, dstAddr string)
}
```

### Registration Functions

```go
// Register a reporter for the given GUID
func RegisterStatsReporter(guid string, reporter TrafficStatsReporter)

// Remove a reporter for the given GUID
func UnregisterStatsReporter(guid string)
```

### Parameter Conventions

| Parameter | Description | Example Values |
|-----------|-------------|----------------|
| `protocol` | Network protocol | `"udp"` |
| `direction` | Traffic direction | `"rx"` (received), `"tx"` (transmitted) |
| `srcAddr` | Source address | `"10.0.0.1:12345"` |
| `dstAddr` | Destination address | `"8.8.8.8:53"` |
| `byteSize` | Packet size in bytes | `512`, `1024`, etc. |

### Direction Semantics

- **`"rx"`**: Packets received from the client/remote endpoint to the destination
- **`"tx"`**: Packets transmitted from the destination back to the client/remote endpoint

## Wing Integration Pattern

For Wing applications, the recommended integration pattern is:

```go
package main

import (
    "sync/atomic"
    "github.com/go-gost/x/handler/tungo"
)

// WingTrafficMeter implements traffic metering for Wing
type WingTrafficMeter struct {
    rxBytes   atomic.Uint64
    txBytes   atomic.Uint64
    rxPackets atomic.Uint64
    txPackets atomic.Uint64
}

func (w *WingTrafficMeter) OnPacket(protocol, direction, srcAddr, dstAddr string, byteSize int) {
    if direction == "rx" {
        w.rxBytes.Add(uint64(byteSize))
        w.rxPackets.Add(1)
    } else if direction == "tx" {
        w.txBytes.Add(uint64(byteSize))
        w.txPackets.Add(1)
    }
}

func (w *WingTrafficMeter) OnConnectionStart(protocol, srcAddr, dstAddr string) {
    // Track session start if needed
}

func (w *WingTrafficMeter) OnConnectionEnd(protocol, srcAddr, dstAddr string) {
    // Clean up session state if needed
}

func (w *WingTrafficMeter) GetStats() (rxBytes, txBytes, rxPkts, txPkts uint64) {
    return w.rxBytes.Load(), w.txBytes.Load(), 
           w.rxPackets.Load(), w.txPackets.Load()
}

func main() {
    // Wing initialization
    meter := &WingTrafficMeter{}
    wingGUID := "wing-instance-123"
    
    tungo.RegisterStatsReporter(wingGUID, meter)
    defer tungo.UnregisterStatsReporter(wingGUID)
    
    // Configure handler with the same GUID in metadata
    // ... run Wing application ...
    
    // Periodically read stats
    rx, tx, rxPkts, txPkts := meter.GetStats()
    // ... report or log stats ...
}
```

## Performance Considerations

1. **Fast Path**: When no reporter is registered, the dispatcher performs only a quick map lookup and returns immediately.

2. **Hot Path**: `OnPacket` is called for every UDP packet. Keep implementations efficient:
   - Use atomic operations instead of mutexes for counters
   - Avoid blocking operations
   - Consider buffering/batching for external reporting

3. **Thread Safety**: All callbacks may be invoked from multiple goroutines concurrently. Your implementation must be thread-safe.

## Testing

Run the test suite to verify the implementation:

```bash
go test -v ./handler/tungo -run TestStats
go test -v ./handler/tungo -run TestIntegration
go test -v ./handler/tungo -run TestWingLikeUsage
```

## Troubleshooting

### Reporter not receiving callbacks

- Verify the GUID matches between registration and handler metadata
- Check that the handler was initialized with the correct metadata
- Ensure the reporter was registered before the handler started processing traffic

### Performance issues

- Profile your `OnPacket` implementation to identify bottlenecks
- Consider sampling (only process every Nth packet) for very high throughput
- Use buffering if writing to external systems

## See Also

- [stats_doc.go](stats_doc.go) - Detailed API documentation and examples
- [stats_test.go](stats_test.go) - Unit tests for the registry
- [stats_integration_test.go](stats_integration_test.go) - Integration tests and examples
