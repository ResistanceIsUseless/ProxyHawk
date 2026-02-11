package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/errors"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/logging"
)

// NewChecker creates a new proxy checker
func NewChecker(config Config, debug bool, logger *logging.Logger) *Checker {
	checker := &Checker{
		config:      config,
		debug:       debug,
		logger:      logger,
		rateLimiter: make(map[string]time.Time),
	}

	// Validate and normalize retry configuration
	checker.validateRetryConfig()

	// Validate and normalize authentication configuration
	checker.validateAuthConfig()

	return checker
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
		result.Error = errors.NewProxyError(errors.ErrorProxyInvalidURL, "invalid proxy URL", proxyURL, err)
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
		// Proxy doesn't work as a forward proxy, but it might still have vulnerabilities
		// Try direct vulnerability scanning as fallback if advanced checks are enabled
		if c.hasAdvancedChecks() {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[FALLBACK] Proxy connection failed, attempting direct vulnerability scan\n")
			}

			// Try to scan the target as a web server directly
			if directResult := c.performDirectScan(parsedURL, result); directResult {
				// Direct scan found something useful
				if c.debug {
					result.DebugInfo += fmt.Sprintf("[FALLBACK] Direct scan completed with findings\n")
				}
				return result
			}
		}

		// Create a more concise error message
		result.Error = errors.NewProxyError(errors.ErrorProxyNotWorking, "proxy check failed", proxyURL, err)
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[RESULT] Proxy type detection failed and no vulnerabilities found: %v\n", err)
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
		result.Error = errors.NewProxyError(errors.ErrorProxyValidationFailed, "validation failed", proxyURL, err)
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[RESULT] Validation checks failed: %v\n", err)
		}
		return result
	}

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[PHASE 2/2 COMPLETE] Validation successful\n")
	}

	// PHASE 3: Advanced Security Checks (if enabled)
	if c.hasAdvancedChecks() {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[PHASE 3/3] Running advanced security checks\n")
		}
		if err := c.performAdvancedChecks(client, result); err != nil {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[PHASE 3/3] Advanced checks encountered error: %v\n", err)
			}
			// Don't fail the entire check if advanced checks fail
			// Just log the error and continue
		}
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[PHASE 3/3 COMPLETE] Advanced security checks finished\n")
		}
	}

	// PHASE 4: Anonymity Detection and Proxy Chain Detection
	if c.debug {
		result.DebugInfo += fmt.Sprintf("[PHASE 4/4] Checking proxy anonymity and chain detection\n")
	}
	anonymous, anonLevel, detectedIP, leakingHeaders, chainDetected, chainInfo, anonErr := c.checkAnonymity(client)
	if anonErr == nil {
		result.IsAnonymous = anonymous
		result.AnonymityLevel = anonLevel
		result.DetectedIP = detectedIP
		result.LeakingHeaders = leakingHeaders
		result.ProxyChainDetected = chainDetected
		result.ProxyChainInfo = chainInfo
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[PHASE 4/4 COMPLETE] Anonymity: %t, Level: %s\n", anonymous, anonLevel)
			if chainDetected {
				result.DebugInfo += fmt.Sprintf("  - Proxy Chain: YES (%s)\n", chainInfo)
			}
			if len(leakingHeaders) > 0 {
				result.DebugInfo += fmt.Sprintf("  - Leaking Headers: %v\n", leakingHeaders)
			}
		}
	} else if c.debug {
		result.DebugInfo += fmt.Sprintf("[PHASE 4/4] Anonymity check failed: %v\n", anonErr)
	}

	// PHASE 5: Proxy Fingerprinting (if enabled)
	if c.config.EnableFingerprint {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[PHASE 5/5] Fingerprinting proxy software\n")
		}
		fingerprint := c.FingerprintProxy(client, proxyURL)
		result.Fingerprint = fingerprint
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[PHASE 5/5 COMPLETE] Detected: %s (confidence: %.2f)\n", fingerprint.ProxySoftware, fingerprint.Confidence)
			if fingerprint.Version != "" {
				result.DebugInfo += fmt.Sprintf("  - Version: %s\n", fingerprint.Version)
			}
		}
	}

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[SUMMARY] Proxy check results for %s:\n", proxyURL)
		result.DebugInfo += fmt.Sprintf("  - Type: %s\n", result.Type)
		result.DebugInfo += fmt.Sprintf("  - Working: %t\n", result.Working)
		result.DebugInfo += fmt.Sprintf("  - Speed: %v\n", result.Speed)
		result.DebugInfo += fmt.Sprintf("  - Anonymous: %t (%s)\n", result.IsAnonymous, result.AnonymityLevel)
		result.DebugInfo += fmt.Sprintf("  - Check Steps: %d\n", len(result.CheckResults))
		if c.config.EnableFingerprint && result.Fingerprint != nil {
			result.DebugInfo += fmt.Sprintf("  - Fingerprint: %s %s\n", result.Fingerprint.ProxySoftware, result.Fingerprint.Version)
		}
	}

	return result
}

// determineProxyType attempts to determine the type of proxy by testing different protocols
func (c *Checker) determineProxyType(proxyURL *url.URL, result *ProxyResult) (ProxyType, *http.Client, error) {
	var lastError string

	// Use local validation URLs instead of mutating shared config
	validationURLHTTP := "http://api.ipify.org?format=json"
	validationURLHTTPS := "https://api.ipify.org?format=json"

	// Save the original validation URL to restore after testing
	origValidationURL := c.config.ValidationURL
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
				c.config.ValidationURL = validationURLHTTP
				httpSuccess, httpTestErr, httpCheckResult := c.testClientWithDetails(client, proxyType, result)

				// Add the check result to our collection
				if httpCheckResult != nil {
					result.CheckResults = append(result.CheckResults, *httpCheckResult)
				}

				// Then test with HTTPS endpoint
				c.config.ValidationURL = validationURLHTTPS
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
		c.config.ValidationURL = validationURLHTTP
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
		c.config.ValidationURL = validationURLHTTPS
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

	// If HTTP/HTTPS failed, try HTTP/2 and HTTP/3 if enabled
	if c.config.EnableHTTP2 || c.config.EnableHTTP3 {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[TYPE] Testing advanced HTTP protocols (HTTP/2, HTTP/3): %s\n", proxyURL.Host)
		}

		// Test HTTP/2 support if enabled
		if c.config.EnableHTTP2 {
			if success, client := c.detectHTTP2Protocol(proxyURL, result); success {
				result.SupportsHTTP2 = true
				if c.debug {
					result.DebugInfo += fmt.Sprintf("[TYPE] Selected HTTP/2 proxy\n")
				}
				return ProxyTypeHTTP2, client, nil
			}
		}

		// Test HTTP/3 support if enabled
		if c.config.EnableHTTP3 {
			if success, client := c.detectHTTP3Protocol(proxyURL, result); success {
				result.SupportsHTTP3 = true
				if c.debug {
					result.DebugInfo += fmt.Sprintf("[TYPE] Selected HTTP/3 proxy\n")
				}
				return ProxyTypeHTTP3, client, nil
			}
		}
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
		c.config.ValidationURL = validationURLHTTP
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
		c.config.ValidationURL = validationURLHTTPS
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

// performChecks runs all configured checks for the proxy
func (c *Checker) performChecks(client *http.Client, result *ProxyResult) error {
	start := time.Now()

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[VALIDATE] Running validation checks\n")
	}

	// Make the request to the validation URL (with retry logic if enabled)
	resp, err := c.makeRequestWithRetry(client, c.config.ValidationURL, result)
	if err != nil {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[VALIDATE] Request failed: %v\n", err)
		}
		return errors.NewHTTPError(errors.ErrorHTTPRequestFailed, "request failed", c.config.ValidationURL, err)
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
		return errors.NewHTTPError(errors.ErrorHTTPUnexpectedStatus, "unexpected status code", c.config.ValidationURL, nil).
			WithDetail("status_code", resp.StatusCode).
			WithDetail("expected_code", c.config.RequireStatusCode)
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

// performDirectScan attempts to scan the target directly as a web server when proxy connection fails
// This allows us to detect SSRF vulnerabilities, misconfigurations, and information leaks
// even when the target doesn't function as a forward proxy
func (c *Checker) performDirectScan(proxyURL *url.URL, result *ProxyResult) bool {
	foundSomething := false

	// Extract the target host and port
	targetHost := proxyURL.Hostname()
	targetPort := proxyURL.Port()
	if targetPort == "" {
		if proxyURL.Scheme == "https" || proxyURL.Scheme == "socks5" {
			targetPort = "443"
		} else {
			targetPort = "80"
		}
	}

	// Build the target URL
	targetURL := fmt.Sprintf("http://%s:%s", targetHost, targetPort)

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] Attempting direct vulnerability scan on %s\n", targetURL)
	}

	// Create a direct HTTP client (not using the target as a proxy)
	directClient := &http.Client{
		Timeout: c.config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			MaxConnsPerHost:     50,
			IdleConnTimeout:     90 * time.Second,
			DisableKeepAlives:   false,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	// Test 1: Try to access root path to see if it responds
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] Failed to create request: %v\n", err)
		}
		return false
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	for key, value := range c.config.DefaultHeaders {
		req.Header.Set(key, value)
	}

	resp, err := directClient.Do(req)
	if err != nil {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] No response from target: %v\n", err)
		}
		return false
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] Failed to read response: %v\n", err)
		}
		return false
	}

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] Received response: HTTP %d (%d bytes)\n", resp.StatusCode, len(body))
	}

	// Mark as not working as proxy, but note we got a response
	result.Working = false
	result.Type = ProxyTypeUnknown

	// Check for information leaks in headers
	leakedInfo := []string{}

	// Check for server header
	if serverHeader := resp.Header.Get("Server"); serverHeader != "" {
		leakedInfo = append(leakedInfo, fmt.Sprintf("Server: %s", serverHeader))
		foundSomething = true
	}

	// Check for internal IP leaks
	internalHeaders := []string{
		"X-Forwarded-For", "X-Real-IP", "X-Original-IP", "X-Client-IP",
		"X-Forwarded-Host", "X-Forwarded-Server", "Via", "X-Proxy-ID",
	}

	for _, header := range internalHeaders {
		if value := resp.Header.Get(header); value != "" {
			// Check if it contains internal IP addresses
			if strings.Contains(value, "10.") || strings.Contains(value, "172.") ||
				strings.Contains(value, "192.168.") || strings.Contains(value, "127.0.0.1") {
				leakedInfo = append(leakedInfo, fmt.Sprintf("%s: %s [INTERNAL IP LEAK]", header, value))
				foundSomething = true
			}
		}
	}

	// Check response body for server identification
	bodyStr := string(body)
	bodyLower := strings.ToLower(bodyStr)

	serverTypes := []string{"nginx", "apache", "haproxy", "traefik", "envoy", "kong", "varnish", "squid"}
	for _, serverType := range serverTypes {
		if strings.Contains(bodyLower, serverType) {
			leakedInfo = append(leakedInfo, fmt.Sprintf("Server type in body: %s", serverType))
			foundSomething = true
			break
		}
	}

	// Check for kubernetes or cloud-specific headers
	cloudHeaders := map[string]string{
		"X-Kong-Upstream-Latency": "Kong API Gateway",
		"X-Kong-Proxy-Latency":    "Kong API Gateway",
		"CF-Ray":                  "Cloudflare",
		"X-Amz-Cf-Id":             "AWS CloudFront",
		"X-Azure-Ref":             "Azure",
		"X-GUploader-UploadID":    "Google Cloud",
	}

	for header, service := range cloudHeaders {
		if resp.Header.Get(header) != "" {
			leakedInfo = append(leakedInfo, fmt.Sprintf("%s detected", service))
			foundSomething = true
		}
	}

	// Add findings to result
	if len(leakedInfo) > 0 {
		result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] Information leaks detected:\n")
		for _, info := range leakedInfo {
			result.DebugInfo += fmt.Sprintf("  - %s\n", info)
		}

		// Store findings in a check result
		checkResult := CheckResult{
			URL:        targetURL,
			Success:    true,
			StatusCode: resp.StatusCode,
			BodySize:   int64(len(body)),
			Error:      fmt.Sprintf("Not a working proxy, but detected: %s", strings.Join(leakedInfo, ", ")),
		}
		result.CheckResults = append(result.CheckResults, checkResult)
	}

	// Test 2: Try common SSRF targets to see if server will proxy them
	if c.hasSSRFChecks() {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] Testing for SSRF vulnerabilities\n")
		}

		ssrfTargets := []string{
			"http://169.254.169.254/latest/meta-data/", // AWS metadata
			"http://metadata.google.internal/",          // GCP metadata
			"http://localhost:8080/",                    // Localhost
			"http://127.0.0.1:6379/",                    // Redis
		}

		for _, ssrfTarget := range ssrfTargets {
			// Try to make the target fetch the SSRF URL
			testURL := fmt.Sprintf("%s?url=%s", targetURL, url.QueryEscape(ssrfTarget))
			req, err := http.NewRequest("GET", testURL, nil)
			if err != nil {
				continue
			}

			req.Header.Set("User-Agent", c.config.UserAgent)
			resp, err := directClient.Do(req)
			if err != nil {
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()

			if err == nil && len(body) > 0 {
				// Check if response contains metadata-like content
				bodyStr := string(body)
				if strings.Contains(bodyStr, "ami-id") || strings.Contains(bodyStr, "instance-id") ||
					strings.Contains(bodyStr, "metadata") || strings.Contains(bodyStr, "computeMetadata") {
					result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] ⚠️  Possible SSRF vulnerability: target may fetch %s\n", ssrfTarget)
					foundSomething = true

					checkResult := CheckResult{
						URL:        testURL,
						Success:    true,
						StatusCode: resp.StatusCode,
						BodySize:   int64(len(body)),
						Error:      fmt.Sprintf("POSSIBLE SSRF: Target may be vulnerable to SSRF via URL parameter"),
					}
					result.CheckResults = append(result.CheckResults, checkResult)
					break
				}
			}
		}
	}

	// Test 3: Run all advanced security checks using the direct client
	// Initialize Interactsh tester unless explicitly disabled
	var tester *InteractshTester
	if !c.config.AdvancedChecks.DisableInteractsh {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] Initializing Interactsh for OOB testing\n")
		}
		var interactshErr error
		tester, interactshErr = NewInteractshTester()
		if interactshErr != nil {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] Failed to initialize Interactsh tester: %v\nFalling back to basic checks.\n", interactshErr)
			}
		}
		if tester != nil {
			defer tester.Close()
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] Interactsh initialized successfully\n")
			}
		}
	}

	// Use target URL as test domain
	testDomain := targetHost
	if targetPort != "" && targetPort != "80" && targetPort != "443" {
		testDomain = fmt.Sprintf("%s:%s", targetHost, targetPort)
	}

	// Protocol Smuggling Test
	if c.config.AdvancedChecks.TestProtocolSmuggling {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] Testing protocol smuggling\n")
		}
		if tester != nil {
			res, err := tester.PerformInteractshTest(directClient, c, func(url string) (*http.Request, error) {
				req, err := http.NewRequest("POST", fmt.Sprintf("http://%s", url), strings.NewReader("test"))
				if err != nil {
					return nil, err
				}
				req.Header.Add("Content-Length", "4")
				req.Header.Add("Transfer-Encoding", "chunked")
				return req, nil
			})
			if err == nil && res != nil && res.Success {
				result.CheckResults = append(result.CheckResults, *res)
				foundSomething = true
			}
		} else {
			if res, err := c.checkProtocolSmuggling(directClient, testDomain); err == nil && res != nil && res.Success {
				result.CheckResults = append(result.CheckResults, *res)
				foundSomething = true
			}
		}
	}

	// DNS Rebinding Test
	if c.config.AdvancedChecks.TestDNSRebinding {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] Testing DNS rebinding\n")
		}
		if tester != nil {
			res, err := tester.PerformInteractshTest(directClient, c, func(url string) (*http.Request, error) {
				req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", url), nil)
				if err != nil {
					return nil, err
				}
				req.Header.Set("X-Forwarded-Host", url)
				req.Header.Set("Host", url)
				return req, nil
			})
			if err == nil && res != nil && res.Success {
				result.CheckResults = append(result.CheckResults, *res)
				foundSomething = true
			}
		} else {
			if res, err := c.checkDNSRebinding(directClient, testDomain); err == nil && res != nil && res.Success {
				result.CheckResults = append(result.CheckResults, *res)
				foundSomething = true
			}
		}
	}

	// IPv6 Test
	if c.config.AdvancedChecks.TestIPv6 {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] Testing IPv6 support\n")
		}
		if tester != nil {
			res, err := tester.PerformInteractshTest(directClient, c, func(url string) (*http.Request, error) {
				return http.NewRequest("GET", fmt.Sprintf("http://[%s]", url), nil)
			})
			if err == nil && res != nil && res.Success {
				result.CheckResults = append(result.CheckResults, *res)
				foundSomething = true
			}
		} else {
			if res, err := c.checkIPv6Support(directClient, testDomain); err == nil && res != nil && res.Success {
				result.CheckResults = append(result.CheckResults, *res)
				foundSomething = true
			}
		}
	}

	// HTTP Methods Test
	if len(c.config.AdvancedChecks.TestHTTPMethods) > 0 {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] Testing HTTP methods: %v\n", c.config.AdvancedChecks.TestHTTPMethods)
		}
		var results []*CheckResult
		if tester != nil {
			for _, method := range c.config.AdvancedChecks.TestHTTPMethods {
				res, err := tester.PerformInteractshTest(directClient, c, func(url string) (*http.Request, error) {
					return http.NewRequest(method, fmt.Sprintf("http://%s", url), nil)
				})
				if err == nil && res != nil && res.Success {
					results = append(results, res)
				}
			}
		} else {
			results, _ = c.checkHTTPMethods(directClient, testDomain)
		}
		if len(results) > 0 {
			for _, res := range results {
				if res != nil && res.Success {
					result.CheckResults = append(result.CheckResults, *res)
					foundSomething = true
				}
			}
		}
	}

	// Cache Poisoning Test
	if c.config.AdvancedChecks.TestCachePoisoning {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] Testing cache poisoning\n")
		}
		if tester != nil {
			res, err := tester.PerformInteractshTest(directClient, c, func(url string) (*http.Request, error) {
				req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", url), nil)
				if err != nil {
					return nil, err
				}
				req.Header.Set("Cache-Control", "public, max-age=31536000")
				req.Header.Set("X-Cache-Control", "public, max-age=31536000")
				return req, nil
			})
			if err == nil && res != nil && res.Success {
				result.CheckResults = append(result.CheckResults, *res)
				foundSomething = true
			}
		} else {
			if res, err := c.checkCachePoisoning(directClient, testDomain); err == nil && res != nil && res.Success {
				result.CheckResults = append(result.CheckResults, *res)
				foundSomething = true
			}
		}
	}

	// Host Header Injection Test
	if c.config.AdvancedChecks.TestHostHeaderInjection {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] Testing host header injection\n")
		}
		if tester != nil {
			res, err := tester.PerformInteractshTest(directClient, c, func(url string) (*http.Request, error) {
				req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", url), nil)
				if err != nil {
					return nil, err
				}
				req.Host = url
				req.Header.Set("X-Forwarded-Host", url)
				req.Header.Set("X-Host", url)
				req.Header.Set("X-Forwarded-Server", url)
				req.Header.Set("X-HTTP-Host-Override", url)
				return req, nil
			})
			if err == nil && res != nil && res.Success {
				result.CheckResults = append(result.CheckResults, *res)
				foundSomething = true
			}
		} else {
			if res, err := c.checkHostHeaderInjection(directClient, testDomain); err == nil && res != nil && res.Success {
				result.CheckResults = append(result.CheckResults, *res)
				foundSomething = true
			}
		}
	}

	// SSRF Test (comprehensive check from advanced_checks.go)
	if c.config.AdvancedChecks.TestSSRF {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DIRECT SCAN] Running comprehensive SSRF checks\n")
		}
		if res, err := c.checkSSRF(directClient, testDomain); err == nil && res != nil && res.Success {
			result.CheckResults = append(result.CheckResults, *res)
			foundSomething = true
		}
	}

	// Nginx Vulnerability Tests
	if c.config.AdvancedChecks.TestNginxVulnerabilities {
		if c.debug {
			result.DebugInfo += "[DIRECT SCAN - NGINX VULNS] Running nginx-specific vulnerability checks\n"
		}
		nginxResults := c.performNginxVulnerabilityChecks(directClient, result)
		result.NginxVulnerabilities = nginxResults

		// Count findings
		nginxFindings := 0
		if nginxResults.OffBySlashVuln {
			nginxFindings++
		}
		if nginxResults.K8sAPIExposed {
			nginxFindings++
		}
		if nginxResults.IngressWebhookExposed {
			nginxFindings++
		}
		if nginxResults.DebugEndpointsExposed {
			nginxFindings++
		}
		if nginxResults.VulnerableAnnotations {
			nginxFindings++
		}

		if nginxFindings > 0 {
			foundSomething = true
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[DIRECT SCAN - NGINX VULNS] Complete - Found %d vulnerabilities\n", nginxFindings)
			}
		}
	}

	// Apache Vulnerability Tests
	if c.config.AdvancedChecks.TestApacheVulnerabilities {
		if c.debug {
			result.DebugInfo += "[DIRECT SCAN - APACHE VULNS] Running Apache mod_proxy vulnerability checks\n"
		}
		apacheResults := c.performApacheVulnerabilityChecks(directClient, result)
		result.ApacheVulnerabilities = apacheResults

		// Count findings
		apacheFindings := 0
		if apacheResults.CVE_2021_40438_SSRF {
			apacheFindings++
		}
		if apacheResults.CVE_2020_11984_RCE {
			apacheFindings++
		}
		if apacheResults.CVE_2021_41773_PathTraversal {
			apacheFindings++
		}
		if apacheResults.CVE_2024_38473_ACLBypass {
			apacheFindings++
		}
		if apacheResults.SSRFVulnerable {
			apacheFindings++
		}
		if apacheResults.PathTraversalVuln {
			apacheFindings++
		}

		if apacheFindings > 0 {
			foundSomething = true
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[DIRECT SCAN - APACHE VULNS] Complete - Found %d vulnerabilities\n", apacheFindings)
			}
		}
	}

	// Kong Vulnerability Tests
	if c.config.AdvancedChecks.TestKongVulnerabilities {
		if c.debug {
			result.DebugInfo += "[DIRECT SCAN - KONG VULNS] Running Kong API Gateway vulnerability checks\n"
		}
		kongResults := c.performKongVulnerabilityChecks(directClient, result)
		result.KongVulnerabilities = kongResults

		// Count findings
		kongFindings := 0
		if kongResults.ManagerExposed {
			kongFindings++
		}
		if kongResults.AdminAPIExposed {
			kongFindings++
		}
		if kongResults.UnauthorizedAccess {
			kongFindings++
		}

		if kongFindings > 0 {
			foundSomething = true
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[DIRECT SCAN - KONG VULNS] Complete - Found %d vulnerabilities\n", kongFindings)
			}
		}
	}

	// Generic Vulnerability Tests
	if c.config.AdvancedChecks.TestGenericVulnerabilities {
		if c.debug {
			result.DebugInfo += "[DIRECT SCAN - GENERIC VULNS] Running generic proxy misconfiguration checks\n"
		}
		genericResults := c.performGenericVulnerabilityChecks(directClient, result)
		result.GenericVulnerabilities = genericResults

		// Count findings
		genericFindings := 0
		if genericResults.OpenProxyToLocalhost {
			genericFindings++
		}
		if genericResults.XForwardedForBypass {
			genericFindings++
		}
		if genericResults.CachePoisonVulnerable {
			genericFindings++
		}
		if genericResults.LinkerdSSRF {
			genericFindings++
		}
		if genericResults.SpringBootActuator {
			genericFindings++
		}

		if genericFindings > 0 {
			foundSomething = true
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[DIRECT SCAN - GENERIC VULNS] Complete - Found %d vulnerabilities\n", genericFindings)
			}
		}
	}

	// Extended Vulnerability Tests
	if c.config.AdvancedChecks.TestExtendedVulnerabilities {
		if c.debug {
			result.DebugInfo += "[DIRECT SCAN - EXTENDED VULNS] Running extended vulnerability checks\n"
		}
		extendedResults := c.performExtendedVulnerabilityChecks(directClient, result)
		result.ExtendedVulnerabilities = extendedResults

		// Count findings
		extendedFindings := 0
		if extendedResults.NginxVersionDetected {
			extendedFindings++
		}
		if extendedResults.NginxConfigExposed {
			extendedFindings++
		}
		if extendedResults.WebSocketAbuseVulnerable {
			extendedFindings++
		}
		if extendedResults.HTTP2SmugglingVulnerable {
			extendedFindings++
		}
		if extendedResults.ProxyAuthBypass {
			extendedFindings++
		}

		if extendedFindings > 0 {
			foundSomething = true
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[DIRECT SCAN - EXTENDED VULNS] Complete - Found %d vulnerabilities\n", extendedFindings)
			}
		}
	}

	// Vendor-Specific Vulnerability Tests
	if c.config.AdvancedChecks.TestVendorVulnerabilities {
		if c.debug {
			result.DebugInfo += "[DIRECT SCAN - VENDOR VULNS] Running vendor-specific vulnerability checks\n"
		}
		vendorResults := c.performVendorVulnerabilityChecks(directClient, result)
		result.VendorVulnerabilities = vendorResults

		// Count findings
		vendorFindings := 0
		if vendorResults.HAProxyStatsExposed || vendorResults.HAProxyCVE_2023_40225 {
			vendorFindings++
		}
		if vendorResults.SquidCacheManagerExposed || vendorResults.SquidCVE_2021_46784 {
			vendorFindings++
		}
		if vendorResults.TraefikDashboardExposed || vendorResults.TraefikAPIExposed {
			vendorFindings++
		}
		if vendorResults.EnvoyAdminExposed || vendorResults.EnvoyCVE_2022_21654 {
			vendorFindings++
		}
		if vendorResults.CaddyAdminAPIExposed {
			vendorFindings++
		}
		if vendorResults.VarnishBanLurkExposed || vendorResults.VarnishCVE_2022_45060 {
			vendorFindings++
		}

		if vendorFindings > 0 {
			foundSomething = true
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[DIRECT SCAN - VENDOR VULNS] Complete - Found %d vulnerabilities\n", vendorFindings)
			}
		}
	}

	// Advanced SSRF Vulnerability Tests (Parser Differentials, IP Obfuscation, etc.)
	if c.config.AdvancedChecks.TestSSRF {
		if c.debug {
			result.DebugInfo += "[DIRECT SCAN - ADVANCED SSRF] Running advanced SSRF vulnerability checks\n"
		}
		advancedSSRFResults := c.performAdvancedSSRFChecks(directClient, result)
		result.AdvancedSSRFVulnerabilities = advancedSSRFResults

		// Count findings
		advancedSSRFFindings := 0
		if advancedSSRFResults.ParserDifferentialVuln {
			advancedSSRFFindings++
		}
		if advancedSSRFResults.IPObfuscationBypass {
			advancedSSRFFindings++
		}
		if advancedSSRFResults.RedirectChainVuln {
			advancedSSRFFindings++
		}
		if advancedSSRFResults.ProtocolSmugglingVuln {
			advancedSSRFFindings++
		}
		if advancedSSRFResults.HeaderInjectionSSRF {
			advancedSSRFFindings++
		}
		if advancedSSRFResults.ProxyPassTraversalVuln {
			advancedSSRFFindings++
		}
		if advancedSSRFResults.HostHeaderSSRF {
			advancedSSRFFindings++
		}

		if advancedSSRFFindings > 0 {
			foundSomething = true
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[DIRECT SCAN - ADVANCED SSRF] Complete - Found %d vulnerabilities\n", advancedSSRFFindings)
			}
		}
	}

	// Summary of what was tested (ALWAYS log this, not just in debug mode)
	totalChecks := 0
	totalRequests := 0
	checksRun := []string{}

	if c.config.AdvancedChecks.TestProtocolSmuggling {
		totalChecks++
		totalRequests += 3
		checksRun = append(checksRun, "Protocol Smuggling")
	}
	if c.config.AdvancedChecks.TestDNSRebinding {
		totalChecks++
		totalRequests += 2
		checksRun = append(checksRun, "DNS Rebinding")
	}
	if c.config.AdvancedChecks.TestIPv6 {
		totalChecks++
		totalRequests += 2
		checksRun = append(checksRun, "IPv6 Support")
	}
	if len(c.config.AdvancedChecks.TestHTTPMethods) > 0 {
		totalChecks++
		totalRequests += len(c.config.AdvancedChecks.TestHTTPMethods)
		checksRun = append(checksRun, fmt.Sprintf("HTTP Methods (%d)", len(c.config.AdvancedChecks.TestHTTPMethods)))
	}
	if c.config.AdvancedChecks.TestCachePoisoning {
		totalChecks++
		totalRequests += 3
		checksRun = append(checksRun, "Cache Poisoning")
	}
	if c.config.AdvancedChecks.TestHostHeaderInjection {
		totalChecks++
		totalRequests += 5
		checksRun = append(checksRun, "Host Header Injection")
	}
	if c.config.AdvancedChecks.TestSSRF {
		totalChecks++
		totalRequests += 4
		checksRun = append(checksRun, "SSRF (Basic)")

		// Count advanced SSRF subchecks that ran
		if result.AdvancedSSRFVulnerabilities != nil {
			checksRun = append(checksRun, "  └─ Advanced SSRF: Parser Differentials (13 patterns)")
			totalRequests += 13

			checksRun = append(checksRun, "  └─ Advanced SSRF: IP Obfuscation (15 formats)")
			totalRequests += 15

			checksRun = append(checksRun, "  └─ Advanced SSRF: Redirect Chains (3 scenarios)")
			totalRequests += 3

			checksRun = append(checksRun, "  └─ Advanced SSRF: Protocol Smuggling (9 schemes)")
			totalRequests += 9

			checksRun = append(checksRun, "  └─ Advanced SSRF: Header Injection (40 tests)")
			totalRequests += 40

			checksRun = append(checksRun, "  └─ Advanced SSRF: proxy_pass Traversal (7 patterns)")
			totalRequests += 7

			checksRun = append(checksRun, "  └─ Advanced SSRF: Host Header (5 targets)")
			totalRequests += 5

			// Priority 2 Advanced Checks
			checksRun = append(checksRun, "  └─ Advanced SSRF: SNI Proxy (5 targets)")
			totalRequests += 5

			checksRun = append(checksRun, "  └─ Advanced SSRF: DNS Rebinding (3 services)")
			totalRequests += 6 // 2 requests per service

			checksRun = append(checksRun, "  └─ Advanced SSRF: HTTP/2 Header Injection (4 patterns)")
			totalRequests += 4

			checksRun = append(checksRun, "  └─ Advanced SSRF: AWS IMDSv2 Bypass (7 tests)")
			totalRequests += 7

			// Priority 3 Advanced Checks
			checksRun = append(checksRun, "  └─ Advanced SSRF: URL Encoding Bypass (12 patterns)")
			totalRequests += 12

			checksRun = append(checksRun, "  └─ Advanced SSRF: Multiple Host Headers (5 combinations)")
			totalRequests += 5

			checksRun = append(checksRun, "  └─ Advanced SSRF: Cloud-Specific Headers (5 providers)")
			totalRequests += 5

			checksRun = append(checksRun, "  └─ Advanced SSRF: Port Specification Tricks (8 patterns)")
			totalRequests += 8

			checksRun = append(checksRun, "  └─ Advanced SSRF: Fragment/Query Manipulation (10 patterns)")
			totalRequests += 10

			totalChecks += 16 // 7 Priority 1 + 4 Priority 2 + 5 Priority 3
		}
	}
	if c.config.AdvancedChecks.TestNginxVulnerabilities {
		totalChecks++
		totalRequests += 25
		checksRun = append(checksRun, "Nginx Vulnerabilities (CVEs + misconfigs)")
	}
	if c.config.AdvancedChecks.TestApacheVulnerabilities {
		totalChecks++
		totalRequests += 20
		checksRun = append(checksRun, "Apache Vulnerabilities (CVEs + mod_proxy)")
	}
	if c.config.AdvancedChecks.TestKongVulnerabilities {
		totalChecks++
		totalRequests += 12
		checksRun = append(checksRun, "Kong API Gateway Vulnerabilities")
	}
	if c.config.AdvancedChecks.TestGenericVulnerabilities {
		totalChecks++
		totalRequests += 18
		checksRun = append(checksRun, "Generic Proxy Misconfigurations")
	}
	if c.config.AdvancedChecks.TestExtendedVulnerabilities {
		totalChecks++
		totalRequests += 15
		checksRun = append(checksRun, "Extended Vulnerabilities (HTTP/2, WebSocket)")
	}
	if c.config.AdvancedChecks.TestVendorVulnerabilities {
		totalChecks++
		totalRequests += 25
		checksRun = append(checksRun, "Vendor Vulnerabilities (HAProxy, Squid, Traefik, etc.)")
	}

	// ALWAYS log the summary (not just in debug mode)
	if c.logger != nil {
		c.logger.Info("╔════════════════════════════════════════════════════════════════╗")
		c.logger.Info("║              DIRECT SCAN TEST SUMMARY                          ║")
		c.logger.Info("╠════════════════════════════════════════════════════════════════╣")
		c.logger.Info(fmt.Sprintf("║ Total Check Categories: %-39d ║", totalChecks))
		c.logger.Info(fmt.Sprintf("║ Total HTTP Requests:    %-39d ║", totalRequests))
		c.logger.Info("╠════════════════════════════════════════════════════════════════╣")
		c.logger.Info("║ Checks Performed:                                              ║")
		for _, check := range checksRun {
			// Pad the check name to fit in the box
			checkLine := fmt.Sprintf("║   • %-58s ║", check)
			c.logger.Info(checkLine)
		}
		c.logger.Info("╚════════════════════════════════════════════════════════════════╝")

		if foundSomething {
			c.logger.Info("[DIRECT SCAN] ✓ VULNERABILITIES DETECTED via direct scanning")
		} else {
			c.logger.Info("[DIRECT SCAN] ✓ No vulnerabilities detected (target appears well-configured)")
		}
	}

	return foundSomething
}

// hasSSRFChecks returns true if SSRF checks are enabled
func (c *Checker) hasSSRFChecks() bool {
	return c.config.AdvancedChecks.TestProtocolSmuggling ||
		c.config.AdvancedChecks.TestDNSRebinding ||
		c.config.AdvancedChecks.TestCachePoisoning ||
		c.config.AdvancedChecks.TestHostHeaderInjection
}
