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
	c.debugLog("Checking connectivity")
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

	// Set headers (consistent with other requests)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", BuildUserAgent())

	// Use a very short timeout for connectivity checks
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.updateConnectivityState(false)
		return
	}
	defer resp.Body.Close()

	// ANY HTTP response proves we reached the server — that is what
	// "connectivity" means here, not authorization or a specific route
	// existing. Requiring 2xx/3xx meant a 404 (e.g. this build has no
	// /api/health) or a 401 pinned IsOnline() to false forever, which silently
	// disabled the offline-retry drain even with the server fully reachable.
	// Only a transport error (handled above) counts as offline.
	c.updateConnectivityState(true)
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
		c.debugLog("Connectivity changed: offline -> online")
		c.connectivity.LastOnline = now
	} else if !online && wasOnline {
		c.debugLog("Connectivity changed: online -> offline")
		c.connectivity.LastOffline = now
	}
}
