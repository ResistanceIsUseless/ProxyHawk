package tests

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
		config, err := testhelpers.LoadConfig("../config.yaml")
		if err != nil {
			t.Errorf("Failed to load valid config: %v", err)
			progress.AddResult(testhelpers.TestResult{
				Name:    "Valid Config",
				Passed:  false,
				Message: err.Error(),
			})
			return
		}

		if len(config.ProxyTypes) == 0 {
			t.Error("Config should have proxy types")
			progress.AddResult(testhelpers.TestResult{
				Name:    "Valid Config",
				Passed:  false,
				Message: "No proxy types found in config",
			})
			return
		}

		if config.ValidationURL == "" {
			t.Error("Config should have validation URL")
			progress.AddResult(testhelpers.TestResult{
				Name:    "Valid Config",
				Passed:  false,
				Message: "No validation URL found in config",
			})
			return
		}

		progress.AddResult(testhelpers.TestResult{
			Name:    "Valid Config",
			Passed:  true,
			Message: "Successfully loaded config",
		})
	})

	// Test loading invalid config
	progress.StartTest("Invalid Config")
	t.Run("Invalid Config", func(t *testing.T) {
		_, err := testhelpers.LoadConfig("nonexistent.yaml")
		if err == nil {
			t.Error("Should fail to load nonexistent config")
			progress.AddResult(testhelpers.TestResult{
				Name:    "Invalid Config",
				Passed:  false,
				Message: "No error when loading nonexistent config",
			})
			return
		}

		progress.AddResult(testhelpers.TestResult{
			Name:    "Invalid Config",
			Passed:  true,
			Message: "Correctly failed to load nonexistent config",
		})
	})
}
