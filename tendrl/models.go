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
}

// Message represents a message to be sent to Tendrl
type Message struct {
	Data        interface{}    `json:"data,omitempty"`
	Context     MessageContext `json:"context,omitempty"`
	MsgType     string         `json:"msg_type,omitempty"`
	Destination string         `json:"dest,omitempty"`
	Timestamp   string         `json:"timestamp,omitempty"`
}

// MessageContext contains message metadata
type MessageContext struct {
	Tags         []string `json:"tags,omitempty"`
	Entity       string   `json:"entity,omitempty"`
	WaitResponse bool     `json:"wait,omitempty"`
	Timeout      int      `json:"timeout,omitempty"`
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
