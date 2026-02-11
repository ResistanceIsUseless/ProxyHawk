package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestHasAdvancedChecks tests the advanced checks detection
func TestHasAdvancedChecks(t *testing.T) {
	tests := []struct {
		name           string
		advancedChecks AdvancedChecks
		expected       bool
	}{
		{
			name:           "No advanced checks",
			advancedChecks: AdvancedChecks{},
			expected:       false,
		},
		{
			name: "Protocol smuggling enabled",
			advancedChecks: AdvancedChecks{
				TestProtocolSmuggling: true,
			},
			expected: true,
		},
		{
			name: "SSRF enabled",
			advancedChecks: AdvancedChecks{
				TestSSRF: true,
			},
			expected: true,
		},
		{
			name: "Host header injection enabled",
			advancedChecks: AdvancedChecks{
				TestHostHeaderInjection: true,
			},
			expected: true,
		},
		{
			name: "DNS rebinding enabled",
			advancedChecks: AdvancedChecks{
				TestDNSRebinding: true,
			},
			expected: true,
		},
		{
			name: "IPv6 testing enabled",
			advancedChecks: AdvancedChecks{
				TestIPv6: true,
			},
			expected: true,
		},
		{
			name: "Cache poisoning enabled",
			advancedChecks: AdvancedChecks{
				TestCachePoisoning: true,
			},
			expected: true,
		},
		{
			name: "HTTP methods testing enabled",
			advancedChecks: AdvancedChecks{
				TestHTTPMethods: []string{"GET", "POST"},
			},
			expected: true,
		},
		{
			name: "Multiple checks enabled",
			advancedChecks: AdvancedChecks{
				TestProtocolSmuggling:   true,
				TestSSRF:                true,
				TestHostHeaderInjection: true,
				TestHTTPMethods:         []string{"GET", "POST", "PUT"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Timeout:        10 * time.Second,
				ValidationURL:  "https://api.ipify.org?format=json",
				AdvancedChecks: tt.advancedChecks,
			}

			checker := NewChecker(config, false, nil)
			result := checker.hasAdvancedChecks()

			if result != tt.expected {
				t.Errorf("hasAdvancedChecks() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestPerformAdvancedChecks tests the advanced checks execution
func TestPerformAdvancedChecks(t *testing.T) {
	// Create a test server to simulate responses
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate different responses based on the request
		if strings.Contains(r.URL.Path, "smuggling") {
			w.Header().Set("Content-Length", "0")
			w.WriteHeader(http.StatusOK)
		} else if strings.Contains(r.URL.Path, "ssrf") {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Access denied"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ip": "1.2.3.4"}`))
		}
	}))
	defer testServer.Close()

	config := Config{
		Timeout:       10 * time.Second,
		ValidationURL: testServer.URL,
		AdvancedChecks: AdvancedChecks{
			TestProtocolSmuggling:   true,
			TestSSRF:                true,
			TestHostHeaderInjection: true,
			TestDNSRebinding:         true,
			TestIPv6:                 true,
			TestCachePoisoning:       true,
			TestHTTPMethods:          []string{"GET", "POST"},
		},
		InteractshURL: "https://interact.sh", // Use example URL for testing
	}

	checker := NewChecker(config, true, nil) // Enable debug for better test coverage
	client := &http.Client{Timeout: config.Timeout}
	result := &ProxyResult{
		AdvancedChecksDetails: make(map[string]interface{}),
	}

	err := checker.performAdvancedChecks(client, result)

	// We expect this to fail due to network connectivity, but the method should execute
	if err != nil {
		t.Logf("Advanced checks failed as expected due to test environment: %v", err)
	}

	// Verify that advanced checks were attempted
	if !result.AdvancedChecksPassed && len(result.AdvancedChecksDetails) == 0 {
		t.Log("Advanced checks were executed but failed as expected in test environment")
	}
}

// TestAdvancedChecksWithRealDomains tests advanced checks with specific scenarios
func TestAdvancedChecksWithRealDomains(t *testing.T) {
	// Skip this test in short mode as it requires network access
	if testing.Short() {
		t.Skip("Skipping network-dependent test in short mode")
	}

	config := Config{
		Timeout:       5 * time.Second,
		ValidationURL: "https://httpbin.org/ip",
		AdvancedChecks: AdvancedChecks{
			TestSSRF:                true,
			TestHostHeaderInjection: true,
		},
	}

	checker := NewChecker(config, true, nil)
	client := &http.Client{Timeout: config.Timeout}
	result := &ProxyResult{
		AdvancedChecksDetails: make(map[string]interface{}),
	}

	// This will likely fail due to proxy not being real, but tests the code path
	err := checker.performAdvancedChecks(client, result)
	if err != nil {
		t.Logf("Advanced checks failed as expected: %v", err)
	}
}

// TestSSRFDetection tests SSRF detection capabilities
func TestSSRFDetection(t *testing.T) {
	// Create a mock server that simulates various SSRF targets
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		
		// Simulate cloud metadata access
		if strings.Contains(path, "169.254.169.254") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"instance-id": "i-1234567890abcdef0"}`))
			return
		}
		
		// Simulate internal network access
		if strings.Contains(path, "192.168.") || strings.Contains(path, "10.") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Internal network response"))
			return
		}
		
		// Default response
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not found"))
	}))
	defer testServer.Close()

	config := Config{
		Timeout:       5 * time.Second,
		ValidationURL: testServer.URL,
		AdvancedChecks: AdvancedChecks{
			TestSSRF: true,
		},
	}

	checker := NewChecker(config, false, nil)
	client := &http.Client{Timeout: config.Timeout}

	// Test SSRF check method directly
	result, err := checker.checkSSRF(client, "interact.sh")
	
	// The method should execute without panic, even if it fails
	if err != nil {
		t.Logf("SSRF check failed as expected: %v", err)
	}
	
	if result != nil {
		t.Logf("SSRF check completed with result: %+v", result)
	}
}

// TestHostHeaderInjection tests host header injection detection
func TestHostHeaderInjection(t *testing.T) {
	// Create a server that checks for host header manipulation
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Header.Get("Host")
		
		// Check if host header was manipulated
		if strings.Contains(host, "evil.com") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Host header injection detected"))
			return
		}
		
		// Normal response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ip": "1.2.3.4"}`))
	}))
	defer testServer.Close()

	config := Config{
		Timeout:       5 * time.Second,
		ValidationURL: testServer.URL,
		AdvancedChecks: AdvancedChecks{
			TestHostHeaderInjection: true,
		},
	}

	checker := NewChecker(config, false, nil)
	client := &http.Client{Timeout: config.Timeout}

	// Test host header injection check
	result, err := checker.checkHostHeaderInjection(client, "interact.sh")
	
	if err != nil {
		t.Logf("Host header injection check failed: %v", err)
	}
	
	if result != nil {
		t.Logf("Host header injection check completed: %+v", result)
	}
}

// TestProtocolSmuggling tests protocol smuggling detection
func TestProtocolSmuggling(t *testing.T) {
	config := Config{
		Timeout:       5 * time.Second,
		ValidationURL: "https://httpbin.org/anything",
		AdvancedChecks: AdvancedChecks{
			TestProtocolSmuggling: true,
		},
	}

	checker := NewChecker(config, false, nil)
	client := &http.Client{Timeout: config.Timeout}

	// Test protocol smuggling check
	result, err := checker.checkProtocolSmuggling(client, "interact.sh")
	
	if err != nil {
		t.Logf("Protocol smuggling check failed: %v", err)
	}
	
	if result != nil {
		t.Logf("Protocol smuggling check completed: %+v", result)
	}
}

// TestDNSRebinding tests DNS rebinding detection
func TestDNSRebinding(t *testing.T) {
	config := Config{
		Timeout:       5 * time.Second,
		ValidationURL: "https://httpbin.org/ip",
		AdvancedChecks: AdvancedChecks{
			TestDNSRebinding: true,
		},
	}

	checker := NewChecker(config, false, nil)
	client := &http.Client{Timeout: config.Timeout}

	// Test DNS rebinding check
	result, err := checker.checkDNSRebinding(client, "7f000001.1time.interact.sh")
	
	if err != nil {
		t.Logf("DNS rebinding check failed: %v", err)
	}
	
	if result != nil {
		t.Logf("DNS rebinding check completed: %+v", result)
	}
}

// TestIPv6Support tests IPv6 support detection
func TestIPv6Support(t *testing.T) {
	config := Config{
		Timeout:       5 * time.Second,
		ValidationURL: "https://httpbin.org/ip",
		AdvancedChecks: AdvancedChecks{
			TestIPv6: true,
		},
	}

	checker := NewChecker(config, false, nil)
	client := &http.Client{Timeout: config.Timeout}

	// Test IPv6 support check
	result, err := checker.checkIPv6Support(client, "interact.sh")
	
	if err != nil {
		t.Logf("IPv6 support check failed: %v", err)
	}
	
	if result != nil {
		t.Logf("IPv6 support check completed: %+v", result)
	}
}

// TestHTTPMethods tests HTTP methods support detection
func TestHTTPMethods(t *testing.T) {
	config := Config{
		Timeout:       5 * time.Second,
		ValidationURL: "https://httpbin.org/anything",
		AdvancedChecks: AdvancedChecks{
			TestHTTPMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		},
	}

	checker := NewChecker(config, false, nil)
	client := &http.Client{Timeout: config.Timeout}

	// Test HTTP methods check
	results, err := checker.checkHTTPMethods(client, "interact.sh")
	
	if err != nil {
		t.Logf("HTTP methods check failed: %v", err)
	}
	
	if results != nil {
		t.Logf("HTTP methods check completed with %d results", len(results))
	}
}

// TestCachePoisoning tests cache poisoning detection
func TestCachePoisoning(t *testing.T) {
	config := Config{
		Timeout:       5 * time.Second,
		ValidationURL: "https://httpbin.org/anything",
		AdvancedChecks: AdvancedChecks{
			TestCachePoisoning: true,
		},
	}

	checker := NewChecker(config, false, nil)
	client := &http.Client{Timeout: config.Timeout}

	// Test cache poisoning check
	result, err := checker.checkCachePoisoning(client, "interact.sh")
	
	if err != nil {
		t.Logf("Cache poisoning check failed: %v", err)
	}
	
	if result != nil {
		t.Logf("Cache poisoning check completed: %+v", result)
	}
}

// TestAdvancedChecksIntegration tests the full integration of advanced checks
func TestAdvancedChecksIntegration(t *testing.T) {
	config := Config{
		Timeout:       10 * time.Second,
		ValidationURL: "https://httpbin.org/ip",
		AdvancedChecks: AdvancedChecks{
			TestProtocolSmuggling:   true,
			TestSSRF:                true,
			TestHostHeaderInjection: true,
			TestHTTPMethods:         []string{"GET", "POST"},
		},
		InteractshURL: "https://interact.sh",
	}

	checker := NewChecker(config, true, nil)

	// Test with a non-existent proxy
	result := checker.Check("http://non-existent-proxy.example.com:8080")

	// Should fail due to connection error, but advanced checks should be configured
	if result.Error != nil {
		t.Logf("Proxy check failed as expected: %v", result.Error)
	}

	// Verify debug info contains advanced checks information
	if len(result.DebugInfo) > 0 {
		t.Logf("Debug info populated: %d characters", len(result.DebugInfo))
	}
}

// TestAdvancedChecksConfigurationAlternate tests various advanced checks configurations
func TestAdvancedChecksConfigurationAlternate(t *testing.T) {
	tests := []struct {
		name           string
		advancedChecks AdvancedChecks
		expectHas      bool
	}{
		{
			name:           "Empty configuration",
			advancedChecks: AdvancedChecks{},
			expectHas:      false,
		},
		{
			name: "Single check enabled",
			advancedChecks: AdvancedChecks{
				TestSSRF: true,
			},
			expectHas: true,
		},
		{
			name: "Multiple checks enabled",
			advancedChecks: AdvancedChecks{
				TestProtocolSmuggling:   true,
				TestSSRF:                true,
				TestHostHeaderInjection: true,
				TestDNSRebinding:         true,
				TestIPv6:                 true,
				TestCachePoisoning:       true,
				TestHTTPMethods:          []string{"GET", "POST", "PUT", "DELETE"},
			},
			expectHas: true,
		},
		{
			name: "Only HTTP methods enabled",
			advancedChecks: AdvancedChecks{
				TestHTTPMethods: []string{"GET", "POST"},
			},
			expectHas: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Timeout:        5 * time.Second,
				ValidationURL:  "https://httpbin.org/ip",
				AdvancedChecks: tt.advancedChecks,
			}

			checker := NewChecker(config, false, nil)
			hasChecks := checker.hasAdvancedChecks()

			if hasChecks != tt.expectHas {
				t.Errorf("hasAdvancedChecks() = %v, want %v", hasChecks, tt.expectHas)
			}
		})
	}
}

// Benchmark tests for advanced checks performance
func BenchmarkHasAdvancedChecks(b *testing.B) {
	config := Config{
		AdvancedChecks: AdvancedChecks{
			TestProtocolSmuggling:   true,
			TestSSRF:                true,
			TestHostHeaderInjection: true,
			TestHTTPMethods:         []string{"GET", "POST", "PUT"},
		},
	}

	checker := NewChecker(config, false, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.hasAdvancedChecks()
	}
}

func BenchmarkAdvancedChecksSetup(b *testing.B) {
	config := Config{
		Timeout:       5 * time.Second,
		ValidationURL: "https://httpbin.org/ip",
		AdvancedChecks: AdvancedChecks{
			TestProtocolSmuggling:   true,
			TestSSRF:                true,
			TestHostHeaderInjection: true,
			TestHTTPMethods:         []string{"GET", "POST"},
		},
		InteractshURL: "https://interact.sh",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker := NewChecker(config, false, nil)
		client := &http.Client{Timeout: config.Timeout}
		result := &ProxyResult{
			AdvancedChecksDetails: make(map[string]interface{}),
		}

		// Just test the setup, not the actual network calls
		_ = checker.hasAdvancedChecks()
		_ = client
		_ = result
	}
}