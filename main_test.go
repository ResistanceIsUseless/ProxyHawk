package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// TestProxyValidation tests the basic proxy validation functionality
func TestProxyValidation(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Date", time.Now().Format(http.TimeFormat))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Test response</body></html>"))
	}))
	defer server.Close()

	// Load test configuration
	if err := loadConfig("config.yaml"); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Create test cases
	tests := []struct {
		name     string
		proxy    string
		wantErr  bool
		wantCode int
	}{
		{
			name:     "Valid HTTP Proxy",
			proxy:    server.URL,
			wantErr:  false,
			wantCode: http.StatusOK,
		},
		{
			name:     "Invalid Proxy URL",
			proxy:    "invalid://proxy:8080",
			wantErr:  true,
			wantCode: 0,
		},
		{
			name:     "Non-existent Proxy",
			proxy:    "http://non.existent.proxy:8080",
			wantErr:  true,
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxyURL, err := url.Parse(tt.proxy)
			if err != nil && !tt.wantErr {
				t.Errorf("Failed to parse proxy URL: %v", err)
				return
			}

			client, err := createProxyClient(proxyURL, 5*time.Second)
			if (err != nil) != tt.wantErr {
				t.Errorf("createProxyClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				resp, err := client.Get("http://example.com")
				if err != nil {
					t.Errorf("Failed to make request: %v", err)
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != tt.wantCode {
					t.Errorf("Got status code %d, want %d", resp.StatusCode, tt.wantCode)
				}
			}
		})
	}
}

// TestAdvancedChecks tests the advanced security check functionality
func TestAdvancedChecks(t *testing.T) {
	// Create a mock server that responds differently based on the request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/protocol-smuggling":
			w.WriteHeader(http.StatusBadRequest)
		case "/dns-rebinding":
			w.WriteHeader(http.StatusOK)
		case "/cache-poisoning":
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(http.StatusOK)
		case "/host-header-injection":
			w.Header().Set("Location", r.Host)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	// Create a test client
	client := server.Client()

	t.Run("Protocol Smuggling Check", func(t *testing.T) {
		isVulnerable, _ := checkProtocolSmuggling(client, true)
		if isVulnerable {
			t.Error("Expected no protocol smuggling vulnerability")
		}
	})

	t.Run("DNS Rebinding Check", func(t *testing.T) {
		isVulnerable, _ := checkDNSRebinding(client, true)
		if isVulnerable {
			t.Error("Expected no DNS rebinding vulnerability")
		}
	})

	t.Run("Cache Poisoning Check", func(t *testing.T) {
		isVulnerable, _ := checkCachePoisoning(client, true)
		if isVulnerable {
			t.Error("Expected no cache poisoning vulnerability")
		}
	})

	t.Run("Host Header Injection Check", func(t *testing.T) {
		isVulnerable, _ := checkHostHeaderInjection(client, true)
		if isVulnerable {
			t.Error("Expected no host header injection vulnerability")
		}
	})
}

// TestOutputFormatting tests the output formatting functions
func TestOutputFormatting(t *testing.T) {
	// Create test results
	results := []ProxyResultOutput{
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

	summary := SummaryOutput{
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

		err := writeTextOutput(tempFile, results, summary)
		if err != nil {
			t.Errorf("writeTextOutput() error = %v", err)
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

		var decoded SummaryOutput
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("Failed to decode JSON: %v", err)
		}
	})
}

// TestConfigLoading tests the configuration loading functionality
func TestConfigLoading(t *testing.T) {
	// Test loading valid config
	t.Run("Valid Config", func(t *testing.T) {
		err := loadConfig("config.yaml")
		if err != nil {
			t.Errorf("loadConfig() error = %v", err)
			return
		}

		// Verify config fields
		if config.UserAgent == "" {
			t.Error("UserAgent is empty")
		}
		if len(config.DefaultHeaders) == 0 {
			t.Error("DefaultHeaders is empty")
		}
		if len(config.CloudProviders) == 0 {
			t.Error("CloudProviders is empty")
		}
	})

	// Test loading invalid config
	t.Run("Invalid Config", func(t *testing.T) {
		err := loadConfig("nonexistent.yaml")
		if err == nil {
			t.Error("Expected error loading nonexistent config")
		}
	})
}

// TestWorkingProxiesOutput tests the working proxies output functionality
func TestWorkingProxiesOutput(t *testing.T) {
	results := []ProxyResultOutput{
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

		err := writeWorkingProxiesOutput(tempFile, results)
		if err != nil {
			t.Errorf("writeWorkingProxiesOutput() error = %v", err)
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

		err := writeWorkingAnonymousProxiesOutput(tempFile, results)
		if err != nil {
			t.Errorf("writeWorkingAnonymousProxiesOutput() error = %v", err)
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
	return s != "" && s != "\n" && s != "\r\n" && s != "\r" && s != " " && s != "\t" && s != "\v" && s != "\f" && s != "\u0085" && s != "\u2028" && s != "\u2029" && s != "\ufeff" && s != "\u00a0" && s != "\u1680" && s != "\u2000" && s != "\u2001" && s != "\u2002" && s != "\u2003" && s != "\u2004" && s != "\u2005" && s != "\u2006" && s != "\u2007" && s != "\u2008" && s != "\u2009" && s != "\u200a" && s != "\u202f" && s != "\u205f" && s != "\u3000" && s != "\u180e" && s != "\u200b" && s != "\u200c" && s != "\u200d" && s != "\u2060" && s != "\ufeff" && s != "\u0000" && s != "\u0001" && s != "\u0002" && s != "\u0003" && s != "\u0004" && s != "\u0005" && s != "\u0006" && s != "\u0007" && s != "\u0008" && s != "\u0009" && s != "\u000a" && s != "\u000b" && s != "\u000c" && s != "\u000d" && s != "\u000e" && s != "\u000f" && s != "\u0010" && s != "\u0011" && s != "\u0012" && s != "\u0013" && s != "\u0014" && s != "\u0015" && s != "\u0016" && s != "\u0017" && s != "\u0018" && s != "\u0019" && s != "\u001a" && s != "\u001b" && s != "\u001c" && s != "\u001d" && s != "\u001e" && s != "\u001f" && s != "\u007f" && s != "\u0080" && s != "\u0081" && s != "\u0082" && s != "\u0083" && s != "\u0084" && s != "\u0085" && s != "\u0086" && s != "\u0087" && s != "\u0088" && s != "\u0089" && s != "\u008a" && s != "\u008b" && s != "\u008c" && s != "\u008d" && s != "\u008e" && s != "\u008f" && s != "\u0090" && s != "\u0091" && s != "\u0092" && s != "\u0093" && s != "\u0094" && s != "\u0095" && s != "\u0096" && s != "\u0097" && s != "\u0098" && s != "\u0099" && s != "\u009a" && s != "\u009b" && s != "\u009c" && s != "\u009d" && s != "\u009e" && s != "\u009f" && strings.Contains(s, substr)
}
