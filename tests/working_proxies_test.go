package tests

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/output"
)

// TestWorkingProxiesOutput tests the working proxies output functionality
func TestWorkingProxiesOutput(t *testing.T) {
	results := []output.ProxyResultOutput{
		{
			Proxy:       "http://working1.com:8080",
			Working:     true,
			Speed:       100 * time.Millisecond,
			IsAnonymous: true,
		},
		{
			Proxy:       "http://working2.com:8080",
			Working:     true,
			Speed:       200 * time.Millisecond,
			IsAnonymous: false,
		},
		{
			Proxy:   "http://notworking.com:8080",
			Working: false,
			Error:   "connection refused",
		},
	}

	// Test working proxies output
	t.Run("Working Proxies Output", func(t *testing.T) {
		tempFile := "test_working.txt"
		defer os.Remove(tempFile)

		err := output.WriteWorkingProxiesOutput(tempFile, results)
		if err != nil {
			t.Errorf("output.WriteWorkingProxiesOutput() error = %v", err)
			return
		}

		// Verify file contains only working proxies
		data, err := os.ReadFile(tempFile)
		if err != nil {
			t.Errorf("Failed to read output file: %v", err)
			return
		}

		content := string(data)
		if !contains(content, "working1.com") || !contains(content, "working2.com") {
			t.Error("Missing working proxies in output")
		}
		if contains(content, "notworking.com") {
			t.Error("Non-working proxy found in output")
		}
	})

	// Test working anonymous proxies output
	t.Run("Working Anonymous Proxies Output", func(t *testing.T) {
		tempFile := "test_working_anonymous.txt"
		defer os.Remove(tempFile)

		err := output.WriteAnonymousProxiesOutput(tempFile, results)
		if err != nil {
			t.Errorf("output.WriteAnonymousProxiesOutput() error = %v", err)
			return
		}

		// Verify file contains only working anonymous proxies
		data, err := os.ReadFile(tempFile)
		if err != nil {
			t.Errorf("Failed to read output file: %v", err)
			return
		}

		content := string(data)
		if !contains(content, "working1.com") {
			t.Error("Missing working anonymous proxy in output")
		}
		if contains(content, "working2.com") || contains(content, "notworking.com") {
			t.Error("Non-anonymous or non-working proxy found in output")
		}
	})
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
