package proxy

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// createClient creates an HTTP client for the given proxy type
func (c *Checker) createClient(proxyURL *url.URL, scheme string, result *ProxyResult) (*http.Client, error) {
	if c.debug {
		result.DebugInfo += fmt.Sprintf("[DEBUG] Creating client for scheme: %s\n", scheme)
		result.DebugInfo += fmt.Sprintf("[DEBUG] Proxy URL: %s\n", c.cleanProxyURL(proxyURL))
	}

	// Extract authentication information
	auth := c.getProxyAuth(proxyURL, result)

	// Try to use connection pool if available
	if c.config.ConnectionPool != nil {
		if pool, ok := c.config.ConnectionPool.(interface {
			GetClient(string, time.Duration) (*http.Client, error)
		}); ok {
			fullProxyURL := fmt.Sprintf("%s://%s", scheme, proxyURL.Host)
			client, err := pool.GetClient(fullProxyURL, c.config.Timeout)
			if err == nil {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("[DEBUG] Using connection pool client for: %s\n", fullProxyURL)
				}
				return client, nil
			}
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[DEBUG] Connection pool failed, falling back to manual client creation: %v\n", err)
			}
		}
	}

	// Fallback to manual client creation with authentication support
	var transport *http.Transport

	switch {
	case scheme == "http" || scheme == "https":
		transport = c.createAuthenticatedHTTPTransport(proxyURL, scheme, auth, result)

	case scheme == "socks4" || scheme == "socks5":
		transport = &http.Transport{
			TLSHandshakeTimeout:   c.config.Timeout / 2,
			ResponseHeaderTimeout: c.config.Timeout / 2,
			ExpectContinueTimeout: 1 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			DisableKeepAlives:     true,
			ForceAttemptHTTP2:     false,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
		transport.DialContext = c.createAuthenticatedSOCKSDialer(proxyURL, scheme, auth, result)
	}

	// Set TLS config if not already set
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	// Add warning about disabled TLS verification
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		result.SecurityWarnings = append(result.SecurityWarnings,
			"TLS certificate verification disabled - proxy could perform MITM attacks")
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   c.config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[DEBUG] Created client with timeout: %v\n", client.Timeout)
	}

	return client, nil
}

// testClientWithDetails tests if the client works with a simple request and returns detailed information
func (c *Checker) testClientWithDetails(client *http.Client, proxyType ProxyType, result *ProxyResult) (bool, string, *CheckResult) {
	// Use different validation URLs based on proxy type
	testURL := c.config.ValidationURL
	if proxyType == ProxyTypeSOCKS4 || proxyType == ProxyTypeSOCKS5 {
		// For SOCKS proxies, try a plain HTTP URL first
		testURL = "http://api.ipify.org?format=json"
	}

	checkResult := &CheckResult{
		URL: testURL,
	}

	start := time.Now()
	
	// Apply rate limiting
	c.applyRateLimit(testURL, result)

	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		checkResult.Error = err.Error()
		return false, err.Error(), checkResult
	}

	// Add default headers
	for key, value := range c.config.DefaultHeaders {
		req.Header.Set(key, value)
	}

	if c.config.UserAgent != "" {
		req.Header.Set("User-Agent", c.config.UserAgent)
	}

	resp, err := client.Do(req)
	if err != nil {
		checkResult.Error = err.Error()
		checkResult.Speed = time.Since(start)
		return false, err.Error(), checkResult
	}
	defer resp.Body.Close()

	checkResult.StatusCode = resp.StatusCode
	checkResult.Speed = time.Since(start)

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		checkResult.Error = err.Error()
		return false, err.Error(), checkResult
	}

	checkResult.BodySize = int64(len(body))

	// Check if response is valid
	if !c.validateResponse(resp, body) {
		checkResult.Error = "response validation failed"
		return false, "response validation failed", checkResult
	}

	checkResult.Success = true
	return true, "", checkResult
}

// testClientWithError tests if the client works with a simple request and returns an error message
func (c *Checker) testClientWithError(client *http.Client, proxyType ProxyType, result *ProxyResult) (bool, string) {
	success, errorMsg, _ := c.testClientWithDetails(client, proxyType, result)
	return success, errorMsg
}

// testClient tests if the client works with a simple request
func (c *Checker) testClient(client *http.Client, proxyType ProxyType, result *ProxyResult) bool {
	success, _, _ := c.testClientWithDetails(client, proxyType, result)
	return success
}