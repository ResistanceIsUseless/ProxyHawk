package proxy

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestExtractAuthFromURL tests authentication extraction from URLs
func TestExtractAuthFromURL(t *testing.T) {
	config := Config{}
	checker := NewChecker(config, false)

	tests := []struct {
		name          string
		proxyURL      string
		expectedAuth  *ProxyAuth
		shouldHaveAuth bool
	}{
		{
			name:           "URL with username and password",
			proxyURL:       "http://user:pass@proxy.example.com:8080",
			expectedAuth:   &ProxyAuth{Username: "user", Password: "pass", Method: AuthMethodBasic},
			shouldHaveAuth: true,
		},
		{
			name:           "URL with username only",
			proxyURL:       "http://user@proxy.example.com:8080",
			expectedAuth:   &ProxyAuth{Username: "user", Password: "", Method: AuthMethodBasic},
			shouldHaveAuth: true,
		},
		{
			name:           "URL with empty username and password",
			proxyURL:       "http://:@proxy.example.com:8080",
			expectedAuth:   nil,
			shouldHaveAuth: false,
		},
		{
			name:           "URL without authentication",
			proxyURL:       "http://proxy.example.com:8080",
			expectedAuth:   nil,
			shouldHaveAuth: false,
		},
		{
			name:           "SOCKS5 URL with authentication",
			proxyURL:       "socks5://admin:secret@proxy.example.com:1080",
			expectedAuth:   &ProxyAuth{Username: "admin", Password: "secret", Method: AuthMethodBasic},
			shouldHaveAuth: true,
		},
		{
			name:           "URL with special characters in password",
			proxyURL:       "http://user:p%40ssw0rd@proxy.example.com:8080", // p@ssw0rd URL encoded
			expectedAuth:   &ProxyAuth{Username: "user", Password: "p@ssw0rd", Method: AuthMethodBasic}, // Go automatically decodes
			shouldHaveAuth: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.proxyURL)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			auth := checker.extractAuthFromURL(parsedURL)

			if tt.shouldHaveAuth {
				if auth == nil {
					t.Error("Expected authentication info but got nil")
					return
				}

				if auth.Username != tt.expectedAuth.Username {
					t.Errorf("Expected username %s, got %s", tt.expectedAuth.Username, auth.Username)
				}

				if auth.Password != tt.expectedAuth.Password {
					t.Errorf("Expected password %s, got %s", tt.expectedAuth.Password, auth.Password)
				}

				if auth.Method != tt.expectedAuth.Method {
					t.Errorf("Expected method %s, got %s", tt.expectedAuth.Method, auth.Method)
				}
			} else {
				if auth != nil {
					t.Errorf("Expected no authentication info but got: %+v", auth)
				}
			}
		})
	}
}

// TestGetProxyAuth tests the proxy authentication decision logic
func TestGetProxyAuth(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		proxyURL       string
		expectedAuth   *ProxyAuth
		shouldHaveAuth bool
	}{
		{
			name: "URL auth takes precedence",
			config: Config{
				AuthEnabled:     true,
				DefaultUsername: "default_user",
				DefaultPassword: "default_pass",
			},
			proxyURL:       "http://url_user:url_pass@proxy.example.com:8080",
			expectedAuth:   &ProxyAuth{Username: "url_user", Password: "url_pass", Method: AuthMethodBasic},
			shouldHaveAuth: true,
		},
		{
			name: "Default auth when URL has no auth",
			config: Config{
				AuthEnabled:     true,
				DefaultUsername: "default_user",
				DefaultPassword: "default_pass",
			},
			proxyURL:       "http://proxy.example.com:8080",
			expectedAuth:   &ProxyAuth{Username: "default_user", Password: "default_pass", Method: AuthMethodBasic},
			shouldHaveAuth: true,
		},
		{
			name: "No auth when disabled",
			config: Config{
				AuthEnabled:     false,
				DefaultUsername: "user",
				DefaultPassword: "pass",
			},
			proxyURL:       "http://proxy.example.com:8080",
			expectedAuth:   nil,
			shouldHaveAuth: false,
		},
		{
			name: "No auth when enabled but no default username",
			config: Config{
				AuthEnabled:     true,
				DefaultUsername: "",
				DefaultPassword: "pass",
			},
			proxyURL:       "http://proxy.example.com:8080",
			expectedAuth:   nil,
			shouldHaveAuth: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(tt.config, false)
			result := &ProxyResult{}

			parsedURL, err := url.Parse(tt.proxyURL)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			auth := checker.getProxyAuth(parsedURL, result)

			if tt.shouldHaveAuth {
				if auth == nil {
					t.Error("Expected authentication info but got nil")
					return
				}

				if auth.Username != tt.expectedAuth.Username {
					t.Errorf("Expected username %s, got %s", tt.expectedAuth.Username, auth.Username)
				}

				if auth.Password != tt.expectedAuth.Password {
					t.Errorf("Expected password %s, got %s", tt.expectedAuth.Password, auth.Password)
				}
			} else {
				if auth != nil {
					t.Errorf("Expected no authentication info but got: %+v", auth)
				}
			}
		})
	}
}

// TestCleanProxyURL tests URL cleaning for logging
func TestCleanProxyURL(t *testing.T) {
	config := Config{}
	checker := NewChecker(config, false)

	tests := []struct {
		name        string
		proxyURL    string
		expectedURL string
	}{
		{
			name:        "URL with authentication",
			proxyURL:    "http://user:pass@proxy.example.com:8080",
			expectedURL: "http://proxy.example.com:8080",
		},
		{
			name:        "URL without authentication",
			proxyURL:    "http://proxy.example.com:8080",
			expectedURL: "http://proxy.example.com:8080",
		},
		{
			name:        "HTTPS URL with authentication",
			proxyURL:    "https://admin:secret@secure-proxy.example.com:8443/path?query=1",
			expectedURL: "https://secure-proxy.example.com:8443/path?query=1",
		},
		{
			name:        "SOCKS5 URL with authentication",
			proxyURL:    "socks5://user:pass@proxy.example.com:1080",
			expectedURL: "socks5://proxy.example.com:1080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.proxyURL)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			cleanURL := checker.cleanProxyURL(parsedURL)

			if cleanURL != tt.expectedURL {
				t.Errorf("Expected clean URL %s, got %s", tt.expectedURL, cleanURL)
			}
		})
	}
}

// TestCreateAuthenticatedHTTPTransport tests HTTP transport creation with auth
func TestCreateAuthenticatedHTTPTransport(t *testing.T) {
	config := Config{
		Timeout: 30 * time.Second,
	}
	checker := NewChecker(config, false)
	result := &ProxyResult{}

	tests := []struct {
		name   string
		auth   *ProxyAuth
		scheme string
	}{
		{
			name:   "HTTP transport with authentication",
			auth:   &ProxyAuth{Username: "user", Password: "pass", Method: AuthMethodBasic},
			scheme: "http",
		},
		{
			name:   "HTTPS transport with authentication",
			auth:   &ProxyAuth{Username: "admin", Password: "secret", Method: AuthMethodBasic},
			scheme: "https",
		},
		{
			name:   "HTTP transport without authentication",
			auth:   nil,
			scheme: "http",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxyURL, _ := url.Parse("http://proxy.example.com:8080")
			transport := checker.createAuthenticatedHTTPTransport(proxyURL, tt.scheme, tt.auth, result)

			if transport == nil {
				t.Error("Expected transport but got nil")
				return
			}

			// Test that proxy function is set
			if transport.Proxy == nil {
				t.Error("Expected proxy function to be set")
				return
			}

			// Test proxy function
			req, _ := http.NewRequest("GET", "http://example.com", nil)
			proxyURLResult, err := transport.Proxy(req)
			if err != nil {
				t.Errorf("Proxy function failed: %v", err)
				return
			}

			if proxyURLResult == nil {
				t.Error("Expected proxy URL but got nil")
				return
			}

			// Check if authentication is properly embedded
			if tt.auth != nil {
				if proxyURLResult.User == nil {
					t.Error("Expected authentication in proxy URL but got none")
				} else {
					username := proxyURLResult.User.Username()
					password, _ := proxyURLResult.User.Password()
					
					if username != tt.auth.Username {
						t.Errorf("Expected username %s, got %s", tt.auth.Username, username)
					}
					if password != tt.auth.Password {
						t.Errorf("Expected password %s, got %s", tt.auth.Password, password)
					}
				}

				// Check for Proxy-Authorization header
				if transport.ProxyConnectHeader == nil {
					t.Error("Expected ProxyConnectHeader to be set for authenticated proxy")
				} else {
					authHeader := transport.ProxyConnectHeader.Get("Proxy-Authorization")
					if !strings.HasPrefix(authHeader, "Basic ") {
						t.Errorf("Expected Basic auth header, got: %s", authHeader)
					}
				}
			}
		})
	}
}

// TestValidateAuthConfig tests authentication configuration validation
func TestValidateAuthConfig(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		expectedEnabled bool
		expectedMethods []string
	}{
		{
			name: "Valid configuration",
			config: Config{
				AuthEnabled: true,
				AuthMethods: []string{"basic", "digest"},
			},
			expectedEnabled: true,
			expectedMethods: []string{"basic", "digest"},
		},
		{
			name: "Default methods when none specified",
			config: Config{
				AuthEnabled: true,
				AuthMethods: []string{},
			},
			expectedEnabled: true,
			expectedMethods: []string{"basic"},
		},
		{
			name: "Invalid methods filtered out",
			config: Config{
				AuthEnabled: true,
				AuthMethods: []string{"basic", "invalid", "BASIC", "digest"},
			},
			expectedEnabled: true,
			expectedMethods: []string{"basic", "basic", "digest"}, // BASIC becomes basic
		},
		{
			name: "All invalid methods disable auth",
			config: Config{
				AuthEnabled: true,
				AuthMethods: []string{"invalid1", "invalid2"},
			},
			expectedEnabled: false,
			expectedMethods: []string{},
		},
		{
			name: "Disabled configuration",
			config: Config{
				AuthEnabled: false,
				AuthMethods: []string{"basic"},
			},
			expectedEnabled: false,
			expectedMethods: []string{"basic"}, // Methods preserved even when disabled
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(tt.config, false)

			if checker.config.AuthEnabled != tt.expectedEnabled {
				t.Errorf("Expected AuthEnabled %v, got %v", tt.expectedEnabled, checker.config.AuthEnabled)
			}

			if len(checker.config.AuthMethods) != len(tt.expectedMethods) {
				t.Errorf("Expected %d auth methods, got %d", len(tt.expectedMethods), len(checker.config.AuthMethods))
			}

			for i, expectedMethod := range tt.expectedMethods {
				if i < len(checker.config.AuthMethods) && checker.config.AuthMethods[i] != expectedMethod {
					t.Errorf("Expected method %s at index %d, got %s", expectedMethod, i, checker.config.AuthMethods[i])
				}
			}
		})
	}
}

// TestTestProxyAuth tests the proxy authentication testing functionality
func TestTestProxyAuth(t *testing.T) {
	tests := []struct {
		name           string
		auth           *ProxyAuth
		expectedSuccess bool
		expectedError   string
	}{
		{
			name:           "No authentication required",
			auth:           nil,
			expectedSuccess: true,
			expectedError:   "",
		},
		{
			name:           "Authentication provided but connection fails",
			auth:           &ProxyAuth{Username: "user", Password: "pass", Method: AuthMethodBasic},
			expectedSuccess: false,
			expectedError:   "unexpected response status",
		},
	}

	config := Config{
		Timeout: 1 * time.Second, // Short timeout to fail quickly
		DefaultHeaders: map[string]string{
			"Accept": "application/json",
		},
		UserAgent: "ProxyHawk-Test/1.0",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(config, true) // Enable debug
			result := &ProxyResult{DebugInfo: ""}

			// Create a simple HTTP client for testing
			client := &http.Client{Timeout: config.Timeout}

			success, errMsg := checker.testProxyAuth(client, tt.auth, result)

			if success != tt.expectedSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectedSuccess, success)
			}

			if tt.expectedError != "" && !strings.Contains(errMsg, tt.expectedError) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, errMsg)
			}

			if tt.expectedError == "" && errMsg != "" {
				t.Errorf("Expected no error, got '%s'", errMsg)
			}
		})
	}
}

// TestAuthIntegration tests authentication integration with proxy checking
func TestAuthIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := Config{
		Timeout:         10 * time.Second,
		AuthEnabled:     true,
		DefaultUsername: "testuser",
		DefaultPassword: "testpass",
		ValidationURL:   "https://httpbin.org/ip",
	}

	checker := NewChecker(config, true)

	// Test with a non-existent authenticated proxy (will fail but test the flow)
	result := checker.Check("http://authenticated-proxy.example.com:8080")

	// Should fail due to connection error, but authentication logic should be exercised
	if result.Error == nil {
		t.Log("Unexpected success - proxy may actually exist")
	}

	// Verify debug info contains authentication information
	if !strings.Contains(result.DebugInfo, "[AUTH]") {
		t.Error("Expected authentication debug info to be present")
	}
}

// Benchmark tests for authentication performance
func BenchmarkExtractAuthFromURL(b *testing.B) {
	config := Config{}
	checker := NewChecker(config, false)
	proxyURL, _ := url.Parse("http://user:password@proxy.example.com:8080")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.extractAuthFromURL(proxyURL)
	}
}

func BenchmarkGetProxyAuth(b *testing.B) {
	config := Config{
		AuthEnabled:     true,
		DefaultUsername: "user",
		DefaultPassword: "pass",
	}
	checker := NewChecker(config, false)
	proxyURL, _ := url.Parse("http://proxy.example.com:8080")
	result := &ProxyResult{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.getProxyAuth(proxyURL, result)
	}
}

func BenchmarkCleanProxyURL(b *testing.B) {
	config := Config{}
	checker := NewChecker(config, false)
	proxyURL, _ := url.Parse("http://user:password@proxy.example.com:8080")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.cleanProxyURL(proxyURL)
	}
}