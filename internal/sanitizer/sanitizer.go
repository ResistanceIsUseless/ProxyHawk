package sanitizer

import (
	"html"
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

// Sanitizer provides XSS protection and content sanitization for output
type Sanitizer struct {
	// Configuration options
	allowHTML bool
	maxLength int
}

// Config represents sanitizer configuration
type Config struct {
	AllowHTML bool // Whether to allow HTML tags (default: false)
	MaxLength int  // Maximum length for string fields (default: 1000)
}

// NewSanitizer creates a new sanitizer with the given configuration
func NewSanitizer(config Config) *Sanitizer {
	if config.MaxLength == 0 {
		config.MaxLength = 1000 // Default max length
	}
	return &Sanitizer{
		allowHTML: config.AllowHTML,
		maxLength: config.MaxLength,
	}
}

// DefaultSanitizer returns a sanitizer with secure defaults
func DefaultSanitizer() *Sanitizer {
	return NewSanitizer(Config{
		AllowHTML: false, // Strict XSS protection
		MaxLength: 1000,  // Reasonable limit
	})
}

// Patterns for detecting potentially malicious content
var (
	// XSS patterns - common script injection attempts
	xssPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`),
		regexp.MustCompile(`(?i)<iframe[^>]*>.*?</iframe>`),
		regexp.MustCompile(`(?i)<embed[^>]*>`),
		regexp.MustCompile(`(?i)<object[^>]*>.*?</object>`),
		regexp.MustCompile(`(?i)<applet[^>]*>.*?</applet>`),
		regexp.MustCompile(`(?i)<meta[^>]*>`),
		regexp.MustCompile(`(?i)<link[^>]*>`),
		regexp.MustCompile(`(?i)<form[^>]*>.*?</form>`),
		regexp.MustCompile(`(?i)javascript:`),
		regexp.MustCompile(`(?i)data:`),
		regexp.MustCompile(`(?i)vbscript:`),
		regexp.MustCompile(`(?i)onload\s*=`),
		regexp.MustCompile(`(?i)onerror\s*=`),
		regexp.MustCompile(`(?i)onclick\s*=`),
		regexp.MustCompile(`(?i)onmouseover\s*=`),
		regexp.MustCompile(`(?i)eval\s*\(`),
		regexp.MustCompile(`(?i)expression\s*\(`),
	}

	// Control character patterns
	controlCharPattern = regexp.MustCompile(`[\x00-\x08\x0B-\x0C\x0E-\x1F\x7F]`)
	
	// URL validation pattern
	urlPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
)

// SanitizeString performs comprehensive string sanitization
func (s *Sanitizer) SanitizeString(input string) string {
	if input == "" {
		return input
	}

	// Limit length
	if len(input) > s.maxLength {
		input = input[:s.maxLength] + "..."
	}

	// Remove control characters
	input = controlCharPattern.ReplaceAllString(input, "")

	// HTML escape unless HTML is explicitly allowed
	if !s.allowHTML {
		input = html.EscapeString(input)
	} else {
		// If HTML is allowed, still remove dangerous patterns
		for _, pattern := range xssPatterns {
			input = pattern.ReplaceAllString(input, "[FILTERED]")
		}
	}

	// Normalize whitespace
	input = strings.TrimSpace(input)
	input = regexp.MustCompile(`\s+`).ReplaceAllString(input, " ")

	return input
}

// SanitizeURL validates and sanitizes URL strings
func (s *Sanitizer) SanitizeURL(input string) string {
	if input == "" {
		return input
	}

	// First apply basic string sanitization
	input = s.SanitizeString(input)

	// Additional URL-specific validation
	parsedURL, err := url.Parse(input)
	if err != nil {
		// If URL is invalid, return sanitized version but don't fail
		return input
	}

	// Check for allowed schemes
	allowedSchemes := map[string]bool{
		"http":   true,
		"https":  true,
		"socks4": true,
		"socks5": true,
	}

	if !allowedSchemes[parsedURL.Scheme] {
		// Remove potentially dangerous schemes
		return "[INVALID_SCHEME]"
	}

	// Rebuild URL to ensure it's properly formatted
	return parsedURL.String()
}

// SanitizeIP validates and sanitizes IP address strings
func (s *Sanitizer) SanitizeIP(input string) string {
	if input == "" {
		return input
	}

	// Apply basic sanitization first
	input = s.SanitizeString(input)

	// Validate IP address format
	if net.ParseIP(input) == nil {
		// If not a valid IP, return sanitized string
		return input
	}

	return input
}

// SanitizeError sanitizes error messages while preserving useful information
func (s *Sanitizer) SanitizeError(input string) string {
	if input == "" {
		return input
	}

	// Apply standard sanitization
	sanitized := s.SanitizeString(input)

	// Additional error-specific cleaning
	// Remove potential file paths that might leak system information
	sanitized = regexp.MustCompile(`[A-Za-z]:\\[\w\\.-]+`).ReplaceAllString(sanitized, "[PATH]")
	sanitized = regexp.MustCompile(`/[\w/.-]+/[\w.-]+`).ReplaceAllString(sanitized, "[PATH]")

	// Remove potential internal hostnames/IPs that might leak network info
	sanitized = regexp.MustCompile(`\b(?:192\.168|10\.|172\.(?:1[6-9]|2[0-9]|3[01]))\.[0-9]{1,3}\.[0-9]{1,3}\b`).ReplaceAllString(sanitized, "[INTERNAL_IP]")
	sanitized = regexp.MustCompile(`\blocalhost\b`).ReplaceAllString(sanitized, "[LOCALHOST]")

	return sanitized
}

// SanitizeHostname sanitizes hostname strings
func (s *Sanitizer) SanitizeHostname(input string) string {
	if input == "" {
		return input
	}

	// Apply basic sanitization
	input = s.SanitizeString(input)

	// Additional hostname validation
	// Check for valid hostname characters
	validHostname := regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
	if !validHostname.MatchString(input) {
		return "[INVALID_HOSTNAME]"
	}

	// Limit hostname length (RFC 1123)
	if len(input) > 253 {
		input = input[:253]
	}

	return input
}

// SanitizeDebugInfo sanitizes debug information that might contain response content
func (s *Sanitizer) SanitizeDebugInfo(input string) string {
	if input == "" {
		return input
	}

	// Apply basic cleaning but preserve line structure for debug info
	sanitized := input
	
	// Limit length
	if len(sanitized) > 5000 {
		sanitized = sanitized[:5000] + "..."
	}

	// Remove control characters but preserve newlines
	sanitized = controlCharPattern.ReplaceAllString(sanitized, "")

	// HTML escape unless HTML is explicitly allowed
	if !s.allowHTML {
		sanitized = html.EscapeString(sanitized)
	} else {
		// If HTML is allowed, still remove dangerous patterns
		for _, pattern := range xssPatterns {
			sanitized = pattern.ReplaceAllString(sanitized, "[FILTERED]")
		}
	}

	// Remove potentially sensitive information from debug logs
	// Remove Authorization headers
	authPattern := regexp.MustCompile(`(?i)authorization:\s*[^\n\r]*`)
	sanitized = authPattern.ReplaceAllString(sanitized, "authorization: [REDACTED]")
	
	// Remove Proxy-Authorization headers
	proxyAuthPattern := regexp.MustCompile(`(?i)proxy-authorization:\s*[^\n\r]*`)
	sanitized = proxyAuthPattern.ReplaceAllString(sanitized, "proxy-authorization: [REDACTED]")
	
	// Remove potential passwords in URLs
	passwordPattern := regexp.MustCompile(`://[^:]+:[^@]+@`)
	sanitized = passwordPattern.ReplaceAllString(sanitized, "://[USER]:[PASS]@")

	// Remove internal IP addresses (more specific pattern)
	// 10.0.0.0/8
	sanitized = regexp.MustCompile(`\b10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\b`).ReplaceAllString(sanitized, "[INTERNAL_IP]")
	// 192.168.0.0/16
	sanitized = regexp.MustCompile(`\b192\.168\.[0-9]{1,3}\.[0-9]{1,3}\b`).ReplaceAllString(sanitized, "[INTERNAL_IP]")
	// 172.16.0.0/12
	sanitized = regexp.MustCompile(`\b172\.(?:1[6-9]|2[0-9]|3[01])\.[0-9]{1,3}\.[0-9]{1,3}\b`).ReplaceAllString(sanitized, "[INTERNAL_IP]")

	return strings.TrimSpace(sanitized)
}

// IsControlCharacter checks if a rune is a control character
func IsControlCharacter(r rune) bool {
	return unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t'
}

// ContainsPotentialXSS checks if a string contains potential XSS patterns
func ContainsPotentialXSS(input string) bool {
	lowerInput := strings.ToLower(input)
	
	// Check for script tags
	if strings.Contains(lowerInput, "<script") || strings.Contains(lowerInput, "javascript:") {
		return true
	}
	
	// Check for event handlers
	eventHandlers := []string{"onload", "onerror", "onclick", "onmouseover", "onfocus", "onblur"}
	for _, handler := range eventHandlers {
		if strings.Contains(lowerInput, handler+"=") {
			return true
		}
	}
	
	// Check for data URLs
	if strings.Contains(lowerInput, "data:") {
		return true
	}
	
	return false
}

// SanitizeMap sanitizes all string values in a map
func (s *Sanitizer) SanitizeMap(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return input
	}

	result := make(map[string]interface{})
	for key, value := range input {
		sanitizedKey := s.SanitizeString(key)
		
		switch v := value.(type) {
		case string:
			result[sanitizedKey] = s.SanitizeString(v)
		case map[string]interface{}:
			result[sanitizedKey] = s.SanitizeMap(v)
		default:
			result[sanitizedKey] = value
		}
	}
	
	return result
}