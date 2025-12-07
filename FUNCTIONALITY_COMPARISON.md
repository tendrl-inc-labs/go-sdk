# Go SDK vs Python SDK Functionality Comparison

This document compares the functionality of the Go SDK and Python SDK to ensure feature parity.

## Core Features

| Feature | Python SDK | Go SDK | Status |
|---------|-----------|--------|--------|
| **HTTP/HTTP2 API Communication** | ✅ Yes (httpx with http2=True) | ✅ Yes (http.Client with HTTP2) | ✅ Match |
| **Message Publishing** | ✅ Yes | ✅ Yes | ✅ Match |
| **Message Callbacks** | ✅ Yes | ✅ Yes | ✅ Match |
| **Offline Storage** | ✅ Yes (SQLite) | ✅ Yes (BoltDB) | ✅ Match |
| **Automatic Retry** | ✅ Yes | ✅ Yes | ✅ Match |
| **Dynamic Batching** | ✅ Yes | ✅ Yes | ✅ Match |
| **System Resource Monitoring** | ✅ Yes | ✅ Yes | ✅ Match |
| **Automatic Heartbeats** | ✅ Yes | ✅ Yes | ✅ Match |
| **Connectivity Monitoring** | ✅ Yes | ✅ Yes | ✅ Match |
| **Headless Mode** | ✅ Yes | ✅ Yes | ✅ Match |
| **Debug Mode** | ✅ Yes | ✅ Yes | ✅ Match |

## API Methods

### Message Publishing

| Method | Python SDK | Go SDK | Notes |
|--------|-----------|--------|-------|
| `publish(msg, tags, entity, wait_response, timeout)` | ✅ | ✅ `Publish(data, tags, entity, waitResponse, timeout)` | ✅ Match |
| `publish()` async (fire-and-forget) | ✅ | ✅ `PublishAsync(data, tags)` | ✅ Match |
| String data wrapping | ✅ Wraps in `{"data": "..."}` | ✅ Wraps in `{"data": "..."}` | ✅ Match |
| Automatic timestamp | ✅ Yes | ✅ Yes | ✅ Match |
| Message type default | ✅ "publish" | ✅ "publish" | ✅ Match |

### Helper Methods

| Method | Python SDK | Go SDK | Status |
|--------|-----------|--------|--------|
| `publish_heartbeat()` | ✅ | ✅ `PublishHeartbeat(data)` | ✅ Match |
| `publish_sensor_data()` | ✅ | ✅ `PublishSensorData(data, tags)` | ✅ Match |
| `publish_cross_account_message()` | ✅ | ✅ `PublishCrossAccount(data, destination, tags)` | ✅ Match |

### Message Receiving

| Method | Python SDK | Go SDK | Status |
|--------|-----------|--------|--------|
| `check_msg()` / `check_messages()` | ✅ | ✅ `CheckMessages()` | ✅ Match |
| `set_message_callback()` | ✅ | ✅ `SetMessageCallback()` | ✅ Match |
| `set_message_check_rate()` | ✅ | ✅ `SetMessageCheckRate()` | ✅ Match |
| `set_message_check_limit()` | ✅ | ✅ `SetMessageCheckLimit()` | ✅ Match |
| Background message checking | ✅ Yes | ✅ Yes | ✅ Match |

### Tethering

| Method | Python SDK | Go SDK | Status |
|--------|-----------|--------|--------|
| `@client.tether()` decorator | ✅ | ✅ `Tether(name, fn, tags, interval)` | ✅ Match |
| Periodic data collection | ✅ Yes | ✅ Yes | ✅ Match |
| Returns stop function | ✅ No (decorator) | ✅ Yes | ⚠️ Different API style |

## Configuration Options

### Core Settings

| Option | Python SDK | Go SDK | Default (Both) |
|--------|-----------|--------|----------------|
| `mode` / `Managed` | ✅ "api" or "agent" | ✅ `true` (managed) or `false` (headless) | Python: "api", Go: `true` |
| `api_key` | ✅ | ✅ | Both: env var or parameter |
| `headless` | ✅ | ✅ (via `Managed=false`) | Both: `false` |
| `debug` | ✅ | ✅ | Both: `false` |

### Performance & Batching

| Option | Python SDK | Go SDK | Default (Both) |
|--------|-----------|--------|----------------|
| `target_cpu_percent` | ✅ | ✅ | Both: 65.0 / 70.0 |
| `target_mem_percent` | ✅ | ✅ | Both: 75.0 / 80.0 |
| `min_batch_size` | ✅ | ✅ | Python: 10, Go: 10 |
| `max_batch_size` | ✅ | ✅ | Python: 100, Go: 500 |
| `min_batch_interval` | ✅ | ✅ | Both: 0.1s / 100ms |
| `max_batch_interval` | ✅ | ✅ | Both: 1.0s |
| `max_queue_size` | ✅ | ✅ | Both: 1000 |

### Offline Storage

| Option | Python SDK | Go SDK | Default (Both) |
|--------|-----------|--------|----------------|
| `offline_storage` | ✅ | ✅ | Both: `false` |
| `db_path` | ✅ | ✅ `StoragePath` | Python: "tendrl_offline.db", Go: "tendrl_storage.db" |
| `offline_retry_enabled` | ✅ | ✅ | Both: `true` |
| `offline_retry_interval` | ✅ | ✅ | Both: 30s |
| `offline_retry_limit` | ✅ | ✅ | Both: 5 |

### Message Checking

| Option | Python SDK | Go SDK | Default (Both) |
|--------|-----------|--------|----------------|
| `check_msg_rate` | ✅ | ✅ `checkMsgRate` | Both: 3s |
| `check_msg_limit` | ✅ | ✅ `checkMsgLimit` | Both: 1 |

### Heartbeat

| Option | Python SDK | Go SDK | Default (Both) |
|--------|-----------|--------|----------------|
| `send_heartbeat` | ✅ | ✅ `SendHeartbeat` | Both: `true` (managed mode) |
| `heartbeat_interval` | ✅ | ✅ `HeartbeatInterval` | Both: 30s |

### Connectivity

| Option | Python SDK | Go SDK | Default (Both) |
|--------|-----------|--------|----------------|
| Connectivity checking | ✅ Built-in | ✅ `ConnectivityCheckEnabled` | Both: `true` |
| Connectivity interval | ✅ Built-in | ✅ `ConnectivityCheckInterval` | Both: 30s |

## API Endpoints

| Endpoint | Python SDK | Go SDK | Status |
|----------|-----------|--------|--------|
| Single message | ✅ `POST /entities/message` | ✅ `POST /entities/message` | ✅ Match |
| Batch messages | ✅ `POST /entities/messages` | ✅ `POST /entities/messages` | ✅ Match |
| Check messages | ✅ `GET /entities/check_messages` | ✅ `GET /entities/check_messages` | ✅ Match |
| Claims validation | ✅ `GET /api/claims` | ✅ `GET /api/claims` | ✅ Match |

## Message Format

| Field | Python SDK | Go SDK | Status |
|-------|-----------|--------|--------|
| `msg_type` | ✅ | ✅ `MsgType` | ✅ Match |
| `data` | ✅ | ✅ `Data` | ✅ Match |
| `context.tags` | ✅ | ✅ `Context.Tags` | ✅ Match |
| `context.wait` | ✅ | ✅ `Context.WaitResponse` | ✅ Match |
| `dest` | ✅ | ✅ `Destination` | ✅ Match |
| `timestamp` | ✅ | ✅ `Timestamp` | ✅ Match |

## System Resources

| Feature | Python SDK | Go SDK | Status |
|---------|-----------|--------|--------|
| Get system resources | ✅ `get_system_resources()` | ✅ `getSystemResources()` | ✅ Match |
| Memory info (mem_free, mem_total) | ✅ Yes | ✅ Yes | ✅ Match |
| Disk info (disk_free, disk_size) | ✅ Yes | ✅ Yes | ✅ Match |
| Uses psutil | ✅ Yes | ✅ Uses gopsutil | ✅ Match |

## Operating Modes

| Mode | Python SDK | Go SDK | Status |
|------|-----------|--------|--------|
| **API Mode** (Direct HTTP) | ✅ `mode="api"` | ✅ `Managed=false` | ✅ Match |
| **Managed Mode** (Background processing) | ✅ `mode="api"` + `headless=False` | ✅ `Managed=true` | ✅ Match |
| **Headless Mode** (Synchronous) | ✅ `headless=True` | ✅ `Managed=false` | ✅ Match |
| **Agent Mode** (Unix socket) | ✅ `mode="agent"` | ❌ Not supported | ⚠️ Go SDK doesn't support agent mode |

## Key Differences

### 1. Agent Mode
- **Python SDK**: Supports both API mode (direct HTTP) and Agent mode (Unix socket to Go agent)
- **Go SDK**: Only supports direct HTTP/HTTP2 API mode
- **Reason**: Go SDK is designed to be the agent itself or communicate directly

### 2. Tether API Style
- **Python SDK**: Uses decorator `@client.tether()`
- **Go SDK**: Uses function `client.Tether(name, fn, tags, interval)` returning stop function
- **Reason**: Go doesn't have decorators, uses function-based approach

### 3. Configuration
- **Python SDK**: Constructor parameters
- **Go SDK**: Config file + constructor parameters
- **Both**: Support environment variables

### 4. Debug Mode
- **Python SDK**: Has `debug` parameter in configuration
- **Go SDK**: Has `debug` parameter in configuration (via config file)
- **Status**: ✅ Both SDKs now support debug mode
- **Implementation**: Both log detailed SDK operations when enabled

## Feature Parity Summary

✅ **Fully Matched Features:**
- Message publishing (sync and async)
- Message callbacks and checking
- Offline storage and retry
- Dynamic batching
- System resource monitoring
- Automatic heartbeats
- Connectivity monitoring
- Helper methods (heartbeat, sensor data, cross-account)
- String data wrapping
- Message format
- API endpoints
- Debug mode

⚠️ **Partial Differences:**
- Agent mode (Python only - by design)
- Tether API style (different but equivalent functionality)

❌ **Missing in Go SDK:**
- Agent mode (Unix socket communication) - **Not needed** (Go SDK is the agent)

## Conclusion

The Go SDK has **full feature parity** with the Python SDK for all core functionality. The only differences are:

1. **Agent mode** - Not applicable to Go SDK (it would be the agent itself)
2. **API style differences** - Due to language differences (decorators vs functions)

All essential features for communicating with the Tendrl backend are present and working in both SDKs, including:
- ✅ Debug mode (now available in both SDKs)
- ✅ All message publishing and receiving features
- ✅ Offline storage and retry mechanisms
- ✅ System resource monitoring and automatic heartbeats
- ✅ Dynamic batching and connectivity monitoring
