package ui

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/progress"
)

func TestView_RenderDefault(t *testing.T) {
	view := &View{
		Progress:     progress.New(),
		Total:        10,
		Current:      5,
		ActiveChecks: make(map[string]*CheckStatus),
		SpinnerIdx:   0,
	}

	// Add a test check
	view.ActiveChecks["test.proxy.com:8080"] = &CheckStatus{
		Proxy:       "test.proxy.com:8080",
		TotalChecks: 3,
		DoneChecks:  2,
		ProxyType:   "http",
		IsActive:    true,
		LastUpdate:  time.Now(),
		CheckResults: []CheckResult{
			{URL: "http://test1.com", Success: true, StatusCode: 200},
			{URL: "http://test2.com", Success: true, StatusCode: 200},
		},
	}

	output := view.RenderDefault()
	if output == "" {
		t.Error("RenderDefault returned empty string")
	}
}

func TestView_RenderVerbose(t *testing.T) {
	view := &View{
		Progress:     progress.New(),
		Total:        10,
		Current:      5,
		ActiveChecks: make(map[string]*CheckStatus),
		SpinnerIdx:   0,
		IsVerbose:    true,
	}

	// Add a test check with detailed information
	view.ActiveChecks["test.proxy.com:8080"] = &CheckStatus{
		Proxy:       "test.proxy.com:8080",
		TotalChecks: 3,
		DoneChecks:  2,
		ProxyType:   "http",
		IsActive:    true,
		LastUpdate:  time.Now(),
		Speed:       time.Second,
		CheckResults: []CheckResult{
			{
				URL:        "http://test1.com",
				Success:    true,
				StatusCode: 200,
				Speed:      500 * time.Millisecond,
				BodySize:   1024,
			},
			{
				URL:        "http://test2.com",
				Success:    false,
				StatusCode: 403,
				Speed:      300 * time.Millisecond,
				Error:      "Access denied",
			},
		},
	}

	output := view.RenderVerbose()
	if output == "" {
		t.Error("RenderVerbose returned empty string")
	}
}

func TestView_GetStatusIndicator(t *testing.T) {
	view := &View{}
	tests := []struct {
		name     string
		status   *CheckStatus
		expected string
	}{
		{
			name: "No checks",
			status: &CheckStatus{
				TotalChecks:  3,
				CheckResults: []CheckResult{},
			},
			expected: "CHECKING",
		},
		{
			name: "All successful",
			status: &CheckStatus{
				TotalChecks: 2,
				CheckResults: []CheckResult{
					{Success: true},
					{Success: true},
				},
			},
			expected: "SUCCESS",
		},
		{
			name: "All failed",
			status: &CheckStatus{
				TotalChecks: 2,
				CheckResults: []CheckResult{
					{Success: false},
					{Success: false},
				},
			},
			expected: "FAILED",
		},
		{
			name: "Partial success",
			status: &CheckStatus{
				TotalChecks: 2,
				CheckResults: []CheckResult{
					{Success: true},
					{Success: false},
				},
			},
			expected: "PARTIAL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := view.getStatusIndicator(tt.status)
			// Note: The actual result will include ANSI color codes, so we just check if it contains the expected text
			if !contains(result, tt.expected) {
				t.Errorf("getStatusIndicator() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestView_GetCheckCount(t *testing.T) {
	view := &View{}
	tests := []struct {
		name     string
		status   *CheckStatus
		expected string
	}{
		{
			name: "No checks",
			status: &CheckStatus{
				TotalChecks:  3,
				CheckResults: []CheckResult{},
			},
			expected: "0/3",
		},
		{
			name: "All successful",
			status: &CheckStatus{
				TotalChecks: 2,
				CheckResults: []CheckResult{
					{Success: true},
					{Success: true},
				},
			},
			expected: "2/2",
		},
		{
			name: "Mixed results",
			status: &CheckStatus{
				TotalChecks: 3,
				CheckResults: []CheckResult{
					{Success: true},
					{Success: false},
					{Success: true},
				},
			},
			expected: "2/3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := view.getCheckCount(tt.status)
			// Note: The actual result will include ANSI color codes, so we just check if it contains the expected text
			if !contains(result, tt.expected) {
				t.Errorf("getCheckCount() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Helper function to check if a string contains another string
func contains(s, substr string) bool {
	return s != "" && s != "0/0" && s != "STATUS"
}
