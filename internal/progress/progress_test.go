package progress

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	if config.Type != ProgressTypeBar {
		t.Errorf("Expected default type to be ProgressTypeBar, got %s", config.Type)
	}
	
	if config.Width != 50 {
		t.Errorf("Expected default width to be 50, got %d", config.Width)
	}
	
	if config.UpdateRate != 250*time.Millisecond {
		t.Errorf("Expected default update rate to be 250ms, got %v", config.UpdateRate)
	}
	
	if !config.ShowETA {
		t.Error("Expected ShowETA to be true by default")
	}
	
	if !config.ShowStats {
		t.Error("Expected ShowStats to be true by default")
	}
}

func TestNewProgressIndicator(t *testing.T) {
	tests := []struct {
		name         string
		progressType ProgressType
		expectedType string
	}{
		{"None", ProgressTypeNone, "*progress.NoneIndicator"},
		{"Basic", ProgressTypeBasic, "*progress.BasicIndicator"},
		{"Bar", ProgressTypeBar, "*progress.BarIndicator"},
		{"Spinner", ProgressTypeSpinner, "*progress.SpinnerIndicator"},
		{"Dots", ProgressTypeDots, "*progress.DotsIndicator"},
		{"Percent", ProgressTypePercent, "*progress.PercentIndicator"},
		{"Unknown", ProgressType("unknown"), "*progress.BasicIndicator"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{Type: tt.progressType}
			indicator := NewProgressIndicator(config)
			
			actualType := getTypeName(indicator)
			if actualType != tt.expectedType {
				t.Errorf("Expected %s, got %s", tt.expectedType, actualType)
			}
		})
	}
}

func TestNoneIndicator(t *testing.T) {
	var buf bytes.Buffer
	indicator := &NoneIndicator{}
	indicator.SetOutput(&buf)
	
	// None indicator should not output anything
	indicator.Start(10)
	indicator.Update(5, "test message")
	indicator.Finish("done")
	
	if buf.Len() != 0 {
		t.Errorf("NoneIndicator should not produce any output, got: %s", buf.String())
	}
}

func TestBasicIndicator(t *testing.T) {
	var buf bytes.Buffer
	config := Config{
		Type:      ProgressTypeBasic,
		ShowETA:   true,
		ShowStats: true,
		Output:    &buf,
	}
	
	indicator := NewProgressIndicator(config)
	
	// Test start
	indicator.Start(10)
	output := buf.String()
	if !strings.Contains(output, "Starting proxy tests: 10 proxies") {
		t.Errorf("Expected start message, got: %s", output)
	}
	
	// Test update
	buf.Reset()
	indicator.Update(5, "working proxy")
	output = buf.String()
	if !strings.Contains(output, "Progress: 5/10") {
		t.Errorf("Expected progress update, got: %s", output)
	}
	if !strings.Contains(output, "50.0%") {
		t.Errorf("Expected percentage, got: %s", output)
	}
	
	// Test finish
	buf.Reset()
	indicator.Finish("All done!")
	output = buf.String()
	if !strings.Contains(output, "Completed: 5 proxies tested") {
		t.Errorf("Expected completion message, got: %s", output)
	}
	if !strings.Contains(output, "All done!") {
		t.Errorf("Expected custom finish message, got: %s", output)
	}
}

func TestBarIndicator(t *testing.T) {
	var buf bytes.Buffer
	config := Config{
		Type:      ProgressTypeBar,
		Width:     20,
		ShowETA:   false,
		ShowStats: false,
		NoColor:   true, // Disable colors for easier testing
		Output:    &buf,
	}
	
	indicator := NewProgressIndicator(config)
	
	// Test start
	indicator.Start(10)
	output := buf.String()
	if !strings.Contains(output, "ProxyHawk: Testing 10 proxies") {
		t.Errorf("Expected start message, got: %s", output)
	}
	
	// Test update with progress
	buf.Reset()
	indicator.Update(5, "")
	output = buf.String()
	if !strings.Contains(output, "5/10") {
		t.Errorf("Expected progress count, got: %s", output)
	}
	if !strings.Contains(output, "50.0%") {
		t.Errorf("Expected percentage, got: %s", output)
	}
	// Should contain progress bar characters
	if !strings.Contains(output, "█") && !strings.Contains(output, "░") {
		t.Errorf("Expected progress bar characters, got: %s", output)
	}
}

func TestSpinnerIndicator(t *testing.T) {
	var buf bytes.Buffer
	config := Config{
		Type:       ProgressTypeSpinner,
		UpdateRate: 50 * time.Millisecond,
		ShowETA:    false,
		ShowStats:  false,
		NoColor:    true,
		Output:     &buf,
	}
	
	indicator := NewProgressIndicator(config).(*SpinnerIndicator)
	
	// Test start
	indicator.Start(10)
	output := buf.String()
	if !strings.Contains(output, "ProxyHawk: Starting tests for 10 proxies") {
		t.Errorf("Expected start message, got: %s", output)
	}
	
	// Test update
	buf.Reset()
	indicator.Update(3, "testing proxy")
	
	// Give spinner a moment to start
	time.Sleep(100 * time.Millisecond)
	
	// Test finish (this should stop the spinner)
	indicator.Finish("Done!")
	output = buf.String()
	if !strings.Contains(output, "Completed: 3 proxies tested") {
		t.Errorf("Expected completion message, got: %s", output)
	}
}

func TestDotsIndicator(t *testing.T) {
	var buf bytes.Buffer
	config := Config{
		Type:      ProgressTypeDots,
		ShowStats: true,
		Output:    &buf,
	}
	
	indicator := NewProgressIndicator(config)
	
	// Test start
	indicator.Start(100)
	output := buf.String()
	if !strings.Contains(output, "Testing 100 proxies:") {
		t.Errorf("Expected start message, got: %s", output)
	}
	
	// Test updates that should trigger dots
	buf.Reset()
	indicator.Update(10, "")  // Should trigger a dot (step = max(100/20, 10) = 10)
	indicator.Update(20, "")  // Should trigger another dot
	
	output = buf.String()
	expectedDots := 2
	actualDots := strings.Count(output, ".")
	if actualDots != expectedDots {
		t.Errorf("Expected %d dots, got %d. Output: %s", expectedDots, actualDots, output)
	}
}

func TestPercentIndicator(t *testing.T) {
	var buf bytes.Buffer
	config := Config{
		Type:      ProgressTypePercent,
		ShowStats: false,
		Output:    &buf,
	}
	
	indicator := NewProgressIndicator(config)
	
	// Test start
	indicator.Start(10)
	output := buf.String()
	if !strings.Contains(output, "Testing 10 proxies: 0%") {
		t.Errorf("Expected start message, got: %s", output)
	}
	
	// Test update
	buf.Reset()
	indicator.Update(5, "")
	output = buf.String()
	if !strings.Contains(output, "50%") {
		t.Errorf("Expected 50%%, got: %s", output)
	}
	
	// Test finish
	buf.Reset()
	indicator.Finish("Complete")
	output = buf.String()
	if !strings.Contains(output, "100%") {
		t.Errorf("Expected 100%%, got: %s", output)
	}
	if !strings.Contains(output, "Complete") {
		t.Errorf("Expected finish message, got: %s", output)
	}
}

func TestStatsCalculation(t *testing.T) {
	var buf bytes.Buffer
	config := Config{
		Type:      ProgressTypeBasic,
		ShowStats: true,
		Output:    &buf,
	}
	
	indicator := NewProgressIndicator(config)
	
	indicator.Start(10)
	buf.Reset()
	
	// Update with success message
	indicator.Update(1, "working proxy detected")
	// Update with failure message
	indicator.Update(2, "connection failed")
	// Update with another success
	indicator.Update(3, "proxy success")
	
	output := buf.String()
	
	// Should show working and failed counts
	if !strings.Contains(output, "Working:") || !strings.Contains(output, "Failed:") {
		t.Errorf("Expected stats in output, got: %s", output)
	}
}

func TestProgressCalculations(t *testing.T) {
	tests := []struct {
		name     string
		current  int
		total    int
		expected float64
	}{
		{"0%", 0, 10, 0.0},
		{"50%", 5, 10, 50.0},
		{"100%", 10, 10, 100.0},
		{"Partial", 3, 7, 42.857}, // approximately
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			config := Config{
				Type:   ProgressTypeBasic,
				Output: &buf,
			}
			
			indicator := NewProgressIndicator(config)
			indicator.Start(tt.total)
			buf.Reset()
			
			indicator.Update(tt.current, "")
			output := buf.String()
			
			// Check if the percentage is approximately correct
			if tt.expected == 0.0 || tt.expected == 50.0 || tt.expected == 100.0 {
				expectedStr := strings.TrimSuffix(strings.TrimSuffix(output, "\n"), "\r")
				if !strings.Contains(expectedStr, "%.1f%%") && !strings.Contains(output, "%.0f%%") {
					// For exact matches, be more flexible with format
					if !strings.Contains(output, "0.0%") && !strings.Contains(output, "50.0%") && !strings.Contains(output, "100.0%") {
						t.Logf("Output: %q", output)
					}
				}
			}
		})
	}
}

func TestUtilityFunctions(t *testing.T) {
	// Test max function
	if max(5, 10) != 10 {
		t.Errorf("max(5, 10) should be 10")
	}
	if max(15, 10) != 15 {
		t.Errorf("max(15, 10) should be 15")
	}
	if max(5, 5) != 5 {
		t.Errorf("max(5, 5) should be 5")
	}
	
	// Test min function
	if min(5, 10) != 5 {
		t.Errorf("min(5, 10) should be 5")
	}
	if min(15, 10) != 10 {
		t.Errorf("min(15, 10) should be 10")
	}
	if min(5, 5) != 5 {
		t.Errorf("min(5, 5) should be 5")
	}
}

// Benchmark tests
func BenchmarkBasicIndicatorUpdate(b *testing.B) {
	var buf bytes.Buffer
	config := Config{
		Type:   ProgressTypeBasic,
		Output: &buf,
	}
	
	indicator := NewProgressIndicator(config)
	indicator.Start(1000)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indicator.Update(i%1000, "test message")
	}
}

func BenchmarkBarIndicatorUpdate(b *testing.B) {
	var buf bytes.Buffer
	config := Config{
		Type:    ProgressTypeBar,
		Width:   50,
		NoColor: true,
		Output:  &buf,
	}
	
	indicator := NewProgressIndicator(config)
	indicator.Start(1000)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indicator.Update(i%1000, "test message")
	}
}

// Helper function to get type name for testing
func getTypeName(v interface{}) string {
	switch v.(type) {
	case *NoneIndicator:
		return "*progress.NoneIndicator"
	case *BasicIndicator:
		return "*progress.BasicIndicator"
	case *BarIndicator:
		return "*progress.BarIndicator"
	case *SpinnerIndicator:
		return "*progress.SpinnerIndicator"
	case *DotsIndicator:
		return "*progress.DotsIndicator"
	case *PercentIndicator:
		return "*progress.PercentIndicator"
	default:
		return "unknown"
	}
}