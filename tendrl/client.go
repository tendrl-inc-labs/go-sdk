package tendrl

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
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

	// Message callback functionality
	callback      MessageCallback
	checkMsgRate  time.Duration // How often to check for messages
	checkMsgLimit int           // Maximum number of messages to retrieve per check
	lastMsgCheck  time.Time     // Last time messages were checked
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
		config:        config,
		apiKey:        apiKey,          // Set API key if provided
		checkMsgRate:  3 * time.Second, // Default: check every 3 seconds
		checkMsgLimit: 1,               // Default: get 1 message per check
		lastMsgCheck:  time.Now(),
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

		// Start message checking if callback is set
		client.startMessageChecking()
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
		config:        config.toConfig(),
		apiKey:        apiKey,          // Set API key if provided
		checkMsgRate:  3 * time.Second, // Default: check every 3 seconds
		checkMsgLimit: 1,               // Default: get 1 message per check
		lastMsgCheck:  time.Now(),
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

		// Start message checking if callback is set
		client.startMessageChecking()
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
