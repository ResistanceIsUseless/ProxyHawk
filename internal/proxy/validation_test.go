package proxy

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestValidateResponseEdgeCases tests edge cases in response validation
func TestValidateResponseEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		statusCode     int
		body           string
		expectedResult bool
		description    string
	}{
		{
			name: "Empty response body with zero min bytes",
			config: Config{
				MinResponseBytes:   0,
				DisallowedKeywords: []string{},
			},
			statusCode:     200,
			body:           "",
			expectedResult: true,
			description:    "Should pass with zero min bytes requirement",
		},
		{
			name: "Exact minimum response bytes",
			config: Config{
				MinResponseBytes:   10,
				DisallowedKeywords: []string{},
			},
			statusCode:     200,
			body:           "1234567890", // Exactly 10 bytes
			expectedResult: true,
			description:    "Should pass with exact minimum bytes",
		},
		{
			name: "One byte less than minimum",
			config: Config{
				MinResponseBytes:   10,
				DisallowedKeywords: []string{},
			},
			statusCode:     200,
			body:           "123456789", // 9 bytes
			expectedResult: false,
			description:    "Should fail with one byte less than minimum",
		},
		{
			name: "Case sensitive keyword matching",
			config: Config{
				MinResponseBytes:   1,
				DisallowedKeywords: []string{"ERROR"},
			},
			statusCode:     200,
			body:           "This is an error message",
			expectedResult: true,
			description:    "Should pass as keyword matching is case sensitive",
		},
		{
			name: "Multiple disallowed keywords - first match",
			config: Config{
				MinResponseBytes:   1,
				DisallowedKeywords: []string{"denied", "blocked", "forbidden"},
			},
			statusCode:     200,
			body:           "Access denied by proxy server",
			expectedResult: false,
			description:    "Should fail on first matching keyword",
		},
		{
			name: "Multiple disallowed keywords - last match",
			config: Config{
				MinResponseBytes:   1,
				DisallowedKeywords: []string{"denied", "blocked", "forbidden"},
			},
			statusCode:     200,
			body:           "Request forbidden by policy",
			expectedResult: false,
			description:    "Should fail on last matching keyword",
		},
		{
			name: "Boundary status codes",
			config: Config{
				MinResponseBytes:   1,
				DisallowedKeywords: []string{},
			},
			statusCode:     399,
			body:           "OK",
			expectedResult: true,
			description:    "Status 399 should pass (< 400)",
		},
		{
			name: "Boundary status codes - 400",
			config: Config{
				MinResponseBytes:   1,
				DisallowedKeywords: []string{},
			},
			statusCode:     400,
			body:           "Bad Request",
			expectedResult: false,
			description:    "Status 400 should fail (>= 400)",
		},
		{
			name: "Large response body",
			config: Config{
				MinResponseBytes:   100,
				DisallowedKeywords: []string{"error"},
			},
			statusCode:     200,
			body:           string(make([]byte, 10000)), // 10KB of null bytes
			expectedResult: true,
			description:    "Large response should pass validation",
		},
		{
			name: "Unicode in response body",
			config: Config{
				MinResponseBytes:   10,
				DisallowedKeywords: []string{"錯誤"}, // "error" in Chinese
			},
			statusCode:     200,
			body:           "操作成功完成，沒有錯誤發生",
			expectedResult: false,
			description:    "Unicode keyword matching should work",
		},
		{
			name: "Keyword at beginning of response",
			config: Config{
				MinResponseBytes:   1,
				DisallowedKeywords: []string{"ERROR:"},
			},
			statusCode:     200,
			body:           "ERROR: Authentication failed",
			expectedResult: false,
			description:    "Keyword at beginning should be detected",
		},
		{
			name: "Keyword at end of response",
			config: Config{
				MinResponseBytes:   1,
				DisallowedKeywords: []string{"FAILED"},
			},
			statusCode:     200,
			body:           "Connection attempt FAILED",
			expectedResult: false,
			description:    "Keyword at end should be detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(tt.config, false, nil)
			resp := &http.Response{StatusCode: tt.statusCode}
			body := []byte(tt.body)

			result := checker.validateResponse(resp, body)
			if result != tt.expectedResult {
				t.Errorf("%s: Expected %v, got %v", tt.description, tt.expectedResult, result)
			}
		})
	}
}

// TestRateLimitingEdgeCases tests edge cases in rate limiting
func TestRateLimitingEdgeCases(t *testing.T) {
	t.Run("Rate limiting disabled", func(t *testing.T) {
		config := Config{RateLimitEnabled: false}
		checker := NewChecker(config, false, nil)
		result := &ProxyResult{}

		start := time.Now()
		checker.applyRateLimit("example.com", result)
		elapsed := time.Since(start)

		if elapsed > 10*time.Millisecond {
			t.Errorf("Rate limiting should be bypassed when disabled, but took %v", elapsed)
		}
	})

	t.Run("Zero rate limit delay", func(t *testing.T) {
		config := Config{
			RateLimitEnabled: true,
			RateLimitDelay:   0,
			RateLimitPerHost: true,
		}
		checker := NewChecker(config, false, nil)
		result := &ProxyResult{}

		// First call
		checker.applyRateLimit("example.com", result)

		// Second call should not be delayed with zero delay
		start := time.Now()
		checker.applyRateLimit("example.com", result)
		elapsed := time.Since(start)

		if elapsed > 10*time.Millisecond {
			t.Errorf("Zero delay should not cause waiting, but took %v", elapsed)
		}
	})

	t.Run("Global rate limiting", func(t *testing.T) {
		config := Config{
			RateLimitEnabled: true,
			RateLimitDelay:   50 * time.Millisecond,
			RateLimitPerHost: false, // Global rate limiting
		}
		checker := NewChecker(config, false, nil)
		result := &ProxyResult{}

		// First call
		checker.applyRateLimit("host1.com", result)

		// Second call to different host should still be rate limited
		start := time.Now()
		checker.applyRateLimit("host2.com", result)
		elapsed := time.Since(start)

		if elapsed < 40*time.Millisecond {
			t.Errorf("Global rate limiting should affect different hosts, but only took %v", elapsed)
		}
	})

	t.Run("Rate limiting with debug info", func(t *testing.T) {
		config := Config{
			RateLimitEnabled: true,
			RateLimitDelay:   10 * time.Millisecond,
			RateLimitPerHost: true,
		}
		checker := NewChecker(config, true, nil) // Enable debug
		result := &ProxyResult{DebugInfo: ""}

		// First call
		checker.applyRateLimit("example.com", result)

		// Check debug info was added
		if len(result.DebugInfo) == 0 {
			t.Error("Expected debug info to be added")
		}

		// Second call should add more debug info
		originalDebugLen := len(result.DebugInfo)
		checker.applyRateLimit("example.com", result)

		if len(result.DebugInfo) <= originalDebugLen {
			t.Error("Expected more debug info to be added on second call")
		}
	})
}

// TestPerProxyRateLimitingEdgeCases tests edge cases in per-proxy rate limiting
func TestPerProxyRateLimitingEdgeCases(t *testing.T) {
	t.Run("Empty proxy URL", func(t *testing.T) {
		config := Config{
			RateLimitEnabled:  true,
			RateLimitDelay:    50 * time.Millisecond,
			RateLimitPerProxy: true,
		}
		checker := NewChecker(config, false, nil)
		result := &ProxyResult{DebugInfo: ""}

		// Should not crash with empty proxy URL
		start := time.Now()
		checker.applyProxyRateLimit("", result)
		elapsed := time.Since(start)

		if elapsed > 10*time.Millisecond {
			t.Errorf("Empty proxy URL should not cause delay, but took %v", elapsed)
		}
	})

	t.Run("Same proxy different ports", func(t *testing.T) {
		config := Config{
			RateLimitEnabled:  true,
			RateLimitDelay:    50 * time.Millisecond,
			RateLimitPerProxy: true,
		}
		checker := NewChecker(config, false, nil)
		result1 := &ProxyResult{DebugInfo: ""}
		result2 := &ProxyResult{DebugInfo: ""}

		// First proxy on port 8080
		checker.applyProxyRateLimit("http://proxy.example.com:8080", result1)

		// Same proxy on port 8081 should not be rate limited
		start := time.Now()
		checker.applyProxyRateLimit("http://proxy.example.com:8081", result2)
		elapsed := time.Since(start)

		if elapsed > 10*time.Millisecond {
			t.Errorf("Different ports should not be rate limited together, but took %v", elapsed)
		}
	})

	t.Run("Different protocols same host", func(t *testing.T) {
		config := Config{
			RateLimitEnabled:  true,
			RateLimitDelay:    50 * time.Millisecond,
			RateLimitPerProxy: true,
		}
		checker := NewChecker(config, false, nil)
		result1 := &ProxyResult{DebugInfo: ""}
		result2 := &ProxyResult{DebugInfo: ""}

		// HTTP proxy
		checker.applyProxyRateLimit("http://proxy.example.com:8080", result1)

		// HTTPS proxy same host should not be rate limited
		start := time.Now()
		checker.applyProxyRateLimit("https://proxy.example.com:8080", result2)
		elapsed := time.Since(start)

		if elapsed > 10*time.Millisecond {
			t.Errorf("Different protocols should not be rate limited together, but took %v", elapsed)
		}
	})
}

// TestRateLimitingConcurrency tests rate limiting under concurrent access
func TestRateLimitingConcurrency(t *testing.T) {
	config := Config{
		RateLimitEnabled: true,
		RateLimitDelay:   10 * time.Millisecond,
		RateLimitPerHost: true,
	}
	checker := NewChecker(config, false, nil)

	const numGoroutines = 10
	results := make(chan time.Duration, numGoroutines)

	// Launch multiple goroutines accessing rate limiter concurrently
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			result := &ProxyResult{}
			start := time.Now()
			checker.applyRateLimit("example.com", result)
			elapsed := time.Since(start)
			results <- elapsed
		}(i)
	}

	// Collect results
	var totalTime time.Duration
	for i := 0; i < numGoroutines; i++ {
		elapsed := <-results
		totalTime += elapsed
	}

	// With proper rate limiting, we should see significant delays
	// (not all requests can complete immediately)
	if totalTime < 50*time.Millisecond {
		t.Errorf("Expected significant delays with concurrent rate limiting, but total time was %v", totalTime)
	}
}

// TestValidateResponsePerformance tests response validation performance
func TestValidateResponsePerformance(t *testing.T) {
	config := Config{
		MinResponseBytes:   100,
		DisallowedKeywords: []string{"error", "denied", "blocked", "forbidden", "timeout"},
	}
	checker := NewChecker(config, false, nil)

	// Create large response body
	largeBody := make([]byte, 100000) // 100KB
	for i := range largeBody {
		largeBody[i] = 'a'
	}

	resp := &http.Response{StatusCode: 200}

	// Measure validation time
	start := time.Now()
	for i := 0; i < 1000; i++ {
		checker.validateResponse(resp, largeBody)
	}
	elapsed := time.Since(start)

	// Should be able to validate 1000 large responses in reasonable time
	if elapsed > 100*time.Millisecond {
		t.Errorf("Response validation took too long: %v for 1000 iterations", elapsed)
	}
}

// TestRateLimiterMemoryUsage tests that rate limiter doesn't leak memory
func TestRateLimiterMemoryUsage(t *testing.T) {
	config := Config{
		RateLimitEnabled: true,
		RateLimitDelay:   1 * time.Millisecond,
		RateLimitPerHost: true,
	}
	checker := NewChecker(config, false, nil)

	// Add many different hosts to rate limiter
	result := &ProxyResult{}
	for i := 0; i < 10000; i++ {
		host := fmt.Sprintf("host%d.example.com", i)
		checker.applyRateLimit(host, result)
	}

	// Rate limiter should contain entries for all hosts
	checker.rateLimiterLock.Lock()
	entryCount := len(checker.rateLimiter)
	checker.rateLimiterLock.Unlock()

	if entryCount != 10000 {
		t.Errorf("Expected 10000 rate limiter entries, got %d", entryCount)
	}

	// In a real implementation, you might want to test memory cleanup
	// This is a basic test to ensure the data structure grows as expected
}

// TestRateLimitingTimePrecision tests rate limiting time precision
func TestRateLimitingTimePrecision(t *testing.T) {
	config := Config{
		RateLimitEnabled: true,
		RateLimitDelay:   5 * time.Millisecond, // Very short delay
		RateLimitPerHost: true,
	}
	checker := NewChecker(config, false, nil)
	result := &ProxyResult{}

	// First call
	checker.applyRateLimit("example.com", result)

	// Measure second call timing
	start := time.Now()
	checker.applyRateLimit("example.com", result)
	elapsed := time.Since(start)

	// Should be close to the configured delay (within 2ms tolerance)
	expectedDelay := 5 * time.Millisecond
	tolerance := 2 * time.Millisecond

	if elapsed < expectedDelay-tolerance || elapsed > expectedDelay+tolerance*3 {
		t.Errorf("Rate limiting precision issue: expected ~%v, got %v", expectedDelay, elapsed)
	}
}