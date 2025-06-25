package tendrl

import (
	"time"
)

// processOfflineRetries periodically checks for stored messages and retries sending them
func (c *Client) processOfflineRetries() {
	ticker := time.NewTicker(c.config.OfflineRetryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if c.storage != nil {
				c.retryOfflineMessages()
			}

		case <-c.done:
			return
		}
	}
}

// retryOfflineMessages attempts to send stored offline messages in manageable batches
func (c *Client) retryOfflineMessages() {
	// Skip retry if we know we're offline
	if c.connectivity != nil && !c.IsOnline() {
		return
	}

	const maxBatchSize = 50   // Process at most 50 messages per retry cycle
	const maxRetryBatches = 5 // Process at most 5 batches per retry cycle to avoid blocking

	batchesProcessed := 0

	for batchesProcessed < maxRetryBatches {
		// Get a limited batch of messages
		storedMessages, err := c.storage.GetBatch(maxBatchSize)
		if err != nil {
			return // Silently continue, will retry on next interval
		}

		if len(storedMessages) == 0 {
			return // No more messages to retry
		}

		var batch []Message
		var idsToDelete []string
		var expiredIds []string
		now := time.Now().Unix()

		// Process this batch of stored messages
		for id, storedMsg := range storedMessages {
			// Check if message has expired
			if now >= storedMsg.Timestamp+storedMsg.TTL {
				expiredIds = append(expiredIds, id)
				continue
			}

			// Convert stored message back to Message format
			msg := Message{
				Data: storedMsg.Data,
				Context: MessageContext{
					Tags: storedMsg.Tags,
				},
			}
			batch = append(batch, msg)
			idsToDelete = append(idsToDelete, id)
		}

		// Delete expired messages immediately
		if len(expiredIds) > 0 {
			c.storage.DeleteBatch(expiredIds)
		}

		// Send valid messages if any
		if len(batch) > 0 {
			if c.sendOfflineBatch(batch) {
				// Success - delete these messages in batch
				c.storage.DeleteBatch(idsToDelete)
			} else {
				// Failed to send - keep messages for next retry
				// Add small delay to avoid rapid retry loops
				time.Sleep(1 * time.Second)
				return
			}
		}

		batchesProcessed++

		// Small delay between batches to avoid overwhelming the server
		if batchesProcessed < maxRetryBatches {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// sendOfflineBatch attempts to send a batch of offline messages
func (c *Client) sendOfflineBatch(batch []Message) bool {
	// Use a shorter timeout for offline retries to fail fast
	originalTimeout := c.httpClient.Timeout
	c.httpClient.Timeout = 5 * time.Second
	defer func() {
		c.httpClient.Timeout = originalTimeout
	}()

	// Try with fewer retries for offline messages
	originalMaxRetries := c.config.MaxRetries
	c.config.MaxRetries = 1
	defer func() {
		c.config.MaxRetries = originalMaxRetries
	}()

	_, err := c.sendMessages(batch, false)
	return err == nil
}
