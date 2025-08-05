package ui

import (
	"strings"
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
		DisplayMode:  ViewDisplayMode{IsVerbose: true},
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

func TestGetStatusIcon(t *testing.T) {
	tests := []struct {
		name     string
		success  bool
		partial  bool
		complete bool
		expected string
	}{
		{
			name:     "Incomplete check",
			success:  false,
			partial:  false,
			complete: false,
			expected: IconSpinner,
		},
		{
			name:     "All successful",
			success:  true,
			partial:  false,
			complete: true,
			expected: IconSuccess,
		},
		{
			name:     "All failed",
			success:  false,
			partial:  false,
			complete: true,
			expected: IconError,
		},
		{
			name:     "Partial success",
			success:  false,
			partial:  true,
			complete: true,
			expected: IconWarning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStatusIcon(tt.success, tt.partial, tt.complete)
			if result != tt.expected {
				t.Errorf("GetStatusIcon() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestView_CalculateWorkingCount(t *testing.T) {
	view := &View{
		ActiveChecks: make(map[string]*CheckStatus),
	}
	
	// Add test data
	view.ActiveChecks["proxy1"] = &CheckStatus{
		CheckResults: []CheckResult{
			{Success: true},
			{Success: true},
		},
	}
	view.ActiveChecks["proxy2"] = &CheckStatus{
		CheckResults: []CheckResult{
			{Success: false},
			{Success: false},
		},
	}
	view.ActiveChecks["proxy3"] = &CheckStatus{
		CheckResults: []CheckResult{
			{Success: true},
			{Success: false},
		},
	}

	result := view.calculateWorkingCount()
	expected := 2 // proxy1 and proxy3 have at least one successful check
	
	if result != expected {
		t.Errorf("calculateWorkingCount() = %v, want %v", result, expected)
	}
}

// Helper function to check if a string contains another string
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestNewView(t *testing.T) {
	view := NewView()
	
	if view == nil {
		t.Error("NewView returned nil")
	}
	
	if !view.IsValid() {
		t.Error("NewView created invalid view")
	}
	
	if view.ActiveChecks == nil {
		t.Error("NewView did not initialize ActiveChecks map")
	}
}

func TestView_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		view     *View
		expected bool
	}{
		{
			name:     "Valid view",
			view:     &View{Total: 10, Current: 5, ActiveChecks: make(map[string]*CheckStatus)},
			expected: true,
		},
		{
			name:     "Invalid: negative total",
			view:     &View{Total: -1, Current: 5, ActiveChecks: make(map[string]*CheckStatus)},
			expected: false,
		},
		{
			name:     "Invalid: current > total",
			view:     &View{Total: 5, Current: 10, ActiveChecks: make(map[string]*CheckStatus)},
			expected: false,
		},
		{
			name:     "Invalid: nil ActiveChecks",
			view:     &View{Total: 10, Current: 5, ActiveChecks: nil},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.view.IsValid()
			if result != tt.expected {
				t.Errorf("IsValid() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestView_GetCompletionPercentage(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		current  int
		expected float64
	}{
		{"Zero total", 0, 0, 0},
		{"Half complete", 10, 5, 50},
		{"Fully complete", 10, 10, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := &View{Total: tt.total, Current: tt.current}
			result := view.GetCompletionPercentage()
			if result != tt.expected {
				t.Errorf("GetCompletionPercentage() = %v, want %v", result, tt.expected)
			}
		})
	}
}
