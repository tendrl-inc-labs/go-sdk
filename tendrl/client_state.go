package tendrl

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// StateCallback handles state table changes detected by polling.
type StateCallback func(map[string]interface{}) error

func stateSnapshot(state map[string]interface{}) string {
	data, err := json.Marshal(state)
	if err != nil {
		return ""
	}
	return string(data)
}

func (c *Client) hasStateHandlers() bool {
	return c.stateHandler != nil || c.stateCallback != nil
}

func (c *Client) hasInboundHandlers() bool {
	return c.hasMessageHandlers() || c.hasStateHandlers()
}

func (c *Client) dispatchState(state map[string]interface{}) error {
	if c.stateHandler != nil {
		return c.stateHandler(state)
	}
	if c.stateCallback != nil {
		return c.stateCallback(state)
	}
	return nil
}

// OnState registers a handler for remote state table changes (polled).
func (c *Client) OnState(handler StateCallback) {
	c.stateHandler = handler
}

// SetStateCallback sets a catch-all state handler when OnState is not used.
func (c *Client) SetStateCallback(callback StateCallback) {
	c.stateCallback = callback
}

// CheckState polls the state table and invokes handlers when it changes.
func (c *Client) CheckState() error {
	if !c.hasStateHandlers() {
		return nil
	}

	endpoint, err := url.JoinPath(c.baseURL, "/entities/status-table")
	if err != nil {
		return fmt.Errorf("failed to construct status-table endpoint URL: %w", err)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create status-table request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", BuildUserAgent())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("status-table request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status-table failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read status-table response: %w", err)
	}

	var payload struct {
		StatusTable map[string]interface{} `json:"statusTable"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to decode status-table response: %w", err)
	}

	state := payload.StatusTable
	if state == nil {
		state = map[string]interface{}{}
	}

	if c.lastStateInit {
		if stateSnapshot(state) != stateSnapshot(c.lastState) {
			if err := c.dispatchState(state); err != nil {
				return err
			}
		}
	}

	c.lastState = cloneStateMap(state)
	c.lastStateInit = true
	return nil
}

func cloneStateMap(state map[string]interface{}) map[string]interface{} {
	if state == nil {
		return map[string]interface{}{}
	}
	data, err := json.Marshal(state)
	if err != nil {
		return state
	}
	var copy map[string]interface{}
	if err := json.Unmarshal(data, &copy); err != nil {
		return state
	}
	return copy
}
