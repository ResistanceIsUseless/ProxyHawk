package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestProxyError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ProxyError
		expected string
	}{
		{
			name: "basic error",
			err: &ProxyError{
				Code:    ErrorConnectionFailed,
				Message: "connection failed",
			},
			expected: "[1008] connection failed",
		},
		{
			name: "error with proxy context",
			err: &ProxyError{
				Code:    ErrorProxyNotWorking,
				Message: "proxy not responding",
				Proxy:   "192.168.1.1:8080",
			},
			expected: "[1023] proxy not responding [proxy=192.168.1.1:8080]",
		},
		{
			name: "error with full context",
			err: &ProxyError{
				Code:      ErrorHTTPRequestFailed,
				Message:   "request failed",
				Operation: "http",
				Proxy:     "proxy.example.com:8080",
				URL:       "https://example.com",
			},
			expected: "[1015] request failed [operation=http, proxy=proxy.example.com:8080, url=https://example.com]",
		},
		{
			name: "error with cause",
			err: &ProxyError{
				Code:    ErrorConnectionTimeout,
				Message: "connection timed out",
				Proxy:   "slow.proxy.com:8080",
				Cause:   fmt.Errorf("dial timeout"),
			},
			expected: "[1009] connection timed out [proxy=slow.proxy.com:8080]: dial timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("ProxyError.Error() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestProxyError_Unwrap(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	proxyErr := &ProxyError{
		Code:    ErrorConnectionFailed,
		Message: "connection failed",
		Cause:   originalErr,
	}

	unwrapped := proxyErr.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("ProxyError.Unwrap() = %v, want %v", unwrapped, originalErr)
	}
}

func TestProxyError_Is(t *testing.T) {
	err1 := &ProxyError{Code: ErrorConnectionFailed, Message: "test"}
	err2 := &ProxyError{Code: ErrorConnectionFailed, Message: "different message"}
	err3 := &ProxyError{Code: ErrorConnectionTimeout, Message: "test"}
	genericErr := fmt.Errorf("generic error")

	// Same error code should match
	if !err1.Is(err2) {
		t.Error("ProxyError.Is() should return true for same error codes")
	}

	// Different error codes should not match
	if err1.Is(err3) {
		t.Error("ProxyError.Is() should return false for different error codes")
	}

	// Generic error should not match
	if err1.Is(genericErr) {
		t.Error("ProxyError.Is() should return false for generic errors")
	}
}

func TestProxyError_WithMethods(t *testing.T) {
	err := &ProxyError{
		Code:    ErrorProxyNotWorking,
		Message: "proxy failed",
	}

	// Test chaining methods
	err.WithProxy("test.proxy.com:8080").
		WithURL("https://example.com").
		WithOperation("test").
		WithDetail("attempts", 3).
		WithDetail("duration", "5s")

	if err.Proxy != "test.proxy.com:8080" {
		t.Errorf("WithProxy() proxy = %v, want %v", err.Proxy, "test.proxy.com:8080")
	}
	if err.URL != "https://example.com" {
		t.Errorf("WithURL() url = %v, want %v", err.URL, "https://example.com")
	}
	if err.Operation != "test" {
		t.Errorf("WithOperation() operation = %v, want %v", err.Operation, "test")
	}
	if err.Details["attempts"] != 3 {
		t.Errorf("WithDetail() attempts = %v, want %v", err.Details["attempts"], 3)
	}
	if err.Details["duration"] != "5s" {
		t.Errorf("WithDetail() duration = %v, want %v", err.Details["duration"], "5s")
	}
}

func TestConstructorFunctions(t *testing.T) {
	tests := []struct {
		name        string
		constructor func() *ProxyError
		wantCode    ErrorCode
		wantOp      string
	}{
		{
			name: "NewConfigError",
			constructor: func() *ProxyError {
				return NewConfigError(ErrorConfigNotFound, "config not found", nil)
			},
			wantCode: ErrorConfigNotFound,
			wantOp:   "config",
		},
		{
			name: "NewFileError",
			constructor: func() *ProxyError {
				return NewFileError(ErrorFileNotFound, "file not found", "test.txt", nil)
			},
			wantCode: ErrorFileNotFound,
			wantOp:   "file",
		},
		{
			name: "NewNetworkError",
			constructor: func() *ProxyError {
				return NewNetworkError(ErrorConnectionFailed, "connection failed", "proxy.com:8080", nil)
			},
			wantCode: ErrorConnectionFailed,
			wantOp:   "network",
		},
		{
			name: "NewHTTPError",
			constructor: func() *ProxyError {
				return NewHTTPError(ErrorHTTPRequestFailed, "request failed", "https://example.com", nil)
			},
			wantCode: ErrorHTTPRequestFailed,
			wantOp:   "http",
		},
		{
			name: "NewProxyError",
			constructor: func() *ProxyError {
				return NewProxyError(ErrorProxyNotWorking, "proxy not working", "proxy.com:8080", nil)
			},
			wantCode: ErrorProxyNotWorking,
			wantOp:   "proxy",
		},
		{
			name: "NewValidationError",
			constructor: func() *ProxyError {
				return NewValidationError(ErrorValidationFailed, "validation failed", "invalid-url", nil)
			},
			wantCode: ErrorValidationFailed,
			wantOp:   "validation",
		},
		{
			name: "NewAdvancedCheckError",
			constructor: func() *ProxyError {
				return NewAdvancedCheckError(ErrorAdvancedCheckFailed, "check failed", "proxy.com:8080", "dns_rebinding", nil)
			},
			wantCode: ErrorAdvancedCheckFailed,
			wantOp:   "advanced_check",
		},
		{
			name: "NewSystemError",
			constructor: func() *ProxyError {
				return NewSystemError(ErrorSystemTimeout, "system timeout", nil)
			},
			wantCode: ErrorSystemTimeout,
			wantOp:   "system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.constructor()
			if err.Code != tt.wantCode {
				t.Errorf("%s code = %v, want %v", tt.name, err.Code, tt.wantCode)
			}
			if err.Operation != tt.wantOp {
				t.Errorf("%s operation = %v, want %v", tt.name, err.Operation, tt.wantOp)
			}
		})
	}
}

func TestErrorCategoryFunctions(t *testing.T) {
	tests := []struct {
		name     string
		err      *ProxyError
		checkFn  func(error) bool
		expected bool
	}{
		{
			name:     "IsConfigError true",
			err:      &ProxyError{Code: ErrorConfigNotFound},
			checkFn:  IsConfigError,
			expected: true,
		},
		{
			name:     "IsConfigError false",
			err:      &ProxyError{Code: ErrorConnectionFailed},
			checkFn:  IsConfigError,
			expected: false,
		},
		{
			name:     "IsFileError true",
			err:      &ProxyError{Code: ErrorFileNotFound},
			checkFn:  IsFileError,
			expected: true,
		},
		{
			name:     "IsNetworkError true",
			err:      &ProxyError{Code: ErrorConnectionTimeout},
			checkFn:  IsNetworkError,
			expected: true,
		},
		{
			name:     "IsHTTPError true",
			err:      &ProxyError{Code: ErrorHTTPRequestFailed},
			checkFn:  IsHTTPError,
			expected: true,
		},
		{
			name:     "IsProxyError true",
			err:      &ProxyError{Code: ErrorProxyNotWorking},
			checkFn:  IsProxyError,
			expected: true,
		},
		{
			name:     "IsValidationError true",
			err:      &ProxyError{Code: ErrorValidationFailed},
			checkFn:  IsValidationError,
			expected: true,
		},
		{
			name:     "IsAdvancedCheckError true",
			err:      &ProxyError{Code: ErrorAdvancedCheckFailed},
			checkFn:  IsAdvancedCheckError,
			expected: true,
		},
		{
			name:     "IsSystemError true",
			err:      &ProxyError{Code: ErrorSystemTimeout},
			checkFn:  IsSystemError,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.checkFn(tt.err)
			if result != tt.expected {
				t.Errorf("%s = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      *ProxyError
		expected bool
	}{
		{
			name:     "retryable connection timeout",
			err:      &ProxyError{Code: ErrorConnectionTimeout},
			expected: true,
		},
		{
			name:     "retryable connection refused",
			err:      &ProxyError{Code: ErrorConnectionRefused},
			expected: true,
		},
		{
			name:     "non-retryable config error",
			err:      &ProxyError{Code: ErrorConfigNotFound},
			expected: false,
		},
		{
			name:     "non-retryable validation error",
			err:      &ProxyError{Code: ErrorValidationFailed},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryable(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetryable() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsCritical(t *testing.T) {
	tests := []struct {
		name     string
		err      *ProxyError
		expected bool
	}{
		{
			name:     "critical config not found",
			err:      &ProxyError{Code: ErrorConfigNotFound},
			expected: true,
		},
		{
			name:     "critical system shutdown",
			err:      &ProxyError{Code: ErrorSystemShutdown},
			expected: true,
		},
		{
			name:     "non-critical connection timeout",
			err:      &ProxyError{Code: ErrorConnectionTimeout},
			expected: false,
		},
		{
			name:     "non-critical proxy not working",
			err:      &ProxyError{Code: ErrorProxyNotWorking},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCritical(tt.err)
			if result != tt.expected {
				t.Errorf("IsCritical() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetErrorCategory(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "config error",
			err:      &ProxyError{Code: ErrorConfigNotFound},
			expected: "Configuration",
		},
		{
			name:     "network error",
			err:      &ProxyError{Code: ErrorConnectionFailed},
			expected: "Network",
		},
		{
			name:     "validation error",
			err:      &ProxyError{Code: ErrorValidationFailed},
			expected: "Validation",
		},
		{
			name:     "generic error",
			err:      errors.New("generic error"),
			expected: "Generic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetErrorCategory(tt.err)
			if result != tt.expected {
				t.Errorf("GetErrorCategory() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestErrorWrapping(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	wrappedErr := &ProxyError{
		Code:    ErrorConnectionFailed,
		Message: "connection failed",
		Cause:   originalErr,
	}

	// Test errors.Is
	if !errors.Is(wrappedErr, wrappedErr) {
		t.Error("errors.Is should work with ProxyError")
	}

	// Test errors.Unwrap
	if !errors.Is(wrappedErr, originalErr) {
		t.Error("errors.Is should find the original error through unwrapping")
	}

	// Test errors.As
	var pe *ProxyError
	if !errors.As(wrappedErr, &pe) {
		t.Error("errors.As should work with ProxyError")
	}
	if pe.Code != ErrorConnectionFailed {
		t.Errorf("errors.As result code = %v, want %v", pe.Code, ErrorConnectionFailed)
	}
}