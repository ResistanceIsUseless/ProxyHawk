package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/cloudcheck"
	proxylib "github.com/ResistanceIsUseless/ProxyHawk/proxy"
	testconfig "github.com/ResistanceIsUseless/ProxyHawk/tests/config"
	"github.com/ResistanceIsUseless/ProxyHawk/tests/testhelpers"
)

func TestProxyHawkFeatures(t *testing.T) {
	progress := testhelpers.NewTestProgress()
	defer progress.PrintSummary()

	// Load test configuration
	progress.StartTest("Configuration Loading")
	if err := testconfig.LoadTestConfig("../../config.yaml"); err != nil {
		progress.AddResult(testhelpers.TestResult{
			Name:    "Config Loading",
			Passed:  false,
			Message: err.Error(),
		})
		t.Fatal(err)
	}

	// Create test servers
	// 1. Main test server
	mainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<h1>Test Response</h1>
<p>This is a test response that meets the minimum size requirements.</p>
<p>Additional content to ensure we have enough bytes for validation.</p>
</body></html>`))
	}))
	defer mainServer.Close()

	// 2. Cloud metadata mock server
	metadataServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always respond with metadata for test purposes
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"instance-id": "test-1","region": "test-region"}`))
	}))
	defer metadataServer.Close()

	// 3. Internal endpoint mock server
	internalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "internal-access-success"}`))
	}))
	defer internalServer.Close()

	// Get internal server host:port
	internalURL, _ := url.Parse(internalServer.URL)
	internalHostPort := internalURL.Host

	// Create a test proxy server that supports all features
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add headers to test anonymity
		w.Header().Set("X-Forwarded-For", "test-proxy-ip")
		w.Header().Set("Via", "1.1 test-proxy")

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
			io.Copy(w, resp.Body)
		}
	}))
	defer proxyServer.Close()

	// Test configuration
	cfg := &proxylib.Config{
		Timeout:            10 * time.Second,
		MaxConcurrent:      1,
		ValidationURL:      mainServer.URL,
		ValidationPattern:  "Test Response",
		MinResponseBytes:   testconfig.TestConfigInstance.Validation.MinResponseBytes,
		DisallowedKeywords: testconfig.TestConfigInstance.Validation.DisallowedKeywords,
		DefaultHeaders: map[string]string{
			"Accept":          "text/html",
			"Accept-Language": "en-US,en;q=0.9",
		},
		UserAgent: "ProxyHawk Test/1.0",
	}

	// Test cases for different features
	tests := []struct {
		name     string
		testFunc func(t *testing.T, progress *testhelpers.TestProgress)
	}{
		{
			name: "Basic Connectivity",
			testFunc: func(t *testing.T, progress *testhelpers.TestProgress) {
				result, err := proxylib.Check(proxyServer.URL, cfg, true)
				if err != nil || !result.Working {
					progress.AddResult(testhelpers.TestResult{
						Name:    "Basic Connectivity",
						Passed:  false,
						Message: "Proxy should be working",
					})
					t.Error("Basic connectivity check failed")
					return
				}
				progress.AddResult(testhelpers.TestResult{
					Name:   "Basic Connectivity",
					Passed: true,
				})
			},
		},
		{
			name: "Response Validation",
			testFunc: func(t *testing.T, progress *testhelpers.TestProgress) {
				result, err := proxylib.Check(proxyServer.URL, cfg, true)
				if err != nil || len(result.CheckResults) == 0 {
					progress.AddResult(testhelpers.TestResult{
						Name:    "Response Validation",
						Passed:  false,
						Message: "Response validation failed",
					})
					t.Error("Response validation check failed")
					return
				}
				progress.AddResult(testhelpers.TestResult{
					Name:   "Response Validation",
					Passed: true,
				})
			},
		},
		{
			name: "Cloud Provider Detection",
			testFunc: func(t *testing.T, progress *testhelpers.TestProgress) {
				// Create test cloud provider
				provider := &cloudcheck.CloudProvider{
					Name:           "TestCloud",
					MetadataIPs:    []string{strings.TrimPrefix(metadataServer.URL, "http://")},
					MetadataURLs:   []string{metadataServer.URL + "/metadata"},
					InternalRanges: []string{internalHostPort},
				}
				client := &http.Client{Timeout: 5 * time.Second}
				cloudResult, err := cloudcheck.CheckInternalAccess(client, provider, true)
				if err != nil {
					progress.AddResult(testhelpers.TestResult{
						Name:    "Cloud Provider Detection",
						Passed:  false,
						Message: err.Error(),
					})
					t.Error("Cloud provider detection failed")
					return
				}
				if !cloudResult.InternalAccess {
					progress.AddResult(testhelpers.TestResult{
						Name:    "Cloud Provider Detection",
						Passed:  false,
						Message: "Expected internal access to be detected",
					})
					t.Error("Cloud provider detection failed - internal access not detected")
					return
				}
				progress.AddResult(testhelpers.TestResult{
					Name:   "Cloud Provider Detection",
					Passed: true,
				})
			},
		},
	}

	// Run all feature tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			progress.StartTest(tt.name)
			tt.testFunc(t, progress)
		})
	}
}
