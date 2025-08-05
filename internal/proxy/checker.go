package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/cloudcheck"
	"h12.io/socks"
)

// ProxyType represents the type of proxy
type ProxyType string

const (
	ProxyTypeUnknown ProxyType = "unknown"
	ProxyTypeHTTP    ProxyType = "http"
	ProxyTypeHTTPS   ProxyType = "https"
	ProxyTypeSOCKS4  ProxyType = "socks4"
	ProxyTypeSOCKS5  ProxyType = "socks5"
)

// Config represents proxy checker configuration
type Config struct {
	// General settings
	Timeout            time.Duration
	ValidationURL      string
	ValidationPattern  string
	DisallowedKeywords []string
	MinResponseBytes   int
	DefaultHeaders     map[string]string
	UserAgent          string
	EnableCloudChecks  bool
	CloudProviders     []cloudcheck.CloudProvider
	UseRDNS            bool // Whether to use rDNS lookup for host headers

	// Rate limiting settings
	RateLimitEnabled bool          // Whether rate limiting is enabled
	RateLimitDelay   time.Duration // Delay between requests to the same host
	RateLimitPerHost bool          // Whether to apply rate limiting per host or globally

	// Response validation settings
	RequireStatusCode   int
	RequireContentMatch string
	RequireHeaderFields []string

	// Advanced security checks
	AdvancedChecks AdvancedChecks

	// Interactsh settings (used only when advanced checks are enabled)
	InteractshURL   string // URL of the Interactsh server (optional)
	InteractshToken string // Token for the Interactsh server (optional)
}

// CheckResult represents the result of checking a proxy
type CheckResult struct {
	URL        string        `json:"url"`
	Success    bool          `json:"success"`
	Speed      time.Duration `json:"speed"`
	Error      string        `json:"error,omitempty"`
	StatusCode int           `json:"status_code"`
	BodySize   int64         `json:"body_size"`
}

// ProxyResult represents the complete result of proxy checking
type ProxyResult struct {
	ProxyURL       string
	Type           ProxyType
	Working        bool
	Speed          time.Duration
	Error          string
	IsAnonymous    bool
	RealIP         string
	ProxyIP        string
	CloudProvider  string
	InternalAccess bool
	MetadataAccess bool
	CheckResults   []CheckResult
	DebugInfo      string

	// New fields for protocol support
	SupportsHTTP  bool
	SupportsHTTPS bool
}

// Checker handles proxy checking functionality
type Checker struct {
	config Config
	debug  bool

	// Rate limiting
	rateLimiter     map[string]time.Time // Map of host to last request time
	rateLimiterLock sync.Mutex           // Mutex to protect the rate limiter map
}

// NewChecker creates a new proxy checker
func NewChecker(config Config, debug bool) *Checker {
	return &Checker{
		config:      config,
		debug:       debug,
		rateLimiter: make(map[string]time.Time),
	}
}

// Check validates a proxy and returns detailed information about its functionality
func (c *Checker) Check(proxyURL string) *ProxyResult {
	result := &ProxyResult{
		ProxyURL:      proxyURL,
		Type:          ProxyTypeUnknown,
		CheckResults:  []CheckResult{},
		SupportsHTTP:  false,
		SupportsHTTPS: false,
	}

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[PROXY CHECK] Starting check for: %s\n", proxyURL)
	}

	// Parse the proxy URL
	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		result.Error = fmt.Sprintf("invalid proxy URL: %v", err)
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[ERROR] Failed to parse URL: %v\n", err)
		}
		return result
	}

	// Create a phased approach with clear stage markers in debug output
	if c.debug {
		result.DebugInfo += fmt.Sprintf("[PHASE 1/2] Detecting proxy type for %s\n", proxyURL)
	}

	// Determine proxy type
	proxyType, client, err := c.determineProxyType(parsedURL, result)
	if err != nil {
		// Create a more concise error message
		result.Error = fmt.Sprintf("proxy check failed: %v", err)
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[RESULT] Proxy type detection failed: %v\n", err)
		}
		return result
	}

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[PHASE 1/2 COMPLETE] Successfully detected proxy type: %s\n", proxyType)
		result.DebugInfo += fmt.Sprintf("[PHASE 2/2] Performing validation checks for %s proxy\n", proxyType)
	}

	result.Type = proxyType

	// Perform checks using the determined client
	if err := c.performChecks(client, result); err != nil {
		result.Error = fmt.Sprintf("validation failed: %v", err)
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[RESULT] Validation checks failed: %v\n", err)
		}
		return result
	}

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[PHASE 2/2 COMPLETE] Validation successful\n")
		result.DebugInfo += fmt.Sprintf("[SUMMARY] Proxy check results for %s:\n", proxyURL)
		result.DebugInfo += fmt.Sprintf("  - Type: %s\n", result.Type)
		result.DebugInfo += fmt.Sprintf("  - Working: %t\n", result.Working)
		result.DebugInfo += fmt.Sprintf("  - Speed: %v\n", result.Speed)
		result.DebugInfo += fmt.Sprintf("  - Check Steps: %d\n", len(result.CheckResults))
	}

	return result
}

// determineProxyType attempts to determine the type of proxy by testing different protocols
func (c *Checker) determineProxyType(proxyURL *url.URL, result *ProxyResult) (ProxyType, *http.Client, error) {
	var lastError string

	// Save the original validation URL at the beginning of the function
	origValidationURL := c.config.ValidationURL
	// Ensure we restore the original URL at the end of the function
	defer func() {
		c.config.ValidationURL = origValidationURL
	}()

	// First check if the proxy URL already specifies a scheme we can use
	if proxyURL.Scheme != "" {
		proxyType := ProxyTypeUnknown
		scheme := proxyURL.Scheme

		// Map URL scheme to ProxyType
		switch strings.ToLower(scheme) {
		case "http":
			proxyType = ProxyTypeHTTP
		case "https":
			proxyType = ProxyTypeHTTPS
		case "socks4":
			proxyType = ProxyTypeSOCKS4
		case "socks5":
			proxyType = ProxyTypeSOCKS5
		}

		if proxyType != ProxyTypeUnknown {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[TYPE] Using scheme from URL: %s\n", scheme)
			}

			// Try this scheme first
			client, err := c.createClient(proxyURL, scheme, result)
			if err == nil {
				// Test with HTTP endpoint
				c.config.ValidationURL = "http://api.ipify.org?format=json"
				httpSuccess, httpTestErr, httpCheckResult := c.testClientWithDetails(client, proxyType, result)

				// Add the check result to our collection
				if httpCheckResult != nil {
					result.CheckResults = append(result.CheckResults, *httpCheckResult)
				}

				// Then test with HTTPS endpoint
				c.config.ValidationURL = "https://api.ipify.org?format=json"
				httpsSuccess, httpsTestErr, httpsCheckResult := c.testClientWithDetails(client, proxyType, result)

				// Add the check result to our collection
				if httpsCheckResult != nil {
					result.CheckResults = append(result.CheckResults, *httpsCheckResult)
				}

				// Set protocol support based on results
				if httpSuccess {
					result.SupportsHTTP = true
					if c.debug {
						result.DebugInfo += fmt.Sprintf("[TYPE] Success! %s proxy supports HTTP\n", proxyType)
					}
				}

				if httpsSuccess {
					result.SupportsHTTPS = true
					if c.debug {
						result.DebugInfo += fmt.Sprintf("[TYPE] Success! %s proxy supports HTTPS\n", proxyType)
					}
				}

				if httpSuccess || httpsSuccess {
					if c.debug {
						if httpSuccess && httpsSuccess {
							result.DebugInfo += fmt.Sprintf("[TYPE] Using %s proxy with both HTTP and HTTPS support\n", proxyType)
						} else if httpSuccess {
							result.DebugInfo += fmt.Sprintf("[TYPE] Using %s proxy with HTTP support only\n", proxyType)
						} else {
							result.DebugInfo += fmt.Sprintf("[TYPE] Using %s proxy with HTTPS support only\n", proxyType)
						}
					}
					return proxyType, client, nil
				}

				if c.debug && !httpSuccess && !httpsSuccess {
					result.DebugInfo += fmt.Sprintf("[TYPE] Specified scheme %s failed: HTTP: %s, HTTPS: %s\n",
						scheme, httpTestErr, httpsTestErr)
				}
			} else if c.debug {
				result.DebugInfo += fmt.Sprintf("[TYPE] Failed to create client for specified scheme %s: %v\n",
					scheme, err)
			}
		}
	}

	// If URL scheme detection failed, now try protocols in order: HTTP, HTTPS, SOCKS4, SOCKS5
	// First try HTTP/HTTPS proxies
	httpProxyCandidates := []struct {
		proxyType ProxyType
		scheme    string
	}{
		{ProxyTypeHTTP, "http"},
		{ProxyTypeHTTPS, "https"},
	}

	// Try HTTP/HTTPS proxy types first
	if c.debug {
		result.DebugInfo += fmt.Sprintf("[TYPE] Testing as HTTP/HTTPS proxy: %s\n", proxyURL.Host)
	}

	type httpTestResult struct {
		proxyType ProxyType
		client    *http.Client
		success   bool
		protocol  string // "http" or "https"
		speed     time.Duration
	}

	var httpResults []httpTestResult

	for _, candidate := range httpProxyCandidates {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[TYPE] Trying as %s proxy\n", candidate.proxyType)
		}

		client, err := c.createClient(proxyURL, candidate.scheme, result)
		if err != nil {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[TYPE] Failed to create client for %s: %v\n", candidate.proxyType, err)
			}
			lastError = fmt.Sprintf("client creation failed for %s: %v", candidate.proxyType, err)
			continue
		}

		// Test with HTTP endpoint
		c.config.ValidationURL = "http://api.ipify.org?format=json"
		httpSuccess, httpTestErr, httpCheckResult := c.testClientWithDetails(client, candidate.proxyType, result)

		// Add the check result to our collection
		if httpCheckResult != nil {
			result.CheckResults = append(result.CheckResults, *httpCheckResult)
		}

		if httpSuccess {
			httpResults = append(httpResults, httpTestResult{
				proxyType: candidate.proxyType,
				client:    client,
				success:   true,
				protocol:  "http",
				speed:     httpCheckResult.Speed,
			})

			// Set HTTP support flag
			result.SupportsHTTP = true

			if c.debug {
				result.DebugInfo += fmt.Sprintf("[TYPE] Success! Working as %s proxy with HTTP endpoint\n", candidate.proxyType)
			}
		}

		// Then test with HTTPS endpoint
		c.config.ValidationURL = "https://api.ipify.org?format=json"
		httpsSuccess, httpsTestErr, httpsCheckResult := c.testClientWithDetails(client, candidate.proxyType, result)

		// Add the check result to our collection
		if httpsCheckResult != nil {
			result.CheckResults = append(result.CheckResults, *httpsCheckResult)
		}

		if httpsSuccess {
			httpResults = append(httpResults, httpTestResult{
				proxyType: candidate.proxyType,
				client:    client,
				success:   true,
				protocol:  "https",
				speed:     httpsCheckResult.Speed,
			})

			// Set HTTPS support flag
			result.SupportsHTTPS = true

			if c.debug {
				result.DebugInfo += fmt.Sprintf("[TYPE] Success! Working as %s proxy with HTTPS endpoint\n", candidate.proxyType)
			}
		}

		// If both HTTP and HTTPS succeeded, return right away
		if httpSuccess && httpsSuccess {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[TYPE] %s proxy supports both HTTP and HTTPS\n", candidate.proxyType)
			}
			return candidate.proxyType, client, nil
		}

		// If only HTTP succeeded, continue checking other proxy types
		if httpSuccess && !httpsSuccess {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[TYPE] %s proxy supports HTTP but not HTTPS: %s\n",
					candidate.proxyType, httpsTestErr)
			}
			// Don't return immediately - we'll store this as a fallback
		}

		// If neither succeeded, log errors
		if !httpSuccess && !httpsSuccess {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[TYPE] Failed as %s proxy: HTTP: %s, HTTPS: %s\n",
					candidate.proxyType, httpTestErr, httpsTestErr)
			}
			lastError = fmt.Sprintf("HTTP: %s, HTTPS: %s", httpTestErr, httpsTestErr)
		}
	}

	// If we found HTTP proxies but none supported HTTPS, still use the best HTTP proxy
	if len(httpResults) > 0 {
		// Try to find a proxy that supports HTTPS
		for _, r := range httpResults {
			if r.protocol == "https" {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("[TYPE] Selected %s proxy with HTTPS support\n", r.proxyType)
				}
				return r.proxyType, r.client, nil
			}
		}

		// If none support HTTPS, use the first HTTP result
		best := httpResults[0]
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[TYPE] Selected %s proxy with HTTP support only\n", best.proxyType)
		}
		return best.proxyType, best.client, nil
	}

	// If HTTP/HTTPS failed, try SOCKS proxies
	if c.debug {
		result.DebugInfo += fmt.Sprintf("[TYPE] Testing as SOCKS proxy: %s\n", proxyURL.Host)
	}

	// Define SOCKS proxy candidates, testing SOCKS5 first
	socksProxyCandidates := []struct {
		proxyType ProxyType
		scheme    string
	}{
		{ProxyTypeSOCKS5, "socks5"},
		{ProxyTypeSOCKS4, "socks4"},
	}

	type socksTestResult struct {
		proxyType ProxyType
		client    *http.Client
		success   bool
		protocol  string // "http" or "https"
		speed     time.Duration
	}

	var socksResults []socksTestResult

	for _, candidate := range socksProxyCandidates {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[TYPE] Trying as %s proxy\n", candidate.proxyType)
		}

		client, err := c.createClient(proxyURL, candidate.scheme, result)
		if err != nil {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[TYPE] Failed to create client for %s: %v\n", candidate.proxyType, err)
			}
			lastError = fmt.Sprintf("client creation failed for %s: %v", candidate.proxyType, err)
			continue
		}

		// Test with HTTP endpoint
		c.config.ValidationURL = "http://api.ipify.org?format=json"
		httpSuccess, httpTestErr, httpCheckResult := c.testClientWithDetails(client, candidate.proxyType, result)

		// Add the check result to our collection
		if httpCheckResult != nil {
			result.CheckResults = append(result.CheckResults, *httpCheckResult)
		}

		if httpSuccess {
			socksResults = append(socksResults, socksTestResult{
				proxyType: candidate.proxyType,
				client:    client,
				success:   true,
				protocol:  "http",
				speed:     httpCheckResult.Speed,
			})

			// Set HTTP support flag
			result.SupportsHTTP = true

			if c.debug {
				result.DebugInfo += fmt.Sprintf("[TYPE] Success! Working as %s proxy with HTTP endpoint\n", candidate.proxyType)
			}
		}

		// Test with HTTPS endpoint
		c.config.ValidationURL = "https://api.ipify.org?format=json"
		httpsSuccess, httpsTestErr, httpsCheckResult := c.testClientWithDetails(client, candidate.proxyType, result)

		// Add the check result to our collection
		if httpsCheckResult != nil {
			result.CheckResults = append(result.CheckResults, *httpsCheckResult)
		}

		if httpsSuccess {
			socksResults = append(socksResults, socksTestResult{
				proxyType: candidate.proxyType,
				client:    client,
				success:   true,
				protocol:  "https",
				speed:     httpsCheckResult.Speed,
			})

			// Set HTTPS support flag
			result.SupportsHTTPS = true

			if c.debug {
				result.DebugInfo += fmt.Sprintf("[TYPE] Success! Working as %s proxy with HTTPS endpoint\n", candidate.proxyType)
			}
		}

		// If both HTTP and HTTPS succeeded, return right away (prefer SOCKS5 over SOCKS4)
		if httpSuccess && httpsSuccess {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[TYPE] %s proxy supports both HTTP and HTTPS\n", candidate.proxyType)
			}
			return candidate.proxyType, client, nil
		}

		// If only one protocol succeeded, continue checking other proxy types
		if (httpSuccess || httpsSuccess) && candidate.proxyType == ProxyTypeSOCKS5 {
			// For SOCKS5, if either protocol works, we consider it a strong candidate
			if c.debug {
				if httpSuccess && !httpsSuccess {
					result.DebugInfo += fmt.Sprintf("[TYPE] SOCKS5 proxy supports HTTP but not HTTPS: %s\n", httpsTestErr)
				} else if !httpSuccess && httpsSuccess {
					result.DebugInfo += fmt.Sprintf("[TYPE] SOCKS5 proxy supports HTTPS but not HTTP: %s\n", httpTestErr)
				}
			}
			// We prefer SOCKS5 when possible, so return immediately
			return candidate.proxyType, client, nil
		}

		// If neither succeeded, log errors
		if !httpSuccess && !httpsSuccess {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[TYPE] Failed as %s proxy: HTTP: %s, HTTPS: %s\n",
					candidate.proxyType, httpTestErr, httpsTestErr)
			}
			lastError = fmt.Sprintf("HTTP: %s, HTTPS: %s", httpTestErr, httpsTestErr)
		}
	}

	// If we have SOCKS results but didn't return earlier, select the best one
	if len(socksResults) > 0 {
		// First try to find a SOCKS5 proxy that supports HTTPS
		for _, r := range socksResults {
			if r.proxyType == ProxyTypeSOCKS5 && r.protocol == "https" {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("[TYPE] Selected SOCKS5 proxy with HTTPS support\n")
				}
				return r.proxyType, r.client, nil
			}
		}

		// Then try a SOCKS5 proxy with HTTP only
		for _, r := range socksResults {
			if r.proxyType == ProxyTypeSOCKS5 && r.protocol == "http" {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("[TYPE] Selected SOCKS5 proxy with HTTP support only\n")
				}
				return r.proxyType, r.client, nil
			}
		}

		// Then try a SOCKS4 proxy with HTTPS
		for _, r := range socksResults {
			if r.proxyType == ProxyTypeSOCKS4 && r.protocol == "https" {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("[TYPE] Selected SOCKS4 proxy with HTTPS support\n")
				}
				return r.proxyType, r.client, nil
			}
		}

		// Finally, use any SOCKS proxy we found
		best := socksResults[0]
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[TYPE] Selected %s proxy with %s support\n",
				best.proxyType, best.protocol)
		}
		return best.proxyType, best.client, nil
	}

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[TYPE] All proxy types failed for %s\n", proxyURL.Host)
	}

	if lastError == "" {
		lastError = "all proxy types failed with unknown errors"
	}

	return ProxyTypeUnknown, nil, fmt.Errorf("could not determine proxy type: %s", lastError)
}

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
// This is an enhanced version of testClientWithError that also returns the CheckResult
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

	// For HTTPS URLs through HTTP proxies, we need to handle the CONNECT method
	isHTTPSOverHTTP := strings.HasPrefix(testURL, "https://") && proxyType == ProxyTypeHTTP

	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		errMsg := fmt.Sprintf("error creating request: %v", err)
		checkResult.Error = errMsg
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[CHECK] %s\n", errMsg)
		}
		return false, errMsg, checkResult
	}

	// Add headers
	req.Header.Set("User-Agent", c.config.UserAgent)
	for key, value := range c.config.DefaultHeaders {
		req.Header.Set(key, value)
	}

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[CHECK] Testing %s proxy connection to: %s\n", proxyType, testURL)
	}

	start := time.Now()
	resp, err := client.Do(req)
	checkResult.Speed = time.Since(start)

	if err != nil {
		// For HTTPS over HTTP proxy, we need to check if this is a CONNECT error
		if isHTTPSOverHTTP {
			if strings.Contains(err.Error(), "Proxy Authentication Required") ||
				strings.Contains(err.Error(), "407") {
				errMsg := fmt.Sprintf("%s proxy requires authentication", proxyType)
				checkResult.Error = errMsg
				if c.debug {
					result.DebugInfo += fmt.Sprintf("[CHECK] %s\n", errMsg)
				}
				return false, errMsg, checkResult
			}

			// Some proxies might return different errors for CONNECT
			if strings.Contains(err.Error(), "CONNECT") {
				// This might be a CONNECT handshake error, which is expected
				// Let's try to determine if it's a valid proxy despite the error
				if strings.Contains(err.Error(), "connection established") ||
					strings.Contains(err.Error(), "200") {
					// This is actually a successful CONNECT, but the subsequent request failed
					if c.debug {
						result.DebugInfo += fmt.Sprintf("[CHECK] CONNECT succeeded but subsequent request failed: %v\n", err)
					}
					// We'll mark this as a potential HTTP proxy that supports CONNECT
					result.SupportsHTTPS = true
					return true, "", checkResult
				}
			}
		}

		errMsg := fmt.Sprintf("%s connection error: %v", proxyType, err)
		checkResult.Error = errMsg
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[CHECK] Connection error for %s proxy: %v\n", proxyType, err)
		}
		return false, errMsg, checkResult
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		errMsg := fmt.Sprintf("error reading response body: %v", err)
		checkResult.Error = errMsg
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[CHECK] %s\n", errMsg)
		}
		return false, errMsg, checkResult
	}

	checkResult.StatusCode = resp.StatusCode
	checkResult.BodySize = int64(len(body))
	checkResult.Success = true

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[CHECK] Response status: %d\n", resp.StatusCode)
		result.DebugInfo += fmt.Sprintf("[CHECK] Response size: %d bytes\n", len(body))
		result.DebugInfo += fmt.Sprintf("[CHECK] Response time: %v\n", checkResult.Speed)
	}

	// For HTTPS over HTTP proxy, we need to be more lenient with status codes
	// as some proxies might return different success codes for CONNECT
	if isHTTPSOverHTTP {
		if resp.StatusCode == http.StatusOK || // 200 OK
			resp.StatusCode == http.StatusCreated || // 201 Created
			resp.StatusCode == http.StatusAccepted || // 202 Accepted
			resp.StatusCode == http.StatusNoContent { // 204 No Content
			result.SupportsHTTPS = true
		}
	} else if resp.StatusCode >= 400 {
		errMsg := fmt.Sprintf("server returned error status: %d %s", resp.StatusCode, resp.Status)
		checkResult.Error = errMsg
		checkResult.Success = false
		return false, errMsg, checkResult
	}

	if len(body) < c.config.MinResponseBytes {
		errMsg := fmt.Sprintf("response too small: %d bytes (min: %d)", len(body), c.config.MinResponseBytes)
		checkResult.Error = errMsg
		checkResult.Success = false
		return false, errMsg, checkResult
	}

	return true, "", checkResult
}

// testClientWithError tests if the client works with a simple request and returns detailed error information
// This is kept for backward compatibility
func (c *Checker) testClientWithError(client *http.Client, proxyType ProxyType, result *ProxyResult) (bool, string) {
	success, errMsg, _ := c.testClientWithDetails(client, proxyType, result)
	return success, errMsg
}

// testClient tests if the client works with a simple request
func (c *Checker) testClient(client *http.Client, proxyType ProxyType, result *ProxyResult) bool {
	success, _, _ := c.testClientWithDetails(client, proxyType, result)
	return success
}

// performChecks runs all configured checks for the proxy
func (c *Checker) performChecks(client *http.Client, result *ProxyResult) error {
	start := time.Now()

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[VALIDATE] Running validation checks\n")
	}

	// Make the request to the validation URL
	resp, err := c.makeRequest(client, c.config.ValidationURL, result)
	if err != nil {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[VALIDATE] Request failed: %v\n", err)
		}
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Record the time taken
	duration := time.Since(start)
	result.Speed = duration

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[VALIDATE] Failed to read response body: %v\n", err)
		}
		return fmt.Errorf("failed to read response body: %v", err)
	}

	// Create a check result for the validation
	validationCheck := CheckResult{
		URL:        c.config.ValidationURL,
		Success:    true,
		Speed:      duration,
		StatusCode: resp.StatusCode,
		BodySize:   int64(len(body)),
	}

	// Perform validation checks
	if c.debug {
		result.DebugInfo += fmt.Sprintf("[VALIDATE] Checking response status code: %d\n", resp.StatusCode)
	}

	// Check response status code
	if c.config.RequireStatusCode > 0 && resp.StatusCode != c.config.RequireStatusCode {
		validationCheck.Success = false
		validationCheck.Error = fmt.Sprintf("unexpected status code: %d (expected: %d)",
			resp.StatusCode, c.config.RequireStatusCode)
		result.CheckResults = append(result.CheckResults, validationCheck)
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[VALIDATE] Status code check failed: %s\n", validationCheck.Error)
		}
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Check response size
	if c.debug {
		result.DebugInfo += fmt.Sprintf("[VALIDATE] Checking response size: %d bytes\n", len(body))
	}
	if len(body) < c.config.MinResponseBytes {
		validationCheck.Success = false
		validationCheck.Error = fmt.Sprintf("response too small: %d bytes (min: %d)",
			len(body), c.config.MinResponseBytes)
		result.CheckResults = append(result.CheckResults, validationCheck)
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[VALIDATE] Response size check failed: %s\n", validationCheck.Error)
		}
		return fmt.Errorf("response too small: %d bytes", len(body))
	}

	// Check for disallowed keywords
	if c.debug && len(c.config.DisallowedKeywords) > 0 {
		result.DebugInfo += fmt.Sprintf("[VALIDATE] Checking for disallowed keywords\n")
	}
	for _, keyword := range c.config.DisallowedKeywords {
		if strings.Contains(string(body), keyword) {
			validationCheck.Success = false
			validationCheck.Error = fmt.Sprintf("response contains disallowed keyword: %s", keyword)
			result.CheckResults = append(result.CheckResults, validationCheck)
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[VALIDATE] Disallowed keyword found: %s\n", keyword)
			}
			return fmt.Errorf("response contains disallowed keyword: %s", keyword)
		}
	}

	// Check for required content match
	if c.config.RequireContentMatch != "" {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[VALIDATE] Checking for required content match: %s\n",
				c.config.RequireContentMatch)
		}
		if !strings.Contains(string(body), c.config.RequireContentMatch) {
			validationCheck.Success = false
			validationCheck.Error = "response does not contain required content"
			result.CheckResults = append(result.CheckResults, validationCheck)
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[VALIDATE] Required content not found\n")
			}
			return fmt.Errorf("response does not contain required content")
		}
	}

	// Check for required header fields
	if c.debug && len(c.config.RequireHeaderFields) > 0 {
		result.DebugInfo += fmt.Sprintf("[VALIDATE] Checking for required header fields\n")
	}
	for _, field := range c.config.RequireHeaderFields {
		if resp.Header.Get(field) == "" {
			validationCheck.Success = false
			validationCheck.Error = fmt.Sprintf("response missing required header: %s", field)
			result.CheckResults = append(result.CheckResults, validationCheck)
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[VALIDATE] Missing required header: %s\n", field)
			}
			return fmt.Errorf("response missing required header: %s", field)
		}
	}

	// All checks passed, add the successful validation result
	result.CheckResults = append(result.CheckResults, validationCheck)

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[VALIDATE] All validation checks passed\n")
	}

	// Mark the proxy as working
	result.Working = true

	return nil
}

// performSingleCheck performs a single URL check
func (c *Checker) performSingleCheck(client *http.Client, testURL string, result *ProxyResult) (*CheckResult, error) {
	start := time.Now()
	checkResult := &CheckResult{
		URL: testURL,
	}

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[DEBUG] Testing URL: %s\n", testURL)
	}

	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		checkResult.Error = err.Error()
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DEBUG] Error creating request: %v\n", err)
		}
		return checkResult, err
	}

	// Add headers
	req.Header.Set("User-Agent", c.config.UserAgent)
	for key, value := range c.config.DefaultHeaders {
		req.Header.Set(key, value)
	}

	// If rDNS lookup is enabled, try to use it for the Host header
	if c.config.UseRDNS {
		if host, err := lookupRDNS(req.URL.Hostname()); err == nil && host != "" {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[DEBUG] Using rDNS host: %s\n", host)
			}
			req.Host = host
		}
	}

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[DEBUG] Sending request with headers: %v\n", req.Header)
	}

	resp, err := client.Do(req)
	if err != nil {
		checkResult.Error = err.Error()
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DEBUG] Request error: %v\n", err)
		}
		return checkResult, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		checkResult.Error = err.Error()
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DEBUG] Error reading response body: %v\n", err)
		}
		return checkResult, err
	}

	checkResult.StatusCode = resp.StatusCode
	checkResult.BodySize = int64(len(body))
	checkResult.Speed = time.Since(start)
	checkResult.Success = c.validateResponse(resp, body)

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[DEBUG] Response: status=%d, size=%d bytes, time=%v, success=%v\n",
			checkResult.StatusCode, checkResult.BodySize, checkResult.Speed, checkResult.Success)
	}

	return checkResult, nil
}

// lookupRDNS performs a reverse DNS lookup on an IP address
func lookupRDNS(ip string) (string, error) {
	names, err := net.LookupAddr(ip)
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "", nil
	}
	// Remove trailing dot from PTR record
	return strings.TrimSuffix(names[0], "."), nil
}

// validateResponse validates the HTTP response
func (c *Checker) validateResponse(resp *http.Response, body []byte) bool {
	if resp.StatusCode >= 400 {
		return false
	}

	if len(body) < c.config.MinResponseBytes {
		return false
	}

	for _, keyword := range c.config.DisallowedKeywords {
		if strings.Contains(string(body), keyword) {
			return false
		}
	}

	return true
}

// checkAnonymity checks if the proxy is anonymous
func (c *Checker) checkAnonymity(client *http.Client) (bool, string, string, error) {
	// Implementation for checking proxy anonymity
	return false, "", "", nil // Placeholder
}

// applyRateLimit applies rate limiting based on configuration
func (c *Checker) applyRateLimit(host string, result *ProxyResult) {
	if !c.config.RateLimitEnabled {
		return
	}

	c.rateLimiterLock.Lock()
	defer c.rateLimiterLock.Unlock()

	// Determine the key for rate limiting
	rateLimitKey := "global"
	if c.config.RateLimitPerHost {
		rateLimitKey = host
	}

	// Check if we need to wait
	if lastTime, exists := c.rateLimiter[rateLimitKey]; exists {
		elapsed := time.Since(lastTime)
		if elapsed < c.config.RateLimitDelay {
			waitTime := c.config.RateLimitDelay - elapsed
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[DEBUG] Rate limiting: waiting %v before request to %s\n", waitTime, host)
			}
			time.Sleep(waitTime)
		}
	}

	// Update the last request time
	c.rateLimiter[rateLimitKey] = time.Now()

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[DEBUG] Rate limiting applied for %s\n", rateLimitKey)
	}
}

func (c *Checker) makeRequest(client *http.Client, urlStr string, result *ProxyResult) (*http.Response, error) {
	// Create a context with the configured timeout
	ctx, cancel := context.WithTimeout(context.Background(), c.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DEBUG] Error creating request: %v\n", err)
		}
		return nil, err
	}

	// Apply rate limiting if enabled
	if parsedURL, err := url.Parse(urlStr); err == nil {
		c.applyRateLimit(parsedURL.Hostname(), result)
	}

	// Set headers
	for key, value := range c.config.DefaultHeaders {
		req.Header.Set(key, value)
	}
	req.Header.Set("User-Agent", c.config.UserAgent)

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[DEBUG] Making request to: %s\n", urlStr)

		// Get proxy information in a more readable format
		proxyInfo := "direct connection"
		if transport, ok := client.Transport.(*http.Transport); ok && transport.Proxy != nil {
			// Try to get the proxy URL by making a test request
			if proxyURL, err := transport.Proxy(req); err == nil && proxyURL != nil {
				proxyInfo = proxyURL.String()
			} else {
				proxyInfo = "configured (address unavailable)"
			}
		}

		result.DebugInfo += fmt.Sprintf("[DEBUG] Using proxy: %s\n", proxyInfo)
		result.DebugInfo += fmt.Sprintf("[DEBUG] Full request:\n")
		result.DebugInfo += fmt.Sprintf("  Method: %s\n", req.Method)
		result.DebugInfo += fmt.Sprintf("  URL: %s\n", req.URL.String())
		result.DebugInfo += fmt.Sprintf("[DEBUG] Headers:\n")
		for key, values := range req.Header {
			result.DebugInfo += fmt.Sprintf("    %s: %v\n", key, values)
		}
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if c.debug {
		if err != nil {
			result.DebugInfo += fmt.Sprintf("[DEBUG] Request error: %v\n", err)
		} else if resp != nil {
			result.DebugInfo += fmt.Sprintf("[DEBUG] Response received in %v:\n", duration)
			result.DebugInfo += fmt.Sprintf("  Status: %s\n", resp.Status)
			result.DebugInfo += fmt.Sprintf("[DEBUG] Headers:\n")
			for key, values := range resp.Header {
				result.DebugInfo += fmt.Sprintf("    %s: %v\n", key, values)
			}
		}
	}

	return resp, err
}
