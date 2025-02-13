package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"proxycheck/testhelpers"
)

// proxyHandler implements a simple HTTP proxy
type proxyHandler struct {
	client *http.Client
}

func (p *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		// Handle HTTPS proxy requests
		host := r.URL.Host
		conn, err := net.Dial("tcp", host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
			return
		}
		clientConn, _, err := hijacker.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		go transfer(clientConn, conn)
		go transfer(conn, clientConn)
	} else {
		// Handle HTTP proxy requests
		resp, err := p.client.Get(r.URL.String())
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
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

func TestProxyLoading(t *testing.T) {
	progress := testhelpers.NewTestProgress()
	defer progress.PrintSummary()

	// Create a temporary file with test proxies
	progress.StartTest("Proxy Loading")
	content := `http://proxy1.example.com:8080
https://proxy2.example.com:443
proxy3.example.com:3128
socks5://proxy4.example.com:1080
http://proxy5.example.com:80
https://proxy6.example.com
`
	tmpfile, err := os.CreateTemp("", "proxies*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test loading proxies
	proxies, warnings, err := loadProxies(tmpfile.Name())
	if err != nil {
		progress.AddResult(testhelpers.TestResult{
			Name:    "Load Proxies",
			Passed:  false,
			Message: fmt.Sprintf("Failed to load proxies: %v", err),
		})
		t.Fatalf("loadProxies() error = %v", err)
	}

	// Expected results after cleaning
	expected := []string{
		"http://proxy1.example.com:8080",
		"https://proxy2.example.com",
		"http://proxy3.example.com:3128",
		"socks5://proxy4.example.com:1080",
		"http://proxy5.example.com",
		"https://proxy6.example.com",
	}

	// Check proxy count
	if len(proxies) != len(expected) {
		progress.AddResult(testhelpers.TestResult{
			Name:    "Proxy Count Check",
			Passed:  false,
			Message: fmt.Sprintf("Got %d proxies, want %d", len(proxies), len(expected)),
			Details: []string{fmt.Sprintf("Actual proxies: %v", proxies)},
		})
	} else {
		progress.AddResult(testhelpers.TestResult{
			Name:   "Proxy Count Check",
			Passed: true,
		})
	}

	// Check proxy formatting
	var formatErrors []string
	for i, proxy := range proxies {
		if i >= len(expected) {
			break
		}
		if proxy != expected[i] {
			formatErrors = append(formatErrors,
				fmt.Sprintf("proxy[%d] = %s, want %s", i, proxy, expected[i]))
		}
	}

	if len(formatErrors) > 0 {
		progress.AddResult(testhelpers.TestResult{
			Name:    "Proxy Format Check",
			Passed:  false,
			Message: "Proxy format mismatch",
			Details: formatErrors,
		})
	} else {
		progress.AddResult(testhelpers.TestResult{
			Name:   "Proxy Format Check",
			Passed: true,
		})
	}

	// Check SOCKS warnings
	foundSocksWarning := false
	var foundWarnings []string
	for _, warning := range warnings {
		foundWarnings = append(foundWarnings, warning)
		if strings.Contains(warning, "Warning: skipping unsupported scheme 'socks' for proxy") {
			foundSocksWarning = true
		}
	}

	progress.AddResult(testhelpers.TestResult{
		Name:    "SOCKS Warning Check",
		Passed:  foundSocksWarning,
		Message: "SOCKS proxy warning check",
		Details: foundWarnings,
	})
}

func TestProxyCheckingBasic(t *testing.T) {
	progress := testhelpers.NewTestProgress()
	defer progress.PrintSummary()

	progress.StartTest("Basic Proxy Checking")

	// Load default config for testing
	if err := loadConfig("config.yaml"); err != nil {
		progress.AddResult(testhelpers.TestResult{
			Name:    "Config Loading",
			Passed:  false,
			Message: fmt.Sprintf("Failed to load config: %v", err),
		})
		t.Fatalf("Failed to load config: %v", err)
	}

	progress.AddResult(testhelpers.TestResult{
		Name:   "Config Loading",
		Passed: true,
	})

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

	// Create a proxy server
	proxy := &proxyHandler{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
	proxyServer := httptest.NewServer(proxy)
	defer proxyServer.Close()

	// Create a channel for results
	results := make(chan ProxyResult, 1)

	// Test with the test server URL as proxy
	progress.StartTest("Working Proxy Check")
	go checkProxyWithInteractsh(proxyServer.URL, targetServer.URL, false, false, false, true, 5*time.Second, results)

	// Get the result
	result := <-results

	// Check the result
	if !result.Working {
		progress.AddResult(testhelpers.TestResult{
			Name:    "Working Proxy",
			Passed:  false,
			Message: fmt.Sprintf("Proxy check failed: %v", result.Error),
			Details: func() []string {
				var details []string
				if len(result.CheckResults) > 0 {
					for _, check := range result.CheckResults {
						details = append(details,
							fmt.Sprintf("URL: %s, Success: %v, Error: %s",
								check.URL, check.Success, check.Error))
					}
				}
				return details
			}(),
		})
	} else {
		progress.AddResult(testhelpers.TestResult{
			Name:   "Working Proxy",
			Passed: true,
			Details: []string{
				fmt.Sprintf("Response time: %v", result.Speed),
				fmt.Sprintf("Successful checks: %d", len(result.CheckResults)),
			},
		})
	}

	// Test with an invalid proxy
	progress.StartTest("Invalid Proxy Check")
	invalidProxyURL := "http://invalid.proxy:8080"
	go checkProxyWithInteractsh(invalidProxyURL, targetServer.URL, false, false, false, true, 2*time.Second, results)

	// Get the result
	result = <-results

	// Check invalid proxy results
	if result.Working {
		progress.AddResult(testhelpers.TestResult{
			Name:    "Invalid Proxy",
			Passed:  false,
			Message: "Invalid proxy check unexpectedly succeeded",
		})
	} else {
		progress.AddResult(testhelpers.TestResult{
			Name:   "Invalid Proxy",
			Passed: true,
			Details: []string{
				fmt.Sprintf("Error: %v", result.Error),
			},
		})
	}
}
