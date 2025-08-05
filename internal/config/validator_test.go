package config

import (
	"testing"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/cloudcheck"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/proxy"
)

// testConfig creates a minimal valid config for testing
func testConfig() *Config {
	return &Config{
		Timeout: 10,
		Concurrency: 10,
		UserAgent: "test-agent",
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		expectValid  bool
		expectErrors int
		expectWarns  int
	}{
		{
			name: "valid default config",
			config: func() *Config {
				cfg := GetDefaultConfig()
				cfg.Concurrency = 10 // Ensure concurrency is set
				return cfg
			}(),
			expectValid: true,
			expectErrors: 0,
			expectWarns: 1, // Warning about no security checks
		},
		{
			name: "invalid timeout",
			config: &Config{
				Timeout: -1,
				Concurrency: 10,
				UserAgent: "test-agent",
			},
			expectValid: false,
			expectErrors: 1,
			expectWarns: 1, // no security checks warning
		},
		{
			name: "very high timeout warning",
			config: func() *Config {
				cfg := testConfig()
				cfg.Timeout = 400
				return cfg
			}(),
			expectValid: true,
			expectErrors: 0,
			expectWarns: 2, // high timeout + no security checks
		},
		{
			name: "invalid concurrency",
			config: func() *Config {
				cfg := testConfig()
				cfg.Concurrency = 0
				return cfg
			}(),
			expectValid: false,
			expectErrors: 1,
			expectWarns: 1, // no security checks
		},
		{
			name: "very high concurrency warning",
			config: func() *Config {
				cfg := testConfig()
				cfg.Concurrency = 150
				return cfg
			}(),
			expectValid: true,
			expectErrors: 0,
			expectWarns: 2, // high concurrency + no security checks
		},
		{
			name: "invalid rate limit delay",
			config: func() *Config {
				cfg := testConfig()
				cfg.RateLimitEnabled = true
				cfg.RateLimitDelay = -1 * time.Second
				return cfg
			}(),
			expectValid: false,
			expectErrors: 1,
			expectWarns: 1, // no security checks
		},
		{
			name: "zero rate limit delay warning",
			config: func() *Config {
				cfg := testConfig()
				cfg.RateLimitEnabled = true
				cfg.RateLimitDelay = 0
				return cfg
			}(),
			expectValid: true,
			expectErrors: 0,
			expectWarns: 2, // zero delay + no security checks
		},
		{
			name: "invalid test URL",
			config: func() *Config {
				cfg := testConfig()
				cfg.TestURLs = TestURLConfig{
					DefaultURL: "not://a valid url",
				}
				return cfg
			}(),
			expectValid: false,
			expectErrors: 1,
			expectWarns: 1, // no security checks
		},
		{
			name: "empty header name",
			config: func() *Config {
				cfg := testConfig()
				cfg.DefaultHeaders = map[string]string{
					"": "value",
				}
				return cfg
			}(),
			expectValid: false,
			expectErrors: 1,
			expectWarns: 1, // no security checks
		},
		{
			name: "problematic header warning",
			config: func() *Config {
				cfg := testConfig()
				cfg.DefaultHeaders = map[string]string{
					"Host": "example.com",
				}
				return cfg
			}(),
			expectValid: true,
			expectErrors: 0,
			expectWarns: 2, // problematic header + no security checks
		},
		{
			name: "negative min response bytes",
			config: func() *Config {
				cfg := testConfig()
				cfg.Validation = ValidationConfig{
					MinResponseBytes: -1,
				}
				return cfg
			}(),
			expectValid: false,
			expectErrors: 1,
			expectWarns: 1, // no security checks
		},
		{
			name: "very high min response bytes warning",
			config: func() *Config {
				cfg := testConfig()
				cfg.Validation = ValidationConfig{
					MinResponseBytes: 2000000,
				}
				return cfg
			}(),
			expectValid: true,
			expectErrors: 0,
			expectWarns: 2, // high min bytes + no security checks
		},
		{
			name: "duplicate cloud provider",
			config: func() *Config {
				cfg := testConfig()
				cfg.CloudProviders = []cloudcheck.CloudProvider{
					{Name: "AWS"},
					{Name: "AWS"},
				}
				return cfg
			}(),
			expectValid: false,
			expectErrors: 1,
			expectWarns: 1, // no security checks
		},
		{
			name: "empty cloud provider name",
			config: func() *Config {
				cfg := testConfig()
				cfg.CloudProviders = []cloudcheck.CloudProvider{
					{Name: ""},
				}
				return cfg
			}(),
			expectValid: false,
			expectErrors: 1,
			expectWarns: 1, // no security checks
		},
		{
			name: "invalid HTTP method",
			config: func() *Config {
				cfg := testConfig()
				cfg.AdvancedChecks = proxy.AdvancedChecks{
					TestHTTPMethods: []string{""},
				}
				return cfg
			}(),
			expectValid: false,
			expectErrors: 1,
			expectWarns: 0, // has advanced checks configured
		},
		{
			name: "non-standard HTTP method warning",
			config: func() *Config {
				cfg := testConfig()
				cfg.AdvancedChecks = proxy.AdvancedChecks{
					TestHTTPMethods: []string{"CUSTOM"},
				}
				return cfg
			}(),
			expectValid: true,
			expectErrors: 0,
			expectWarns: 1, // non-standard method
		},
		{
			name: "no security checks warning",
			config: func() *Config {
				cfg := testConfig()
				cfg.AdvancedChecks = proxy.AdvancedChecks{}
				return cfg
			}(),
			expectValid: true,
			expectErrors: 0,
			expectWarns: 1, // no security checks
		},
		{
			name: "invalid status code requirement",
			config: func() *Config {
				cfg := testConfig()
				cfg.RequireStatusCode = 99
				return cfg
			}(),
			expectValid: false,
			expectErrors: 1,
			expectWarns: 1, // no security checks
		},
		{
			name: "duplicate disallowed keywords",
			config: func() *Config {
				cfg := testConfig()
				cfg.Validation = ValidationConfig{
					DisallowedKeywords: []string{"error", "Error", "ERROR"},
				}
				return cfg
			}(),
			expectValid: true,
			expectErrors: 0,
			expectWarns: 3, // 2 duplicates + no security checks
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateConfig(tt.config)
			
			if result.Valid != tt.expectValid {
				t.Errorf("ValidateConfig() valid = %v, want %v", result.Valid, tt.expectValid)
			}
			
			if len(result.Errors) != tt.expectErrors {
				t.Errorf("ValidateConfig() errors = %d, want %d", len(result.Errors), tt.expectErrors)
				for _, err := range result.Errors {
					t.Logf("  Error: %v", err)
				}
			}
			
			if len(result.Warnings) != tt.expectWarns {
				t.Errorf("ValidateConfig() warnings = %d, want %d", len(result.Warnings), tt.expectWarns)
				for _, warn := range result.Warnings {
					t.Logf("  Warning: %s", warn)
				}
			}
		})
	}
}

func TestConfigValidationError_Error(t *testing.T) {
	err := ConfigValidationError{
		Field:   "timeout",
		Value:   -1,
		Message: "timeout must be positive",
	}
	
	expected := "config validation error in timeout: timeout must be positive (value: -1)"
	if err.Error() != expected {
		t.Errorf("ConfigValidationError.Error() = %v, want %v", err.Error(), expected)
	}
}

func TestValidateURLs(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectValid bool
	}{
		{
			name: "valid URLs",
			config: &Config{
				TestURLs: TestURLConfig{
					DefaultURL: "https://example.com",
					TestURLs: []TestURL{
						{URL: "http://test.com"},
						{URL: "https://another.com"},
					},
				},
				InteractshURL: "https://interact.sh",
			},
			expectValid: true,
		},
		{
			name: "invalid default URL",
			config: &Config{
				TestURLs: TestURLConfig{
					DefaultURL: "://invalid url with no scheme",
				},
			},
			expectValid: false,
		},
		{
			name: "empty test URL",
			config: &Config{
				TestURLs: TestURLConfig{
					TestURLs: []TestURL{
						{URL: ""},
					},
				},
			},
			expectValid: false,
		},
		{
			name: "invalid interactsh URL",
			config: &Config{
				InteractshURL: "://invalid",
			},
			expectValid: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set required fields
			tt.config.Timeout = 10
			tt.config.Concurrency = 10
			
			result := ValidateConfig(tt.config)
			if result.Valid != tt.expectValid {
				t.Errorf("ValidateConfig() valid = %v, want %v", result.Valid, tt.expectValid)
				for _, err := range result.Errors {
					t.Logf("  Error: %v", err)
				}
			}
		})
	}
}