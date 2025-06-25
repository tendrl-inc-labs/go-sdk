package tendrl

import (
	"net/http"
	"net/url"
	"time"
)

// IsOnline returns true if the client believes it's currently online (only available in managed mode)
func (c *Client) IsOnline() bool {
	if !c.config.Managed || c.connectivity == nil {
		return true // Assume online in headless mode
	}
	c.connectivity.RLock()
	defer c.connectivity.RUnlock()
	return c.connectivity.Online
}

// monitorConnectivity periodically checks network connectivity
func (c *Client) monitorConnectivity() {
	ticker := time.NewTicker(c.config.ConnectivityCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.checkConnectivity()

		case <-c.done:
			return
		}
	}
}

// checkConnectivity performs a lightweight connectivity check
func (c *Client) checkConnectivity() {
	// Try a quick HEAD request to the API
	endpoint, err := url.JoinPath(c.baseURL, "/health")
	if err != nil {
		c.updateConnectivityState(false)
		return
	}

	req, err := http.NewRequest("HEAD", endpoint, nil)
	if err != nil {
		c.updateConnectivityState(false)
		return
	}

	// Use a very short timeout for connectivity checks
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.updateConnectivityState(false)
		return
	}
	defer resp.Body.Close()

	// Consider 2xx and 3xx as "online", anything else as potentially offline
	isOnline := resp.StatusCode >= 200 && resp.StatusCode < 400
	c.updateConnectivityState(isOnline)
}

// updateConnectivityState updates the connectivity state thread-safely
func (c *Client) updateConnectivityState(online bool) {
	if c.connectivity == nil {
		return
	}

	c.connectivity.Lock()
	defer c.connectivity.Unlock()

	now := time.Now()
	wasOnline := c.connectivity.Online

	c.connectivity.LastCheck = now
	c.connectivity.Online = online

	if online && !wasOnline {
		c.connectivity.LastOnline = now
	} else if !online && wasOnline {
		c.connectivity.LastOffline = now
	}
}
