package tendrl

import (
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

// updateMetrics updates system metrics for dynamic batching
func (c *Client) updateMetrics() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cpuPercent, _ := cpu.Percent(100*time.Millisecond, false)
			memStats, _ := mem.VirtualMemory()

			c.metrics.Lock()
			if len(cpuPercent) > 0 {
				c.metrics.CPUUsage = cpuPercent[0]
			}
			c.metrics.MemoryUsage = memStats.UsedPercent
			c.metrics.QueueLoad = float64(len(c.queue)) / float64(c.config.MaxQueueSize) * 100
			c.metrics.Unlock()

		case <-c.done:
			return
		}
	}
}

// calculateDynamicBatchSize calculates batch size based on system performance
func (c *Client) calculateDynamicBatchSize() int {
	c.metrics.RLock()
	defer c.metrics.RUnlock()

	cpuFactor := max(0.0, 1-(c.metrics.CPUUsage/c.config.TargetCPUPercent))
	memFactor := max(0.0, 1-(c.metrics.MemoryUsage/c.config.TargetMemPercent))
	queueFactor := min(1.0, c.metrics.QueueLoad/50)

	resourceFactor := (cpuFactor*0.4 + memFactor*0.4 + queueFactor*0.2)
	batchSize := int(float64(c.config.MaxBatchSize) * resourceFactor)

	return max(c.config.MinBatchSize, min(batchSize, c.config.MaxBatchSize))
}

// processQueue processes messages from the queue in dynamic batches
func (c *Client) processQueue() {
	batch := make([]Message, 0, c.config.MaxBatchSize)
	ticker := time.NewTicker(c.config.MinBatchInterval)
	defer ticker.Stop()

	for {
		select {
		case msg := <-c.queue:
			batch = append(batch, msg)

			if len(batch) >= c.calculateDynamicBatchSize() {
				if _, err := c.sendMessages(batch, false); err != nil && c.storage != nil {
					// Store failed messages
					for _, m := range batch {
						if dataStr, err := c.dataAsString(m.Data); err == nil {
							c.storage.Store(
								time.Now().Format(time.RFC3339),
								dataStr,
								m.Context.Tags,
								3600,
							)
						}
					}
				}
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				if _, err := c.sendMessages(batch, false); err != nil && c.storage != nil {
					// Store failed messages
					for _, m := range batch {
						if dataStr, err := c.dataAsString(m.Data); err == nil {
							c.storage.Store(
								time.Now().Format(time.RFC3339),
								dataStr,
								m.Context.Tags,
								3600,
							)
						}
					}
				}
				batch = batch[:0]
			}

			if c.storage != nil {
				c.storage.CleanupExpired()
			}

		case <-c.done:
			if len(batch) > 0 {
				c.sendMessages(batch, false)
			}
			return
		}
	}
}
