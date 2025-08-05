package output

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/proxy"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/sanitizer"
)

func TestConvertToOutputFormatWithSanitization(t *testing.T) {
	// Create test proxy results with potentially malicious content
	results := []*proxy.ProxyResult{
		{
			ProxyURL:      "http://normal-proxy.com:8080",
			Working:       true,
			Speed:         2 * time.Second,
			RealIP:        "203.0.113.1",
			ProxyIP:       "198.51.100.1",
			IsAnonymous:   true,
			CloudProvider: "AWS",
			Type:          proxy.ProxyTypeHTTP,
			Error:         nil,
		},
		{
			ProxyURL:      "http://malicious<script>alert('xss')</script>proxy.com:8080",
			Working:       false,
			Speed:         0,
			RealIP:        "192.168.1.1<iframe src=\"evil.com\"></iframe>",
			ProxyIP:       "10.0.0.1",
			IsAnonymous:   false,
			CloudProvider: "Evil<script>alert('xss')</script>Cloud",
			Type:          proxy.ProxyTypeHTTP,
			Error:         errors.New("Connection failed: <script>alert('xss')</script>"),
		},
		{
			ProxyURL:      "javascript:alert('xss')",
			Working:       false,
			Speed:         0,
			RealIP:        "",
			ProxyIP:       "",
			IsAnonymous:   false,
			CloudProvider: "",
			Type:          proxy.ProxyTypeUnknown,
			Error:         errors.New("File path leaked: C:\\Windows\\System32\\file.txt"),
		},
	}

	output := ConvertToOutputFormat(results)

	// Test that XSS content is properly sanitized
	if strings.Contains(output[1].Proxy, "<script>") {
		t.Errorf("XSS content not sanitized in Proxy field: %s", output[1].Proxy)
	}

	if strings.Contains(output[1].RealIP, "<iframe>") {
		t.Errorf("XSS content not sanitized in RealIP field: %s", output[1].RealIP)
	}

	if strings.Contains(output[1].CloudProvider, "<script>") {
		t.Errorf("XSS content not sanitized in CloudProvider field: %s", output[1].CloudProvider)
	}

	if strings.Contains(output[1].Error, "<script>") {
		t.Errorf("XSS content not sanitized in Error field: %s", output[1].Error)
	}

	// Test that JavaScript URL is handled
	if output[2].Proxy != "[INVALID_SCHEME]" {
		t.Errorf("JavaScript URL not properly sanitized: %s", output[2].Proxy)
	}

	// Test that file paths are sanitized
	if strings.Contains(output[2].Error, "C:\\Windows") {
		t.Errorf("File path not sanitized in Error field: %s", output[2].Error)
	}

	// Test that normal content is preserved
	if output[0].Proxy != "http://normal-proxy.com:8080" {
		t.Errorf("Normal content was incorrectly modified: %s", output[0].Proxy)
	}
}

func TestWriteJSONOutputSanitization(t *testing.T) {
	// Create test data with XSS content
	results := []*proxy.ProxyResult{
		{
			ProxyURL:      "http://test<script>alert('xss')</script>.com:8080",
			Working:       true,
			Speed:         1 * time.Second,
			RealIP:        "203.0.113.1",
			ProxyIP:       "198.51.100.1",
			IsAnonymous:   false,
			CloudProvider: "TestCloud",
			Type:          proxy.ProxyTypeHTTP,
			Error:         nil,
		},
	}

	summary := GenerateSummary(results)

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test_output_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Write JSON output
	err = WriteJSONOutput(tmpFile.Name(), summary)
	if err != nil {
		t.Fatalf("Failed to write JSON output: %v", err)
	}

	// Read and verify the output
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	// Verify XSS content is not present in raw form
	if strings.Contains(string(content), "<script>") {
		t.Errorf("XSS content found in JSON output: %s", string(content))
	}

	// Verify JSON is valid
	var result map[string]interface{}
	if err := json.Unmarshal(content, &result); err != nil {
		t.Errorf("Invalid JSON output: %v", err)
	}
}

func TestWriteTextOutputSanitization(t *testing.T) {
	// Create test data with potentially malicious content
	results := []ProxyResultOutput{
		{
			Proxy:         "http://test<script>alert('xss')</script>.com:8080",
			Working:       false,
			Speed:         0,
			RealIP:        "203.0.113.1",
			ProxyIP:       "198.51.100.1",
			IsAnonymous:   false,
			CloudProvider: "Evil<script>Cloud</script>",
			Type:          "http",
			Error:         "Connection failed: <iframe src=\"evil.com\"></iframe>",
		},
	}

	summary := SummaryOutput{
		TotalProxies:   1,
		WorkingProxies: 0,
		Results:        results,
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test_output_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Write text output
	err = WriteTextOutput(tmpFile.Name(), results, summary)
	if err != nil {
		t.Fatalf("Failed to write text output: %v", err)
	}

	// Read and verify the output
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	contentStr := string(content)

	// Verify XSS content is properly escaped
	if strings.Contains(contentStr, "<script>") {
		t.Errorf("Unescaped XSS content found in text output: %s", contentStr)
	}

	if strings.Contains(contentStr, "<iframe>") {
		t.Errorf("Unescaped XSS content found in text output: %s", contentStr)
	}

	// Verify content is still readable (should contain escaped versions)
	if !strings.Contains(contentStr, "&lt;script&gt;") && !strings.Contains(contentStr, "script") {
		t.Errorf("Content appears to be over-sanitized: %s", contentStr)
	}
}

func TestWriteWorkingProxiesOutputSanitization(t *testing.T) {
	// Create test data with XSS content
	results := []ProxyResultOutput{
		{
			Proxy:   "http://working<script>alert('xss')</script>.com:8080",
			Working: true,
			Speed:   1 * time.Second,
			Type:    "http<script>",
		},
		{
			Proxy:   "http://not-working.com:8080",
			Working: false,
			Speed:   0,
			Type:    "http",
		},
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test_working_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Write working proxies output
	err = WriteWorkingProxiesOutput(tmpFile.Name(), results)
	if err != nil {
		t.Fatalf("Failed to write working proxies output: %v", err)
	}

	// Read and verify the output
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	contentStr := string(content)

	// Verify only working proxies are included
	if !strings.Contains(contentStr, "working") {
		t.Errorf("Working proxy not found in output")
	}

	if strings.Contains(contentStr, "not-working") {
		t.Errorf("Non-working proxy found in output")
	}

	// Verify XSS content is sanitized
	if strings.Contains(contentStr, "<script>") {
		t.Errorf("Unescaped XSS content found in working proxies output: %s", contentStr)
	}
}

func TestWriteAnonymousProxiesOutputSanitization(t *testing.T) {
	// Create test data
	results := []ProxyResultOutput{
		{
			Proxy:       "http://anonymous<script>alert('xss')</script>.com:8080",
			Working:     true,
			Speed:       1 * time.Second,
			Type:        "http",
			IsAnonymous: true,
		},
		{
			Proxy:       "http://not-anonymous.com:8080",
			Working:     true,
			Speed:       1 * time.Second,
			Type:        "http",
			IsAnonymous: false,
		},
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test_anonymous_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Write anonymous proxies output
	err = WriteAnonymousProxiesOutput(tmpFile.Name(), results)
	if err != nil {
		t.Fatalf("Failed to write anonymous proxies output: %v", err)
	}

	// Read and verify the output
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	contentStr := string(content)

	// Verify only anonymous proxies are included
	if !strings.Contains(contentStr, "anonymous") {
		t.Errorf("Anonymous proxy not found in output")
	}

	if strings.Contains(contentStr, "not-anonymous") {
		t.Errorf("Non-anonymous proxy found in output")
	}

	// Verify XSS content is sanitized
	if strings.Contains(contentStr, "<script>") {
		t.Errorf("Unescaped XSS content found in anonymous proxies output: %s", contentStr)
	}
}

func TestCustomSanitizerConfig(t *testing.T) {
	// Create a sanitizer that allows HTML
	customSanitizer := sanitizer.NewSanitizer(sanitizer.Config{
		AllowHTML: true,
		MaxLength: 500,
	})

	results := []*proxy.ProxyResult{
		{
			ProxyURL:      "http://test.com:8080",
			Working:       true,
			Speed:         1 * time.Second,
			CloudProvider: "<p>Normal HTML</p>",
			Type:          proxy.ProxyTypeHTTP,
			Error:         nil,
		},
		{
			ProxyURL:      "http://malicious.com:8080",
			Working:       false,
			CloudProvider: "<script>alert('xss')</script>",
			Type:          proxy.ProxyTypeHTTP,
			Error:         errors.New("<script>alert('xss')</script>"),
		},
	}

	output := ConvertToOutputFormatWithSanitizer(results, customSanitizer)

	// Normal HTML should be preserved
	if output[0].CloudProvider != "<p>Normal HTML</p>" {
		t.Errorf("Normal HTML was incorrectly sanitized: %s", output[0].CloudProvider)
	}

	// Dangerous script should be filtered
	if strings.Contains(output[1].CloudProvider, "<script>") {
		t.Errorf("Dangerous script was not filtered: %s", output[1].CloudProvider)
	}
}

func TestEdgeCases(t *testing.T) {
	// Test with empty and nil values
	results := []*proxy.ProxyResult{
		{
			ProxyURL:      "",
			Working:       false,
			Speed:         0,
			RealIP:        "",
			ProxyIP:       "",
			CloudProvider: "",
			Type:          proxy.ProxyTypeUnknown,
			Error:         nil,
		},
	}

	output := ConvertToOutputFormat(results)

	// Should handle empty values gracefully
	if len(output) != 1 {
		t.Errorf("Expected 1 result, got %d", len(output))
	}

	if output[0].Error != "" {
		t.Errorf("Expected empty error, got %s", output[0].Error)
	}
}

func TestLongContentSanitization(t *testing.T) {
	// Test with very long content that should be truncated
	longString := strings.Repeat("A", 2000) + "<script>alert('xss')</script>"

	results := []*proxy.ProxyResult{
		{
			ProxyURL:      longString,
			Working:       false,
			Speed:         0,
			CloudProvider: longString,
			Type:          proxy.ProxyTypeUnknown,
			Error:         errors.New(longString),
		},
	}

	output := ConvertToOutputFormat(results)

	// Content should be truncated
	if len(output[0].Proxy) > 1100 { // 1000 + "..." + some buffer
		t.Errorf("Long content was not truncated: length %d", len(output[0].Proxy))
	}

	if len(output[0].CloudProvider) > 1100 {
		t.Errorf("Long CloudProvider was not truncated: length %d", len(output[0].CloudProvider))
	}

	// XSS should still be sanitized even in truncated content
	if strings.Contains(output[0].Proxy, "<script>") {
		t.Errorf("XSS content found even after truncation")
	}
}

// Benchmark tests
func BenchmarkConvertToOutputFormat(b *testing.B) {
	results := []*proxy.ProxyResult{
		{
			ProxyURL:      "http://example.com:8080",
			Working:       true,
			Speed:         1 * time.Second,
			RealIP:        "203.0.113.1",
			ProxyIP:       "198.51.100.1",
			IsAnonymous:   true,
			CloudProvider: "AWS",
			Type:          proxy.ProxyTypeHTTP,
			Error:         nil,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConvertToOutputFormat(results)
	}
}

func BenchmarkConvertToOutputFormatWithXSS(b *testing.B) {
	results := []*proxy.ProxyResult{
		{
			ProxyURL:      "http://test<script>alert('xss')</script>.com:8080",
			Working:       false,
			Speed:         0,
			RealIP:        "192.168.1.1<iframe src=\"evil.com\"></iframe>",
			CloudProvider: "Evil<script>alert('xss')</script>Cloud",
			Type:          proxy.ProxyTypeHTTP,
			Error:         errors.New("Error: <script>alert('xss')</script>"),
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConvertToOutputFormat(results)
	}
}
