package proxy

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// HTTP2Config represents HTTP/2 specific configuration
type HTTP2Config struct {
	Enabled                    bool          `yaml:"enabled"`
	MaxConcurrentStreams       uint32        `yaml:"max_concurrent_streams"`
	InitialWindowSize          uint32        `yaml:"initial_window_size"`
	MaxFrameSize               uint32        `yaml:"max_frame_size"`
	MaxHeaderListSize          uint32        `yaml:"max_header_list_size"`
	PingTimeout                time.Duration `yaml:"ping_timeout"`
	ReadIdleTimeout            time.Duration `yaml:"read_idle_timeout"`
	WriteByteTimeout           time.Duration `yaml:"write_byte_timeout"`
	AllowHTTP                  bool          `yaml:"allow_http"`
	StrictMaxConcurrentStreams bool          `yaml:"strict_max_concurrent_streams"`
}

// GetDefaultHTTP2Config returns default HTTP/2 configuration
func GetDefaultHTTP2Config() HTTP2Config {
	return HTTP2Config{
		Enabled:               true,
		MaxConcurrentStreams:  100,
		InitialWindowSize:     65535,
		MaxFrameSize:          16384,
		MaxHeaderListSize:     10 << 20, // 10MB
		PingTimeout:           15 * time.Second,
		ReadIdleTimeout:       30 * time.Second,
		WriteByteTimeout:      10 * time.Second,
		AllowHTTP:             false,
		StrictMaxConcurrentStreams: false,
	}
}

// createHTTP2Transport creates an HTTP/2 capable transport
func (c *Checker) createHTTP2Transport(proxyURL *url.URL, scheme string, auth *ProxyAuth, result *ProxyResult) *http.Transport {
	if c.debug {
		result.DebugInfo += fmt.Sprintf("[HTTP2] Creating HTTP/2 transport for: %s\n", scheme)
	}

	transport := &http.Transport{
		TLSHandshakeTimeout:   c.config.Timeout / 2,
		ResponseHeaderTimeout: c.config.Timeout / 2,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false,
		ForceAttemptHTTP2:     true, // Force HTTP/2 upgrade
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"h2", "http/1.1"}, // Prefer HTTP/2
		},
	}

	// Configure HTTP/2 specific settings
	http2Config := GetDefaultHTTP2Config()
	
	// Note: HTTP/2 configuration is applied to the standard transport
	// which will automatically use HTTP/2 when ForceAttemptHTTP2 is true

	// Set up proxy configuration
	if scheme == "http" || scheme == "https" {
		proxyFunc := func(req *http.Request) (*url.URL, error) {
			// Build proxy URL with authentication if available
			proxyURLWithAuth := &url.URL{
				Scheme: scheme,
				Host:   proxyURL.Host,
			}
			
			if auth != nil && auth.Username != "" {
				proxyURLWithAuth.User = url.UserPassword(auth.Username, auth.Password)
			}
			
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[HTTP2] Using proxy: %s\n", c.cleanProxyURL(proxyURLWithAuth))
			}
			
			return proxyURLWithAuth, nil
		}
		
		transport.Proxy = proxyFunc
		// Note: HTTP/2 over HTTP proxy is complex and requires custom dialer implementation
		// For now, we'll use the standard transport with HTTP/2 enabled
		// In production, you would implement a custom DialTLS function
	}

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[HTTP2] HTTP/2 transport configured with max concurrent streams: %d\n", 
			http2Config.MaxConcurrentStreams)
	}

	return transport
}

// testHTTP2Support tests if a proxy supports HTTP/2
func (c *Checker) testHTTP2Support(proxyURL *url.URL, scheme string, result *ProxyResult) (bool, *http.Client, error) {
	if c.debug {
		result.DebugInfo += fmt.Sprintf("[HTTP2] Testing HTTP/2 support for: %s\n", scheme)
	}

	// Extract authentication
	auth := c.getProxyAuth(proxyURL, result)

	// Create HTTP/2 transport
	transport := c.createHTTP2Transport(proxyURL, scheme, auth, result)

	client := &http.Client{
		Transport: transport,
		Timeout:   c.config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Test with HTTPS endpoint (HTTP/2 typically requires TLS)
	testURL := "https://httpbin.org/get"
	if c.config.ValidationURL != "" {
		testURL = c.config.ValidationURL
	}

	start := time.Now()
	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[HTTP2] Failed to create request: %v\n", err)
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
			result.DebugInfo += fmt.Sprintf("[HTTP2] HTTP/2 test failed: %v\n", err)
		}
		return false, nil, err
	}
	defer resp.Body.Close()

	duration := time.Since(start)

	// Check if the response was served over HTTP/2
	isHTTP2 := resp.ProtoMajor == 2
	if c.debug {
		result.DebugInfo += fmt.Sprintf("[HTTP2] Response protocol: %s, HTTP/2: %v, Status: %d, Time: %v\n",
			resp.Proto, isHTTP2, resp.StatusCode, duration)
	}

	// Create check result
	checkResult := CheckResult{
		URL:        testURL,
		Success:    isHTTP2 && resp.StatusCode == 200,
		Speed:      duration,
		StatusCode: resp.StatusCode,
	}

	if resp.StatusCode != 200 {
		checkResult.Error = fmt.Sprintf("unexpected status code: %d", resp.StatusCode)
	} else if !isHTTP2 {
		checkResult.Error = "response not served over HTTP/2"
	}

	result.CheckResults = append(result.CheckResults, checkResult)

	if isHTTP2 && resp.StatusCode == 200 {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("[HTTP2] Successfully verified HTTP/2 support\n")
		}
		return true, client, nil
	}

	return false, nil, fmt.Errorf("HTTP/2 not supported or request failed")
}

// detectHTTP2Protocol detects if a proxy supports HTTP/2
func (c *Checker) detectHTTP2Protocol(proxyURL *url.URL, result *ProxyResult) (bool, *http.Client) {
	if c.debug {
		result.DebugInfo += fmt.Sprintf("[HTTP2] Detecting HTTP/2 protocol support\n")
	}

	// Test HTTP/2 with HTTPS proxy
	if success, client, err := c.testHTTP2Support(proxyURL, "https", result); success {
		return true, client
	} else if c.debug {
		result.DebugInfo += fmt.Sprintf("[HTTP2] HTTPS HTTP/2 test failed: %v\n", err)
	}

	// Test HTTP/2 with HTTP proxy (less common but possible with ALPN)
	if success, client, err := c.testHTTP2Support(proxyURL, "http", result); success {
		return true, client
	} else if c.debug {
		result.DebugInfo += fmt.Sprintf("[HTTP2] HTTP HTTP/2 test failed: %v\n", err)
	}

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[HTTP2] No HTTP/2 support detected\n")
	}

	return false, nil
}