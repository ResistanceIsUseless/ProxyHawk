package config

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidationResult represents the result of configuration validation
type ValidationResult struct {
	Valid    bool
	Errors   []ConfigValidationError
	Warnings []string
}

// ConfigValidationError represents a configuration validation error
type ConfigValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e ConfigValidationError) Error() string {
	return fmt.Sprintf("config validation error in %s: %s (value: %v)", e.Field, e.Message, e.Value)
}

// ValidateConfig performs comprehensive validation on a configuration
func ValidateConfig(config *Config) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []ConfigValidationError{},
		Warnings: []string{},
	}

	// Validate timeout
	if config.Timeout <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ConfigValidationError{
			Field:   "timeout",
			Value:   config.Timeout,
			Message: "timeout must be positive",
		})
	} else if config.Timeout > 300 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("timeout of %d seconds is very high, may cause long delays", config.Timeout))
	}

	// Validate concurrency
	if config.Concurrency <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ConfigValidationError{
			Field:   "concurrency",
			Value:   config.Concurrency,
			Message: "concurrency must be positive",
		})
	} else if config.Concurrency > 100 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("concurrency of %d is very high, may overwhelm target servers", config.Concurrency))
	}

	// Validate rate limiting
	if config.RateLimitEnabled {
		if config.RateLimitDelay < 0 {
			result.Valid = false
			result.Errors = append(result.Errors, ConfigValidationError{
				Field:   "rate_limit_delay",
				Value:   config.RateLimitDelay,
				Message: "rate limit delay cannot be negative",
			})
		} else if config.RateLimitDelay == 0 {
			result.Warnings = append(result.Warnings, "rate limiting is enabled but delay is 0, this will have no effect")
		}
	}

	// Validate URLs
	validateURLs(config, result)

	// Validate headers
	validateHeaders(config, result)

	// Validate validation settings
	validateValidationSettings(config, result)

	// Validate cloud providers
	validateCloudProviders(config, result)

	// Validate advanced checks
	validateAdvancedChecks(config, result)

	// Validate response requirements
	validateResponseRequirements(config, result)

	return result
}

// validateURLs validates all URLs in the configuration
func validateURLs(config *Config, result *ValidationResult) {
	// Validate default test URL
	if config.TestURLs.DefaultURL != "" {
		if _, err := url.Parse(config.TestURLs.DefaultURL); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ConfigValidationError{
				Field:   "test_urls.default_url",
				Value:   config.TestURLs.DefaultURL,
				Message: fmt.Sprintf("invalid URL: %v", err),
			})
		}
	}

	// Validate test URLs
	for i, testURL := range config.TestURLs.TestURLs {
		if testURL.URL == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ConfigValidationError{
				Field:   fmt.Sprintf("test_urls.test_urls[%d].url", i),
				Value:   testURL.URL,
				Message: "URL cannot be empty",
			})
			continue
		}

		if _, err := url.Parse(testURL.URL); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ConfigValidationError{
				Field:   fmt.Sprintf("test_urls.test_urls[%d].url", i),
				Value:   testURL.URL,
				Message: fmt.Sprintf("invalid URL: %v", err),
			})
		}
	}

	// Validate Interactsh URL if provided
	if config.InteractshURL != "" {
		if _, err := url.Parse(config.InteractshURL); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ConfigValidationError{
				Field:   "interactsh_url",
				Value:   config.InteractshURL,
				Message: fmt.Sprintf("invalid URL: %v", err),
			})
		}
	}
}

// validateHeaders validates HTTP headers
func validateHeaders(config *Config, result *ValidationResult) {
	// Check for empty header names or values
	for name, value := range config.DefaultHeaders {
		if strings.TrimSpace(name) == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ConfigValidationError{
				Field:   "default_headers",
				Value:   fmt.Sprintf("%s: %s", name, value),
				Message: "header name cannot be empty",
			})
		}

		// Warn about potentially problematic headers
		lowerName := strings.ToLower(name)
		if lowerName == "host" || lowerName == "content-length" || lowerName == "transfer-encoding" {
			result.Warnings = append(result.Warnings, 
				fmt.Sprintf("header '%s' may interfere with proxy functionality", name))
		}
	}

	// Validate User-Agent
	if strings.TrimSpace(config.UserAgent) == "" {
		result.Warnings = append(result.Warnings, "empty User-Agent may cause requests to be blocked")
	}
}

// validateValidationSettings validates the validation configuration
func validateValidationSettings(config *Config, result *ValidationResult) {
	// Check minimum response bytes
	if config.Validation.MinResponseBytes < 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ConfigValidationError{
			Field:   "validation.min_response_bytes",
			Value:   config.Validation.MinResponseBytes,
			Message: "minimum response bytes cannot be negative",
		})
	} else if config.Validation.MinResponseBytes > 1048576 { // 1MB
		result.Warnings = append(result.Warnings, 
			fmt.Sprintf("minimum response bytes of %d is very high, may reject valid proxies", 
				config.Validation.MinResponseBytes))
	}

	// Check for duplicate disallowed keywords
	seen := make(map[string]bool)
	for _, keyword := range config.Validation.DisallowedKeywords {
		lower := strings.ToLower(keyword)
		if seen[lower] {
			result.Warnings = append(result.Warnings, 
				fmt.Sprintf("duplicate disallowed keyword: %s", keyword))
		}
		seen[lower] = true
	}
}

// validateCloudProviders validates cloud provider configurations
func validateCloudProviders(config *Config, result *ValidationResult) {
	seenNames := make(map[string]bool)
	
	for i, provider := range config.CloudProviders {
		// Validate provider name
		if strings.TrimSpace(provider.Name) == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ConfigValidationError{
				Field:   fmt.Sprintf("cloud_providers[%d].name", i),
				Value:   provider.Name,
				Message: "cloud provider name cannot be empty",
			})
		} else if seenNames[provider.Name] {
			result.Valid = false
			result.Errors = append(result.Errors, ConfigValidationError{
				Field:   fmt.Sprintf("cloud_providers[%d].name", i),
				Value:   provider.Name,
				Message: "duplicate cloud provider name",
			})
		}
		seenNames[provider.Name] = true

		// Validate metadata IPs
		for j, ip := range provider.MetadataIPs {
			if strings.TrimSpace(ip) == "" {
				result.Valid = false
				result.Errors = append(result.Errors, ConfigValidationError{
					Field:   fmt.Sprintf("cloud_providers[%d].metadata_ips[%d]", i, j),
					Value:   ip,
					Message: "metadata IP cannot be empty",
				})
			}
		}

		// Validate ASNs
		for j, asn := range provider.ASNs {
			if strings.TrimSpace(asn) == "" {
				result.Valid = false
				result.Errors = append(result.Errors, ConfigValidationError{
					Field:   fmt.Sprintf("cloud_providers[%d].asns[%d]", i, j),
					Value:   asn,
					Message: "ASN cannot be empty",
				})
			}
		}
	}
}

// validateAdvancedChecks validates advanced security check settings
func validateAdvancedChecks(config *Config, result *ValidationResult) {
	checks := &config.AdvancedChecks

	// Validate HTTP methods
	for i, method := range checks.TestHTTPMethods {
		method = strings.ToUpper(strings.TrimSpace(method))
		if method == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ConfigValidationError{
				Field:   fmt.Sprintf("advanced_checks.test_http_methods[%d]", i),
				Value:   checks.TestHTTPMethods[i],
				Message: "HTTP method cannot be empty",
			})
			continue
		}

		// Check for valid HTTP methods
		validMethods := []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS", "PATCH", "CONNECT", "TRACE"}
		valid := false
		for _, vm := range validMethods {
			if method == vm {
				valid = true
				break
			}
		}
		if !valid {
			result.Warnings = append(result.Warnings, 
				fmt.Sprintf("HTTP method '%s' is non-standard", method))
		}
	}

	// Warn if no security checks are enabled
	if !checks.TestProtocolSmuggling && !checks.TestDNSRebinding && !checks.TestIPv6 &&
		len(checks.TestHTTPMethods) == 0 && !checks.TestCachePoisoning && 
		!checks.TestHostHeaderInjection && !checks.TestSSRF {
		result.Warnings = append(result.Warnings, 
			"no advanced security checks are enabled, consider enabling some for better security validation")
	}
}

// validateResponseRequirements validates response requirement settings
func validateResponseRequirements(config *Config, result *ValidationResult) {
	// Validate status code requirement
	if config.RequireStatusCode != 0 {
		if config.RequireStatusCode < 100 || config.RequireStatusCode >= 600 {
			result.Valid = false
			result.Errors = append(result.Errors, ConfigValidationError{
				Field:   "require_status_code",
				Value:   config.RequireStatusCode,
				Message: "status code must be between 100 and 599",
			})
		}
	}

	// Validate required header fields
	seenHeaders := make(map[string]bool)
	for i, header := range config.RequireHeaderFields {
		if strings.TrimSpace(header) == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ConfigValidationError{
				Field:   fmt.Sprintf("require_header_fields[%d]", i),
				Value:   header,
				Message: "required header field cannot be empty",
			})
		}
		
		lower := strings.ToLower(header)
		if seenHeaders[lower] {
			result.Warnings = append(result.Warnings, 
				fmt.Sprintf("duplicate required header field: %s", header))
		}
		seenHeaders[lower] = true
	}
}

// ValidateAndLoad loads and validates a configuration file
func ValidateAndLoad(filename string) (*Config, *ValidationResult, error) {
	config, err := LoadConfig(filename)
	if err != nil {
		return nil, nil, err
	}

	validationResult := ValidateConfig(config)
	return config, validationResult, nil
}