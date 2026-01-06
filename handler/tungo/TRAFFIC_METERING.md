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

The Wing integration pattern above demonstrates a production-ready implementation. For additional examples, see:

- [stats_test.go](stats_test.go) - Unit tests including `realisticStatsReporter` 
- [stats_integration_test.go](stats_integration_test.go) - Integration tests with concurrent scenarios
- [example_usage.go](example_usage.go) - Simple standalone example

Key implementation patterns demonstrated:
- Per-connection metering with normalized connection IDs
- Thread-safe meter creation using double-check locking
- Metadata enrichment and tracking
- Proper lifecycle management and cleanup

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

For Wing applications, the recommended integration pattern uses per-connection metering with metadata tracking:

```go
package tun

import (
    "fmt"
    "strings"
    "sync"
    "time"

    "ac-wing/internal/config"
    "ac-wing/internal/traffic"
    "github.com/go-gost/x/handler/tungo"
    "go.uber.org/zap"
)

// normalizeConnID creates a normalized connection identifier
func normalizeConnID(srcAddr, dstAddr, protocol string) string {
    return fmt.Sprintf("%s->%s/%s", srcAddr, dstAddr, strings.ToLower(protocol))
}

// StatsReporter implements tungo.TrafficStatsReporter interface
type StatsReporter struct {
    logger        *zap.SugaredLogger
    metersMu      sync.RWMutex
    meters        map[string]*traffic.Meter
    meterInterval time.Duration

    // Store metadata for connections
    connMetaMu sync.RWMutex
    connMeta   map[string]traffic.Metadata // key: normalized connID
}

func NewStatsReporter(logger *zap.SugaredLogger, cfg *config.Config) *StatsReporter {
    return &StatsReporter{
        logger:        logger,
        meters:        make(map[string]*traffic.Meter),
        meterInterval: cfg.Flexible.Performance.TrafficLogIntervalDuration(),
        connMeta:      make(map[string]traffic.Metadata),
    }
}

// UpdateMetadata updates metadata for a connection (optional, called externally)
func (r *StatsReporter) UpdateMetadata(srcAddr, dstAddr, protocol string, meta traffic.Metadata) {
    connID := normalizeConnID(srcAddr, dstAddr, protocol)

    r.connMetaMu.Lock()
    if existing, ok := r.connMeta[connID]; ok {
        // Merge with existing metadata
        for k, v := range meta {
            existing[k] = v
        }
        r.connMeta[connID] = existing
    } else {
        r.connMeta[connID] = meta
    }
    r.connMetaMu.Unlock()

    // Also update the meter if it already exists
    r.metersMu.RLock()
    if meter, exists := r.meters[connID]; exists {
        meter.AddMetadata(meta)
    }
    r.metersMu.RUnlock()
}

// OnPacket implements the tungo.TrafficStatsReporter interface
func (r *StatsReporter) OnPacket(protocol, direction, srcAddr, dstAddr string, bytes int) {
    connID := normalizeConnID(srcAddr, dstAddr, protocol)
    meter := r.getOrCreateMeter(connID, protocol, srcAddr, dstAddr)

    if meter != nil {
        // Record bytes based on direction
        if direction == "rx" {
            meter.RecordTargetToClient(int64(bytes))
        } else if direction == "tx" {
            meter.RecordClientToTarget(int64(bytes))
        }
    }
}

// OnConnectionStart implements the tungo.TrafficStatsReporter interface
func (r *StatsReporter) OnConnectionStart(protocol, srcAddr, dstAddr string) {
    connID := normalizeConnID(srcAddr, dstAddr, protocol)

    r.logger.Infow("📡 TUN connection started",
        "conn_id", connID,
        "protocol", strings.ToUpper(protocol),
        "src", srcAddr,
        "dst", dstAddr,
    )

    // Pre-create meter
    r.getOrCreateMeter(connID, protocol, srcAddr, dstAddr)
}

// OnConnectionEnd implements the tungo.TrafficStatsReporter interface
func (r *StatsReporter) OnConnectionEnd(protocol, srcAddr, dstAddr string) {
    connID := normalizeConnID(srcAddr, dstAddr, protocol)

    r.metersMu.Lock()
    meter, exists := r.meters[connID]
    if exists {
        delete(r.meters, connID)
    }
    r.metersMu.Unlock()

    r.connMetaMu.Lock()
    delete(r.connMeta, connID)
    r.connMetaMu.Unlock()

    if exists {
        meter.Stop()

        r.logger.Infow("📡 TUN connection ended",
            "conn_id", connID,
            "protocol", strings.ToUpper(protocol),
        )
    }
}

// getOrCreateMeter uses double-check locking for thread-safe meter creation
func (r *StatsReporter) getOrCreateMeter(connID, protocol, srcAddr, dstAddr string) *traffic.Meter {
    r.metersMu.RLock()
    if meter, exists := r.meters[connID]; exists {
        r.metersMu.RUnlock()
        return meter
    }
    r.metersMu.RUnlock()

    r.metersMu.Lock()
    defer r.metersMu.Unlock()

    // Double-check after acquiring write lock
    if meter, exists := r.meters[connID]; exists {
        return meter
    }

    meta := traffic.Metadata{
        "conn_type": "tun",
        "protocol":  strings.ToUpper(protocol),
        "src_addr":  srcAddr,
        "dst_addr":  dstAddr,
    }

    // Merge with pre-populated metadata (from external sources)
    r.connMetaMu.RLock()
    if existingMeta, ok := r.connMeta[connID]; ok {
        for k, v := range existingMeta {
            meta[k] = v
        }
    }
    r.connMetaMu.RUnlock()

    meter := traffic.NewMeter(r.logger, connID, r.meterInterval, meta)
    meter.Start()

    r.meters[connID] = meter

    r.logger.Infow("📊 TUN traffic meter started",
        "conn_id", connID,
        "protocol", strings.ToUpper(protocol),
        "src", srcAddr,
        "dst", dstAddr)

    return meter
}

// Close stops all active meters
func (r *StatsReporter) Close() {
    r.metersMu.Lock()
    defer r.metersMu.Unlock()

    for connID, meter := range r.meters {
        meter.Stop()
        r.logger.Debugw("TUN traffic meter stopped", "conn_id", connID)
    }
    r.meters = make(map[string]*traffic.Meter)

    r.connMetaMu.Lock()
    r.connMeta = make(map[string]traffic.Metadata)
    r.connMetaMu.Unlock()
}

// Usage in Wing initialization:
func main() {
    logger := zap.NewExample().Sugar()
    cfg := config.Load()
    
    reporter := NewStatsReporter(logger, cfg)
    wingGUID := "wing-instance-123"
    
    tungo.RegisterStatsReporter(wingGUID, reporter)
    defer tungo.UnregisterStatsReporter(wingGUID)
    defer reporter.Close()
    
    // Configure handler with the same GUID in metadata
    // ... run Wing application ...
}
```

### Key Features

1. **Per-Connection Metering**: Each connection gets its own meter identified by normalized connID
2. **Metadata Tracking**: Supports enriching connections with metadata (app ID, hostname, etc.)
3. **Thread-Safe**: Uses double-check locking pattern for efficient concurrent access
4. **Lifecycle Management**: Automatic cleanup of meters on connection end
5. **Logging Integration**: Rich logging for debugging and monitoring

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
