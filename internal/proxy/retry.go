package proxy

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RetryConfig holds configuration for the retry mechanism
type RetryConfig struct {
	Enabled       bool
	MaxRetries    int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
	RetryableErrors []string
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		Enabled:       false, // Disabled by default for backward compatibility
		MaxRetries:    3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		RetryableErrors: []string{
			"connection refused",
			"connection timed out",
			"connection reset",
			"temporary failure",
			"network unreachable",
			"host unreachable",
			"no route to host",
			"operation timed out",
			"dial tcp",
			"dial udp",
			"context deadline exceeded",
			"i/o timeout",
			"EOF",
		},
	}
}

// isRetryableError checks if an error should trigger a retry
func (c *Checker) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errorText := strings.ToLower(err.Error())
	
	// Check custom retryable error patterns from config
	for _, pattern := range c.config.RetryableErrors {
		if strings.Contains(errorText, strings.ToLower(pattern)) {
			return true
		}
	}
	
	// Check default retryable error patterns
	defaultConfig := DefaultRetryConfig()
	for _, pattern := range defaultConfig.RetryableErrors {
		if strings.Contains(errorText, pattern) {
			return true
		}
	}
	
	// Check for specific error types
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout() || netErr.Temporary()
	}
	
	// Check for URL errors (often DNS or connection issues)
	if _, ok := err.(*url.Error); ok {
		return true
	}
	
	return false
}

// calculateBackoffDelay calculates the delay for a retry attempt using exponential backoff
func (c *Checker) calculateBackoffDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return c.config.InitialDelay
	}
	
	// Calculate exponential backoff: initialDelay * (backoffFactor ^ attempt)
	delay := float64(c.config.InitialDelay) * math.Pow(c.config.BackoffFactor, float64(attempt))
	
	// Add jitter to avoid thundering herd (±25% randomization)
	jitter := 0.75 + (0.5 * (float64(time.Now().UnixNano()%1000) / 1000.0))
	delay *= jitter
	
	// Cap at maximum delay
	if time.Duration(delay) > c.config.MaxDelay {
		delay = float64(c.config.MaxDelay)
	}
	
	return time.Duration(delay)
}

// executeWithRetry executes a function with retry logic and exponential backoff
func (c *Checker) executeWithRetry(ctx context.Context, operation func() error, operationName string, result *ProxyResult) error {
	// If retries are disabled, execute once
	if !c.config.RetryEnabled {
		return operation()
	}
	
	var lastErr error
	maxAttempts := c.config.MaxRetries + 1 // +1 for the initial attempt
	
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// If this is a retry attempt, wait with exponential backoff
		if attempt > 0 {
			delay := c.calculateBackoffDelay(attempt - 1)
			
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[RETRY] Attempt %d/%d for %s failed: %v\n", 
					attempt, c.config.MaxRetries, operationName, lastErr)
				result.DebugInfo += fmt.Sprintf("[RETRY] Waiting %v before retry %d...\n", delay, attempt)
			}
			
			// Use context-aware sleep
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// Continue to retry
			}
		}
		
		// Execute the operation
		if c.debug && attempt > 0 {
			result.DebugInfo += fmt.Sprintf("[RETRY] Executing %s (attempt %d/%d)\n", 
				operationName, attempt+1, maxAttempts)
		}
		
		err := operation()
		
		// If successful, return immediately
		if err == nil {
			if c.debug && attempt > 0 {
				result.DebugInfo += fmt.Sprintf("[RETRY] %s succeeded on attempt %d\n", 
					operationName, attempt+1)
			}
			return nil
		}
		
		lastErr = err
		
		// Check if this error is retryable
		if !c.isRetryableError(err) {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[RETRY] %s failed with non-retryable error: %v\n", 
					operationName, err)
			}
			return err
		}
		
		// If this was the last attempt, don't retry
		if attempt == maxAttempts-1 {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[RETRY] %s failed after %d attempts, giving up: %v\n", 
					operationName, maxAttempts, err)
			}
			break
		}
	}
	
	return lastErr
}

// makeRequestWithRetry wraps makeRequest with retry logic
func (c *Checker) makeRequestWithRetry(client *http.Client, urlStr string, result *ProxyResult) (*http.Response, error) {
	var response *http.Response
	
	// Create a context for the entire retry operation (separate from individual request timeouts)
	ctx, cancel := context.WithTimeout(context.Background(), c.config.Timeout*time.Duration(c.config.MaxRetries+1))
	defer cancel()
	
	operation := func() error {
		resp, err := c.makeRequest(client, urlStr, result)
		if err != nil {
			return err
		}
		response = resp
		return nil
	}
	
	err := c.executeWithRetry(ctx, operation, fmt.Sprintf("request to %s", urlStr), result)
	if err != nil {
		return nil, err
	}
	
	return response, nil
}

// validateRetryConfig validates and normalizes retry configuration
func (c *Checker) validateRetryConfig() {
	// Set defaults if not configured
	if c.config.MaxRetries <= 0 {
		c.config.MaxRetries = 3
	}
	if c.config.InitialDelay <= 0 {
		c.config.InitialDelay = 1 * time.Second
	}
	if c.config.MaxDelay <= 0 {
		c.config.MaxDelay = 30 * time.Second
	}
	if c.config.BackoffFactor <= 1.0 {
		c.config.BackoffFactor = 2.0
	}
	
	// Ensure max delay is at least initial delay
	if c.config.MaxDelay < c.config.InitialDelay {
		c.config.MaxDelay = c.config.InitialDelay
	}
	
	// Cap max retries to prevent excessive attempts
	if c.config.MaxRetries > 10 {
		c.config.MaxRetries = 10
	}
	
	// If no custom retryable errors specified, use defaults
	if len(c.config.RetryableErrors) == 0 {
		defaultConfig := DefaultRetryConfig()
		c.config.RetryableErrors = defaultConfig.RetryableErrors
	}
}

// RetryStats represents statistics about retry attempts
type RetryStats struct {
	TotalAttempts   int
	SuccessfulRetries int
	FailedRetries   int
	TotalDelay      time.Duration
	AverageDelay    time.Duration
}

// GetRetryStats returns retry statistics (placeholder for future metrics integration)
func (c *Checker) GetRetryStats() RetryStats {
	// This could be expanded to track actual retry statistics
	return RetryStats{}
}