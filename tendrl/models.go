package tendrl

import (
	"sync"
	"time"
)

// Config holds client configuration
type Config struct {
	// Core configuration
	Managed    bool          // Enable background processing, queuing, and batching
	Timeout    time.Duration // HTTP request timeout
	MaxRetries int           // Max retry attempts for HTTP requests

	// Managed mode configuration (only used when Managed=true)
	MinBatchSize     int
	MaxBatchSize     int
	MaxQueueSize     int
	TargetCPUPercent float64
	TargetMemPercent float64
	MinBatchInterval time.Duration
	MaxBatchInterval time.Duration
	OfflineStorage   bool
	StoragePath      string
	// Offline retry configuration
	OfflineRetryEnabled  bool          // Enable offline message retry
	OfflineRetryInterval time.Duration // How often to check for offline messages
	OfflineRetryLimit    int           // Max retry attempts per message

	// Connectivity monitoring
	ConnectivityCheckEnabled  bool          // Enable background connectivity checks
	ConnectivityCheckInterval time.Duration // How often to check connectivity

	// Heartbeat configuration
	SendHeartbeat     bool          // Enable automatic heartbeat messages (default: true in managed mode)
	HeartbeatInterval time.Duration // Interval between heartbeats (default: 30 seconds)

	// Debug configuration
	Debug bool // Enable debug logging (default: false)
}

// Message represents a message to be sent to Tendrl
// Matches Python SDK Message model format
type Message struct {
	MsgType     string          `json:"msg_type"`            // Message type: "publish", "heartbeat", "cmd", etc.
	Data        interface{}     `json:"data"`                // Message payload
	Context     *MessageContext `json:"context,omitempty"`   // Optional context (tags, wait, etc.)
	Destination string          `json:"dest,omitempty"`      // Destination entity for cross-account messaging
	Timestamp   string          `json:"timestamp,omitempty"` // ISO8601 timestamp (set automatically if empty)
}

// MessageContext contains message metadata
// Matches Python SDK Context model
type MessageContext struct {
	Tags         []string `json:"tags,omitempty"`    // Tags for flow triggering
	WaitResponse bool     `json:"wait,omitempty"`    // Whether to wait for response
	Timeout      int      `json:"timeout,omitempty"` // Response timeout in seconds
	// Note: Entity field removed - use dest field in Message for cross-account messaging
}

// BatchResponse represents the response from a batch message send
type BatchResponse struct {
	Success    bool     `json:"success"`
	MessageIDs []string `json:"message_ids,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// MessageResponse represents the response from a single message send
type MessageResponse struct {
	Success   bool   `json:"success"`
	MessageID string `json:"message_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

// SystemMetrics tracks system performance for dynamic batching
type SystemMetrics struct {
	sync.RWMutex
	CPUUsage    float64
	MemoryUsage float64
	QueueLoad   float64
}

// OfflineStorageStats represents offline storage statistics
type OfflineStorageStats struct {
	Enabled        bool `json:"enabled"`
	MessageCount   int  `json:"message_count"`
	RetryEnabled   bool `json:"retry_enabled"`
	BatchSize      int  `json:"batch_size"`       // Max messages processed per retry cycle
	MaxBatchCycles int  `json:"max_batch_cycles"` // Max batches processed per retry interval
}

// ConnectivityState represents the current network connectivity state
type ConnectivityState struct {
	sync.RWMutex
	Online       bool      `json:"online"`
	LastCheck    time.Time `json:"last_check"`
	LastOnline   time.Time `json:"last_online"`
	LastOffline  time.Time `json:"last_offline"`
	CheckEnabled bool      `json:"check_enabled"`
}

// DataFunc is a function that returns data to publish
type DataFunc func() (interface{}, error)

// IncomingMessage represents a message received from the server
type IncomingMessage struct {
	MsgType   string                 `json:"msg_type"`
	Source    string                 `json:"source"`
	Dest      string                 `json:"dest,omitempty"`
	Timestamp string                 `json:"timestamp"`
	Data      interface{}            `json:"data"`
	Context   IncomingMessageContext `json:"context,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}

// HeartbeatData represents system resource information for heartbeat messages
type HeartbeatData struct {
	MemFree  float64 `json:"mem_free"`  // Available RAM in bytes
	MemTotal float64 `json:"mem_total"` // Total RAM in bytes
	DiskFree float64 `json:"disk_free"` // Available filesystem space in bytes
	DiskSize float64 `json:"disk_size"` // Total filesystem size in bytes
}

// IncomingMessageContext contains metadata for incoming messages
type IncomingMessageContext struct {
	Tags           []string               `json:"tags,omitempty"`
	DynamicActions map[string]interface{} `json:"dynamicActions,omitempty"`
}

// MessageCheckResponse represents the response from the message check endpoint
type MessageCheckResponse struct {
	Messages []IncomingMessage `json:"messages,omitempty"`
}

// MessageCallback is a function type for handling incoming messages
type MessageCallback func(IncomingMessage) error
