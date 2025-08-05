package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/cloudcheck"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/proxy"
	"gopkg.in/yaml.v3"
)

// Config represents the main application configuration
type Config struct {
	Timeout              int                    `yaml:"timeout"`
	InsecureSkipVerify   bool                   `yaml:"insecure_skip_verify"`
	EnableCloudChecks    bool                   `yaml:"enable_cloud_checks"`
	EnableAnonymityCheck bool                   `yaml:"enable_anonymity_check"`
	RateLimitEnabled     bool                   `yaml:"rate_limit_enabled"`
	RateLimitDelay       time.Duration          `yaml:"rate_limit_delay"`
	RateLimitPerHost     bool                   `yaml:"rate_limit_per_host"`
	DefaultHeaders       map[string]string      `yaml:"default_headers"`
	UserAgent            string                 `yaml:"user_agent"`
	Validation           ValidationConfig       `yaml:"validation"`
	TestURLs             TestURLConfig          `yaml:"test_urls"`
	Concurrency          int                    `yaml:"concurrency"`
	InteractshURL        string                 `yaml:"interactsh_url"`
	InteractshToken      string                 `yaml:"interactsh_token"`

	// Cloud provider settings
	CloudProviders []cloudcheck.CloudProvider `yaml:"cloud_providers"`

	// Advanced security checks
	AdvancedChecks proxy.AdvancedChecks `yaml:"advanced_checks"`

	// Response validation settings
	RequireStatusCode   int      `yaml:"require_status_code"`
	RequireContentMatch string   `yaml:"require_content_match"`
	RequireHeaderFields []string `yaml:"require_header_fields"`
}

// TestURLConfig contains configuration for test URLs
type TestURLConfig struct {
	DefaultURL string    `yaml:"default_url"`
	TestURLs   []TestURL `yaml:"test_urls"`
}

// TestURL represents a single test URL configuration
type TestURL struct {
	URL        string `yaml:"url"`
	ExpectText string `yaml:"expect_text"`
}

// ValidationConfig contains validation settings
type ValidationConfig struct {
	DisallowedKeywords []string `yaml:"disallowed_keywords"`
	MinResponseBytes   int      `yaml:"min_response_bytes"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(filename string) (*Config, error) {
	// Check if file exists, if not, return default config
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// Note: We can't use structured logging here since this is called before logger initialization
		// This could be improved by passing a logger instance to LoadConfig
		return GetDefaultConfig(), nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	// Set default concurrency if not specified
	if config.Concurrency <= 0 {
		config.Concurrency = 10
	}

	// Merge with defaults for any missing fields
	defaults := GetDefaultConfig()
	if len(config.DefaultHeaders) == 0 {
		config.DefaultHeaders = defaults.DefaultHeaders
	}
	if config.UserAgent == "" {
		config.UserAgent = defaults.UserAgent
	}
	if len(config.Validation.DisallowedKeywords) == 0 {
		config.Validation = defaults.Validation
	}
	if config.TestURLs.DefaultURL == "" {
		config.TestURLs.DefaultURL = "https://api.ipify.org?format=json"
	}

	return &config, nil
}

// GetDefaultConfig returns a configuration with default values
func GetDefaultConfig() *Config {
	return &Config{
		Timeout:              10,
		InsecureSkipVerify:   false,
		EnableCloudChecks:    false,
		EnableAnonymityCheck: false,

		// Default rate limiting settings
		RateLimitEnabled: false,
		RateLimitDelay:   1 * time.Second,
		RateLimitPerHost: true,

		DefaultHeaders: map[string]string{
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
			"Accept-Language": "en-US,en;q=0.9",
			"Accept-Encoding": "gzip, deflate",
			"Connection":      "keep-alive",
			"Cache-Control":   "no-cache",
			"Pragma":          "no-cache",
		},
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Validation: ValidationConfig{
			DisallowedKeywords: []string{
				"Access Denied",
				"Proxy Error",
				"Bad Gateway",
				"Gateway Timeout",
				"Service Unavailable",
			},
			MinResponseBytes: 100,
		},
	}
}