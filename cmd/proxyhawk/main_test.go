package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/config"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/loader"
)

func TestLoadConfig(t *testing.T) {
	// Test loading default config when file doesn't exist
	cfg, err := config.LoadConfig("nonexistent.yaml")
	if err != nil {
		t.Errorf("loadConfig() error = %v", err)
	}
	if cfg == nil {
		t.Error("loadConfig() returned nil config")
	}
	if len(cfg.DefaultHeaders) == 0 {
		t.Error("loadConfig() returned empty default headers")
	}

	// Test loading valid config file
	tempFile := filepath.Join(t.TempDir(), "config.yaml")
	validConfig := `
timeout: 30
user_agent: "Test Agent"
default_headers:
  Accept: "*/*"
test_urls:
  default_url: "http://example.com"
  required_success_count: 2
validation:
  min_response_bytes: 200
  disallowed_keywords:
    - "Error"
`
	if err := os.WriteFile(tempFile, []byte(validConfig), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cfg2, err := config.LoadConfig(tempFile)
	if err != nil {
		t.Errorf("loadConfig() error = %v", err)
	}
	if cfg2.Timeout != 30 {
		t.Errorf("loadConfig() got timeout = %v, want %v", cfg2.Timeout, 30)
	}
	if cfg2.UserAgent != "Test Agent" {
		t.Errorf("loadConfig() got user agent = %v, want %v", cfg2.UserAgent, "Test Agent")
	}
}

func TestLoadProxies(t *testing.T) {
	// Create temporary file with test proxies
	tempFile := filepath.Join(t.TempDir(), "proxies.txt")
	testProxies := `
203.0.113.1:8080
https://proxy.example.com:443
invalid:proxy:format
socks5://socks.example.com:1080
127.0.0.1:9000
`
	if err := os.WriteFile(tempFile, []byte(testProxies), 0644); err != nil {
		t.Fatalf("Failed to create test proxies file: %v", err)
	}

	proxies, warnings, err := loader.LoadProxies(tempFile)
	if err != nil {
		t.Errorf("loadProxies() error = %v", err)
	}

	// Check number of valid proxies - 3 valid, 2 invalid (invalid format + loopback IP)
	expectedValidCount := 3
	if len(proxies) != expectedValidCount {
		t.Errorf("loadProxies() got %v proxies, want %v", len(proxies), expectedValidCount)
	}

	// Check warnings for invalid proxies - should have 2 warnings (invalid format + loopback IP)
	expectedWarningCount := 2
	if len(warnings) != expectedWarningCount {
		t.Errorf("loadProxies() got %v warnings, want %v", len(warnings), expectedWarningCount)
	}

	// Test with empty file
	emptyFile := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	_, _, err = loader.LoadProxies(emptyFile)
	if err == nil {
		t.Error("loadProxies() expected error for empty file")
	}
}

func TestGetDefaultConfig(t *testing.T) {
	cfg := config.GetDefaultConfig()

	// Check required default headers
	requiredHeaders := []string{
		"Accept",
		"Accept-Language",
		"Accept-Encoding",
		"Connection",
		"Cache-Control",
		"Pragma",
	}

	for _, header := range requiredHeaders {
		if _, ok := cfg.DefaultHeaders[header]; !ok {
			t.Errorf("getDefaultConfig() missing required header %s", header)
		}
	}

	// Check User-Agent
	if cfg.UserAgent == "" {
		t.Error("getDefaultConfig() returned empty User-Agent")
	}

	// Check validation settings
	if len(cfg.Validation.DisallowedKeywords) == 0 {
		t.Error("getDefaultConfig() returned empty DisallowedKeywords")
	}
	if cfg.Validation.MinResponseBytes <= 0 {
		t.Error("getDefaultConfig() returned invalid MinResponseBytes")
	}
}
