# Tendrl Go SDK

[![Version](https://img.shields.io/badge/version-0.1.0-blue.svg)](https://github.com/tendrl-inc/clients/tendrl_go_sdk)
[![Go Version](https://img.shields.io/badge/go-1.16+-00ADD8.svg)](https://golang.org/doc/devel/release.html)
[![License](https://img.shields.io/badge/license-Proprietary-red.svg)](LICENSE)

A simple, flexible SDK for high-performance messaging.

## ⚠️ License Notice

**This software is licensed for use with Tendrl services only.**

### ✅ Allowed

- Use the software with Tendrl services
- Inspect and learn from the code for educational purposes
- Modify or extend the software for personal or Tendrl-related use

### ❌ Not Allowed

- Use in any competing product or service
- Connect to any backend not operated by Tendrl, Inc.
- Package into any commercial or hosted product (e.g., SaaS, PaaS)
- Copy design patterns or protocol logic for another system without permission

For licensing questions, contact: `support@tendrl.com`

## Features

- **Flexibility**: Works with any JSON-serializable type - strings (automatically wrapped), maps (any key/value types), structs, arrays, primitives - no complex formatting
- **Offline Message Storage**: BoltDB-based persistence with TTL
- **Automatic Retry**: Background retry process for offline messages
- **Automatic Heartbeats**: System resource monitoring with automatic heartbeat messages (managed mode)
- **Resource Monitoring**: Automatic system resource adaptation
- **Dynamic Batch Processing**: CPU/memory-aware batching (10-500 messages)
- **Thread-Safe Operations**: Concurrent-safe message handling
- **Consolidated API**: Single `Publish()` method handles all use cases

**Benefits:**

- Simple setup with no additional components
- Direct HTTPS communication with Tendrl servers
- Built-in retry logic and error handling
- Dynamic batching based on system resources
- Full control over request lifecycle
- Clean code structure with separated models and client logic

## Installation

```bash
go get github.com/tendrl-inc-labs/go-sdk@latest
```

## Configuration

The Go SDK supports multiple ways to configure the client:

### 1. Environment Variables (Default)

```bash
export TENDRL_KEY="your_api_key_here"
```

### 2. API Key Parameters

For programmatic use, you can pass the API key directly:

```go
// Managed mode with API key parameter (recommended)
client, err := tendrl.NewClient(true, "your_api_key_here")

// Direct API mode with API key parameter  
client, err := tendrl.NewClient(false, "your_api_key_here")
```

### 3. Configuration Priority

**API Key Sources (highest to lowest priority):**

1. **API Key Parameter** (passed to constructor)
2. **Environment Variable** (TENDRL_KEY)

**Mode Parameter:**

- `true` = Managed mode (full features: queuing, batching, offline storage) - **Recommended**
- `false` = Direct API mode (immediate API calls only)

### 4. Configuration File

You can also configure the client using a JSON configuration file. The SDK will automatically look for a config file at `~/.tendrl/config.json` or you can specify a custom path.

```json
{
  "managed": true,
  "timeout_seconds": 10,
  "max_retries": 3,
  "debug": false,
  "send_heartbeat": true,
  "heartbeat_interval_seconds": 30,
  "offline_storage": true,
  "storage_path": "tendrl_storage.db"
}
```

To generate an example configuration file:

```go
import "github.com/tendrl-inc-labs/go-sdk/tendrl"

// Generate example config at default location
err := tendrl.InitializeConfig(tendrl.GetDefaultConfigPath())
```

## Debug Mode

The Go SDK includes a debug mode that provides detailed logging of SDK operations. This is useful for troubleshooting, development, and understanding SDK behavior.

### Enabling Debug Mode

Debug mode can be enabled in several ways:

**1. Configuration File:**

```json
{
  "debug": true
}
```

**2. Programmatic Configuration:**

When loading a config file, set `debug: true` in the JSON configuration.

### What Gets Logged

When debug mode is enabled, the SDK logs:

- **Client Initialization**: SDK startup, configuration loading, API key validation
- **Message Publishing**: All publish operations with details (tags, destination, wait response)
- **API Requests**: HTTP endpoints being called, request/response status
- **Message Callbacks**: Incoming messages received and processed
- **Queue Processing**: Batch operations, queue status
- **Connectivity**: Network state changes (online/offline transitions)
- **Heartbeats**: Automatic heartbeat sending with system resource data
- **Offline Storage**: Message storage and retry operations

### Example Debug Output

```
[DEBUG] TendrlClient initialized (managed=true, debug=true)
[DEBUG] API base URL: https://app.tendrl.com/api
[DEBUG] Validating API key
[DEBUG] API key validated successfully
[DEBUG] Initializing offline storage: tendrl_storage.db
[DEBUG] Publishing message (tags=[sensor temperature], entity=, waitResponse=false)
[DEBUG] Sending 1 message(s) (waitResponse=false)
[DEBUG] Sending request to: https://app.tendrl.com/api/entities/messages
[DEBUG] Message(s) sent successfully (status=200)
[DEBUG] Checking for incoming messages (limit=1)
[DEBUG] Received 2 message(s)
[DEBUG] Processing message: type=cmd, source=123:us-1:entity:sender
[DEBUG] Sending heartbeat
[DEBUG] Heartbeat data: mem_free=8589934592, mem_total=17179869184, disk_free=536870912000, disk_size=1073741824000
[DEBUG] Connectivity changed: online -> offline
[DEBUG] Connectivity changed: offline -> online
[DEBUG] TendrlClient stopping
[DEBUG] TendrlClient stopped
```

### Disabling Debug Mode

Debug mode is disabled by default. To disable it explicitly:

```json
{
  "debug": false
}
```

**Note**: Debug logging uses Go's standard `log` package and outputs to `stderr`. In production environments, you may want to redirect or filter debug output.

## Operating Modes

The Go SDK supports two operating modes:

### Managed Mode (Default & Recommended)

#### Full-featured with automatic background processing

- **Automatic** API key validation on startup
- **Automatic** message queuing and dynamic batching
- **Automatic** offline storage with retry
- **Automatic** heartbeat messages with system resource information
- **Automatic** background system monitoring
- **Automatic** resource-aware batch optimization

### Direct API Mode

#### Lightweight immediate API calls

- ✅ Direct HTTP requests only
- ✅ No background processes
- ✅ Minimal resource usage
- ✅ Synchronous and asynchronous publishing
- ❌ No queuing, batching, or offline storage

## Quick Start

### Method 1: Using Environment Variables (Traditional)

```go
package main

import (
    "log"
    "time"
    
    tendrl "github.com/tendrl-inc-labs/go-sdk/tendrl"
)

func main() {
    // Create managed client (reads TENDRL_KEY environment variable)
    client, err := tendrl.NewClient(true, "") // true = managed mode, "" = use env var
    if err != nil {
        log.Fatal(err)
    }
    defer client.Stop()
}
```

### Method 2: Using API Key Parameters (Recommended for Programmatic Use)

```go
package main

import (
    "log"
    "time"
    
    tendrl "github.com/tendrl-inc-labs/go-sdk/tendrl"
)

func main() {
    // Create managed client with API key parameter (no environment variable needed)
    client, err := tendrl.NewClient(true, "your_api_key_here") // true = managed mode
    if err != nil {
        log.Fatal(err)
    }
    defer client.Stop()
}
```

## API Reference

### Publishing Messages

```go
// Publish with optional response waiting
messageID, err := client.Publish(
    data,                  // Any data type
    []string{"tag1", "tag2"}, // Tags
    "entity_name",         // Target entity (empty string for default)
    true,                  // wait_response
    10,                    // timeout in seconds
) // Returns message ID if wait_response=True

// Async publishing (fire-and-forget)
err := client.PublishAsync(data, []string{"tag1", "tag2"})

// All publishing goes through the main Publish method
// which handles both sync and async cases automatically
```

### Helper Methods

The SDK provides convenience methods for common use cases:

```go
// Publish heartbeat with system resource information (manual)
heartbeatData := tendrl.HeartbeatData{
    MemFree:  15728640,   // Available RAM in bytes
    MemTotal: 8388608,    // Total RAM in bytes
    DiskFree: 536870912,  // Available filesystem space in bytes
    DiskSize: 1073741824, // Total filesystem size in bytes
}
err := client.PublishHeartbeat(heartbeatData)

// Automatic heartbeats are enabled by default in managed mode
// They send system resource information every 30 seconds automatically
// To disable: configure client with SendHeartbeat: false
// To customize interval: set HeartbeatInterval in config

// Publish sensor data (convenience method)
sensorData := map[string]interface{}{
    "temperature": 23.5,
    "humidity": 60.2,
    "pressure": 1013.25,
}
err := client.PublishSensorData(sensorData, []string{"sensor", "environment"})

// Cross-account messaging (send to another entity)
destination := "123:us-1:entity:target-entity"
err := client.PublishCrossAccount(
    map[string]interface{}{"command": "restart"},
    destination,
    []string{"urgent", "command"},
)
```

### Message Callbacks

```go
// Set up callback to handle incoming messages (managed mode only for auto-checking)
client.SetMessageCallback(func(message tendrl.IncomingMessage) error {
    // Process incoming message
    fmt.Printf("Received: %s from %s\n", message.MsgType, message.Source)
    return nil // Return error if processing fails
})

// Configure checking behavior (optional)
client.SetMessageCheckRate(5 * time.Second)  // Check every 5 seconds (default: 3)
client.SetMessageCheckLimit(10)              // Max messages per check (default: 1)

// Manual message check (works in any mode)
err := client.CheckMessages()
```

### IncomingMessage Structure

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `msg_type` | `string` | Message type identifier (e.g., "command", "notification", "alert") | ✅ Yes |
| `source` | `string` | Sender's resource path (set by server) | ✅ Yes |
| `dest` | `string` | Destination entity identifier | ❌ Optional |
| `timestamp` | `string` | RFC3339 timestamp (set by server) | ✅ Yes |
| `data` | `interface{}` | The actual message payload (can be any JSON type) | ✅ Yes |
| `context` | `IncomingMessageContext` | Message metadata | ❌ Optional |
| `request_id` | `string` | Request identifier (if message was a request) | ❌ Optional |

### Message Context Structure

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `tags` | `[]string` | Message tags for categorization | ❌ Optional |
| `dynamicActions` | `map[string]interface{}` | Server-side validation results | ❌ Optional |

#### How It Works

1. **Background Checking**: In managed mode, the SDK automatically checks for messages every 3 seconds (configurable)
2. **Manual Checking**: You can call `CheckMessages()` manually in any mode
3. **Callback Execution**: Your callback function is called for each incoming message
4. **Error Handling**: Failed callbacks don't stop other message processing
5. **Connectivity Aware**: Automatically handles network failures and updates connectivity state

### Tethering Functions to the Cloud

**Note**: Core background processing (queuing, batching, metrics, retry) starts automatically in managed mode. `Tether` is for **additional** user-defined periodic data collection.

```go
// Only available in managed mode - no-op in direct API mode

// Method 1: Tether a heartbeat function
stopHeartbeat := client.Tether("heartbeat", func() (interface{}, error) {
    return "heartbeat", nil
}, []string{"health"}, 1*time.Minute)
defer stopHeartbeat() // Clean up when done

// Method 2: Tether custom metrics collection
stopMetrics := client.Tether("custom_metrics", func() (interface{}, error) {
    return map[string]interface{}{
        "custom_value": 42.5,
        "app_status": "running",
    }, nil
}, []string{"custom"}, 30*time.Second)
defer stopMetrics()

// Method 3: Tether existing functions
func getAppMetrics() (interface{}, error) {
    return map[string]interface{}{
        "active_users": 150,
        "requests_per_sec": 45.2,
    }, nil
}

stopAppMetrics := client.Tether("app_metrics", getAppMetrics, []string{"application"}, 1*time.Minute)
defer stopAppMetrics()
```

### Flexible Data Types

The SDK works with any JSON-serializable data type:

```go
// Strings are automatically wrapped in {"data": "..."} to match backend expectations
client.Publish("Simple log message", []string{"logs"})
// This sends: {"msg_type": "publish", "data": {"data": "Simple log message"}, ...}

// Any map type works (not just map[string]interface{})
client.Publish(map[string]interface{}{
    "user_id": 12345,
    "action": "login",
}, []string{"user", "events"})

client.Publish(map[string]string{
    "name": "John Doe",
    "email": "john@example.com",
    "city": "New York",
}, []string{"user", "profile"})

client.Publish(map[string]int{
    "age": 30,
    "score": 95,
    "level": 5,
}, []string{"user", "stats"})

client.Publish(map[int]string{
    1: "first",
    2: "second", 
    3: "third",
}, []string{"rankings"}) // Keys become strings in JSON

// Structs get JSON serialized
type Event struct {
    Type string `json:"type"`
    Data string `json:"data"`
}
client.Publish(Event{Type: "error", Data: "Something went wrong"}, []string{"errors"})

// Arrays and slices work
client.Publish([]string{"item1", "item2", "item3"}, []string{"arrays"})
client.Publish([]int{1, 2, 3, 4, 5}, []string{"numbers"})

// Numbers work too
client.Publish(42, []string{"numbers"})
client.Publish(3.14159, []string{"numbers"})
client.Publish(true, []string{"booleans"})

// Complex nested structures
client.Publish([]interface{}{
    "string",
    map[string]interface{}{"key": "value"},
    42,
    true,
    []string{"nested", "array"},
}, []string{"mixed"})

// Maps with different value types
client.Publish(map[string]interface{}{
    "name": "Product A",
    "price": 29.99,
    "in_stock": true,
    "tags": []string{"electronics", "gadget"},
    "metadata": map[string]string{
        "color": "black",
        "size": "medium",
    },
}, []string{"products"})
```

**Important**: The data must be JSON-serializable. Types like functions, channels, or complex pointers won't work:

```go
// ❌ These won't work (not JSON-serializable):
client.Publish(map[string]func(){"callback": myFunc}, []string{"invalid"})
client.Publish(map[string]chan int{"ch": myChan}, []string{"invalid"})

// ✅ But these work perfectly:
client.Publish(map[string][]int{"scores": {95, 87, 92}}, []string{"valid"})
client.Publish(map[int]bool{1: true, 2: false}, []string{"valid"})
```

## System Metrics

```go
// Get current system metrics
metrics := client.GetSystemMetrics()
fmt.Printf("CPU: %.1f%%, Memory: %.1f%%, Queue: %.1f%%\n",
    metrics.CPUUsage, metrics.MemoryUsage, metrics.QueueLoad)

// Get offline storage stats
stats := client.GetOfflineStorageStats()
fmt.Printf("Offline messages: %d, Retry enabled: %v\n",
    stats.MessageCount, stats.RetryEnabled)

// Get connectivity state
connectivity := client.GetConnectivityState()
fmt.Printf("Online: %v, Last Check: %v\n",
    connectivity.Online, connectivity.LastCheck.Format("15:04:05"))
```

## Automatic Heartbeats

In managed mode, the SDK automatically sends heartbeat messages with system resource information:

```go
// Automatic heartbeats are enabled by default in managed mode
// They send every 30 seconds with real system metrics:
// - Memory: Available and total RAM
// - Disk: Available and total filesystem space

// To disable automatic heartbeats:
client, err := tendrl.NewClientWithConfigAndAPIKey("config.json", "api_key")
// In config.json: set "send_heartbeat": false

// To customize heartbeat interval (default: 30 seconds):
// In config.json: set "heartbeat_interval_seconds": 60
```

**Heartbeat Features:**
- ✅ Automatically enabled in managed mode
- ✅ Uses real system metrics (memory and disk)
- ✅ Sends every 30 seconds by default
- ✅ Configurable interval
- ✅ Disabled in headless mode
- ✅ Gracefully handles errors (won't break client if heartbeat fails)

## Offline Storage & Retry

**Offline Retry Flow:**

```sh
Network Down → Store Messages in BoltDB
                        ↓
            Background Retry Process (every 15-30s)
                        ↓
                Network Available? ──No──→ Continue Checking
                        ↓ Yes
                Retrieve Stored Messages
                        ↓
                  Send in Batches
                        ↓
                   Success? ──No──→ Keep for Next Retry
                        ↓ Yes
               Delete from Storage
                        ↓
            Continue Normal Operation
```

## Environment Variables

The SDK supports environment variables for configuration:

| Variable | Description | Default |
|----------|-------------|---------|
| `TENDRL_KEY` | API key for authentication | "" |
| `TENDRL_STORAGE_PATH` | Custom path for offline storage | "./tendrl_storage.db" |
| `TENDRL_DEBUG` | Enable debug logging | false |

**API Key Priority:**

1. **API Key Parameter** (passed to constructor)
2. **Environment Variable** (TENDRL_KEY)

**Examples:**

```bash
# Traditional environment variable method
export TENDRL_KEY="your_api_key_here"

# Custom storage path
export TENDRL_STORAGE_PATH="/custom/path/storage.db"
```

## Error Handling

```go
// Basic error handling
if err := client.Publish(data, tags); err != nil {
    log.Printf("Failed to publish: %v", err)
}

// With retries for critical data
for attempts := 0; attempts < 3; attempts++ {
    if err := client.Publish(criticalData, tags); err == nil {
        break // Success
    }
    time.Sleep(time.Duration(attempts+1) * time.Second)
}
```

## Security Best Practices

```go
// ✅ Good: Use environment variables or parameters for API keys
export TENDRL_KEY=your_api_key_here
client, _ := tendrl.NewClient()  // Reads TENDRL_KEY automatically

// ✅ Good: Pass API key as parameter
client, _ := tendrl.NewClientWithAPIKey("your_api_key_here")

// ❌ Avoid: API keys are NOT stored in config files for security
// Config files only contain non-sensitive settings
```

## Compatibility

- Go 1.16+
- All major operating systems (Linux, macOS, Windows)
- Both amd64 and arm64 architectures

## License

Copyright (c) 2025 tendrl, inc.
All rights reserved. Unauthorized copying, distribution, modification, or usage of this code, via any medium, is strictly prohibited without express permission from the author.
