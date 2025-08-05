package sanitizer

import (
	"strings"
	"testing"
)

func TestSanitizeString(t *testing.T) {
	sanitizer := DefaultSanitizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Normal string",
			input:    "This is a normal string",
			expected: "This is a normal string",
		},
		{
			name:     "HTML script tag",
			input:    `<script>alert('xss')</script>`,
			expected: `&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;`,
		},
		{
			name:     "JavaScript URL",
			input:    `javascript:alert('xss')`,
			expected: `javascript:alert(&#39;xss&#39;)`,
		},
		{
			name:     "HTML iframe",
			input:    `<iframe src="evil.com"></iframe>`,
			expected: `&lt;iframe src=&#34;evil.com&#34;&gt;&lt;/iframe&gt;`,
		},
		{
			name:     "Event handler",
			input:    `<img onerror="alert('xss')" src="x">`,
			expected: `&lt;img onerror=&#34;alert(&#39;xss&#39;)&#34; src=&#34;x&#34;&gt;`,
		},
		{
			name:     "Control characters",
			input:    "Hello\x00\x01\x02World",
			expected: "HelloWorld",
		},
		{
			name:     "Multiple whitespace",
			input:    "Hello    \t\n   World",
			expected: "Hello World",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Very long string",
			input:    strings.Repeat("A", 1500),
			expected: strings.Repeat("A", 1000) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.SanitizeString(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSanitizeURL(t *testing.T) {
	sanitizer := DefaultSanitizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Valid HTTP URL",
			input:    "http://example.com:8080",
			expected: "http://example.com:8080",
		},
		{
			name:     "Valid HTTPS URL",
			input:    "https://example.com",
			expected: "https://example.com",
		},
		{
			name:     "Valid SOCKS5 URL",
			input:    "socks5://proxy.example.com:1080",
			expected: "socks5://proxy.example.com:1080",
		},
		{
			name:     "Invalid scheme",
			input:    "ftp://example.com",
			expected: "[INVALID_SCHEME]",
		},
		{
			name:     "JavaScript URL",
			input:    "javascript:alert('xss')",
			expected: "[INVALID_SCHEME]",
		},
		{
			name:     "Data URL",
			input:    "data:text/html,<script>alert('xss')</script>",
			expected: "[INVALID_SCHEME]",
		},
		{
			name:     "URL with script in query",
			input:    "http://example.com?param=<script>alert('xss')</script>",
			expected: "http://example.com?param=&lt;script&gt;alert(&#39;xss&%2339;)&lt;/script&gt;",
		},
		{
			name:     "Empty URL",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.SanitizeURL(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSanitizeIP(t *testing.T) {
	sanitizer := DefaultSanitizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Valid IPv4",
			input:    "192.168.1.1",
			expected: "192.168.1.1",
		},
		{
			name:     "Valid IPv6",
			input:    "2001:db8::1",
			expected: "2001:db8::1",
		},
		{
			name:     "Invalid IP with script",
			input:    "192.168.1.1<script>alert('xss')</script>",
			expected: "192.168.1.1&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
		{
			name:     "Localhost",
			input:    "127.0.0.1",
			expected: "127.0.0.1",
		},
		{
			name:     "Empty IP",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.SanitizeIP(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSanitizeError(t *testing.T) {
	sanitizer := DefaultSanitizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Normal error",
			input:    "connection refused",
			expected: "connection refused",
		},
		{
			name:     "Error with script",
			input:    "Error: <script>alert('xss')</script>",
			expected: "Error: &lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
		{
			name:     "Error with Windows path",
			input:    "Error reading C:\\Windows\\System32\\file.txt",
			expected: "Error reading [PATH]",
		},
		{
			name:     "Error with Unix path",
			input:    "Error reading /etc/passwd",
			expected: "Error reading [PATH]",
		},
		{
			name:     "Error with internal IP",
			input:    "Connection to 192.168.1.100 failed",
			expected: "Connection to [INTERNAL_IP] failed",
		},
		{
			name:     "Error with localhost",
			input:    "Connection to localhost failed",
			expected: "Connection to [LOCALHOST] failed",
		},
		{
			name:     "Empty error",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.SanitizeError(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSanitizeHostname(t *testing.T) {
	sanitizer := DefaultSanitizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Valid hostname",
			input:    "example.com",
			expected: "example.com",
		},
		{
			name:     "Valid hostname with subdomain",
			input:    "proxy.example.com",
			expected: "proxy.example.com",
		},
		{
			name:     "Hostname with invalid characters",
			input:    "example<script>.com",
			expected: "[INVALID_HOSTNAME]",
		},
		{
			name:     "Very long hostname",
			input:    strings.Repeat("a", 300) + ".com",
			expected: strings.Repeat("a", 253),
		},
		{
			name:     "Empty hostname",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.SanitizeHostname(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSanitizeDebugInfo(t *testing.T) {
	sanitizer := DefaultSanitizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Normal debug info",
			input:    "Connection established successfully",
			expected: "Connection established successfully",
		},
		{
			name:     "Debug with Authorization header",
			input:    "Authorization: Bearer token123\nContent-Type: application/json",
			expected: "authorization: [REDACTED]\nContent-Type: application/json",
		},
		{
			name:     "Debug with Proxy-Authorization",
			input:    "Proxy-Authorization: Basic dXNlcjpwYXNz",
			expected: "proxy-authorization: [REDACTED]",
		},
		{
			name:     "Debug with password in URL",
			input:    "Connecting to http://user:secret@proxy.com:8080",
			expected: "Connecting to http://[USER]:[PASS]@proxy.com:8080",
		},
		{
			name:     "Debug with internal IP",
			input:    "Routing through 10.0.0.1 gateway",
			expected: "Routing through [INTERNAL_IP] gateway",
		},
		{
			name:     "Debug with script",
			input:    "Response: <script>alert('xss')</script>",
			expected: "Response: &lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
		{
			name:     "Empty debug info",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.SanitizeDebugInfo(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestContainsPotentialXSS(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Clean string",
			input:    "This is a clean string",
			expected: false,
		},
		{
			name:     "Script tag",
			input:    "<script>alert('xss')</script>",
			expected: true,
		},
		{
			name:     "JavaScript URL",
			input:    "javascript:alert('xss')",
			expected: true,
		},
		{
			name:     "Event handler",
			input:    "onload=alert('xss')",
			expected: true,
		},
		{
			name:     "Data URL",
			input:    "data:text/html,<h1>Test</h1>",
			expected: true,
		},
		{
			name:     "Case insensitive script",
			input:    "<SCRIPT>alert('xss')</SCRIPT>",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsPotentialXSS(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSanitizeMap(t *testing.T) {
	sanitizer := DefaultSanitizer()

	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "Clean map",
			input: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			expected: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "Map with XSS",
			input: map[string]interface{}{
				"key1": "<script>alert('xss')</script>",
				"key2": "normal value",
			},
			expected: map[string]interface{}{
				"key1": "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
				"key2": "normal value",
			},
		},
		{
			name: "Nested map",
			input: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner": "<script>alert('xss')</script>",
				},
			},
			expected: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner": "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
				},
			},
		},
		{
			name:     "Nil map",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.SanitizeMap(tt.input)
			if !mapsEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAllowHTMLConfig(t *testing.T) {
	sanitizer := NewSanitizer(Config{
		AllowHTML: true,
		MaxLength: 1000,
	})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Normal HTML",
			input:    "<p>Hello World</p>",
			expected: "<p>Hello World</p>",
		},
		{
			name:     "Dangerous script",
			input:    "<script>alert('xss')</script>",
			expected: "[FILTERED]",
		},
		{
			name:     "Event handler",
			input:    `<div onload="alert('xss')">Content</div>`,
			expected: `<div [FILTERED]"alert('xss')">Content</div>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.SanitizeString(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestCustomMaxLength(t *testing.T) {
	sanitizer := NewSanitizer(Config{
		AllowHTML: false,
		MaxLength: 10,
	})

	input := "This is a very long string that exceeds the limit"
	expected := "This is a ..."
	result := sanitizer.SanitizeString(input)

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// Benchmark tests
func BenchmarkSanitizeString(b *testing.B) {
	sanitizer := DefaultSanitizer()
	input := "This is a test string with some content"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sanitizer.SanitizeString(input)
	}
}

func BenchmarkSanitizeStringWithXSS(b *testing.B) {
	sanitizer := DefaultSanitizer()
	input := `<script>alert('xss')</script><iframe src="evil.com"></iframe>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sanitizer.SanitizeString(input)
	}
}

func BenchmarkSanitizeURL(b *testing.B) {
	sanitizer := DefaultSanitizer()
	input := "http://example.com:8080/path?param=value"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sanitizer.SanitizeURL(input)
	}
}

// Helper function to compare maps for testing
func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	for key, aVal := range a {
		bVal, exists := b[key]
		if !exists {
			return false
		}

		switch aV := aVal.(type) {
		case string:
			if bV, ok := bVal.(string); !ok || aV != bV {
				return false
			}
		case map[string]interface{}:
			if bV, ok := bVal.(map[string]interface{}); !ok || !mapsEqual(aV, bV) {
				return false
			}
		default:
			if aVal != bVal {
				return false
			}
		}
	}

	return true
}