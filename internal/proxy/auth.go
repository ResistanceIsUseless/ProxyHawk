package proxy

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"h12.io/socks"
)

// AuthMethod represents the type of proxy authentication
type AuthMethod string

const (
	AuthMethodBasic  AuthMethod = "basic"
	AuthMethodDigest AuthMethod = "digest"
)

// ProxyAuth holds authentication credentials for a proxy
type ProxyAuth struct {
	Username string
	Password string
	Method   AuthMethod
}

// extractAuthFromURL extracts authentication information from a proxy URL
func (c *Checker) extractAuthFromURL(proxyURL *url.URL) *ProxyAuth {
	if proxyURL.User == nil {
		return nil
	}

	username := proxyURL.User.Username()
	password, hasPassword := proxyURL.User.Password()

	// Return nil if both username and password are empty
	if username == "" && (!hasPassword || password == "") {
		return nil
	}

	if c.debug {
		// Don't log the actual password for security
		hasPasswordStr := "no"
		if hasPassword {
			hasPasswordStr = "yes"
		}
		c.debugLog(fmt.Sprintf("[AUTH] Extracted credentials from URL - username: %s, password: %s",
			username, hasPasswordStr))
	}

	return &ProxyAuth{
		Username: username,
		Password: password,
		Method:   AuthMethodBasic, // Default to basic auth
	}
}

// getProxyAuth determines the authentication to use for a proxy
func (c *Checker) getProxyAuth(proxyURL *url.URL, result *ProxyResult) *ProxyAuth {
	// First, try to extract auth from the URL
	if auth := c.extractAuthFromURL(proxyURL); auth != nil {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[AUTH] Using authentication from URL for user: %s\n", auth.Username)
		}
		return auth
	}

	// If no auth in URL and authentication is enabled, use default credentials
	if c.config.AuthEnabled && c.config.DefaultUsername != "" {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[AUTH] Using default authentication for user: %s\n", c.config.DefaultUsername)
		}
		return &ProxyAuth{
			Username: c.config.DefaultUsername,
			Password: c.config.DefaultPassword,
			Method:   AuthMethodBasic,
		}
	}

	return nil
}

// cleanProxyURL removes authentication information from a proxy URL for logging/display
func (c *Checker) cleanProxyURL(proxyURL *url.URL) string {
	if proxyURL.User == nil {
		return proxyURL.String()
	}

	// Create a copy without user info
	cleanURL := &url.URL{
		Scheme:   proxyURL.Scheme,
		Host:     proxyURL.Host,
		Path:     proxyURL.Path,
		RawQuery: proxyURL.RawQuery,
		Fragment: proxyURL.Fragment,
	}

	return cleanURL.String()
}

// createAuthenticatedHTTPTransport creates an HTTP transport with proxy authentication
func (c *Checker) createAuthenticatedHTTPTransport(proxyURL *url.URL, scheme string, auth *ProxyAuth, result *ProxyResult) *http.Transport {
	transport := &http.Transport{
		TLSHandshakeTimeout:   c.config.Timeout / 2,
		ResponseHeaderTimeout: c.config.Timeout / 2,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     true,
		ForceAttemptHTTP2:     false,
	}

	if auth != nil {
		// Set up authenticated proxy
		proxyURLWithAuth := &url.URL{
			Scheme: scheme,
			User:   url.UserPassword(auth.Username, auth.Password),
			Host:   proxyURL.Host,
		}

		transport.Proxy = func(req *http.Request) (*url.URL, error) {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[AUTH] Using authenticated %s proxy for %s\n",
					scheme, c.cleanProxyURL(proxyURL))
			}
			return proxyURLWithAuth, nil
		}

		// Add Proxy-Authorization header for additional compatibility
		if auth.Method == AuthMethodBasic {
			credentials := base64.StdEncoding.EncodeToString([]byte(auth.Username + ":" + auth.Password))
			transport.ProxyConnectHeader = http.Header{
				"Proxy-Authorization": []string{"Basic " + credentials},
			}
		}
	} else {
		// Set up non-authenticated proxy
		transport.Proxy = func(_ *http.Request) (*url.URL, error) {
			proxyURLWithScheme := fmt.Sprintf("%s://%s", scheme, proxyURL.Host)
			return url.Parse(proxyURLWithScheme)
		}
	}

	return transport
}

// createAuthenticatedSOCKSDialer creates a SOCKS dialer with authentication
func (c *Checker) createAuthenticatedSOCKSDialer(proxyURL *url.URL, scheme string, auth *ProxyAuth, result *ProxyResult) func(context.Context, string, string) (net.Conn, error) {
	var dialFunc func(string, string) (net.Conn, error)

	if auth != nil {
		// Create SOCKS dialer with authentication
		socksURL := fmt.Sprintf("%s://%s:%s@%s", scheme, auth.Username, auth.Password, proxyURL.Host)
		dialFunc = socks.Dial(socksURL)

		if c.debug {
			result.DebugInfo += fmt.Sprintf("[AUTH] Created authenticated %s dialer for %s\n",
				scheme, c.cleanProxyURL(proxyURL))
		}
	} else {
		// Create SOCKS dialer without authentication
		socksURL := fmt.Sprintf("%s://%s", scheme, proxyURL.Host)
		dialFunc = socks.Dial(socksURL)

		if c.debug {
			result.DebugInfo += fmt.Sprintf("[AUTH] Created non-authenticated %s dialer\n", scheme)
		}
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[AUTH] Dialing %s address: %s through %s proxy\n",
				network, addr, scheme)
		}

		conn, err := dialFunc(network, addr)
		if err != nil && c.debug {
			result.DebugInfo += fmt.Sprintf("[AUTH] Dial error: %v\n", err)
		}
		return conn, err
	}
}

// validateAuthConfig validates authentication configuration
func (c *Checker) validateAuthConfig() {
	if !c.config.AuthEnabled {
		return
	}

	// Set default auth methods if none specified
	if len(c.config.AuthMethods) == 0 {
		c.config.AuthMethods = []string{string(AuthMethodBasic)}
	}

	// Validate auth methods
	validMethods := map[string]bool{
		string(AuthMethodBasic):  true,
		string(AuthMethodDigest): true,
	}

	filteredMethods := make([]string, 0, len(c.config.AuthMethods))
	for _, method := range c.config.AuthMethods {
		if validMethods[strings.ToLower(method)] {
			filteredMethods = append(filteredMethods, strings.ToLower(method))
		}
	}
	c.config.AuthMethods = filteredMethods

	// If no valid methods remain, disable auth
	if len(c.config.AuthMethods) == 0 {
		c.config.AuthEnabled = false
	}
}

// testProxyAuth tests if proxy authentication is working
func (c *Checker) testProxyAuth(client *http.Client, auth *ProxyAuth, result *ProxyResult) (bool, string) {
	if auth == nil {
		return true, "" // No auth required
	}

	// Create a simple test request to verify authentication
	testURL := "http://httpbin.org/ip" // Simple endpoint that returns IP

	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return false, fmt.Sprintf("failed to create auth test request: %v", err)
	}

	// Add headers
	for key, value := range c.config.DefaultHeaders {
		req.Header.Set(key, value)
	}
	if c.config.UserAgent != "" {
		req.Header.Set("User-Agent", c.config.UserAgent)
	}

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		// Check if error indicates authentication failure
		if strings.Contains(strings.ToLower(err.Error()), "407") ||
			strings.Contains(strings.ToLower(err.Error()), "proxy authentication required") ||
			strings.Contains(strings.ToLower(err.Error()), "unauthorized") {
			return false, fmt.Sprintf("proxy authentication failed: %v", err)
		}
		return false, fmt.Sprintf("auth test request failed: %v", err)
	}
	defer resp.Body.Close()

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[AUTH] Authentication test completed in %v with status: %d\n",
			elapsed, resp.StatusCode)
	}

	// Check for proxy authentication required
	if resp.StatusCode == 407 {
		return false, "proxy authentication required (407)"
	}

	// Authentication successful if we get any 2xx response
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if c.debug {
			result.DebugInfo += "[AUTH] Proxy authentication successful\n"
		}
		return true, ""
	}

	return false, fmt.Sprintf("unexpected response status: %d", resp.StatusCode)
}

// debugLog is a small helper to avoid duplication in debug output
func (c *Checker) debugLog(message string) {
	// This could be expanded to use the structured logger
	// For now, it's a placeholder for consistency
}

// AuthStats represents authentication statistics
type AuthStats struct {
	TotalAttempts     int
	SuccessfulAuths   int
	FailedAuths       int
	AuthMethods       map[string]int
	MostCommonFailure string
}

// GetAuthStats returns authentication statistics (placeholder for future metrics integration)
func (c *Checker) GetAuthStats() AuthStats {
	// This could be expanded to track actual auth statistics
	return AuthStats{
		AuthMethods: make(map[string]int),
	}
}
