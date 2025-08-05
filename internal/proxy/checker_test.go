package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/cloudcheck"
)

// TestNewChecker tests the creation of a new proxy checker
func TestNewChecker(t *testing.T) {
	config := Config{
		Timeout:            30 * time.Second,
		ValidationURL:      "https://api.ipify.org?format=json",
		DisallowedKeywords: []string{"error", "denied"},
		MinResponseBytes:   10,
		DefaultHeaders:     map[string]string{"User-Agent": "test"},
		UserAgent:          "ProxyHawk-Test/1.0",
	}

	checker := NewChecker(config, true)

	if checker == nil {
		t.Fatal("NewChecker returned nil")
	}

	if checker.config.Timeout != config.Timeout {
		t.Errorf("Expected timeout %v, got %v", config.Timeout, checker.config.Timeout)
	}

	if checker.config.ValidationURL != config.ValidationURL {
		t.Errorf("Expected validation URL %s, got %s", config.ValidationURL, checker.config.ValidationURL)
	}

	if !checker.debug {
		t.Error("Expected debug mode to be enabled")
	}

	if checker.rateLimiter == nil {
		t.Error("Expected rate limiter to be initialized")
	}
}

// TestValidateResponse tests the response validation logic
func TestValidateResponse(t *testing.T) {
	config := Config{
		DisallowedKeywords: []string{"Access Denied", "Error"},
		MinResponseBytes:   10,
	}
	checker := NewChecker(config, false)

	tests := []struct {
		name           string
		statusCode     int
		body           string
		expectedResult bool
	}{
		{
			name:           "Valid response",
			statusCode:     200,
			body:           `{"ip": "1.2.3.4"}`,
			expectedResult: true,
		},
		{
			name:           "Status code >= 400",
			statusCode:     404,
			body:           `{"ip": "1.2.3.4"}`,
			expectedResult: false,
		},
		{
			name:           "Body too small",
			statusCode:     200,
			body:           "tiny",
			expectedResult: false,
		},
		{
			name:           "Contains disallowed keyword",
			statusCode:     200,
			body:           "Access Denied - proxy blocked",
			expectedResult: false,
		},
		{
			name:           "Contains disallowed keyword case sensitive",
			statusCode:     200,
			body:           "Error occurred while processing",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: tt.statusCode}
			body := []byte(tt.body)

			result := checker.validateResponse(resp, body)
			if result != tt.expectedResult {
				t.Errorf("Expected %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

// TestDetermineProxyType tests proxy type detection
func TestDetermineProxyType(t *testing.T) {
	// Create a test server that responds to proxy requests
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ip": "1.2.3.4"}`))
	}))
	defer testServer.Close()

	config := Config{
		Timeout:            5 * time.Second,
		ValidationURL:      testServer.URL,
		DisallowedKeywords: []string{},
		MinResponseBytes:   5,
		UserAgent:          "ProxyHawk-Test/1.0",
	}
	checker := NewChecker(config, true)

	tests := []struct {
		name        string
		proxyURL    string
		expectedErr bool
	}{
		{
			name:        "Invalid proxy URL",
			proxyURL:    "://invalid-url",
			expectedErr: true,
		},
		{
			name:        "Valid HTTP proxy format",
			proxyURL:    "http://proxy.example.com:8080",
			expectedErr: false, // Will fail connection but URL parsing should work
		},
		{
			name:        "Valid HTTPS proxy format",
			proxyURL:    "https://proxy.example.com:8080",
			expectedErr: false, // Will fail connection but URL parsing should work
		},
		{
			name:        "Valid SOCKS5 proxy format",
			proxyURL:    "socks5://proxy.example.com:1080",
			expectedErr: false, // Will fail connection but URL parsing should work
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.proxyURL)
			if tt.expectedErr && err == nil {
				t.Error("Expected error parsing URL, but got none")
				return
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("Unexpected error parsing URL: %v", err)
				return
			}
			if err != nil {
				return // Skip the rest if URL parsing failed as expected
			}

			result := &ProxyResult{ProxyURL: tt.proxyURL}
			_, _, err = checker.determineProxyType(parsedURL, result)

			// We expect connection errors for non-existent proxies, but not parsing errors
			if err != nil && !strings.Contains(err.Error(), "connection") &&
				!strings.Contains(err.Error(), "timeout") &&
				!strings.Contains(err.Error(), "refused") {
				t.Errorf("Unexpected error type: %v", err)
			}
		})
	}
}

// TestRateLimiting tests the rate limiting functionality
func TestRateLimiting(t *testing.T) {
	config := Config{
		RateLimitEnabled: true,
		RateLimitDelay:   100 * time.Millisecond,
		RateLimitPerHost: true,
	}
	checker := NewChecker(config, true)

	result := &ProxyResult{DebugInfo: ""}

	// First call should not be rate limited
	start := time.Now()
	checker.applyRateLimit("example.com", result)
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("First call should not be rate limited, but took %v", elapsed)
	}

	// Second call should be rate limited
	start = time.Now()
	checker.applyRateLimit("example.com", result)
	elapsed = time.Since(start)

	if elapsed < 80*time.Millisecond {
		t.Errorf("Second call should be rate limited, but only took %v", elapsed)
	}

	// Different host should not be rate limited
	start = time.Now()
	checker.applyRateLimit("different.com", result)
	elapsed = time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("Different host should not be rate limited, but took %v", elapsed)
	}
}

// TestPerProxyRateLimiting tests per-proxy rate limiting
func TestPerProxyRateLimiting(t *testing.T) {
	config := Config{
		RateLimitEnabled:  true,
		RateLimitDelay:    100 * time.Millisecond,
		RateLimitPerProxy: true,
	}
	checker := NewChecker(config, true)

	result1 := &ProxyResult{ProxyURL: "http://proxy1.example.com:8080", DebugInfo: ""}
	result2 := &ProxyResult{ProxyURL: "http://proxy2.example.com:8080", DebugInfo: ""}

	// First call should not be rate limited
	start := time.Now()
	checker.applyProxyRateLimit("http://proxy1.example.com:8080", result1)
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("First call should not be rate limited, but took %v", elapsed)
	}

	// Second call to same proxy should be rate limited
	start = time.Now()
	checker.applyProxyRateLimit("http://proxy1.example.com:8080", result1)
	elapsed = time.Since(start)

	if elapsed < 80*time.Millisecond {
		t.Errorf("Second call to same proxy should be rate limited, but only took %v", elapsed)
	}

	// Different proxy should not be rate limited
	start = time.Now()
	checker.applyProxyRateLimit("http://proxy2.example.com:8080", result2)
	elapsed = time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("Different proxy should not be rate limited, but took %v", elapsed)
	}
}

// TestAdvancedChecksConfiguration tests advanced checks configuration
func TestAdvancedChecksConfiguration(t *testing.T) {
	config := Config{
		AdvancedChecks: AdvancedChecks{
			TestProtocolSmuggling:   true,
			TestDNSRebinding:        true,
			TestIPv6:                true,
			TestHostHeaderInjection: true,
			TestSSRF:                true,
			TestCachePoisoning:      true,
			TestHTTPMethods:         []string{"GET", "POST", "PUT"},
		},
		InteractshURL:   "https://interact.sh",
		InteractshToken: "test-token",
	}

	checker := NewChecker(config, false)

	if !checker.config.AdvancedChecks.TestProtocolSmuggling {
		t.Error("Expected TestProtocolSmuggling to be enabled")
	}

	if !checker.config.AdvancedChecks.TestDNSRebinding {
		t.Error("Expected TestDNSRebinding to be enabled")
	}

	if !checker.config.AdvancedChecks.TestSSRF {
		t.Error("Expected TestSSRF to be enabled")
	}

	if len(checker.config.AdvancedChecks.TestHTTPMethods) != 3 {
		t.Errorf("Expected 3 HTTP methods, got %d", len(checker.config.AdvancedChecks.TestHTTPMethods))
	}

	expectedMethods := []string{"GET", "POST", "PUT"}
	for i, method := range checker.config.AdvancedChecks.TestHTTPMethods {
		if method != expectedMethods[i] {
			t.Errorf("Expected method %s, got %s", expectedMethods[i], method)
		}
	}
}

// TestCloudProviderConfiguration tests cloud provider configuration
func TestCloudProviderConfiguration(t *testing.T) {
	config := Config{
		EnableCloudChecks: true,
		CloudProviders: []cloudcheck.CloudProvider{
			{
				Name:        "AWS",
				MetadataIPs: []string{"169.254.169.254"},
				ASNs:        []string{"AS16509"},
			},
			{
				Name:        "GCP",
				MetadataIPs: []string{"169.254.169.254", "metadata.google.internal"},
				ASNs:        []string{"AS15169"},
			},
		},
	}

	checker := NewChecker(config, false)

	if !checker.config.EnableCloudChecks {
		t.Error("Expected EnableCloudChecks to be true")
	}

	if len(checker.config.CloudProviders) != 2 {
		t.Errorf("Expected 2 cloud providers, got %d", len(checker.config.CloudProviders))
	}

	// Check AWS provider
	aws := checker.config.CloudProviders[0]
	if aws.Name != "AWS" {
		t.Errorf("Expected AWS provider name, got %s", aws.Name)
	}
	if len(aws.MetadataIPs) != 1 {
		t.Errorf("Expected 1 AWS metadata IP, got %d", len(aws.MetadataIPs))
	}
	if aws.MetadataIPs[0] != "169.254.169.254" {
		t.Errorf("Expected AWS metadata IP 169.254.169.254, got %s", aws.MetadataIPs[0])
	}

	// Check GCP provider
	gcp := checker.config.CloudProviders[1]
	if gcp.Name != "GCP" {
		t.Errorf("Expected GCP provider name, got %s", gcp.Name)
	}
	if len(gcp.MetadataIPs) != 2 {
		t.Errorf("Expected 2 GCP metadata IPs, got %d", len(gcp.MetadataIPs))
	}
}

// TestProxyResultStructure tests the ProxyResult structure
func TestProxyResultStructure(t *testing.T) {
	result := &ProxyResult{
		ProxyURL:       "http://proxy.example.com:8080",
		Working:        true,
		Type:           ProxyTypeHTTP,
		Speed:          500 * time.Millisecond,
		IsAnonymous:    true,
		CloudProvider:  "AWS",
		InternalAccess: false,
		MetadataAccess: false,
		SupportsHTTP:   true,
		SupportsHTTPS:  true,
		CheckResults: []CheckResult{
			{
				URL:        "https://api.ipify.org",
				Success:    true,
				Speed:      200 * time.Millisecond,
				StatusCode: 200,
				BodySize:   25,
			},
		},
		DebugInfo: "Debug information",
	}

	if result.ProxyURL != "http://proxy.example.com:8080" {
		t.Errorf("Expected proxy URL http://proxy.example.com:8080, got %s", result.ProxyURL)
	}

	if !result.Working {
		t.Error("Expected proxy to be working")
	}

	if result.Type != ProxyTypeHTTP {
		t.Errorf("Expected proxy type HTTP, got %s", result.Type)
	}

	if result.Speed != 500*time.Millisecond {
		t.Errorf("Expected speed 500ms, got %v", result.Speed)
	}

	if !result.IsAnonymous {
		t.Error("Expected proxy to be anonymous")
	}

	if result.CloudProvider != "AWS" {
		t.Errorf("Expected cloud provider AWS, got %s", result.CloudProvider)
	}

	if len(result.CheckResults) != 1 {
		t.Errorf("Expected 1 check result, got %d", len(result.CheckResults))
	}

	checkResult := result.CheckResults[0]
	if !checkResult.Success {
		t.Error("Expected check result to be successful")
	}

	if checkResult.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", checkResult.StatusCode)
	}
}

// TestProxyTypes tests the proxy type constants
func TestProxyTypes(t *testing.T) {
	expectedTypes := map[ProxyType]string{
		ProxyTypeHTTP:    "http",
		ProxyTypeHTTPS:   "https",
		ProxyTypeSOCKS4:  "socks4",
		ProxyTypeSOCKS5:  "socks5",
		ProxyTypeUnknown: "unknown",
	}

	for proxyType, expectedString := range expectedTypes {
		if string(proxyType) != expectedString {
			t.Errorf("Expected proxy type %s, got %s", expectedString, string(proxyType))
		}
	}
}

// TestConfigValidation tests configuration validation
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		isValid bool
	}{
		{
			name: "Valid config",
			config: Config{
				Timeout:            30 * time.Second,
				ValidationURL:      "https://api.ipify.org",
				DisallowedKeywords: []string{"error"},
				MinResponseBytes:   10,
				UserAgent:          "ProxyHawk-Test/1.0",
			},
			isValid: true,
		},
		{
			name: "Config with advanced checks",
			config: Config{
				Timeout:       30 * time.Second,
				ValidationURL: "https://api.ipify.org",
				AdvancedChecks: AdvancedChecks{
					TestProtocolSmuggling: true,
					TestSSRF:              true,
					TestHTTPMethods:       []string{"GET", "POST"},
				},
			},
			isValid: true,
		},
		{
			name: "Config with cloud providers",
			config: Config{
				Timeout:           30 * time.Second,
				ValidationURL:     "https://api.ipify.org",
				EnableCloudChecks: true,
				CloudProviders: []cloudcheck.CloudProvider{
					{Name: "AWS", MetadataIPs: []string{"169.254.169.254"}},
				},
			},
			isValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(tt.config, false)
			if checker == nil && tt.isValid {
				t.Error("Expected valid config to create checker")
			}
			if checker != nil && !tt.isValid {
				t.Error("Expected invalid config to fail")
			}
		})
	}
}

// TestDebugMode tests debug mode functionality
func TestDebugMode(t *testing.T) {
	config := Config{
		Timeout:       10 * time.Second,
		ValidationURL: "https://example.com",
	}

	// Test with debug enabled
	debugChecker := NewChecker(config, true)
	if !debugChecker.debug {
		t.Error("Expected debug mode to be enabled")
	}

	// Test with debug disabled
	normalChecker := NewChecker(config, false)
	if normalChecker.debug {
		t.Error("Expected debug mode to be disabled")
	}
}

// TestConnectionPoolIntegration tests connection pool integration
func TestConnectionPoolIntegration(t *testing.T) {
	// Mock connection pool
	mockPool := &mockConnectionPool{
		clients: make(map[string]*http.Client),
	}

	config := Config{
		Timeout:        30 * time.Second,
		ValidationURL:  "https://api.ipify.org",
		ConnectionPool: mockPool,
	}

	checker := NewChecker(config, false)

	if checker.config.ConnectionPool == nil {
		t.Error("Expected connection pool to be set")
	}

	// Verify that the connection pool interface is working
	if pool, ok := checker.config.ConnectionPool.(interface {
		GetClient(string, time.Duration) (*http.Client, error)
	}); ok {
		client, err := pool.GetClient("http://proxy.example.com:8080", 30*time.Second)
		if err != nil {
			t.Errorf("Unexpected error from connection pool: %v", err)
		}
		if client == nil {
			t.Error("Expected connection pool to return client")
		}
	} else {
		t.Error("Connection pool does not implement expected interface")
	}
}

// mockConnectionPool is a simple mock implementation for testing
type mockConnectionPool struct {
	clients map[string]*http.Client
}

func (m *mockConnectionPool) GetClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	if client, exists := m.clients[proxyURL]; exists {
		return client, nil
	}

	client := &http.Client{Timeout: timeout}
	m.clients[proxyURL] = client
	return client, nil
}

// Benchmark tests
func BenchmarkValidateResponse(b *testing.B) {
	config := Config{
		DisallowedKeywords: []string{"Access Denied", "Error", "Forbidden"},
		MinResponseBytes:   50,
	}
	checker := NewChecker(config, false)

	resp := &http.Response{StatusCode: 200}
	body := []byte(`{"ip": "192.168.1.100", "country": "US", "region": "California"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.validateResponse(resp, body)
	}
}

func BenchmarkRateLimiting(b *testing.B) {
	config := Config{
		RateLimitEnabled: true,
		RateLimitDelay:   1 * time.Millisecond,
		RateLimitPerHost: true,
	}
	checker := NewChecker(config, false)
	result := &ProxyResult{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.applyRateLimit("example.com", result)
	}
}

func BenchmarkProxyResultCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := &ProxyResult{
			ProxyURL:       "http://proxy.example.com:8080",
			Working:        true,
			Type:           ProxyTypeHTTP,
			Speed:          100 * time.Millisecond,
			IsAnonymous:    false,
			CloudProvider:  "",
			InternalAccess: false,
			MetadataAccess: false,
			SupportsHTTP:   true,
			SupportsHTTPS:  true,
			CheckResults:   []CheckResult{},
			DebugInfo:      "",
		}
		_ = result
	}
}
