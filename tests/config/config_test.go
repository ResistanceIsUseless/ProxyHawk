package config

import (
	"testing"

	"github.com/ResistanceIsUseless/ProxyHawk/tests/testhelpers"
)

// TestConfigLoading tests the configuration loading functionality
func TestConfigLoading(t *testing.T) {
	progress := testhelpers.NewTestProgress()
	defer progress.PrintSummary()

	// Test loading valid config
	progress.StartTest("Valid Config")
	t.Run("Valid Config", func(t *testing.T) {
		err := LoadConfig("../../config.yaml")
		if err != nil {
			t.Errorf("Failed to load valid config: %v", err)
			progress.AddResult(testhelpers.TestResult{
				Name:    "Valid Config",
				Passed:  false,
				Message: err.Error(),
			})
			return
		}

		// Verify config values
		cfg := GetConfig()
		if len(cfg.DefaultHeaders) == 0 {
			t.Error("Expected default headers to be set")
			progress.AddResult(testhelpers.TestResult{
				Name:    "Valid Config",
				Passed:  false,
				Message: "Default headers not set",
			})
			return
		}

		if cfg.UserAgent == "" {
			t.Error("Expected user agent to be set")
			progress.AddResult(testhelpers.TestResult{
				Name:    "Valid Config",
				Passed:  false,
				Message: "User agent not set",
			})
			return
		}

		progress.AddResult(testhelpers.TestResult{
			Name:   "Valid Config",
			Passed: true,
		})
	})

	// Test loading invalid config
	progress.StartTest("Invalid Config")
	t.Run("Invalid Config", func(t *testing.T) {
		err := LoadConfig("nonexistent.yaml")
		if err == nil {
			t.Error("Expected error when loading invalid config")
			progress.AddResult(testhelpers.TestResult{
				Name:    "Invalid Config",
				Passed:  false,
				Message: "Expected error when loading invalid config",
			})
			return
		}

		progress.AddResult(testhelpers.TestResult{
			Name:   "Invalid Config",
			Passed: true,
		})
	})
}
