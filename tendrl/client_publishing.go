package tendrl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// prepareData formats data appropriately - matches Python SDK behavior
// Backend expects data to be a dict/object, not a string
// Wrap string data in a map to match backend expectations (similar to Python SDK's make_message)
func (c *Client) prepareData(data interface{}) (interface{}, error) {
	if data == nil {
		return nil, nil
	}

	// If it's already a string, wrap it in a map like Python SDK does
	if str, ok := data.(string); ok {
		return map[string]interface{}{
			"data": str,
		}, nil
	}

	// For all other types, return as-is and let JSON marshal handle it
	return data, nil
}

// dataAsString converts data to string for storage (always JSON for storage consistency)
func (c *Client) dataAsString(data interface{}) (string, error) {
	if data == nil {
		return "", nil
	}

	// Even if it's a string, JSON marshal it for storage consistency
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data for storage: %w", err)
	}
	return string(jsonData), nil
}

// Publish sends a message with optional response waiting (matches Python SDK API)
func (c *Client) Publish(data interface{}, tags []string, entity string, waitResponse bool, timeout int) (string, error) {
	c.debugLog("Publishing message (tags=%v, entity=%s, waitResponse=%v)", tags, entity, waitResponse)
	// Prepare data for message format
	preparedData, err := c.prepareData(data)
	if err != nil {
		return "", fmt.Errorf("failed to prepare data: %w", err)
	}

	// Create context only if tags or wait_response are set
	var context *MessageContext
	if len(tags) > 0 || waitResponse {
		context = &MessageContext{
			Tags:         tags,
			WaitResponse: waitResponse,
			Timeout:      timeout,
		}
	}

	msg := Message{
		MsgType:     "publish", // Default message type
		Data:        preparedData,
		Context:     context,
		Destination: entity, // Use dest field for cross-account messaging
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
	}

	// In headless mode or when wait_response=true, send immediately
	if !c.config.Managed || waitResponse {
		return c.sendMessages([]Message{msg}, waitResponse)
	}

	// In managed mode, check if we're offline and should store immediately
	if c.connectivity != nil && !c.IsOnline() && c.config.OfflineStorage && c.storage != nil {
		// We're offline - store message immediately instead of queuing
		dataStr, err := c.dataAsString(data)
		if err != nil {
			return "", fmt.Errorf("failed to convert data for offline storage: %w", err)
		}
		err = c.storage.Store(
			fmt.Sprintf("msg_%d", time.Now().UnixNano()),
			dataStr,
			tags,
			3600, // 1 hour TTL
		)
		return "", err
	}

	// In managed mode with async requests, use the queue
	select {
	case c.queue <- msg:
		return "", nil
	default:
		if c.config.OfflineStorage && c.storage != nil {
			dataStr, err := c.dataAsString(data)
			if err != nil {
				return "", fmt.Errorf("failed to convert data for storage: %w", err)
			}
			err = c.storage.Store(
				fmt.Sprintf("msg_%d", time.Now().UnixNano()),
				dataStr, // Use JSON-encoded string for storage
				tags,
				3600, // 1 hour TTL
			)
			return "", err
		}
		return "", fmt.Errorf("queue is full and offline storage not enabled")
	}
}

// PublishAsync sends a message asynchronously (fire-and-forget)
func (c *Client) PublishAsync(data interface{}, tags []string) error {
	c.debugLog("Publishing message asynchronously (tags=%v)", tags)
	_, err := c.Publish(data, tags, "", false, 5)
	return err
}

// sendMessages sends messages (single or batch) and optionally returns message ID
func (c *Client) sendMessages(messages []Message, waitResponse bool) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	c.debugLog("Sending %d message(s) (waitResponse=%v)", len(messages), waitResponse)

	// Determine payload format based on count and wait_response
	var jsonData []byte
	var err error

	if len(messages) == 1 && waitResponse {
		// Single message for synchronous response
		jsonData, err = json.Marshal(messages[0])
	} else {
		// Batch format (array of messages)
		jsonData, err = json.Marshal(messages)
	}

	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request - use different endpoints for single vs batch
	var endpoint string
	if len(messages) == 1 && waitResponse {
		// Single message with response uses /entities/message endpoint
		endpoint, err = url.JoinPath(c.baseURL, "/entities/message")
	} else {
		// Batch messages use /entities/messages endpoint
		endpoint, err = url.JoinPath(c.baseURL, "/entities/messages")
	}
	if err != nil {
		return "", fmt.Errorf("failed to construct endpoint URL: %w", err)
	}
	c.debugLog("Sending request to: %s", endpoint)

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", BuildUserAgent())

	// Execute request with retries
	var lastErr error
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Network error - likely offline
			if c.config.Managed && c.connectivity != nil {
				c.updateConnectivityState(false)
			}
			lastErr = fmt.Errorf("HTTP request failed (attempt %d): %w", attempt+1, err)
			if attempt < c.config.MaxRetries {
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			break
		}
		defer resp.Body.Close()

		// Check for success status codes
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			c.debugLog("Message(s) sent successfully (status=%d)", resp.StatusCode)
			// Success - we're online
			if c.config.Managed && c.connectivity != nil {
				c.updateConnectivityState(true)
			}
			if waitResponse && len(messages) == 1 {
				// Parse single message response for message ID
				var messageResp MessageResponse
				if err := json.NewDecoder(resp.Body).Decode(&messageResp); err == nil && messageResp.MessageID != "" {
					return messageResp.MessageID, nil
				}
				// Fallback if response doesn't have message_id field
				return fmt.Sprintf("msg_%d", time.Now().UnixNano()), nil
			}
			return "", nil // Success for batch/async
		}

		// Handle error response - try both response types
		var errorMsg string
		if len(messages) == 1 && waitResponse {
			var errorResp MessageResponse
			if err := json.NewDecoder(resp.Body).Decode(&errorResp); err == nil && errorResp.Error != "" {
				errorMsg = errorResp.Error
			}
		} else {
			var errorResp BatchResponse
			if err := json.NewDecoder(resp.Body).Decode(&errorResp); err == nil && errorResp.Error != "" {
				errorMsg = errorResp.Error
			}
		}

		if errorMsg != "" {
			lastErr = fmt.Errorf("API error (attempt %d): %s", attempt+1, errorMsg)
		} else {
			lastErr = fmt.Errorf("HTTP error (attempt %d): %d %s", attempt+1, resp.StatusCode, resp.Status)
		}

		// Retry on server errors (5xx) or rate limiting (429)
		if (resp.StatusCode >= 500 || resp.StatusCode == 429) && attempt < c.config.MaxRetries {
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		break // Don't retry on client errors (4xx)
	}

	return "", lastErr
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
