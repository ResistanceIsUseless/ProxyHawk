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
	"time"

	"github.com/ResistanceIsUseless/ProxyCheck/cloudcheck"
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

// AdvancedChecks represents advanced security check settings
type AdvancedChecks struct {
	TestProtocolSmuggling   bool     `yaml:"test_protocol_smuggling"`
	TestDNSRebinding        bool     `yaml:"test_dns_rebinding"`
	TestIPv6                bool     `yaml:"test_ipv6"`
	TestHTTPMethods         []string `yaml:"test_http_methods"`
	TestPathTraversal       bool     `yaml:"test_path_traversal"`
	TestCachePoisoning      bool     `yaml:"test_cache_poisoning"`
	TestHostHeaderInjection bool     `yaml:"test_host_header_injection"`
}

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

	// Response validation settings
	RequireStatusCode   int
	RequireContentMatch string
	RequireHeaderFields []string

	// Advanced security checks
	AdvancedChecks AdvancedChecks
}

// CheckResult represents the result of checking a proxy
type CheckResult struct {
	URL        string
	Success    bool
	Speed      time.Duration
	Error      string
	StatusCode int
	BodySize   int64
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
}

// Checker handles proxy checking functionality
type Checker struct {
	config Config
	debug  bool
}

// NewChecker creates a new proxy checker
func NewChecker(config Config, debug bool) *Checker {
	return &Checker{
		config: config,
		debug:  debug,
	}
}

// Check performs a complete check of a proxy
func (c *Checker) Check(proxyURL string) (*ProxyResult, error) {
	result := &ProxyResult{
		ProxyURL: proxyURL,
		Type:     ProxyTypeUnknown,
	}

	// Parse the proxy URL
	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		result.Error = fmt.Sprintf("invalid proxy URL: %v", err)
		return result, err
	}

	// Determine proxy type
	proxyType, client, err := c.determineProxyType(parsedURL)
	if err != nil {
		result.Error = fmt.Sprintf("failed to determine proxy type: %v", err)
		return result, err
	}

	result.Type = proxyType

	// Perform checks using the determined client
	if err := c.performChecks(client, result); err != nil {
		result.Error = fmt.Sprintf("check failed: %v", err)
		return result, err
	}

	return result, nil
}

// determineProxyType attempts to determine the type of proxy by testing different protocols
func (c *Checker) determineProxyType(proxyURL *url.URL) (ProxyType, *http.Client, error) {
	// Try protocols in order: HTTPS, HTTP, SOCKS4, SOCKS5
	candidates := []struct {
		proxyType ProxyType
		scheme    string
	}{
		{ProxyTypeHTTPS, "https"},
		{ProxyTypeHTTP, "http"},
		{ProxyTypeSOCKS4, "socks4"},
		{ProxyTypeSOCKS5, "socks5"},
	}

	for _, candidate := range candidates {
		client, err := c.createClient(proxyURL, candidate.scheme)
		if err != nil {
			continue
		}

		// Test the client
		if c.testClient(client) {
			return candidate.proxyType, client, nil
		}
	}

	return ProxyTypeUnknown, nil, fmt.Errorf("could not determine proxy type")
}

// createClient creates an HTTP client for the given proxy type
func (c *Checker) createClient(proxyURL *url.URL, scheme string) (*http.Client, error) {
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
			return url.Parse(fmt.Sprintf("%s://%s", scheme, proxyURL.Host))
		}

	case scheme == "socks4" || scheme == "socks5":
		dialSocksProxy := socks.Dial(fmt.Sprintf("%s://%s", scheme, proxyURL.Host))
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialSocksProxy(network, addr)
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   c.config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

// testClient tests if the client works with a simple request
func (c *Checker) testClient(client *http.Client) bool {
	req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode < 400
}

// performChecks runs all configured checks for the proxy
func (c *Checker) performChecks(client *http.Client, result *ProxyResult) error {
	start := time.Now()

	// Check anonymity
	if isAnonymous, realIP, proxyIP, err := c.checkAnonymity(client); err == nil {
		result.IsAnonymous = isAnonymous
		result.RealIP = realIP
		result.ProxyIP = proxyIP
	}

	// Perform validation checks
	for _, testURL := range []string{c.config.ValidationURL} {
		checkResult, err := c.performSingleCheck(client, testURL)
		if err != nil {
			return err
		}
		result.CheckResults = append(result.CheckResults, *checkResult)
	}

	// Perform cloud provider checks if enabled
	if c.config.EnableCloudChecks && len(c.config.CloudProviders) > 0 {
		// Get WHOIS info for the proxy
		proxyIP := result.ProxyIP
		if proxyIP != "" {
			whoisInfo, err := cloudcheck.GetWhoisInfo(proxyIP)
			if err == nil {
				// Try to detect cloud provider from WHOIS data
				if provider := cloudcheck.DetectFromWhois(whoisInfo, c.config.CloudProviders); provider != nil {
					// Check for internal access
					if cloudResult, err := cloudcheck.CheckInternalAccess(client, provider, c.debug); err == nil {
						result.CloudProvider = provider.Name
						result.InternalAccess = cloudResult.InternalAccess
						result.MetadataAccess = cloudResult.MetadataAccess
						if c.debug {
							result.DebugInfo += "\nCloud Check Debug Info:\n" + cloudResult.DebugInfo
						}
					}
				}
			}
		}
	}

	result.Speed = time.Since(start)
	result.Working = len(result.CheckResults) > 0 && result.CheckResults[0].Success

	return nil
}

// performSingleCheck performs a single URL check
func (c *Checker) performSingleCheck(client *http.Client, testURL string) (*CheckResult, error) {
	start := time.Now()
	result := &CheckResult{
		URL: testURL,
	}

	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	// Add headers
	req.Header.Set("User-Agent", c.config.UserAgent)
	for key, value := range c.config.DefaultHeaders {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	result.StatusCode = resp.StatusCode
	result.BodySize = int64(len(body))
	result.Speed = time.Since(start)
	result.Success = c.validateResponse(resp, body)

	return result, nil
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
