package proxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestDefaultRetryConfig tests the default retry configuration
func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.Enabled {
		t.Error("Expected default retry config to be disabled")
	}

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", config.MaxRetries)
	}

	if config.InitialDelay != 1*time.Second {
		t.Errorf("Expected InitialDelay to be 1s, got %v", config.InitialDelay)
	}

	if config.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay to be 30s, got %v", config.MaxDelay)
	}

	if config.BackoffFactor != 2.0 {
		t.Errorf("Expected BackoffFactor to be 2.0, got %f", config.BackoffFactor)
	}

	if len(config.RetryableErrors) == 0 {
		t.Error("Expected default retryable errors to be configured")
	}

	// Check that common error patterns are included
	expectedPatterns := []string{"connection refused", "connection timed out", "network unreachable"}
	for _, pattern := range expectedPatterns {
		found := false
		for _, retryableError := range config.RetryableErrors {
			if retryableError == pattern {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected retryable errors to include '%s'", pattern)
		}
	}
}

// TestIsRetryableError tests the error classification logic
func TestIsRetryableError(t *testing.T) {
	config := Config{
		RetryEnabled: true,
		RetryableErrors: []string{
			"custom error pattern",
			"temporary failure",
		},
	}
	
	checker := NewChecker(config, false)

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "Connection refused error",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "Connection timed out error",
			err:      errors.New("connection timed out"),
			expected: true,
		},
		{
			name:     "Custom retryable error",
			err:      errors.New("custom error pattern occurred"),
			expected: true,
		},
		{
			name:     "Case insensitive matching",
			err:      errors.New("CONNECTION REFUSED"),
			expected: true,
		},
		{
			name:     "Non-retryable error",
			err:      errors.New("invalid credentials"),
			expected: false,
		},
		{
			name:     "Network timeout error",
			err:      &net.OpError{Op: "dial", Net: "tcp", Err: &timeoutError{}},
			expected: true,
		},
		{
			name:     "URL error",
			err:      &url.Error{Op: "Get", URL: "http://example.com", Err: errors.New("connection refused")},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

// timeoutError is a helper type that implements net.Error for testing
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

// TestCalculateBackoffDelay tests the exponential backoff calculation
func TestCalculateBackoffDelay(t *testing.T) {
	config := Config{
		InitialDelay:  1 * time.Second,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
	}
	
	checker := NewChecker(config, false)

	tests := []struct {
		name     string
		attempt  int
		minDelay time.Duration
		maxDelay time.Duration
	}{
		{
			name:     "Initial attempt (attempt 0)",
			attempt:  0,
			minDelay: 750 * time.Millisecond,  // With jitter: 1s * 0.75
			maxDelay: 1250 * time.Millisecond, // With jitter: 1s * 1.25
		},
		{
			name:     "First retry (attempt 1)",
			attempt:  1,
			minDelay: 1500 * time.Millisecond, // With jitter: 2s * 0.75
			maxDelay: 2500 * time.Millisecond, // With jitter: 2s * 1.25
		},
		{
			name:     "Second retry (attempt 2)",
			attempt:  2,
			minDelay: 3 * time.Second,    // With jitter: 4s * 0.75
			maxDelay: 5 * time.Second,    // With jitter: 4s * 1.25
		},
		{
			name:     "Capped at max delay",
			attempt:  10,
			minDelay: 7500 * time.Millisecond, // Should be capped at 10s with jitter
			maxDelay: 10 * time.Second,        // Max delay
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := checker.calculateBackoffDelay(tt.attempt)
			
			if delay < tt.minDelay || delay > tt.maxDelay {
				t.Errorf("calculateBackoffDelay(%d) = %v, want between %v and %v", 
					tt.attempt, delay, tt.minDelay, tt.maxDelay)
			}
		})
	}
}

// TestExecuteWithRetry tests the retry execution logic
func TestExecuteWithRetry(t *testing.T) {
	tests := []struct {
		name          string
		config        Config
		failAttempts  int  // Number of attempts that should fail
		expectedCalls int  // Expected total number of calls
		shouldSucceed bool // Whether the operation should ultimately succeed
	}{
		{
			name: "Success on first attempt",
			config: Config{
				RetryEnabled:  true,
				MaxRetries:    3,
				InitialDelay:  1 * time.Millisecond,
				BackoffFactor: 2.0,
				RetryableErrors: []string{"test error"},
			},
			failAttempts:  0,
			expectedCalls: 1,
			shouldSucceed: true,
		},
		{
			name: "Success on second attempt",
			config: Config{
				RetryEnabled:  true,
				MaxRetries:    3,
				InitialDelay:  1 * time.Millisecond,
				BackoffFactor: 2.0,
				RetryableErrors: []string{"test error"},
			},
			failAttempts:  1,
			expectedCalls: 2,
			shouldSucceed: true,
		},
		{
			name: "Fail all attempts",
			config: Config{
				RetryEnabled:  true,
				MaxRetries:    2,
				InitialDelay:  1 * time.Millisecond,
				BackoffFactor: 2.0,
				RetryableErrors: []string{"test error"},
			},
			failAttempts:  5, // Fail more than max retries
			expectedCalls: 3, // Initial + 2 retries
			shouldSucceed: false,
		},
		{
			name: "Retries disabled",
			config: Config{
				RetryEnabled:  false,
				MaxRetries:    3,
				InitialDelay:  1 * time.Millisecond,
				BackoffFactor: 2.0,
			},
			failAttempts:  1,
			expectedCalls: 1, // Only initial attempt
			shouldSucceed: false,
		},
		{
			name: "Non-retryable error",
			config: Config{
				RetryEnabled:  true,
				MaxRetries:    3,
				InitialDelay:  1 * time.Millisecond,
				BackoffFactor: 2.0,
				RetryableErrors: []string{"retryable pattern only"},
			},
			failAttempts:  1,
			expectedCalls: 1, // Should stop after first non-retryable error
			shouldSucceed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(tt.config, true) // Enable debug for better test coverage
			result := &ProxyResult{DebugInfo: ""}
			
			callCount := 0
			operation := func() error {
				callCount++
				if callCount <= tt.failAttempts {
					if tt.name == "Non-retryable error" {
						return errors.New("custom non-retryable error message") // This won't match any patterns
					}
					return errors.New("test error") // This will match if configured
				}
				return nil // Success
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := checker.executeWithRetry(ctx, operation, "test operation", result)

			if callCount != tt.expectedCalls {
				t.Errorf("Expected %d calls, got %d", tt.expectedCalls, callCount)
			}

			if tt.shouldSucceed && err != nil {
				t.Errorf("Expected success, got error: %v", err)
			}

			if !tt.shouldSucceed && err == nil {
				t.Error("Expected error, got success")
			}

			// Check debug info when retries are enabled
			if tt.config.RetryEnabled && tt.failAttempts > 0 && len(result.DebugInfo) == 0 {
				t.Error("Expected debug info to be populated")
			}
		})
	}
}

// TestMakeRequestWithRetry tests the request retry wrapper
func TestMakeRequestWithRetry(t *testing.T) {
	// Track request attempts
	attemptCount := 0
	
	// Create a test server that fails initially then succeeds
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount <= 2 { // Fail first 2 attempts
			// Simulate connection refused by closing connection
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
			}
			return
		}
		// Succeed on 3rd attempt
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ip": "1.2.3.4"}`))
	}))
	defer testServer.Close()

	config := Config{
		Timeout:       5 * time.Second,
		RetryEnabled:  true,
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
		RetryableErrors: []string{
			"connection refused",
			"EOF",
			"broken pipe",
		},
	}

	checker := NewChecker(config, true)
	result := &ProxyResult{DebugInfo: ""}
	client := &http.Client{Timeout: config.Timeout}

	// This should succeed on the 3rd attempt
	resp, err := checker.makeRequestWithRetry(client, testServer.URL, result)

	if err != nil {
		t.Errorf("Expected success after retries, got error: %v", err)
	}

	if resp == nil {
		t.Error("Expected response, got nil")
	} else {
		resp.Body.Close()
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}

	// Check that debug info was populated
	if !strings.Contains(result.DebugInfo, "[RETRY]") {
		t.Error("Expected retry debug info to be present")
	}
}

// TestValidateRetryConfig tests the retry configuration validation
func TestValidateRetryConfig(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		expectedMaxRetries    int
		expectedInitialDelay  time.Duration
		expectedMaxDelay      time.Duration
		expectedBackoffFactor float64
	}{
		{
			name: "Valid configuration",
			config: Config{
				MaxRetries:    5,
				InitialDelay:  2 * time.Second,
				MaxDelay:      60 * time.Second,
				BackoffFactor: 1.5,
				RetryableErrors: []string{"test"},
			},
			expectedMaxRetries:    5,
			expectedInitialDelay:  2 * time.Second,
			expectedMaxDelay:      60 * time.Second,
			expectedBackoffFactor: 1.5,
		},
		{
			name: "Zero values should get defaults",
			config: Config{
				MaxRetries:    0,
				InitialDelay:  0,
				MaxDelay:      0,
				BackoffFactor: 0,
			},
			expectedMaxRetries:    3,
			expectedInitialDelay:  1 * time.Second,
			expectedMaxDelay:      30 * time.Second,
			expectedBackoffFactor: 2.0,
		},
		{
			name: "Negative values should get defaults",
			config: Config{
				MaxRetries:    -1,
				InitialDelay:  -1 * time.Second,
				MaxDelay:      -1 * time.Second,
				BackoffFactor: -1,
			},
			expectedMaxRetries:    3,
			expectedInitialDelay:  1 * time.Second,
			expectedMaxDelay:      30 * time.Second,
			expectedBackoffFactor: 2.0,
		},
		{
			name: "Max delay less than initial delay",
			config: Config{
				MaxRetries:    3,
				InitialDelay:  10 * time.Second,
				MaxDelay:      5 * time.Second, // Less than initial
				BackoffFactor: 2.0,
			},
			expectedMaxRetries:    3,
			expectedInitialDelay:  10 * time.Second,
			expectedMaxDelay:      10 * time.Second, // Should be set to initial delay
			expectedBackoffFactor: 2.0,
		},
		{
			name: "Excessive max retries should be capped",
			config: Config{
				MaxRetries:    50,
				InitialDelay:  1 * time.Second,
				MaxDelay:      30 * time.Second,
				BackoffFactor: 2.0,
			},
			expectedMaxRetries:    10, // Should be capped
			expectedInitialDelay:  1 * time.Second,
			expectedMaxDelay:      30 * time.Second,
			expectedBackoffFactor: 2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(tt.config, false)

			if checker.config.MaxRetries != tt.expectedMaxRetries {
				t.Errorf("Expected MaxRetries %d, got %d", 
					tt.expectedMaxRetries, checker.config.MaxRetries)
			}

			if checker.config.InitialDelay != tt.expectedInitialDelay {
				t.Errorf("Expected InitialDelay %v, got %v", 
					tt.expectedInitialDelay, checker.config.InitialDelay)
			}

			if checker.config.MaxDelay != tt.expectedMaxDelay {
				t.Errorf("Expected MaxDelay %v, got %v", 
					tt.expectedMaxDelay, checker.config.MaxDelay)
			}

			if checker.config.BackoffFactor != tt.expectedBackoffFactor {
				t.Errorf("Expected BackoffFactor %f, got %f", 
					tt.expectedBackoffFactor, checker.config.BackoffFactor)
			}

			// Check that default retryable errors are set if none provided
			if len(tt.config.RetryableErrors) == 0 && len(checker.config.RetryableErrors) == 0 {
				t.Error("Expected default retryable errors to be set")
			}
		})
	}
}

// TestRetryTimeout tests that retry operations respect context timeouts
func TestRetryTimeout(t *testing.T) {
	config := Config{
		RetryEnabled:  true,
		MaxRetries:    10, // High number to ensure timeout hits first
		InitialDelay:  100 * time.Millisecond,
		BackoffFactor: 2.0,
		RetryableErrors: []string{"test error"},
	}

	checker := NewChecker(config, false)
	result := &ProxyResult{}

	callCount := 0
	operation := func() error {
		callCount++
		time.Sleep(50 * time.Millisecond) // Simulate work
		return errors.New("test error")   // Always fail
	}

	// Short timeout to test cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := checker.executeWithRetry(ctx, operation, "test operation", result)
	elapsed := time.Since(start)

	// Should fail due to context timeout
	if err == nil {
		t.Error("Expected timeout error, got success")
	}

	// Should not take much longer than the timeout
	if elapsed > 500*time.Millisecond {
		t.Errorf("Operation took too long: %v", elapsed)
	}

	// Should have made at least one call but not all 10 retries
	if callCount == 0 {
		t.Error("Expected at least one call")
	}

	if callCount >= 10 {
		t.Errorf("Expected timeout to prevent all retries, but got %d calls", callCount)
	}
}

// Benchmark tests for retry mechanism performance
func BenchmarkCalculateBackoffDelay(b *testing.B) {
	config := Config{
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
	}
	
	checker := NewChecker(config, false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.calculateBackoffDelay(i % 10) // Test different attempt numbers
	}
}

func BenchmarkIsRetryableError(b *testing.B) {
	config := Config{
		RetryableErrors: []string{
			"connection refused",
			"connection timed out",
			"network unreachable",
		},
	}
	
	checker := NewChecker(config, false)
	testError := errors.New("connection refused")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.isRetryableError(testError)
	}
}