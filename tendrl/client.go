package tendrl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

var version = "0.1.0"

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	config     *Config

	// Managed mode components (only initialized if config.Managed=true)
	queue        chan Message
	metrics      *SystemMetrics
	storage      *Storage
	connectivity *ConnectivityState
	wg           sync.WaitGroup
	done         chan struct{}
}

// NewClient creates a new Tendrl client
// managed: true for managed mode (full features: queuing, batching, offline storage), false for headless mode (direct API calls)
// apiKey: API key for authentication (empty string to use TENDRL_KEY environment variable)
func NewClient(managed bool, apiKey string) (*Client, error) {
	return NewClientWithModeAndAPIKey(managed, apiKey)
}

// NewClientWithModeAndAPIKey creates a new Tendrl client with the specified mode and API key
func NewClientWithModeAndAPIKey(managed bool, apiKey string) (*Client, error) {
	// Load configuration from file if available
	configFile, err := LoadConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	// Convert to internal config structure with defaults
	config := configFile.toConfig()

	// Override managed mode if explicitly specified
	config.Managed = managed

	// Disable managed-mode features
	if !config.Managed {
		config.OfflineStorage = false
		config.OfflineRetryEnabled = false
		config.ConnectivityCheckEnabled = false
	}

	client := &Client{
		config: config,
		apiKey: apiKey, // Set API key if provided
	}

	// Initialize managed mode components only if needed
	if managed {
		client.metrics = &SystemMetrics{}
		client.done = make(chan struct{})
	}

	// Initialize HTTP client with timeout from config
	client.httpClient = &http.Client{
		Timeout: client.config.Timeout,
	}

	// Set up API configuration
	if err := client.setupAPIConfig(); err != nil {
		return nil, err
	}

	// Initialize managed mode components
	if client.config.Managed {
		// Initialize connectivity state
		client.connectivity = &ConnectivityState{
			Online:       true, // Assume online initially
			LastCheck:    time.Now(),
			CheckEnabled: client.config.ConnectivityCheckEnabled,
		}

		// Initialize offline storage if enabled
		if client.config.OfflineStorage {
			storage, err := NewStorage(client.config.StoragePath)
			if err != nil {
				return nil, err
			}
			client.storage = storage
		}

		client.queue = make(chan Message, client.config.MaxQueueSize)

		// Start background goroutines
		goroutineCount := 2
		if client.config.OfflineStorage && client.config.OfflineRetryEnabled {
			goroutineCount++ // Add offline retry goroutine
		}
		if client.config.ConnectivityCheckEnabled {
			goroutineCount++ // Add connectivity check goroutine
		}

		client.wg.Add(goroutineCount)
		go func() {
			defer client.wg.Done()
			client.processQueue()
		}()
		go func() {
			defer client.wg.Done()
			client.updateMetrics()
		}()

		// Start offline retry goroutine if enabled
		if client.config.OfflineStorage && client.config.OfflineRetryEnabled {
			go func() {
				defer client.wg.Done()
				client.processOfflineRetries()
			}()
		}

		// Start connectivity check goroutine if enabled
		if client.config.ConnectivityCheckEnabled {
			go func() {
				defer client.wg.Done()
				client.monitorConnectivity()
			}()
		}
	}

	return client, nil
}

// NewClientWithConfig creates a client with a specific configuration file
func NewClientWithConfig(configPath string) (*Client, error) {
	return NewClientWithConfigAndAPIKey(configPath, "")
}

// NewClientWithConfigAndAPIKey creates a client with a specific configuration file and API key
func NewClientWithConfigAndAPIKey(configPath string, apiKey string) (*Client, error) {
	config, err := loadConfigFromPath(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
	}

	client := &Client{
		config: config.toConfig(),
		apiKey: apiKey, // Set API key if provided
	}

	// Initialize managed mode components only if needed
	if client.config.Managed {
		client.metrics = &SystemMetrics{}
		client.done = make(chan struct{})
	}

	// Initialize HTTP client with timeout from config
	client.httpClient = &http.Client{
		Timeout: client.config.Timeout,
	}

	// Set up API configuration
	if err := client.setupAPIConfig(); err != nil {
		return nil, err
	}

	// Initialize managed mode components
	if client.config.Managed {
		// Initialize connectivity state
		client.connectivity = &ConnectivityState{
			Online:       true, // Assume online initially
			LastCheck:    time.Now(),
			CheckEnabled: client.config.ConnectivityCheckEnabled,
		}

		// Initialize offline storage if enabled
		if client.config.OfflineStorage {
			storage, err := NewStorage(client.config.StoragePath)
			if err != nil {
				return nil, err
			}
			client.storage = storage
		}

		client.queue = make(chan Message, client.config.MaxQueueSize)

		// Start background goroutines
		goroutineCount := 2
		if client.config.OfflineStorage && client.config.OfflineRetryEnabled {
			goroutineCount++ // Add offline retry goroutine
		}
		if client.config.ConnectivityCheckEnabled {
			goroutineCount++ // Add connectivity check goroutine
		}

		client.wg.Add(goroutineCount)
		go func() {
			defer client.wg.Done()
			client.processQueue()
		}()
		go func() {
			defer client.wg.Done()
			client.updateMetrics()
		}()

		// Start offline retry goroutine if enabled
		if client.config.OfflineStorage && client.config.OfflineRetryEnabled {
			go func() {
				defer client.wg.Done()
				client.processOfflineRetries()
			}()
		}

		// Start connectivity check goroutine if enabled
		if client.config.ConnectivityCheckEnabled {
			go func() {
				defer client.wg.Done()
				client.monitorConnectivity()
			}()
		}
	}

	return client, nil
}

func (c *Client) setupAPIConfig() error {
	// Get API key from parameter, then environment, then error
	if c.apiKey == "" {
		if envKey := os.Getenv("TENDRL_KEY"); envKey != "" {
			c.apiKey = envKey
		} else {
			return fmt.Errorf("no API key provided - set TENDRL_KEY environment variable or pass API key to client constructor")
		}
	}

	// Set base URL (hardcoded for Tendrl service)
	c.baseURL = "https://app.tendrl.com/api"

	// Validate API key in managed mode
	if c.config.Managed {
		if err := c.validateAPIKey(); err != nil {
			return fmt.Errorf("API key validation failed: %w", err)
		}
	}

	return nil
}

// validateAPIKey checks if the API key is valid by calling the /claims endpoint
func (c *Client) validateAPIKey() error {
	endpoint, err := url.JoinPath(c.baseURL, "/claims")
	if err != nil {
		return fmt.Errorf("failed to construct claims endpoint URL: %w", err)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create claims request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", BuildUserAgent())

	// Use a shorter timeout for validation
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("claims request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for success status codes
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil // API key is valid
	}

	// Handle specific error cases
	switch resp.StatusCode {
	case 401:
		return fmt.Errorf("invalid API key (unauthorized)")
	case 403:
		return fmt.Errorf("API key lacks required permissions (forbidden)")
	case 429:
		return fmt.Errorf("rate limited - too many requests")
	default:
		return fmt.Errorf("claims validation failed with status %d: %s", resp.StatusCode, resp.Status)
	}
}

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

// sendMessages sends messages (single or batch) and optionally returns message ID
func (c *Client) sendMessages(messages []Message, waitResponse bool) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

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
		// Single message with response uses /message endpoint
		endpoint, err = url.JoinPath(c.baseURL, "/message")
	} else {
		// Batch messages use /messages endpoint
		endpoint, err = url.JoinPath(c.baseURL, "/messages")
	}
	if err != nil {
		return "", fmt.Errorf("failed to construct endpoint URL: %w", err)
	}

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

// prepareData formats data appropriately - strings stay as strings, everything else gets JSON marshaled
func (c *Client) prepareData(data interface{}) (interface{}, error) {
	if data == nil {
		return nil, nil
	}

	// If it's already a string, use it directly
	if str, ok := data.(string); ok {
		return str, nil
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
	// Prepare data for message format
	preparedData, err := c.prepareData(data)
	if err != nil {
		return "", fmt.Errorf("failed to prepare data: %w", err)
	}

	msg := Message{
		Data: preparedData,
		Context: MessageContext{
			Tags:         tags,
			Entity:       entity,
			WaitResponse: waitResponse,
			Timeout:      timeout,
		},
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
	_, err := c.Publish(data, tags, "", false, 5)
	return err
}

// GetSystemMetrics returns current system performance metrics (only available in managed mode)
func (c *Client) GetSystemMetrics() SystemMetrics {
	if !c.config.Managed || c.metrics == nil {
		return SystemMetrics{} // Return empty metrics if not in managed mode
	}
	c.metrics.RLock()
	defer c.metrics.RUnlock()
	return SystemMetrics{
		CPUUsage:    c.metrics.CPUUsage,
		MemoryUsage: c.metrics.MemoryUsage,
		QueueLoad:   c.metrics.QueueLoad,
	}
}

// GetOfflineStorageStats returns offline storage statistics (only available in managed mode)
func (c *Client) GetOfflineStorageStats() OfflineStorageStats {
	stats := OfflineStorageStats{
		Enabled:        c.config.Managed && c.config.OfflineStorage,
		RetryEnabled:   c.config.Managed && c.config.OfflineRetryEnabled,
		BatchSize:      50, // Max messages per batch (from retryOfflineMessages)
		MaxBatchCycles: 5,  // Max batches per retry cycle (from retryOfflineMessages)
	}

	if c.config.Managed && c.storage != nil {
		count, err := c.storage.Count()
		if err == nil {
			stats.MessageCount = count
		}
	}

	return stats
}

// GetConnectivityState returns current connectivity information (only available in managed mode)
func (c *Client) GetConnectivityState() ConnectivityState {
	if !c.config.Managed || c.connectivity == nil {
		return ConnectivityState{} // Return empty state if not in managed mode
	}
	c.connectivity.RLock()
	defer c.connectivity.RUnlock()
	return ConnectivityState{
		Online:       c.connectivity.Online,
		LastCheck:    c.connectivity.LastCheck,
		LastOnline:   c.connectivity.LastOnline,
		LastOffline:  c.connectivity.LastOffline,
		CheckEnabled: c.connectivity.CheckEnabled,
	}
}

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

func (c *Client) Stop() {
	if c.config.Managed {
		if c.done != nil {
			close(c.done)
		}
		if c.storage != nil {
			c.storage.Close()
		}
		c.wg.Wait()
	}
}

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

// Tether attaches a function to run periodically and publish results to the cloud.
// Returns a stop function. Only available in managed mode.
func (c *Client) Tether(name string, fn DataFunc, tags []string, interval time.Duration) func() {
	if !c.config.Managed {
		// Return a no-op function for headless mode
		return func() {}
	}

	stopChan := make(chan struct{})

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				data, err := fn()
				if err != nil {
					continue // Skip on error
				}

				// Use the async publish method for background collections
				c.PublishAsync(data, tags)

			case <-stopChan:
				return
			case <-c.done:
				return
			}
		}
	}()

	// Return stop function
	return func() {
		close(stopChan)
	}
}
