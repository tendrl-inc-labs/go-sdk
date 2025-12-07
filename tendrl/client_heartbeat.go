package tendrl

import (
	"fmt"
	"os"
	"time"

	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

// sendHeartbeat sends a heartbeat message with system resource information
// This is called automatically by the background queue processor
func (c *Client) sendHeartbeat() error {
	c.debugLog("Sending heartbeat")
	// Get system resource information
	resources, err := getSystemResources()
	if err != nil {
		c.debugLog("Failed to get system resources: %v", err)
		// If we can't get system resources, send heartbeat with zero values
		// This is better than not sending a heartbeat at all
		resources = HeartbeatData{
			MemFree:  0,
			MemTotal: 0,
			DiskFree: 0,
			DiskSize: 0,
		}
	} else {
		c.debugLog("Heartbeat data: mem_free=%d, mem_total=%d, disk_free=%d, disk_size=%d",
			resources.MemFree, resources.MemTotal, resources.DiskFree, resources.DiskSize)
	}

	// Send heartbeat using the helper method
	return c.PublishHeartbeat(resources)
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
