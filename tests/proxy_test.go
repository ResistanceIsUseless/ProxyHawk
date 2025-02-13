package tests

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// TestProxyValidation tests the basic proxy validation functionality
func TestProxyValidation(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Date", time.Now().Format(http.TimeFormat))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Test response</body></html>"))
	}))
	defer server.Close()

	// Load test configuration
	if err := loadConfig("../config.yaml"); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Create test cases
	tests := []struct {
		name     string
		proxy    string
		wantErr  bool
		wantCode int
	}{
		{
			name:     "Valid HTTP Proxy",
			proxy:    server.URL,
			wantErr:  false,
			wantCode: http.StatusOK,
		},
		{
			name:     "Invalid Proxy URL",
			proxy:    "invalid://proxy:8080",
			wantErr:  true,
			wantCode: 0,
		},
		{
			name:     "Non-existent Proxy",
			proxy:    "http://non.existent.proxy:8080",
			wantErr:  true,
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxyURL, err := url.Parse(tt.proxy)
			if err != nil && !tt.wantErr {
				t.Errorf("Failed to parse proxy URL: %v", err)
				return
			}

			client, err := createProxyClient(proxyURL, 5*time.Second)
			if (err != nil) != tt.wantErr {
				t.Errorf("createProxyClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				resp, err := client.Get(server.URL)
				if err != nil {
					t.Errorf("Failed to make request: %v", err)
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != tt.wantCode {
					t.Errorf("Got status code %d, want %d", resp.StatusCode, tt.wantCode)
				}
			}
		})
	}
}
