package tendrl

import (
	"fmt"
	"os"
	"time"

	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

// HeartbeatData represents system resource information for heartbeat messages
type HeartbeatData struct {
	MemFree  float64 `json:"mem_free"`  // Available RAM in bytes
	MemTotal float64 `json:"mem_total"` // Total RAM in bytes
	DiskFree float64 `json:"disk_free"` // Available filesystem space in bytes
	DiskSize float64 `json:"disk_size"` // Total filesystem size in bytes
}

// PublishHeartbeat sends a heartbeat message with system resource information
// This matches the Python SDK's heartbeat functionality
func (c *Client) PublishHeartbeat(data HeartbeatData) error {
	c.debugLog("Publishing heartbeat")
	msg := Message{
		MsgType:   "heartbeat",
		Data:      data,
		Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05Z"), // ISO8601 with Z suffix
	}

	// Send immediately (heartbeats should not be queued)
	if !c.config.Managed {
		_, err := c.sendMessages([]Message{msg}, false)
		return err
	}

	// In managed mode, send directly (bypass queue for immediate delivery)
	_, err := c.sendMessages([]Message{msg}, false)
	return err
}

// PublishSensorData publishes sensor data with optional tags
// This is a convenience method matching Python SDK's publish_sensor_data
func (c *Client) PublishSensorData(sensorData interface{}, tags []string) error {
	_, err := c.Publish(sensorData, tags, "", false, 5)
	return err
}

// PublishCrossAccount sends a message to another entity (cross-account messaging)
// destination should be in format: "account:region:type:name"
// This matches Python SDK's publish_cross_account_message
func (c *Client) PublishCrossAccount(data interface{}, destination string, tags []string) error {
	c.debugLog("Publishing cross-account message (destination=%s, tags=%v)", destination, tags)
	msg := Message{
		MsgType:     "cmd", // Cross-account messages use "cmd" type
		Data:        data,
		Destination: destination,
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
	}

	// Add context if tags are provided
	if len(tags) > 0 {
		msg.Context = &MessageContext{
			Tags: tags,
		}
	}

	// Send immediately for cross-account messages
	if !c.config.Managed {
		_, err := c.sendMessages([]Message{msg}, false)
		return err
	}

	// In managed mode, send directly (bypass queue for immediate delivery)
	_, err := c.sendMessages([]Message{msg}, false)
	return err
}

// getSystemResources returns system resource information for heartbeat messages
// Uses gopsutil (already imported) to get real system metrics
// Matches Python SDK's get_system_resources function
func getSystemResources() (HeartbeatData, error) {
	// Get memory information
	memStats, err := mem.VirtualMemory()
	if err != nil {
		return HeartbeatData{}, fmt.Errorf("failed to get memory stats: %w", err)
	}

	// Get disk information for root filesystem
	// Try root first, fallback to current directory
	var diskFree, diskSize uint64
	diskStats, err := disk.Usage("/")
	if err != nil {
		// Fallback: try current working directory
		if cwd, err := os.Getwd(); err == nil {
			if diskStats, err = disk.Usage(cwd); err == nil {
				diskFree = diskStats.Free
				diskSize = diskStats.Total
			}
		}
		// If both fail, use zero values (will be handled gracefully)
	} else {
		diskFree = diskStats.Free
		diskSize = diskStats.Total
	}

	return HeartbeatData{
		MemFree:  float64(memStats.Available),
		MemTotal: float64(memStats.Total),
		DiskFree: float64(diskFree),
		DiskSize: float64(diskSize),
	}, nil
}
