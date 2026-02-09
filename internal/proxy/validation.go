package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// validateResponse validates the HTTP response
func (c *Checker) validateResponse(resp *http.Response, body []byte) bool {
	if resp.StatusCode >= 400 {
		return false
	}

	if len(body) < c.config.MinResponseBytes {
		return false
	}

	// Check Content-Type for JSON responses
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		// Try to parse as JSON to validate structure
		var jsonData map[string]interface{}
		if err := json.Unmarshal(body, &jsonData); err != nil {
			// Invalid JSON when JSON was expected
			return false
		}

		// For api.ipify.org, validate IP field exists and is valid
		if strings.Contains(c.config.ValidationURL, "ipify.org") {
			if ipField, ok := jsonData["ip"]; ok {
				if ipStr, ok := ipField.(string); ok {
					// Validate it's a proper IP address
					if !isValidIP(ipStr) {
						return false
					}
				} else {
					return false
				}
			} else {
				return false
			}
		}
	}

	// Case-insensitive keyword matching for disallowed keywords
	bodyLower := strings.ToLower(string(body))
	for _, keyword := range c.config.DisallowedKeywords {
		if strings.Contains(bodyLower, strings.ToLower(keyword)) {
			return false
		}
	}

	return true
}

// isValidIP validates that a string is a valid IPv4 or IPv6 address
func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// checkAnonymity checks if the proxy is anonymous and detects proxy chaining
// Returns: isAnonymous, anonymityLevel, detectedIP, leakingHeaders, chainDetected, chainInfo, error
func (c *Checker) checkAnonymity(client *http.Client) (bool, AnonymityLevel, string, []string, bool, string, error) {
	// First, get our real IP without proxy
	realIP, err := getRealIP()
	if err != nil {
		// If we can't get real IP, we can't properly validate anonymity
		if c.debug {
			// Note: result.DebugInfo not available here, log to stderr if needed
		}
	}

	// Use a service that returns headers to detect IP leaks
	testURL := "https://httpbin.org/headers"

	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return false, AnonymityUnknown, "", nil, false, "", err
	}

	// Set a unique User-Agent to identify our request
	req.Header.Set("User-Agent", c.config.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return false, AnonymityUnknown, "", nil, false, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, AnonymityUnknown, "", nil, false, "", err
	}

	// Parse JSON response from httpbin.org
	var httpbinResp struct {
		Headers map[string]string `json:"headers"`
	}

	if err := json.Unmarshal(body, &httpbinResp); err != nil {
		// Fallback to string-based parsing if JSON fails
		return c.checkAnonymityFallback(string(body), realIP)
	}

	// Headers that indicate proxy usage or leak real IP
	leakHeaders := []string{
		"X-Forwarded-For",
		"X-Real-Ip",
		"Via",
		"Forwarded",
		"X-Proxyuser-Ip",
		"Cf-Connecting-Ip",
		"True-Client-Ip",
		"X-Originating-Ip",
		"X-Client-Ip",
		"Client-Ip",
	}

	var detectedLeaks []string
	var detectedIP string
	var realIPLeaked bool

	// Check for each potentially leaking header in the parsed headers
	for _, leakHeader := range leakHeaders {
		// Case-insensitive header lookup
		for headerName, headerValue := range httpbinResp.Headers {
			if strings.EqualFold(headerName, leakHeader) {
				detectedLeaks = append(detectedLeaks, headerName)

				// Extract and validate IP addresses from header value
				extractedIPs := extractIPAddresses(headerValue)
				for _, ip := range extractedIPs {
					if detectedIP == "" {
						detectedIP = ip
					}
					// Check if our real IP is leaked
					if realIP != "" && ip == realIP {
						realIPLeaked = true
					}
				}
			}
		}
	}

	// Detect proxy chaining from parsed headers
	chainDetected, chainInfo := detectProxyChainFromHeaders(httpbinResp.Headers)

	// Determine anonymity level based on findings
	if len(detectedLeaks) == 0 {
		// No proxy headers detected - Elite/High Anonymous
		return true, AnonymityElite, detectedIP, detectedLeaks, chainDetected, chainInfo, nil
	} else if realIPLeaked {
		// Real IP leaked - Transparent proxy
		return false, AnonymityNone, detectedIP, detectedLeaks, chainDetected, chainInfo, nil
	} else if hasViaHeader(httpbinResp.Headers) {
		// Via header present but no real IP leak - Anonymous
		return true, AnonymityBasic, detectedIP, detectedLeaks, chainDetected, chainInfo, nil
	} else {
		// Headers present but real IP not leaked - Anonymous
		return true, AnonymityBasic, detectedIP, detectedLeaks, chainDetected, chainInfo, nil
	}
}

// getRealIP gets our actual public IP address without using a proxy
func getRealIP() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.ipify.org?format=json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.IP, nil
}

// extractIPAddresses extracts all valid IP addresses from a string
func extractIPAddresses(s string) []string {
	// Match IPv4 addresses
	ipv4Regex := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	// Match IPv6 addresses (simplified pattern)
	ipv6Regex := regexp.MustCompile(`\b(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}\b`)

	var ips []string

	// Find IPv4 addresses
	for _, match := range ipv4Regex.FindAllString(s, -1) {
		if isValidIP(match) {
			ips = append(ips, match)
		}
	}

	// Find IPv6 addresses
	for _, match := range ipv6Regex.FindAllString(s, -1) {
		if isValidIP(match) {
			ips = append(ips, match)
		}
	}

	return ips
}

// hasViaHeader checks if Via header is present (case-insensitive)
func hasViaHeader(headers map[string]string) bool {
	for headerName := range headers {
		if strings.EqualFold(headerName, "Via") {
			return true
		}
	}
	return false
}

// checkAnonymityFallback is the fallback string-based anonymity checking
func (c *Checker) checkAnonymityFallback(responseStr string, realIP string) (bool, AnonymityLevel, string, []string, bool, string, error) {
	// Headers that indicate proxy usage or leak real IP
	leakHeaders := []string{
		"X-Forwarded-For",
		"X-Real-IP",
		"Via",
		"Forwarded",
		"X-ProxyUser-IP",
		"CF-Connecting-IP",
		"True-Client-IP",
		"X-Originating-IP",
		"X-Client-IP",
		"Client-IP",
	}

	var detectedLeaks []string
	var detectedIP string

	// Check for each potentially leaking header
	for _, header := range leakHeaders {
		headerLower := strings.ToLower(header)
		if strings.Contains(strings.ToLower(responseStr), headerLower) {
			detectedLeaks = append(detectedLeaks, header)

			// Try to extract IP address from the response
			if detectedIP == "" {
				// Simple regex-like check for IP patterns after header name
				lines := strings.Split(responseStr, "\n")
				for _, line := range lines {
					if strings.Contains(strings.ToLower(line), headerLower) {
						// Extract potential IP (this is basic, but works for most cases)
						parts := strings.Split(line, ":")
						if len(parts) >= 2 {
							ipPart := strings.TrimSpace(strings.Trim(parts[1], `",`))
							// Basic IP format check
							if isValidIP(ipPart) {
								detectedIP = ipPart
								break
							}
						}
					}
				}
			}
		}
	}

	// Check if real IP leaked
	realIPLeaked := realIP != "" && detectedIP != "" && detectedIP == realIP

	// Detect proxy chaining
	chainDetected, chainInfo := detectProxyChain(responseStr)

	// Determine anonymity level based on findings
	if len(detectedLeaks) == 0 {
		// No proxy headers detected - Elite/High Anonymous
		return true, AnonymityElite, detectedIP, detectedLeaks, chainDetected, chainInfo, nil
	} else if realIPLeaked {
		// Real IP leaked - Transparent
		return false, AnonymityNone, detectedIP, detectedLeaks, chainDetected, chainInfo, nil
	} else if strings.Contains(strings.ToLower(responseStr), "via") {
		// Via header present but no real IP leak - Anonymous
		return true, AnonymityBasic, detectedIP, detectedLeaks, chainDetected, chainInfo, nil
	} else {
		// Headers present but real IP not leaked - Anonymous
		return true, AnonymityBasic, detectedIP, detectedLeaks, chainDetected, chainInfo, nil
	}
}

// detectProxyChainFromHeaders analyzes parsed headers to detect proxy-behind-proxy configurations
func detectProxyChainFromHeaders(headers map[string]string) (bool, string) {
	var chainInfo []string

	// Check Via header for multiple hops (case-insensitive)
	for headerName, headerValue := range headers {
		if strings.EqualFold(headerName, "Via") {
			// Via header format: "1.1 proxy1, 1.1 proxy2"
			// Multiple entries indicate a proxy chain
			if strings.Count(headerValue, ",") > 0 || strings.Count(headerValue, ";") > 0 {
				chainInfo = append(chainInfo, "Multiple Via headers detected")
				return true, strings.Join(chainInfo, "; ")
			}
		}

		// Check X-Forwarded-For for multiple IPs (case-insensitive)
		if strings.EqualFold(headerName, "X-Forwarded-For") {
			// X-Forwarded-For format: "client, proxy1, proxy2"
			// Multiple IPs indicate a proxy chain
			if strings.Count(headerValue, ",") > 1 {
				chainInfo = append(chainInfo, "Multiple IPs in X-Forwarded-For")
				return true, strings.Join(chainInfo, "; ")
			}
		}
	}

	// Check for X-Forwarded-Host with Forwarded header (proxy chain indicator)
	hasForwardedHost := false
	hasForwarded := false
	for headerName := range headers {
		if strings.EqualFold(headerName, "X-Forwarded-Host") {
			hasForwardedHost = true
		}
		if strings.EqualFold(headerName, "Forwarded") {
			hasForwarded = true
		}
	}

	if hasForwardedHost && hasForwarded {
		chainInfo = append(chainInfo, "Both X-Forwarded-Host and Forwarded headers present")
		return true, strings.Join(chainInfo, "; ")
	}

	// No proxy chain detected
	return false, ""
}

// detectProxyChain analyzes headers to detect proxy-behind-proxy configurations
// Returns: chainDetected, chainInfo
func detectProxyChain(responseStr string) (bool, string) {
	var chainInfo []string

	// Check Via header for multiple hops (indicates proxy chain)
	if strings.Contains(strings.ToLower(responseStr), "via") {
		lines := strings.Split(responseStr, "\n")
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), "via") {
				// Via header format: "Via: 1.1 proxy1, 1.1 proxy2"
				// Multiple entries indicate a proxy chain
				if strings.Count(line, ",") > 0 || strings.Count(line, ";") > 0 {
					chainInfo = append(chainInfo, "Multiple Via headers detected")
					return true, strings.Join(chainInfo, "; ")
				}
			}
		}
	}

	// Check X-Forwarded-For for multiple IPs (indicates proxy chain)
	if strings.Contains(strings.ToLower(responseStr), "x-forwarded-for") {
		lines := strings.Split(responseStr, "\n")
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), "x-forwarded-for") {
				// X-Forwarded-For format: "X-Forwarded-For: client, proxy1, proxy2"
				// Multiple IPs indicate a proxy chain
				if strings.Count(line, ",") > 1 {
					chainInfo = append(chainInfo, "Multiple IPs in X-Forwarded-For")
					return true, strings.Join(chainInfo, "; ")
				}
			}
		}
	}

	// Check for X-Forwarded-Host with Forwarded header (proxy chain indicator)
	hasForwardedHost := strings.Contains(strings.ToLower(responseStr), "x-forwarded-host")
	hasForwarded := strings.Contains(strings.ToLower(responseStr), "forwarded:")
	if hasForwardedHost && hasForwarded {
		chainInfo = append(chainInfo, "Both X-Forwarded-Host and Forwarded headers present")
		return true, strings.Join(chainInfo, "; ")
	}

	// No proxy chain detected
	return false, ""
}

// applyRateLimit applies rate limiting based on configuration
func (c *Checker) applyRateLimit(host string, result *ProxyResult) {
	if !c.config.RateLimitEnabled {
		return
	}

	// If per-proxy rate limiting is enabled, delegate to the proxy-specific function
	if c.config.RateLimitPerProxy && result.ProxyURL != "" {
		c.applyProxyRateLimit(result.ProxyURL, result)
		return
	}

	// Determine the key for rate limiting
	rateLimitKey := "global"
	if c.config.RateLimitPerHost {
		rateLimitKey = host
	}

	// Calculate wait time while holding lock
	c.rateLimiterLock.Lock()
	var waitTime time.Duration
	if lastTime, exists := c.rateLimiter[rateLimitKey]; exists {
		elapsed := time.Since(lastTime)
		if elapsed < c.config.RateLimitDelay {
			waitTime = c.config.RateLimitDelay - elapsed
		}
	}
	c.rateLimiterLock.Unlock()

	// Sleep outside the lock to prevent contention
	if waitTime > 0 {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DEBUG] Rate limiting: waiting %v before request to %s\n", waitTime, host)
		}
		time.Sleep(waitTime)
	}

	// Update the last request time
	c.rateLimiterLock.Lock()
	c.rateLimiter[rateLimitKey] = time.Now()
	c.rateLimiterLock.Unlock()

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[DEBUG] Rate limiting applied for %s\n", rateLimitKey)
	}
}

// applyProxyRateLimit applies rate limiting per individual proxy
func (c *Checker) applyProxyRateLimit(proxyURL string, result *ProxyResult) {
	if !c.config.RateLimitEnabled {
		return
	}

	// Use the full proxy URL as the rate limiting key for per-proxy limiting
	rateLimitKey := proxyURL

	// Calculate wait time while holding lock
	c.rateLimiterLock.Lock()
	var waitTime time.Duration
	if lastTime, exists := c.rateLimiter[rateLimitKey]; exists {
		elapsed := time.Since(lastTime)
		if elapsed < c.config.RateLimitDelay {
			waitTime = c.config.RateLimitDelay - elapsed
		}
	}
	c.rateLimiterLock.Unlock()

	// Sleep outside the lock to prevent contention
	if waitTime > 0 {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[DEBUG] Per-proxy rate limiting: waiting %v for proxy %s\n", waitTime, proxyURL)
		}
		time.Sleep(waitTime)
	}

	// Update the last request time for this specific proxy
	c.rateLimiterLock.Lock()
	c.rateLimiter[rateLimitKey] = time.Now()
	c.rateLimiterLock.Unlock()

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[DEBUG] Per-proxy rate limiting applied for %s\n", proxyURL)
	}
}