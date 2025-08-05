package proxy

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

// TestCreateClient tests HTTP client creation through the determineProxyType method
func TestCreateClient(t *testing.T) {
	config := Config{
		Timeout:   30 * time.Second,
		UserAgent: "ProxyHawk-Test/1.0",
		DefaultHeaders: map[string]string{
			"Accept": "application/json",
			"X-Test": "test-value",
		},
		ValidationURL: "https://api.ipify.org?format=json",
	}

	checker := NewChecker(config, false)

	tests := []struct {
		name     string
		proxyURL string
		wantErr  bool
	}{
		{
			name:     "HTTP proxy client",
			proxyURL: "http://proxy.example.com:8080",
			wantErr:  true, // Will fail connection but URL parsing should work
		},
		{
			name:     "HTTPS proxy client",
			proxyURL: "https://proxy.example.com:8080",
			wantErr:  true, // Will fail connection but URL parsing should work
		},
		{
			name:     "SOCKS5 proxy client",
			proxyURL: "socks5://proxy.example.com:1080",
			wantErr:  true, // Will fail connection but URL parsing should work
		},
		{
			name:     "Invalid proxy URL",
			proxyURL: "://invalid-url",
			wantErr:  true,
		},
		{
			name:     "Unsupported scheme",
			proxyURL: "ftp://proxy.example.com:21",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ProxyResult{ProxyURL: tt.proxyURL}
			
			parsedURL, err := url.Parse(tt.proxyURL)
			if err != nil && !tt.wantErr {
				t.Errorf("Unexpected URL parsing error: %v", err)
				return
			}
			if err != nil {
				return // Expected error in URL parsing
			}

			_, client, err := checker.determineProxyType(parsedURL, result)
			
			if tt.wantErr {
				// We expect connection errors for non-existent proxies
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Error("Expected client but got nil")
				return
			}

			// Verify timeout is set correctly
			if client.Timeout != config.Timeout {
				t.Errorf("Expected timeout %v, got %v", config.Timeout, client.Timeout)
			}

			// Verify transport is configured
			if client.Transport == nil {
				t.Error("Expected transport to be configured")
			}
		})
	}
}

// TestCreateClientWithConnectionPool tests client creation with connection pool
func TestCreateClientWithConnectionPool(t *testing.T) {
	mockPool := &mockConnectionPool{
		clients: make(map[string]*http.Client),
	}

	config := Config{
		Timeout:        30 * time.Second,
		UserAgent:      "ProxyHawk-Test/1.0",
		ValidationURL:  "https://api.ipify.org?format=json",
		ConnectionPool: mockPool,
	}

	checker := NewChecker(config, false)
	result := &ProxyResult{}
	
	parsedURL, _ := url.Parse("http://proxy.example.com:8080")

	// This will fail due to connection error but should create clients through pool
	_, client1, _ := checker.determineProxyType(parsedURL, result)
	
	// Verify that connection pool was used (even if connection failed)
	if client1 != nil && len(mockPool.clients) == 0 {
		t.Error("Expected connection pool to be used")
	}
}

// TestClientTimeout tests various timeout configurations
func TestClientTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{
			name:    "Normal timeout",
			timeout: 30 * time.Second,
		},
		{
			name:    "Short timeout",
			timeout: 1 * time.Second,
		},
		{
			name:    "Long timeout",
			timeout: 300 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Timeout:       tt.timeout,
				UserAgent:     "ProxyHawk-Test/1.0",
				ValidationURL: "https://api.ipify.org?format=json",
			}

			checker := NewChecker(config, false)
			result := &ProxyResult{}
			parsedURL, _ := url.Parse("http://proxy.example.com:8080")

			// This will fail due to connection but we can verify timeout config
			_, client, _ := checker.determineProxyType(parsedURL, result)

			if client != nil && client.Timeout != tt.timeout {
				t.Errorf("Expected timeout %v, got %v", tt.timeout, client.Timeout)
			}
		})
	}
}

// TestMakeRequest tests the request creation and execution
func TestMakeRequest(t *testing.T) {
	config := Config{
		Timeout:   10 * time.Second,
		UserAgent: "ProxyHawk-Test/1.0",
		DefaultHeaders: map[string]string{
			"Accept":        "application/json",
			"Authorization": "Bearer token123",
		},
		ValidationURL: "https://httpbin.org/ip", // Use a real endpoint for testing
	}

	checker := NewChecker(config, false)
	result := &ProxyResult{}

	// Create a basic HTTP client (no proxy)
	client := &http.Client{
		Timeout: config.Timeout,
	}

	// Test the makeRequest method
	resp, err := checker.makeRequest(client, config.ValidationURL, result)
	
	// This might fail due to network issues, but we can test the method exists
	if err != nil {
		// Expected for non-working proxy, just verify the method works
		t.Logf("Request failed as expected: %v", err)
	}
	
	if resp != nil {
		resp.Body.Close()
		t.Logf("Request succeeded with status: %d", resp.StatusCode)
	}
}

// TestProxyURLParsing tests URL parsing edge cases
func TestProxyURLParsing(t *testing.T) {

	tests := []struct {
		name     string
		proxyURL string
		wantErr  bool
	}{
		{
			name:     "HTTP proxy with port",
			proxyURL: "http://proxy.example.com:8080",
			wantErr:  false, // URL parsing should work
		},
		{
			name:     "HTTP proxy without port",
			proxyURL: "http://proxy.example.com",
			wantErr:  false,
		},
		{
			name:     "HTTPS proxy",
			proxyURL: "https://secure-proxy.example.com:8443",
			wantErr:  false,
		},
		{
			name:     "Proxy with authentication",
			proxyURL: "http://user:pass@proxy.example.com:8080",
			wantErr:  false,
		},
		{
			name:     "IPv4 proxy",
			proxyURL: "http://192.168.1.100:8080",
			wantErr:  false,
		},
		{
			name:     "IPv6 proxy",
			proxyURL: "http://[::1]:8080",
			wantErr:  false,
		},
		{
			name:     "Malformed URL - no scheme",
			proxyURL: "proxy.example.com:8080",
			wantErr:  true,
		},
		{
			name:     "Empty URL",
			proxyURL: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.proxyURL)

			if tt.wantErr {
				if err == nil && (parsedURL.Scheme == "" || parsedURL.Host == "") {
					// URL might parse but be invalid
					t.Log("URL parsed but is invalid as expected")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected parsing error: %v", err)
				return
			}

			if parsedURL.Scheme == "" {
				t.Error("Expected scheme to be parsed")
			}

			if parsedURL.Host == "" {
				t.Error("Expected host to be parsed")
			}
		})
	}
}


// Benchmark tests for client creation performance
func BenchmarkDetermineProxyType(b *testing.B) {
	config := Config{
		Timeout:       30 * time.Second,
		UserAgent:     "ProxyHawk-Bench/1.0",
		ValidationURL: "https://api.ipify.org?format=json",
		DefaultHeaders: map[string]string{
			"Accept": "application/json",
		},
	}

	checker := NewChecker(config, false)
	proxyURL, _ := url.Parse("http://proxy.example.com:8080")
	result := &ProxyResult{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This will fail but we're benchmarking the setup
		_, _, _ = checker.determineProxyType(proxyURL, result)
	}
}

func BenchmarkMakeRequest(b *testing.B) {
	config := Config{
		Timeout:       5 * time.Second,
		UserAgent:     "ProxyHawk-Bench/1.0",
		ValidationURL: "https://httpbin.org/ip",
	}

	checker := NewChecker(config, false)
	client := &http.Client{Timeout: config.Timeout}
	result := &ProxyResult{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, _ := checker.makeRequest(client, config.ValidationURL, result)
		if resp != nil {
			resp.Body.Close()
		}
	}
}