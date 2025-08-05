package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"h12.io/socks"
)

// createClient creates an HTTP client for the given proxy type
func (c *Checker) createClient(proxyURL *url.URL, scheme string, result *ProxyResult) (*http.Client, error) {
	if c.debug {
		result.DebugInfo += fmt.Sprintf("[DEBUG] Creating client for scheme: %s\n", scheme)
		result.DebugInfo += fmt.Sprintf("[DEBUG] Proxy URL: %s\n", proxyURL.String())
	}

	transport := &http.Transport{
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

	switch {
	case scheme == "http" || scheme == "https":
		transport.Proxy = func(_ *http.Request) (*url.URL, error) {
			proxyURLWithScheme := fmt.Sprintf("%s://%s", scheme, proxyURL.Host)
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[DEBUG] Setting up %s proxy: %s\n", scheme, proxyURLWithScheme)
			}
			return url.Parse(proxyURLWithScheme)
		}

	case scheme == "socks4" || scheme == "socks5":
		dialSocksProxy := socks.Dial(fmt.Sprintf("%s://%s", scheme, proxyURL.Host))
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DEBUG] Setting up %s proxy with dial function\n", scheme)
		}
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[DEBUG] Dialing %s address: %s\n", network, addr)
			}
			conn, err := dialSocksProxy(network, addr)
			if err != nil && c.debug {
				result.DebugInfo += fmt.Sprintf("[DEBUG] Dial error: %v\n", err)
			}
			return conn, err
		}
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