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

	"h12.io/socks"
)

// Config represents the proxy checker configuration
type Config struct {
	Timeout            time.Duration
	MaxConcurrent      int
	ValidationURL      string
	ValidationPattern  string
	DisallowedKeywords []string
	MinResponseBytes   int
	DefaultHeaders     map[string]string
	UserAgent          string
}

// Result represents the result of checking a proxy
type Result struct {
	ProxyURL     string
	Working      bool
	Speed        time.Duration
	Error        error
	DebugInfo    string
	CheckResults []CheckResult
}

// CheckResult represents the result of a single URL check
type CheckResult struct {
	URL        string
	Success    bool
	Speed      time.Duration
	Error      string
	StatusCode int
	BodySize   int64
}

// ProxyType represents the detected type of proxy
type ProxyType int

const (
	ProxyTypeUnknown ProxyType = iota
	ProxyTypeHTTP
	ProxyTypeHTTPS
	ProxyTypeSOCKS4
	ProxyTypeSOCKS5
)

// ProxyDetectionResult contains information about the detected proxy type
type ProxyDetectionResult struct {
	Type            ProxyType
	SupportsHTTPS   bool
	SupportsCONNECT bool
	Error           error
	DebugInfo       string
}

// CreateClient creates an HTTP client with the given proxy configuration
func CreateClient(proxyURL *url.URL, timeout time.Duration) (*http.Client, error) {
	// Extract host:port from the URL
	host := proxyURL.Host
	if proxyURL.Port() == "" {
		if proxyURL.Scheme == "https" {
			host = fmt.Sprintf("%s:443", proxyURL.Hostname())
		} else {
			host = fmt.Sprintf("%s:80", proxyURL.Hostname())
		}
	}

	// Detect proxy type
	detection := detectProxyType(host, timeout/2, false) // Use half timeout for detection

	// Create base transport
	transport := &http.Transport{
		TLSHandshakeTimeout:   timeout / 2,
		ResponseHeaderTimeout: timeout / 2,
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

	// Configure transport based on detected proxy type
	switch detection.Type {
	case ProxyTypeHTTP, ProxyTypeHTTPS:
		proxyFunc := func(_ *http.Request) (*url.URL, error) {
			scheme := "http"
			if detection.Type == ProxyTypeHTTPS {
				scheme = "https"
			}
			return url.Parse(fmt.Sprintf("%s://%s", scheme, host))
		}
		transport.Proxy = proxyFunc

	case ProxyTypeSOCKS4, ProxyTypeSOCKS5:
		var dialSocksProxy func(string, string) (net.Conn, error)

		// Create SOCKS dialer based on type
		if detection.Type == ProxyTypeSOCKS5 {
			dialSocksProxy = socks.Dial("socks5://" + host)
		} else {
			dialSocksProxy = socks.Dial("socks4://" + host)
		}

		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialSocksProxy(network, addr)
		}

	default:
		return nil, fmt.Errorf("unable to determine proxy type for %s", host)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return client, nil
}

// ValidateResponse validates the HTTP response from a proxy
func ValidateResponse(resp *http.Response, body []byte, cfg *Config, debug bool) (bool, string) {
	debugInfo := ""

	if debug {
		debugInfo += fmt.Sprintf("\nValidating Response:\n")
		debugInfo += fmt.Sprintf("Status Code: %d\n", resp.StatusCode)
		debugInfo += fmt.Sprintf("Response Size: %d bytes\n", len(body))
		debugInfo += fmt.Sprintf("Headers: %v\n", resp.Header)
		debugInfo += fmt.Sprintf("Body Preview: %s\n", string(body[:min(len(body), 200)]))
	}

	// Check status code - Accept 200-299 range
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if debug {
			debugInfo += fmt.Sprintf("Status code %d is not in 2xx range\n", resp.StatusCode)
		}
		return false, debugInfo
	}

	// Check response size
	if len(body) < cfg.MinResponseBytes {
		if debug {
			debugInfo += fmt.Sprintf("Response size %d bytes is less than required %d bytes\n",
				len(body), cfg.MinResponseBytes)
		}
		return false, debugInfo
	}

	// Check for disallowed keywords
	for _, keyword := range cfg.DisallowedKeywords {
		if strings.Contains(string(body), keyword) {
			if debug {
				debugInfo += fmt.Sprintf("Response contains disallowed keyword '%s'\n", keyword)
			}
			return false, debugInfo
		}
	}

	if debug {
		debugInfo += "Response validation successful\n"
	}
	return true, debugInfo
}

// Check performs the proxy check with the given configuration
func Check(proxyStr string, cfg *Config, debug bool) (*Result, error) {
	start := time.Now()
	debugInfo := ""
	checkResults := make([]CheckResult, 0)

	if debug {
		debugInfo += fmt.Sprintf("\nStarting proxy check for: %s\n", proxyStr)
		debugInfo += fmt.Sprintf("Target URL: %s\n", cfg.ValidationURL)
		debugInfo += fmt.Sprintf("Timeout: %v\n", cfg.Timeout)
	}

	proxyURL, err := url.Parse(proxyStr)
	if err != nil {
		if debug {
			debugInfo += fmt.Sprintf("Error parsing proxy URL: %v\n", err)
		}
		return &Result{
			ProxyURL:  proxyStr,
			Working:   false,
			Error:     fmt.Errorf("invalid proxy URL: %v", err),
			DebugInfo: debugInfo,
		}, err
	}

	if debug {
		debugInfo += fmt.Sprintf("Parsed proxy URL - Scheme: %s, Host: %s\n", proxyURL.Scheme, proxyURL.Host)
		debugInfo += "Creating proxy client...\n"
	}

	client, err := CreateClient(proxyURL, cfg.Timeout)
	if err != nil {
		if debug {
			debugInfo += fmt.Sprintf("Error creating proxy client: %v\n", err)
		}
		return &Result{
			ProxyURL:  proxyStr,
			Working:   false,
			Error:     fmt.Errorf("failed to create proxy client: %v", err),
			DebugInfo: debugInfo,
		}, err
	}

	// Make the request
	req, err := http.NewRequest("GET", cfg.ValidationURL, nil)
	if err != nil {
		return &Result{
			ProxyURL:  proxyStr,
			Working:   false,
			Error:     err,
			DebugInfo: debugInfo,
		}, err
	}

	// Add headers
	if cfg.UserAgent != "" {
		req.Header.Set("User-Agent", cfg.UserAgent)
	}
	for key, value := range cfg.DefaultHeaders {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return &Result{
			ProxyURL:  proxyStr,
			Working:   false,
			Error:     err,
			DebugInfo: debugInfo,
		}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &Result{
			ProxyURL:  proxyStr,
			Working:   false,
			Error:     err,
			DebugInfo: debugInfo,
		}, err
	}

	valid, validationDebug := ValidateResponse(resp, body, cfg, debug)
	debugInfo += validationDebug

	checkResults = append(checkResults, CheckResult{
		URL:        cfg.ValidationURL,
		Success:    valid,
		Speed:      time.Since(start),
		StatusCode: resp.StatusCode,
		BodySize:   int64(len(body)),
	})

	return &Result{
		ProxyURL:     proxyStr,
		Working:      valid,
		Speed:        time.Since(start),
		DebugInfo:    debugInfo,
		CheckResults: checkResults,
	}, nil
}

// Helper function to get minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// detectProxyType attempts to determine the type of proxy by trying different connection methods
func detectProxyType(host string, timeout time.Duration, debug bool) ProxyDetectionResult {
	result := ProxyDetectionResult{
		Type: ProxyTypeUnknown,
	}

	if debug {
		result.DebugInfo = fmt.Sprintf("Starting proxy detection for %s\n", host)
	}

	// First try HTTP
	httpURL := fmt.Sprintf("http://%s", host)
	if tryHTTPProxy(httpURL, timeout, debug, &result) {
		result.Type = ProxyTypeHTTP
		return result
	}

	// Then try HTTPS
	httpsURL := fmt.Sprintf("https://%s", host)
	if tryHTTPSProxy(httpsURL, timeout, debug, &result) {
		result.Type = ProxyTypeHTTPS
		return result
	}

	// Finally try SOCKS
	if trySocksProxy(host, timeout, debug, &result) {
		return result
	}

	if debug {
		result.DebugInfo += "Failed to detect proxy type\n"
	}
	return result
}

func tryHTTPProxy(proxyURL string, timeout time.Duration, debug bool, result *ProxyDetectionResult) bool {
	if debug {
		result.DebugInfo += fmt.Sprintf("Trying HTTP proxy: %s\n", proxyURL)
	}

	// Try simple HTTP GET first
	transport := &http.Transport{
		Proxy: func(_ *http.Request) (*url.URL, error) {
			return url.Parse(proxyURL)
		},
		DisableKeepAlives: true,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	// Try a simple HTTP request first
	resp, err := client.Get("http://example.com")
	if err == nil {
		resp.Body.Close()
		result.SupportsHTTPS = false
		if debug {
			result.DebugInfo += "HTTP proxy detected (GET method)\n"
		}
		return true
	}

	// Try CONNECT method
	conn, err := net.DialTimeout("tcp", proxyURL[7:], timeout)
	if err != nil {
		if debug {
			result.DebugInfo += fmt.Sprintf("CONNECT failed: %v\n", err)
		}
		return false
	}
	defer conn.Close()

	fmt.Fprintf(conn, "CONNECT example.com:443 HTTP/1.1\r\nHost: example.com:443\r\n\r\n")

	// Read response
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(timeout))
	n, err := conn.Read(buf)
	if err != nil {
		if debug {
			result.DebugInfo += fmt.Sprintf("Failed to read CONNECT response: %v\n", err)
		}
		return false
	}

	if strings.Contains(string(buf[:n]), "200") {
		result.SupportsCONNECT = true
		result.SupportsHTTPS = true
		if debug {
			result.DebugInfo += "HTTP proxy detected (CONNECT method)\n"
		}
		return true
	}

	return false
}

func tryHTTPSProxy(proxyURL string, timeout time.Duration, debug bool, result *ProxyDetectionResult) bool {
	if debug {
		result.DebugInfo += fmt.Sprintf("Trying HTTPS proxy: %s\n", proxyURL)
	}

	transport := &http.Transport{
		Proxy: func(_ *http.Request) (*url.URL, error) {
			return url.Parse(proxyURL)
		},
		DisableKeepAlives: true,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	resp, err := client.Get("https://example.com")
	if err == nil {
		resp.Body.Close()
		result.SupportsHTTPS = true
		if debug {
			result.DebugInfo += "HTTPS proxy detected\n"
		}
		return true
	}

	if debug {
		result.DebugInfo += fmt.Sprintf("HTTPS proxy test failed: %v\n", err)
	}
	return false
}

func trySocksProxy(host string, timeout time.Duration, debug bool, result *ProxyDetectionResult) bool {
	if debug {
		result.DebugInfo += fmt.Sprintf("Trying SOCKS proxy: %s\n", host)
	}

	// Try SOCKS5 first
	dialSocks5 := socks.Dial("socks5://" + host)
	conn, err := dialSocks5("tcp", "example.com:80")
	if err == nil {
		conn.Close()
		result.Type = ProxyTypeSOCKS5
		if debug {
			result.DebugInfo += "SOCKS5 proxy detected\n"
		}
		return true
	}

	// Then try SOCKS4
	dialSocks4 := socks.Dial("socks4://" + host)
	conn, err = dialSocks4("tcp", "example.com:80")
	if err == nil {
		conn.Close()
		result.Type = ProxyTypeSOCKS4
		if debug {
			result.DebugInfo += "SOCKS4 proxy detected\n"
		}
		return true
	}

	if debug {
		result.DebugInfo += "SOCKS proxy detection failed\n"
	}
	return false
}
