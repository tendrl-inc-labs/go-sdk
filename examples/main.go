package main

import (
	"fmt"
	"log"
	"os"
	"time"

	tendrl "github.com/tendrl-inc/clients/tendrl_sdk/tendrl"
)

func userSignupMetrics() (interface{}, error) {
	return map[string]interface{}{
		"event":       "user_signup",
		"user_id":     fmt.Sprintf("user_%d", time.Now().Unix()),
		"email":       "user@example.com",
		"signup_time": time.Now().Format(time.RFC3339),
		"count":       1,
	}, nil
}

func systemMetrics() (interface{}, error) {
	return map[string]interface{}{
		"cpu_usage":    42.5,
		"memory_usage": 68.2,
		"disk_usage":   75.0,
		"timestamp":    time.Now().Format(time.RFC3339),
	}, nil
}

func generateUserData() (interface{}, error) {
	return map[string]interface{}{
		"user_id":    12345,
		"email":      "user@example.com",
		"signup_at":  time.Now(),
		"plan":       "premium",
		"trial_days": 14,
	}, nil
}

func main() {
	// Check for demo mode
	demoMode := len(os.Args) > 1 && os.Args[1] == "demo"

	// Create managed client with full features
	fmt.Println("Creating managed client (with queuing/batching)...")
	client, err := tendrl.NewClient(true, "") // true = managed mode, "" = use env var

	if err != nil {
		log.Fatal(err)
	}
	defer client.Stop()

	// Set up message callback to handle incoming messages
	fmt.Println("Setting up message callback...")
	client.SetMessageCallback(func(message tendrl.IncomingMessage) error {
		fmt.Printf("\n🔔 Received message from server:\n")
		fmt.Printf("   Type: %s\n", message.MsgType)
		fmt.Printf("   Source: %s\n", message.Source)
		fmt.Printf("   Timestamp: %s\n", message.Timestamp)
		fmt.Printf("   Data: %v\n", message.Data)

		if len(message.Context.Tags) > 0 {
			fmt.Printf("   Tags: %v\n", message.Context.Tags)
		}

		if message.RequestID != "" {
			fmt.Printf("   Request ID: %s\n", message.RequestID)
		}

		// Handle different message types with specific logic
		switch message.MsgType {
		case "command":
			fmt.Printf("   ⚡ Processing command: %v\n", message.Data)
			// Here you would implement command processing logic
			// For example: execute system commands, update configurations, etc.

		case "notification":
			fmt.Printf("   📢 Received notification: %v\n", message.Data)
			// Here you would handle notifications
			// For example: show alerts, update UI, send alerts to other systems

		case "request":
			fmt.Printf("   📝 Handling request: %v\n", message.Data)
			// Here you would process requests that may need responses
			// For example: data queries, status requests, configuration requests

		case "config_update":
			fmt.Printf("   ⚙️  Configuration update: %v\n", message.Data)
			// Here you would handle configuration updates
			// For example: update application settings, restart services

		case "alert":
			fmt.Printf("   🚨 Alert received: %v\n", message.Data)
			// Here you would handle alerts
			// For example: trigger emergency procedures, notify administrators

		default:
			fmt.Printf("   ❓ Unknown message type: %s\n", message.MsgType)
			// Log unknown message types for debugging
		}

		fmt.Printf("   ✅ Message processed successfully\n")
		return nil // Return error if callback processing fails
	})

	// Configure message checking (optional - defaults are usually fine)
	client.SetMessageCheckRate(5 * time.Second) // Check every 5 seconds (default: 3 seconds)
	client.SetMessageCheckLimit(5)              // Get up to 5 messages per check (default: 1)

	// You can manually check for messages at any time (useful for testing)
	fmt.Println("Performing initial message check...")
	if err := client.CheckMessages(); err != nil {
		log.Printf("Initial message check failed: %v", err)
	}

	// In managed mode, you can tether functions to run periodically
	// This is just for demonstration - in real apps you'd tether functions as needed
	fmt.Println("Managed mode: Background processing enabled")
	// Optional: Tether custom functions for demo
	stopMetrics := client.Tether("system_metrics", systemMetrics, []string{"system", "metrics"}, 30*time.Second)
	defer stopMetrics()

	stopUserData := client.Tether("user_events", generateUserData, []string{"user", "signup"}, 5*time.Second)
	defer stopUserData()

	// 1. String data (async, fire-and-forget)
	if err := client.PublishAsync("Simple string message", []string{"string", "test"}); err != nil {
		log.Printf("Failed to publish string: %v", err)
	}

	// 2. Map data with immediate response (like Python SDK wait_response=True)
	mapData := map[string]interface{}{
		"event":     "user_action",
		"timestamp": time.Now().Unix(),
		"details": map[string]string{
			"action": "button_click",
			"page":   "dashboard",
		},
	}
	messageID, err := client.Publish(mapData, []string{"user", "interaction"}, "", true, 10)
	if err != nil {
		log.Printf("Failed to publish map: %v", err)
	} else {
		fmt.Printf("Published map data, got message ID: %s\n", messageID)
	}

	// 3. Struct data (async)
	type Event struct {
		Type      string `json:"type"`
		UserID    int    `json:"user_id"`
		Timestamp int64  `json:"timestamp"`
	}
	structData := Event{
		Type:      "app_start",
		UserID:    12345,
		Timestamp: time.Now().Unix(),
	}
	if err := client.PublishAsync(structData, []string{"app", "lifecycle"}); err != nil {
		log.Printf("Failed to publish struct: %v", err)
	}

	// 4. Demonstrate sending to specific entity with response waiting
	entityData := map[string]interface{}{
		"status":    "connected",
		"device_id": "esp32-001",
		"timestamp": time.Now().Unix(),
	}
	entityMsgID, err := client.Publish(entityData, []string{"device", "status"}, "device-manager", true, 5)
	if err != nil {
		log.Printf("Failed to publish to entity: %v", err)
	} else {
		fmt.Printf("Published to entity, got message ID: %s\n", entityMsgID)
	}

	// Monitor offline storage and connectivity
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			stats := client.GetOfflineStorageStats()
			connectivity := client.GetConnectivityState()

			fmt.Printf("Storage - Messages: %d, Retry: %v | Connectivity - Online: %v, Last Check: %v\n",
				stats.MessageCount, stats.RetryEnabled, connectivity.Online,
				connectivity.LastCheck.Format("15:04:05"))
		}
	}()

	// Determine run duration
	duration := 2 * time.Minute
	if demoMode {
		duration = 30 * time.Second
		fmt.Println("Demo mode: Running for 30 seconds...")
	} else {
		fmt.Println("Running client with offline retry for 2 minutes...")
	}

	fmt.Println("🚀 Client Features Active:")
	fmt.Println("  • Messages will be stored offline if network is unavailable")
	fmt.Println("  • Stored messages will be retried every 15 seconds")
	fmt.Println("  • Incoming messages will be checked every 5 seconds and processed by callback")
	fmt.Println("  • Background tethered functions are collecting data every 5-30 seconds")
	fmt.Println("  • Use 'go run examples/main.go demo' for 30-second demo")
	fmt.Println("")
	fmt.Println("📬 Message Callback:")
	fmt.Println("  • The callback will handle command, notification, request, config_update, and alert messages")
	fmt.Println("  • Each message type has specific processing logic")
	fmt.Println("  • Messages are automatically checked in the background every 5 seconds")

	time.Sleep(duration)

	// Stop collectors (automatically handled by defer statements)

	// Get final stats
	finalStats := client.GetOfflineStorageStats()
	fmt.Printf("Final offline storage stats: %+v\n", finalStats)

	fmt.Println("\n✓ Example completed successfully!")
}
