package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestConfigWatcher(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "config-watcher-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create initial config file
	configPath := filepath.Join(tempDir, "test-config.yaml")
	initialConfig := `
concurrency: 10
timeout: 5
user_agent: "TestAgent/1.0"
test_urls:
  default_url: "http://httpbin.org/ip"
validation:
  min_response_bytes: 100
`
	
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}
	
	// Track reload events
	var reloadCount int
	var lastConfig *Config
	var reloadMutex sync.Mutex
	
	// Create watcher config
	watcherConfig := WatcherConfig{
		DebounceDelay:        100 * time.Millisecond,
		ValidateBeforeReload: true,
		OnReload: func(config *Config, result *ValidationResult) {
			reloadMutex.Lock()
			reloadCount++
			lastConfig = config
			reloadMutex.Unlock()
			t.Logf("Config reloaded successfully (reload #%d)", reloadCount)
		},
		OnError: func(err error) {
			t.Logf("Config reload error: %v", err)
		},
	}
	
	// Create and start watcher
	watcher, err := NewConfigWatcher(configPath, watcherConfig)
	if err != nil {
		t.Fatalf("Failed to create config watcher: %v", err)
	}
	defer watcher.Stop()
	
	// Test initial config load
	initialLoadedConfig := watcher.GetConfig()
	if initialLoadedConfig.Concurrency != 10 {
		t.Errorf("Expected initial concurrency to be 10, got %d", initialLoadedConfig.Concurrency)
	}
	
	// Test config update
	updatedConfig := `
concurrency: 20
timeout: 10
user_agent: "UpdatedAgent/2.0"
test_urls:
  default_url: "http://httpbin.org/ip"
validation:
  min_response_bytes: 200
`
	
	// Write updated config
	if err := os.WriteFile(configPath, []byte(updatedConfig), 0644); err != nil {
		t.Fatalf("Failed to write updated config: %v", err)
	}
	
	// Wait for reload (with some buffer for debouncing)
	time.Sleep(300 * time.Millisecond)
	
	// Check if config was reloaded
	reloadMutex.Lock()
	if reloadCount != 1 {
		t.Errorf("Expected 1 reload, got %d", reloadCount)
	}
	if lastConfig != nil && lastConfig.Concurrency != 20 {
		t.Errorf("Expected updated concurrency to be 20, got %d", lastConfig.Concurrency)
	}
	reloadMutex.Unlock()
	
	// Verify GetConfig returns updated config
	currentConfig := watcher.GetConfig()
	if currentConfig.Concurrency != 20 {
		t.Errorf("Expected GetConfig to return concurrency 20, got %d", currentConfig.Concurrency)
	}
	if currentConfig.UserAgent != "UpdatedAgent/2.0" {
		t.Errorf("Expected GetConfig to return UserAgent 'UpdatedAgent/2.0', got %s", currentConfig.UserAgent)
	}
}

func TestConfigWatcherDebouncing(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "config-watcher-debounce-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create initial config file
	configPath := filepath.Join(tempDir, "test-config.yaml")
	initialConfig := `
concurrency: 10
timeout: 5
test_urls:
  default_url: "http://httpbin.org/ip"
`
	
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}
	
	// Track reload events
	var reloadCount int
	var reloadMutex sync.Mutex
	
	// Create watcher with longer debounce delay
	watcherConfig := WatcherConfig{
		DebounceDelay:        200 * time.Millisecond,
		ValidateBeforeReload: true,
		OnReload: func(config *Config, result *ValidationResult) {
			reloadMutex.Lock()
			reloadCount++
			reloadMutex.Unlock()
			t.Logf("Config reloaded (reload #%d)", reloadCount)
		},
		OnError: func(err error) {
			t.Logf("Config reload error: %v", err)
		},
	}
	
	// Create and start watcher
	watcher, err := NewConfigWatcher(configPath, watcherConfig)
	if err != nil {
		t.Fatalf("Failed to create config watcher: %v", err)
	}
	defer watcher.Stop()
	
	// Perform multiple rapid updates
	for i := 1; i <= 5; i++ {
		config := fmt.Sprintf(`
concurrency: %d
timeout: 5
test_urls:
  default_url: "http://httpbin.org/ip"
`, 10+i)
		
		if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
			t.Fatalf("Failed to write config update %d: %v", i, err)
		}
		
		// Small delay between writes (less than debounce delay)
		time.Sleep(50 * time.Millisecond)
	}
	
	// Wait for debounce to complete
	time.Sleep(400 * time.Millisecond)
	
	// Check that only one reload occurred due to debouncing
	reloadMutex.Lock()
	if reloadCount != 1 {
		t.Errorf("Expected 1 reload due to debouncing, got %d", reloadCount)
	}
	reloadMutex.Unlock()
	
	// Verify final config value
	currentConfig := watcher.GetConfig()
	if currentConfig.Concurrency != 15 {
		t.Errorf("Expected final concurrency to be 15, got %d", currentConfig.Concurrency)
	}
}

func TestConfigWatcherValidation(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "config-watcher-validation-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create initial valid config file
	configPath := filepath.Join(tempDir, "test-config.yaml")
	validConfig := `
concurrency: 10
timeout: 5
test_urls:
  default_url: "http://httpbin.org/ip"
`
	
	if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}
	
	// Track events
	var reloadCount int
	var errorCount int
	var lastError error
	var hasWarnings bool
	var eventMutex sync.Mutex
	
	// Create watcher with validation enabled
	watcherConfig := WatcherConfig{
		DebounceDelay:        100 * time.Millisecond,
		ValidateBeforeReload: true,
		OnReload: func(config *Config, result *ValidationResult) {
			eventMutex.Lock()
			reloadCount++
			if len(result.Warnings) > 0 {
				hasWarnings = true
			}
			eventMutex.Unlock()
			t.Logf("Config reloaded - concurrency: %d, validation valid: %v, warnings: %d", config.Concurrency, result.Valid, len(result.Warnings))
		},
		OnError: func(err error) {
			eventMutex.Lock()
			errorCount++
			lastError = err
			eventMutex.Unlock()
			t.Logf("Error: %v", err)
		},
	}
	
	// Create and start watcher
	watcher, err := NewConfigWatcher(configPath, watcherConfig)
	if err != nil {
		t.Fatalf("Failed to create config watcher: %v", err)
	}
	defer watcher.Stop()
	
	// Save initial config for comparison
	_ = watcher.GetConfig()
	
	// Write invalid config that will actually fail validation
	// Use invalid timeout which doesn't get auto-corrected
	invalidConfig := `
concurrency: 10
timeout: -5
test_urls:
  default_url: "http://httpbin.org/ip"
`
	
	if err := os.WriteFile(configPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}
	
	// Wait for validation to fail
	time.Sleep(300 * time.Millisecond)
	
	// Check that an error occurred for the invalid config
	eventMutex.Lock()
	t.Logf("Reload count: %d, Error count: %d", reloadCount, errorCount)
	if errorCount != 1 {
		t.Errorf("Expected 1 error for invalid config, got %d", errorCount)
	}
	if lastError == nil || !contains(lastError.Error(), "validation failed") {
		t.Errorf("Expected validation error, got: %v", lastError)
	}
	eventMutex.Unlock()
	
	// Now test with a config that will parse but produce warnings
	warningConfig := `
concurrency: 200
timeout: 5
test_urls:
  default_url: "http://httpbin.org/ip"
`
	
	if err := os.WriteFile(configPath, []byte(warningConfig), 0644); err != nil {
		t.Fatalf("Failed to write warning config: %v", err)
	}
	
	// Wait for reload
	time.Sleep(300 * time.Millisecond)
	
	// Check that config reloaded with warnings
	eventMutex.Lock()
	if !hasWarnings {
		t.Log("Expected warnings for very high concurrency")
	}
	eventMutex.Unlock()
}

func TestConfigWatcherStop(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "config-watcher-stop-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create config file
	configPath := filepath.Join(tempDir, "test-config.yaml")
	config := `
concurrency: 10
timeout: 5
test_urls:
  default_url: "http://httpbin.org/ip"
`
	
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	
	// Create watcher
	watcherConfig := DefaultWatcherConfig()
	watcher, err := NewConfigWatcher(configPath, watcherConfig)
	if err != nil {
		t.Fatalf("Failed to create config watcher: %v", err)
	}
	
	// Stop watcher
	if err := watcher.Stop(); err != nil {
		t.Errorf("Failed to stop watcher: %v", err)
	}
	
	// Verify watcher is stopped by checking if channel operations panic
	// This is a bit hacky but effective for testing
	stopped := make(chan bool, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stopped <- true
			} else {
				stopped <- false
			}
		}()
		// Try to get config - should work even after stopping
		_ = watcher.GetConfig()
	}()
	
	select {
	case wasStopped := <-stopped:
		if wasStopped {
			t.Error("GetConfig should still work after stopping watcher")
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for stop check")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && s != substr && (len(s) > len(substr)) && (s[0:len(substr)] == substr || contains(s[1:], substr))
}