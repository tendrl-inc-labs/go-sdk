package tendrl

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// SetMessageCallback sets a callback function to handle incoming messages
func (c *Client) SetMessageCallback(callback MessageCallback) {
	c.callback = callback
}

// SetMessageCheckRate sets how often to check for messages (only effective in managed mode)
func (c *Client) SetMessageCheckRate(rate time.Duration) {
	c.checkMsgRate = rate
}

// SetMessageCheckLimit sets the maximum number of messages to retrieve per check
func (c *Client) SetMessageCheckLimit(limit int) {
	c.checkMsgLimit = limit
}

// CheckMessages manually checks for incoming messages and calls the callback if set
func (c *Client) CheckMessages() error {
	if c.callback == nil {
		return nil // No callback set, nothing to do
	}

	// Construct the check messages endpoint
	endpoint, err := url.JoinPath(c.baseURL, "/entities/check_messages")
	if err != nil {
		return fmt.Errorf("failed to construct check messages endpoint URL: %w", err)
	}

	// Add query parameter for message limit
	if c.checkMsgLimit > 0 {
		endpoint += fmt.Sprintf("?limit=%d", c.checkMsgLimit)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create check messages request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", BuildUserAgent())

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Update connectivity state if we're in managed mode
		if c.config.Managed && c.connectivity != nil {
			c.updateConnectivityState(false)
		}
		return fmt.Errorf("check messages request failed: %w", err)
	}
	defer resp.Body.Close()

	// 204 means no messages available
	if resp.StatusCode == 204 {
		if c.config.Managed && c.connectivity != nil {
			c.updateConnectivityState(true)
		}
		return nil
	}

	// Check for success status codes
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if c.config.Managed && c.connectivity != nil {
			c.updateConnectivityState(true)
		}

		var response MessageCheckResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return fmt.Errorf("failed to decode check messages response: %w", err)
		}

		// Call callback for each message
		for _, message := range response.Messages {
			if err := c.callback(message); err != nil {
				// Continue processing other messages even if one callback fails
				// You might want to log this error depending on your needs
				continue
			}
		}

		return nil
	}

	return fmt.Errorf("check messages failed with status %d: %s", resp.StatusCode, resp.Status)
}

// startMessageChecking starts a goroutine to periodically check for messages (only in managed mode)
func (c *Client) startMessageChecking() {
	if !c.config.Managed || c.callback == nil {
		return
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(c.checkMsgRate)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if time.Since(c.lastMsgCheck) >= c.checkMsgRate {
					c.CheckMessages() // Ignore errors for background checking
					c.lastMsgCheck = time.Now()
				}

			case <-c.done:
				return
			}
		}
	}()
}
