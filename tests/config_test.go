package tests

import (
	"testing"
)

// TestConfigLoading tests the configuration loading functionality
func TestConfigLoading(t *testing.T) {
	// Test loading valid config
	t.Run("Valid Config", func(t *testing.T) {
		err := loadConfig("../config.yaml")
		if err != nil {
			t.Errorf("loadConfig() error = %v", err)
			return
		}

		// Verify config fields
		if config.UserAgent == "" {
			t.Error("UserAgent is empty")
		}
		if len(config.DefaultHeaders) == 0 {
			t.Error("DefaultHeaders is empty")
		}
		if len(config.CloudProviders) == 0 {
			t.Error("CloudProviders is empty")
		}
	})

	// Test loading invalid config
	t.Run("Invalid Config", func(t *testing.T) {
		err := loadConfig("nonexistent.yaml")
		if err == nil {
			t.Error("Expected error loading nonexistent config")
		}
	})
}
