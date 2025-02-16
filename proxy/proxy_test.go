package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProxyCheck(t *testing.T) {
	// Create a test server that acts as a target server
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<h1>Test Response</h1>
<p>This is a test response that meets the minimum size requirements.</p>
<p>Additional content to ensure we have enough bytes for validation.</p>
</body>
</html>`))
	}))
	defer targetServer.Close()

	// Create a proxy server for testing
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			// Handle HTTPS proxy requests
			w.WriteHeader(http.StatusOK)
		} else {
			// Handle HTTP proxy requests
			client := &http.Client{
				Timeout: 5 * time.Second,
			}
			resp, err := client.Get(r.URL.String())
			if err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
			defer resp.Body.Close()

			// Copy headers
			for key, values := range resp.Header {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			w.WriteHeader(resp.StatusCode)
			http.NewRequest("GET", r.URL.String(), resp.Body)
		}
	}))
	defer proxyServer.Close()

	// Create test configuration
	cfg := &Config{
		Timeout:           10 * time.Second,
		MaxConcurrent:     1,
		ValidationURL:     targetServer.URL,
		ValidationPattern: ".*",
		DisallowedKeywords: []string{
			"Access Denied",
			"Proxy Error",
		},
		MinResponseBytes: 100,
		DefaultHeaders: map[string]string{
			"Accept":          "text/html",
			"Accept-Language": "en-US,en;q=0.9",
		},
		UserAgent: "ProxyCheck Test/1.0",
	}

	tests := []struct {
		name        string
		proxyURL    string
		wantWorking bool
		wantErr     bool
	}{
		{
			name:        "Working HTTP Proxy",
			proxyURL:    proxyServer.URL,
			wantWorking: true,
			wantErr:     false,
		},
		{
			name:        "Invalid Proxy URL",
			proxyURL:    "not-a-url",
			wantWorking: false,
			wantErr:     true,
		},
		{
			name:        "Non-existent Proxy",
			proxyURL:    "http://localhost:1",
			wantWorking: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Check(tt.proxyURL, cfg, true)

			if (err != nil) != tt.wantErr {
				t.Errorf("Check() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && result.Working != tt.wantWorking {
				t.Errorf("Check() working = %v, want %v", result.Working, tt.wantWorking)
			}

			if err == nil && result.Working {
				if result.Speed <= 0 {
					t.Error("Check() speed should be greater than 0 for working proxy")
				}
				if len(result.CheckResults) == 0 {
					t.Error("Check() should have at least one check result for working proxy")
				}
			}
		})
	}
}

func TestValidateResponse(t *testing.T) {
	cfg := &Config{
		MinResponseBytes: 100,
		DisallowedKeywords: []string{
			"Error",
			"Access Denied",
		},
	}

	tests := []struct {
		name    string
		resp    *http.Response
		body    []byte
		want    bool
		wantErr bool
	}{
		{
			name: "Valid Response",
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
			},
			body: []byte(`<!DOCTYPE html><html><body>
				<h1>Valid Response</h1>
				<p>This is a valid response with sufficient content.</p>
				</body></html>`),
			want:    true,
			wantErr: false,
		},
		{
			name: "Invalid Status Code",
			resp: &http.Response{
				StatusCode: http.StatusNotFound,
				Header:     make(http.Header),
			},
			body:    []byte("Not Found"),
			want:    false,
			wantErr: false,
		},
		{
			name: "Response Too Small",
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
			},
			body:    []byte("Small"),
			want:    false,
			wantErr: false,
		},
		{
			name: "Contains Disallowed Keyword",
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
			},
			body:    []byte("This response contains an Error that should be detected"),
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ValidateResponse(tt.resp, tt.body, cfg, true)
			if got != tt.want {
				t.Errorf("ValidateResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}
