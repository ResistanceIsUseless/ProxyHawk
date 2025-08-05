package tests

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/output"
)

// TestOutputFormatting tests the output formatting functions
func TestOutputFormatting(t *testing.T) {
	// Create test results
	results := []output.ProxyResultOutput{
		{
			Proxy:          "http://test1.com:8080",
			Working:        true,
			Speed:          100 * time.Millisecond,
			InteractshTest: true,
			IsAnonymous:    true,
			CloudProvider:  "AWS",
			Timestamp:      time.Now(),
		},
		{
			Proxy:   "http://test2.com:8080",
			Working: false,
			Error:   "connection refused",
			Speed:   0,
		},
	}

	summary := output.SummaryOutput{
		TotalProxies:      2,
		WorkingProxies:    1,
		InteractshProxies: 1,
		AnonymousProxies:  1,
		CloudProxies:      1,
		SuccessRate:       50.0,
		Results:           results,
	}

	// Test text output
	t.Run("Text Output", func(t *testing.T) {
		tempFile := "test_output.txt"
		defer os.Remove(tempFile)

		err := output.WriteTextOutput(tempFile, results, summary)
		if err != nil {
			t.Errorf("output.WriteTextOutput() error = %v", err)
			return
		}

		// Verify file exists and is not empty
		stat, err := os.Stat(tempFile)
		if err != nil {
			t.Errorf("Failed to stat output file: %v", err)
			return
		}
		if stat.Size() == 0 {
			t.Error("Output file is empty")
		}
	})

	// Test JSON output
	t.Run("JSON Output", func(t *testing.T) {
		tempFile := "test_output.json"
		defer os.Remove(tempFile)

		jsonData, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			t.Errorf("Failed to marshal JSON: %v", err)
			return
		}

		err = os.WriteFile(tempFile, jsonData, 0644)
		if err != nil {
			t.Errorf("Failed to write JSON file: %v", err)
			return
		}

		// Verify file exists and is valid JSON
		data, err := os.ReadFile(tempFile)
		if err != nil {
			t.Errorf("Failed to read JSON file: %v", err)
			return
		}

		var decoded output.SummaryOutput
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("Failed to decode JSON: %v", err)
		}
	})
}
