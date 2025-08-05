package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTP3Config represents HTTP/3 specific configuration
type HTTP3Config struct {
	Enabled               bool          `yaml:"enabled"`
	MaxIdleTimeout        time.Duration `yaml:"max_idle_timeout"`
	MaxIncomingStreams    int64         `yaml:"max_incoming_streams"`
	MaxIncomingUniStreams int64         `yaml:"max_incoming_uni_streams"`
	KeepAlive             bool          `yaml:"keep_alive"`
	EnableDatagrams       bool          `yaml:"enable_datagrams"`
}

// GetDefaultHTTP3Config returns default HTTP/3 configuration
func GetDefaultHTTP3Config() HTTP3Config {
	return HTTP3Config{
		Enabled:               true,
		MaxIdleTimeout:        30 * time.Second,
		MaxIncomingStreams:    100,
		MaxIncomingUniStreams: 100,
		KeepAlive:             true,
		EnableDatagrams:       false,
	}
}

// HTTP/3 support is currently limited due to external dependency requirements
// This is a placeholder for future HTTP/3 implementation when dependencies are available

// testHTTP3Support tests if a URL supports HTTP/3 by checking Alt-Svc headers
// This is a simplified approach that doesn't require external QUIC libraries
func (c *Checker) testHTTP3Support(proxyURL *url.URL, scheme string, result *ProxyResult) (bool, *http.Client, error) {
	if c.debug {
		result.DebugInfo += fmt.Sprintf("[HTTP3] Testing HTTP/3 support indication for: %s\n", scheme)
	}

	// HTTP/3 requires HTTPS
	if scheme != "https" {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[HTTP3] HTTP/3 requires HTTPS, skipping for scheme: %s\n", scheme)
		}
		return false, nil, fmt.Errorf("HTTP/3 requires HTTPS")
	}

	// Create a standard HTTPS client to check for HTTP/3 support indicators
	client := &http.Client{
		Timeout: c.config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Test with a known HTTP/3 endpoint or the configured validation URL
	testURL := "https://www.google.com" // Known to support HTTP/3
	if c.config.ValidationURL != "" && isHTTPS(c.config.ValidationURL) {
		testURL = c.config.ValidationURL
	}

	start := time.Now()
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), c.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[HTTP3] Failed to create request: %v\n", err)
		}
		return false, nil, err
	}

	// Add headers
	for key, value := range c.config.DefaultHeaders {
		req.Header.Set(key, value)
	}
	req.Header.Set("User-Agent", c.config.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[HTTP3] HTTP/3 indication test failed: %v\n", err)
		}
		return false, nil, err
	}
	defer resp.Body.Close()

	duration := time.Since(start)

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[HTTP3] Failed to read response body: %v\n", err)
		}
		return false, nil, err
	}

	// Check for HTTP/3 support indicators in headers
	altSvc := resp.Header.Get("Alt-Svc")
	hasHTTP3Support := strings.Contains(altSvc, "h3=") || strings.Contains(altSvc, "h3-")
	
	if c.debug {
		result.DebugInfo += fmt.Sprintf("[HTTP3] Response protocol: %s, Alt-Svc: %s, HTTP/3 indicated: %v, Status: %d, Time: %v\n",
			resp.Proto, altSvc, hasHTTP3Support, resp.StatusCode, duration)
	}

	// Create check result
	checkResult := CheckResult{
		URL:        testURL,
		Success:    hasHTTP3Support && resp.StatusCode == 200,
		Speed:      duration,
		StatusCode: resp.StatusCode,
		BodySize:   int64(len(body)),
	}

	if resp.StatusCode != 200 {
		checkResult.Error = fmt.Sprintf("unexpected status code: %d", resp.StatusCode)
	} else if !hasHTTP3Support {
		checkResult.Error = "no HTTP/3 support indicated in Alt-Svc header"
	}

	result.CheckResults = append(result.CheckResults, checkResult)

	if hasHTTP3Support && resp.StatusCode == 200 {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[HTTP3] HTTP/3 support indicated via Alt-Svc header\n")
		}
		return true, client, nil
	}

	return false, nil, fmt.Errorf("HTTP/3 not indicated or request failed")
}

// detectHTTP3Protocol detects if a proxy supports HTTP/3
func (c *Checker) detectHTTP3Protocol(proxyURL *url.URL, result *ProxyResult) (bool, *http.Client) {
	if c.debug {
		result.DebugInfo += fmt.Sprintf("[HTTP3] Detecting HTTP/3 protocol support\n")
	}

	// HTTP/3 only works over HTTPS
	if success, client, err := c.testHTTP3Support(proxyURL, "https", result); success {
		return true, client
	} else if c.debug {
		result.DebugInfo += fmt.Sprintf("[HTTP3] HTTP/3 test failed: %v\n", err)
	}

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[HTTP3] No HTTP/3 support detected\n")
	}

	return false, nil
}

// isHTTPS checks if a URL uses HTTPS scheme
func isHTTPS(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	return parsedURL.Scheme == "https"
}