package errors

import (
	"fmt"
	"strings"
)

// ErrorCode represents different types of errors that can occur
type ErrorCode int

const (
	// Configuration errors
	ErrorConfigNotFound ErrorCode = iota + 1000
	ErrorConfigInvalid
	ErrorConfigParsingFailed

	// File I/O errors  
	ErrorFileNotFound
	ErrorFileReadFailed
	ErrorFileWriteFailed
	ErrorFileEmpty
	ErrorFileInvalidFormat

	// Network/Connection errors
	ErrorConnectionFailed
	ErrorConnectionTimeout
	ErrorConnectionRefused
	ErrorDNSResolutionFailed
	ErrorTLSHandshakeFailed
	ErrorProxyAuthRequired
	ErrorProxyConnectionFailed

	// HTTP errors
	ErrorHTTPRequestFailed
	ErrorHTTPInvalidResponse
	ErrorHTTPUnexpectedStatus
	ErrorHTTPInvalidHeaders
	ErrorHTTPResponseTooSmall
	ErrorHTTPResponseInvalid

	// Proxy-specific errors
	ErrorProxyInvalidURL
	ErrorProxyUnsupportedType
	ErrorProxyNotWorking
	ErrorProxyValidationFailed
	ErrorProxyRateLimited
	ErrorProxyBlocked

	// Security/Validation errors
	ErrorValidationFailed
	ErrorSecurityViolation
	ErrorMaliciousContent
	ErrorPrivateIPBlocked
	ErrorSuspiciousActivity

	// Advanced check errors
	ErrorAdvancedCheckFailed
	ErrorInteractshFailed
	ErrorDNSRebindingDetected
	ErrorProtocolSmugglingDetected

	// System errors
	ErrorSystemResourceExhausted
	ErrorSystemTimeout
	ErrorSystemShutdown
	ErrorUnexpectedPanic
)

// ProxyError represents a structured error with context and error codes
type ProxyError struct {
	Code      ErrorCode              `json:"code"`
	Message   string                 `json:"message"`
	Operation string                 `json:"operation,omitempty"`
	Proxy     string                 `json:"proxy,omitempty"`
	URL       string                 `json:"url,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Cause     error                  `json:"-"` // Original error, not serialized
}

func (e *ProxyError) Error() string {
	var parts []string
	
	if e.Operation != "" {
		parts = append(parts, fmt.Sprintf("operation=%s", e.Operation))
	}
	if e.Proxy != "" {
		parts = append(parts, fmt.Sprintf("proxy=%s", e.Proxy))
	}
	if e.URL != "" {
		parts = append(parts, fmt.Sprintf("url=%s", e.URL))
	}
	
	context := ""
	if len(parts) > 0 {
		context = fmt.Sprintf(" [%s]", strings.Join(parts, ", "))
	}
	
	result := fmt.Sprintf("[%d] %s%s", e.Code, e.Message, context)
	
	if e.Cause != nil {
		result += fmt.Sprintf(": %v", e.Cause)
	}
	
	return result
}

// Unwrap returns the underlying error for error unwrapping
func (e *ProxyError) Unwrap() error {
	return e.Cause
}

// Is implements error comparison for errors.Is()
func (e *ProxyError) Is(target error) bool {
	if pe, ok := target.(*ProxyError); ok {
		return e.Code == pe.Code
	}
	return false
}

// WithDetail adds a detail to the error
func (e *ProxyError) WithDetail(key string, value interface{}) *ProxyError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithProxy adds proxy context to the error
func (e *ProxyError) WithProxy(proxy string) *ProxyError {
	e.Proxy = proxy
	return e
}

// WithURL adds URL context to the error
func (e *ProxyError) WithURL(url string) *ProxyError {
	e.URL = url
	return e
}

// WithOperation adds operation context to the error
func (e *ProxyError) WithOperation(operation string) *ProxyError {
	e.Operation = operation
	return e
}

// Constructor functions for common error types

// NewConfigError creates a configuration-related error
func NewConfigError(code ErrorCode, message string, cause error) *ProxyError {
	return &ProxyError{
		Code:      code,
		Message:   message,
		Operation: "config",
		Cause:     cause,
	}
}

// NewFileError creates a file I/O related error
func NewFileError(code ErrorCode, message string, filename string, cause error) *ProxyError {
	return &ProxyError{
		Code:      code,
		Message:   message,
		Operation: "file",
		Cause:     cause,
		Details:   map[string]interface{}{"filename": filename},
	}
}

// NewNetworkError creates a network-related error
func NewNetworkError(code ErrorCode, message string, proxy string, cause error) *ProxyError {
	return &ProxyError{
		Code:      code,
		Message:   message,
		Operation: "network",
		Proxy:     proxy,
		Cause:     cause,
	}
}

// NewHTTPError creates an HTTP-related error
func NewHTTPError(code ErrorCode, message string, url string, cause error) *ProxyError {
	return &ProxyError{
		Code:      code,
		Message:   message,
		Operation: "http",
		URL:       url,
		Cause:     cause,
	}
}

// NewProxyError creates a proxy-specific error
func NewProxyError(code ErrorCode, message string, proxy string, cause error) *ProxyError {
	return &ProxyError{
		Code:      code,
		Message:   message,
		Operation: "proxy",
		Proxy:     proxy,
		Cause:     cause,
	}
}

// NewValidationError creates a validation error
func NewValidationError(code ErrorCode, message string, value string, cause error) *ProxyError {
	return &ProxyError{
		Code:      code,
		Message:   message,
		Operation: "validation",
		Cause:     cause,
		Details:   map[string]interface{}{"value": value},
	}
}

// NewAdvancedCheckError creates an advanced security check error
func NewAdvancedCheckError(code ErrorCode, message string, proxy string, checkType string, cause error) *ProxyError {
	return &ProxyError{
		Code:      code,
		Message:   message,
		Operation: "advanced_check",
		Proxy:     proxy,
		Cause:     cause,
		Details:   map[string]interface{}{"check_type": checkType},
	}
}

// NewSystemError creates a system-level error
func NewSystemError(code ErrorCode, message string, cause error) *ProxyError {
	return &ProxyError{
		Code:      code,
		Message:   message,
		Operation: "system",
		Cause:     cause,
	}
}

// Error category checking functions

// IsConfigError checks if the error is configuration-related
func IsConfigError(err error) bool {
	if pe, ok := err.(*ProxyError); ok {
		return pe.Code >= ErrorConfigNotFound && pe.Code <= ErrorConfigParsingFailed
	}
	return false
}

// IsFileError checks if the error is file I/O related
func IsFileError(err error) bool {
	if pe, ok := err.(*ProxyError); ok {
		return pe.Code >= ErrorFileNotFound && pe.Code <= ErrorFileInvalidFormat
	}
	return false
}

// IsNetworkError checks if the error is network-related
func IsNetworkError(err error) bool {
	if pe, ok := err.(*ProxyError); ok {
		return pe.Code >= ErrorConnectionFailed && pe.Code <= ErrorProxyConnectionFailed
	}
	return false
}

// IsHTTPError checks if the error is HTTP-related
func IsHTTPError(err error) bool {
	if pe, ok := err.(*ProxyError); ok {
		return pe.Code >= ErrorHTTPRequestFailed && pe.Code <= ErrorHTTPResponseInvalid
	}
	return false
}

// IsProxyError checks if the error is proxy-specific
func IsProxyError(err error) bool {
	if pe, ok := err.(*ProxyError); ok {
		return pe.Code >= ErrorProxyInvalidURL && pe.Code <= ErrorProxyBlocked
	}
	return false
}

// IsValidationError checks if the error is validation-related
func IsValidationError(err error) bool {
	if pe, ok := err.(*ProxyError); ok {
		return pe.Code >= ErrorValidationFailed && pe.Code <= ErrorSuspiciousActivity
	}
	return false
}

// IsAdvancedCheckError checks if the error is advanced check related
func IsAdvancedCheckError(err error) bool {
	if pe, ok := err.(*ProxyError); ok {
		return pe.Code >= ErrorAdvancedCheckFailed && pe.Code <= ErrorProtocolSmugglingDetected
	}
	return false
}

// IsSystemError checks if the error is system-related
func IsSystemError(err error) bool {
	if pe, ok := err.(*ProxyError); ok {
		return pe.Code >= ErrorSystemResourceExhausted && pe.Code <= ErrorUnexpectedPanic
	}
	return false
}

// IsRetryable determines if an error should trigger a retry
func IsRetryable(err error) bool {
	if pe, ok := err.(*ProxyError); ok {
		switch pe.Code {
		case ErrorConnectionTimeout, 
			 ErrorConnectionRefused,
			 ErrorHTTPRequestFailed,
			 ErrorProxyConnectionFailed,
			 ErrorSystemTimeout:
			return true
		}
	}
	return false
}

// IsCritical determines if an error is critical and should stop processing
func IsCritical(err error) bool {
	if pe, ok := err.(*ProxyError); ok {
		switch pe.Code {
		case ErrorConfigNotFound,
			 ErrorConfigInvalid,
			 ErrorFileNotFound,
			 ErrorSystemResourceExhausted,
			 ErrorSystemShutdown,
			 ErrorUnexpectedPanic:
			return true
		}
	}
	return false
}

// GetErrorCategory returns a human-readable category for the error
func GetErrorCategory(err error) string {
	if pe, ok := err.(*ProxyError); ok {
		switch {
		case IsConfigError(err):
			return "Configuration"
		case IsFileError(err):
			return "File I/O"
		case IsNetworkError(err):
			return "Network"
		case IsHTTPError(err):
			return "HTTP"
		case IsProxyError(err):
			return "Proxy"
		case IsValidationError(err):
			return "Validation"
		case IsAdvancedCheckError(err):
			return "Security Check"
		case IsSystemError(err):
			return "System"
		default:
			return fmt.Sprintf("Unknown (%d)", pe.Code)
		}
	}
	return "Generic"
}