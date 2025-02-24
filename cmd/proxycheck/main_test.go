package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Test loading default config when file doesn't exist
	config, err := loadConfig("nonexistent.yaml")
	if err != nil {
		t.Errorf("loadConfig() error = %v", err)
	}
	if config == nil {
		t.Error("loadConfig() returned nil config")
	}
	if len(config.DefaultHeaders) == 0 {
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

	config, err = loadConfig(tempFile)
	if err != nil {
		t.Errorf("loadConfig() error = %v", err)
	}
	if config.Timeout != 30 {
		t.Errorf("loadConfig() got timeout = %v, want %v", config.Timeout, 30)
	}
	if config.UserAgent != "Test Agent" {
		t.Errorf("loadConfig() got user agent = %v, want %v", config.UserAgent, "Test Agent")
	}
}

func TestLoadProxies(t *testing.T) {
	// Create temporary file with test proxies
	tempFile := filepath.Join(t.TempDir(), "proxies.txt")
	testProxies := `
127.0.0.1:8080
https://proxy.example.com:443
invalid:proxy:format
socks5://socks.example.com:1080
`
	if err := os.WriteFile(tempFile, []byte(testProxies), 0644); err != nil {
		t.Fatalf("Failed to create test proxies file: %v", err)
	}

	proxies, warnings, err := loadProxies(tempFile)
	if err != nil {
		t.Errorf("loadProxies() error = %v", err)
	}

	// Check number of valid proxies
	expectedValidCount := 3 // excluding the invalid format
	if len(proxies) != expectedValidCount {
		t.Errorf("loadProxies() got %v proxies, want %v", len(proxies), expectedValidCount)
	}

	// Check warnings for invalid proxies
	if len(warnings) != 1 {
		t.Errorf("loadProxies() got %v warnings, want 1", len(warnings))
	}

	// Test with empty file
	emptyFile := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	_, _, err = loadProxies(emptyFile)
	if err == nil {
		t.Error("loadProxies() expected error for empty file")
	}
}

func TestGetDefaultConfig(t *testing.T) {
	config := getDefaultConfig()

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
		if _, ok := config.DefaultHeaders[header]; !ok {
			t.Errorf("getDefaultConfig() missing required header %s", header)
		}
	}

	// Check User-Agent
	if config.UserAgent == "" {
		t.Error("getDefaultConfig() returned empty User-Agent")
	}

	// Check validation settings
	if len(config.Validation.DisallowedKeywords) == 0 {
		t.Error("getDefaultConfig() returned empty DisallowedKeywords")
	}
	if config.Validation.MinResponseBytes <= 0 {
		t.Error("getDefaultConfig() returned invalid MinResponseBytes")
	}
}
