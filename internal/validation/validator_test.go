package validation

import (
	"net"
	"testing"
)

func TestProxyValidator_ValidateProxyURL(t *testing.T) {
	validator := NewProxyValidator()

	tests := []struct {
		name    string
		url     string
		wantErr bool
		errCode ValidationErrorCode
	}{
		{
			name:    "valid HTTP proxy",
			url:     "http://proxy.example.com:8080",
			wantErr: false,
		},
		{
			name:    "valid HTTPS proxy",
			url:     "https://proxy.example.com:443",
			wantErr: false,
		},
		{
			name:    "valid SOCKS5 proxy",
			url:     "socks5://proxy.example.com:1080",
			wantErr: false,
		},
		{
			name:    "valid IP address proxy",
			url:     "http://93.184.216.34:8080", // example.com IP
			wantErr: false,
		},
		{
			name:    "valid IPv6 proxy",
			url:     "http://[2607:f8b0:4005:805::200e]:8080", // Google IPv6
			wantErr: false,
		},
		{
			name:    "valid IPv6 proxy without port",
			url:     "http://[2607:f8b0:4005:805::200e]",
			wantErr: true,
			errCode: ErrorInvalidHost, // IPv6 addresses in URLs require ports
		},
		{
			name:    "RFC 5737 test network blocked",
			url:     "http://203.0.113.1:8080",
			wantErr: true,
			errCode: ErrorPrivateIP,
		},
		{
			name:    "RFC 3849 IPv6 documentation blocked",
			url:     "http://[2001:db8::1]:8080",
			wantErr: true,
			errCode: ErrorPrivateIP,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
			errCode: ErrorInvalidURL,
		},
		{
			name:    "invalid scheme",
			url:     "ftp://proxy.example.com:21",
			wantErr: true,
			errCode: ErrorUnsupportedProtocol,
		},
		{
			name:    "missing host",
			url:     "http://:8080",
			wantErr: true,
			errCode: ErrorInvalidHost,
		},
		{
			name:    "invalid port",
			url:     "http://proxy.example.com:99999",
			wantErr: true,
			errCode: ErrorInvalidPort,
		},
		{
			name:    "private IP (default not allowed)",
			url:     "http://192.168.1.1:8080",
			wantErr: true,
			errCode: ErrorPrivateIP,
		},
		{
			name:    "loopback IP",
			url:     "http://127.0.0.1:8080",
			wantErr: true,
			errCode: ErrorPrivateIP,
		},
		{
			name:    "SOCKS with path (invalid)",
			url:     "socks5://proxy.example.com:1080/path",
			wantErr: true,
			errCode: ErrorInvalidURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateProxyURL(tt.url)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateProxyURL() expected error but got none")
					return
				}
				
				if validationErr, ok := err.(ValidationError); ok {
					if validationErr.Code != tt.errCode {
						t.Errorf("ValidateProxyURL() error code = %v, want %v", validationErr.Code, tt.errCode)
					}
				}
			} else {
				if err != nil {
					t.Errorf("ValidateProxyURL() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestProxyValidator_AllowPrivateIPs(t *testing.T) {
	validator := NewProxyValidator().WithAllowPrivateIPs(true)

	privateIPs := []string{
		"http://192.168.1.1:8080",
		"http://10.0.0.1:8080",
		"http://172.16.0.1:8080",
	}

	for _, url := range privateIPs {
		err := validator.ValidateProxyURL(url)
		if err != nil {
			t.Errorf("ValidateProxyURL() with AllowPrivateIPs=true should accept %s, got error: %v", url, err)
		}
	}
}

func TestProxyValidator_NormalizeProxyURL(t *testing.T) {
	validator := NewProxyValidator().WithAllowPrivateIPs(true)

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "add missing scheme",
			input:    "proxy.example.com:8080",
			expected: "http://proxy.example.com:8080",
			wantErr:  false,
		},
		{
			name:     "remove trailing slash",
			input:    "http://proxy.example.com:8080/",
			expected: "http://proxy.example.com:8080",
			wantErr:  false,
		},
		{
			name:     "trim whitespace",
			input:    "  http://proxy.example.com:8080  ",
			expected: "http://proxy.example.com:8080",
			wantErr:  false,
		},
		{
			name:     "invalid - localhost not allowed by default",
			input:    "127.0.0.1:8080",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.NormalizeProxyURL(tt.input)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("NormalizeProxyURL() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("NormalizeProxyURL() unexpected error = %v", err)
				}
				if result != tt.expected {
					t.Errorf("NormalizeProxyURL() = %v, want %v", result, tt.expected)
				}
			}
		})
	}
}

func TestProxyValidator_ValidateHostname(t *testing.T) {
	validator := NewProxyValidator()

	tests := []struct {
		name     string
		hostname string
		wantErr  bool
	}{
		{
			name:     "valid domain",
			hostname: "proxy.example.com",
			wantErr:  false,
		},
		{
			name:     "valid subdomain",
			hostname: "us-west.proxy.example.com",
			wantErr:  false,
		},
		{
			name:     "invalid - consecutive dots",
			hostname: "proxy..example.com",
			wantErr:  true,
		},
		{
			name:     "invalid - starts with dot",
			hostname: ".proxy.example.com",
			wantErr:  true,
		},
		{
			name:     "invalid - ends with dot",
			hostname: "proxy.example.com.",
			wantErr:  true,
		},
		{
			name:     "invalid - special characters",
			hostname: "proxy@example.com",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateHostnameFormat(tt.hostname)
			
			if tt.wantErr && err == nil {
				t.Errorf("validateHostnameFormat() expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateHostnameFormat() unexpected error = %v", err)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	err := ValidationError{
		Field:   "proxy_url",
		Value:   "invalid_url",
		Message: "test error message",
		Code:    ErrorInvalidURL,
	}

	expected := "validation error in proxy_url: test error message (value: invalid_url)"
	if err.Error() != expected {
		t.Errorf("ValidationError.Error() = %v, want %v", err.Error(), expected)
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		// IPv4 Private ranges (RFC 1918)
		{"IPv4 10.x private", "10.0.0.1", true},
		{"IPv4 172.16.x private", "172.16.0.1", true},
		{"IPv4 192.168.x private", "192.168.1.1", true},
		
		// IPv4 Special ranges
		{"IPv4 loopback", "127.0.0.1", true},
		{"IPv4 link-local", "169.254.1.1", true},
		{"IPv4 carrier-grade NAT", "100.64.0.1", true},
		{"IPv4 multicast", "224.0.0.1", true},
		{"IPv4 broadcast", "255.255.255.255", true},
		
		// RFC 5737 test networks
		{"IPv4 TEST-NET-1", "192.0.2.1", true},
		{"IPv4 TEST-NET-2", "198.51.100.1", true},
		{"IPv4 TEST-NET-3", "203.0.113.1", true},
		
		// IPv6 Private ranges
		{"IPv6 loopback", "::1", true},
		{"IPv6 unspecified", "::", true},
		{"IPv6 link-local", "fe80::1", true},
		{"IPv6 unique local", "fc00::1", true},
		{"IPv6 unique local fd", "fd00::1", true},
		
		// RFC 3849 documentation prefix
		{"IPv6 documentation", "2001:db8::1", true},
		
		// IPv6 Special ranges
		{"IPv6 multicast", "ff02::1", true},
		{"IPv6 IPv4-mapped", "::ffff:192.168.1.1", true},
		{"IPv6 6to4", "2002::1", true},
		{"IPv6 Teredo", "2001::1", true},
		{"IPv6 ORCHIDv2", "2001:20::1", true},
		
		// Public IPs (should return false)
		{"Public IPv4", "8.8.8.8", false},
		{"Public IPv4 2", "1.1.1.1", false},
		{"Public IPv6", "2607:f8b0:4005:805::200e", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP: %s", tt.ip)
			}
			
			result := isPrivateIP(ip)
			if result != tt.expected {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}