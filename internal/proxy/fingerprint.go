package proxy

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ProxySoftware represents the detected software/vendor of proxy
type ProxySoftware string

const (
	ProxySoftwareNginx      ProxySoftware = "nginx"
	ProxySoftwareApache     ProxySoftware = "apache"
	ProxySoftwareHAProxy    ProxySoftware = "haproxy"
	ProxySoftwareNuster     ProxySoftware = "nuster"
	ProxySoftwareVarnish    ProxySoftware = "varnish"
	ProxySoftwareTraefik    ProxySoftware = "traefik"
	ProxySoftwareEnvoy      ProxySoftware = "envoy"
	ProxySoftwareCaddy      ProxySoftware = "caddy"
	ProxySoftwareAWS        ProxySoftware = "aws"
	ProxySoftwareCloudflare ProxySoftware = "cloudflare"
	ProxySoftwareFastly     ProxySoftware = "fastly"
	ProxySoftwareStackpath  ProxySoftware = "stackpath"
	ProxySoftwareKong       ProxySoftware = "kong"
	ProxySoftwareSquid      ProxySoftware = "squid"
	ProxySoftwareUnknown    ProxySoftware = "unknown"
)

// FingerprintResult contains proxy fingerprinting information
type FingerprintResult struct {
	ProxySoftware    ProxySoftware         `json:"proxy_type"`
	Version      string            `json:"version,omitempty"`
	Confidence   float64           `json:"confidence"` // 0.0 - 1.0
	Indicators   []string          `json:"indicators"`
	Behaviors    []string          `json:"behaviors,omitempty"`
	RawHeaders   map[string]string `json:"raw_headers,omitempty"`
	ErrorPattern string            `json:"error_pattern,omitempty"`
}

// FingerprintSignature defines characteristics for identifying a proxy
type FingerprintSignature struct {
	ProxySoftware       ProxySoftware
	Headers         []string                  // Headers that indicate this proxy
	HeaderPatterns  map[string]*regexp.Regexp // Regex patterns for specific headers
	ErrorPatterns   []*regexp.Regexp          // Patterns in error pages
	Behaviors       []BehaviorTest            // Specific behavior tests
	ConfidenceBoost float64                   // How much to boost confidence on match
}

// BehaviorTest defines a specific behavior to test
type BehaviorTest struct {
	Name        string
	Request     func(*http.Client) (*http.Response, error)
	Validate    func(*http.Response, []byte) bool
	Description string
}

var fingerprintSignatures = []FingerprintSignature{
	{
		ProxySoftware: ProxySoftwareNginx,
		Headers:   []string{"Server"},
		HeaderPatterns: map[string]*regexp.Regexp{
			"Server": regexp.MustCompile(`(?i)nginx(/[\d.]+)?`),
		},
		ErrorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`<center><h1>400 Bad Request</h1></center>\s*<hr><center>nginx`),
			regexp.MustCompile(`<center><h1>403 Forbidden</h1></center>\s*<hr><center>nginx`),
			regexp.MustCompile(`<title>400 Bad Request</title>.*nginx`),
		},
		ConfidenceBoost: 0.9,
	},
	{
		ProxySoftware: ProxySoftwareApache,
		Headers:   []string{"Server"},
		HeaderPatterns: map[string]*regexp.Regexp{
			"Server": regexp.MustCompile(`(?i)apache(/[\d.]+)?`),
		},
		ErrorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`<!DOCTYPE HTML PUBLIC "-//IETF//DTD HTML 2.0//EN">.*<title>400 Bad Request</title>`),
			regexp.MustCompile(`<h1>Bad Request</h1>\s*<p>Your browser sent a request that this server could not understand`),
			regexp.MustCompile(`<title>403 Forbidden</title>.*<h1>Forbidden</h1>`),
		},
		ConfidenceBoost: 0.9,
	},
	{
		ProxySoftware: ProxySoftwareVarnish,
		Headers:   []string{"Via", "X-Varnish", "X-Varnish-Host", "X-Varnish-Backend", "X-Cache-Status"},
		HeaderPatterns: map[string]*regexp.Regexp{
			"Via":       regexp.MustCompile(`(?i)varnish`),
			"X-Varnish": regexp.MustCompile(`^\d+(\s+\d+)?$`), // "7" or "65563 29"
			"Server":    regexp.MustCompile(`(?i)varnish`),
		},
		ErrorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`400 Bad Request`),
		},
		ConfidenceBoost: 0.95,
	},
	{
		ProxySoftware: ProxySoftwareHAProxy,
		Headers:   []string{}, // HAProxy doesn't add distinctive headers by default
		ErrorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`<html><body><h1>400 Bad request</h1>\s*Your browser sent an invalid request`),
			regexp.MustCompile(`<h1>403 Forbidden</h1>\s*Request forbidden by administrative rules`),
		},
		ConfidenceBoost: 0.7,
	},
	{
		ProxySoftware: ProxySoftwareEnvoy,
		Headers:   []string{"Server", "x-envoy-upstream-service-time"},
		HeaderPatterns: map[string]*regexp.Regexp{
			"Server": regexp.MustCompile(`(?i)envoy`),
		},
		ErrorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`400 Bad Request.*content-length: 0`),
		},
		ConfidenceBoost: 0.9,
	},
	{
		ProxySoftware: ProxySoftwareTraefik,
		Headers:   []string{"X-Forwarded-Host", "X-Forwarded-Port", "X-Forwarded-Proto", "X-Forwarded-Server", "X-Real-Ip"},
		ErrorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`^400 Bad Request$`),
			regexp.MustCompile(`400 Bad Request: too many Host headers`),
		},
		ConfidenceBoost: 0.6,
	},
	{
		ProxySoftware: ProxySoftwareCaddy,
		Headers:   []string{"Server"},
		HeaderPatterns: map[string]*regexp.Regexp{
			"Server": regexp.MustCompile(`(?i)caddy`),
		},
		ErrorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`400 Bad Request: too many Host headers`),
			regexp.MustCompile(`404 page not found`),
			regexp.MustCompile(`505 HTTP Version Not Supported: unsupported protocol version`),
		},
		ConfidenceBoost: 0.9,
	},
	{
		ProxySoftware: ProxySoftwareCloudflare,
		Headers:   []string{"CF-Ray", "CF-Cache-Status", "CF-Request-ID", "Server"},
		HeaderPatterns: map[string]*regexp.Regexp{
			"Server":  regexp.MustCompile(`(?i)cloudflare`),
			"CF-Ray": regexp.MustCompile(`^[0-9a-f]+-[A-Z]{3}$`),
		},
		ConfidenceBoost: 0.95,
	},
	{
		ProxySoftware: ProxySoftwareFastly,
		Headers:   []string{"X-Fastly-Request-ID", "X-Served-By", "X-Cache"},
		HeaderPatterns: map[string]*regexp.Regexp{
			"X-Served-By": regexp.MustCompile(`(?i)cache-.*\.fastly\.net`),
		},
		ConfidenceBoost: 0.95,
	},
	{
		ProxySoftware: ProxySoftwareAWS,
		Headers:   []string{"X-Amz-Cf-Id", "X-Amz-Cf-Pop", "X-Amzn-RequestId", "X-Amzn-Trace-Id"},
		HeaderPatterns: map[string]*regexp.Regexp{
			"X-Amz-Cf-Id":  regexp.MustCompile(`.+`),
			"X-Amzn-Trace-Id": regexp.MustCompile(`^Root=`),
		},
		ConfidenceBoost: 0.95,
	},
	{
		ProxySoftware: ProxySoftwareKong,
		Headers:   []string{"X-Kong-Upstream-Latency", "X-Kong-Proxy-Latency", "Via"},
		HeaderPatterns: map[string]*regexp.Regexp{
			"Via":    regexp.MustCompile(`(?i)kong`),
			"Server": regexp.MustCompile(`(?i)kong`),
		},
		ConfidenceBoost: 0.95,
	},
	{
		ProxySoftware: ProxySoftwareSquid,
		Headers:   []string{"X-Squid-Error", "X-Cache"},
		HeaderPatterns: map[string]*regexp.Regexp{
			"Server":  regexp.MustCompile(`(?i)squid`),
			"X-Cache": regexp.MustCompile(`(?i)(MISS|HIT) from .+`),
		},
		ConfidenceBoost: 0.9,
	},
}

// FingerprintProxy attempts to identify the proxy type
func (c *Checker) FingerprintProxy(client *http.Client, proxyURL string) *FingerprintResult {
	result := &FingerprintResult{
		ProxySoftware:  ProxySoftwareUnknown,
		Confidence: 0.0,
		Indicators: []string{},
		Behaviors:  []string{},
		RawHeaders: make(map[string]string),
	}

	// Test 1: Normal request to validation URL
	normalResp, normalBody := c.fingerprintNormalRequest(client)
	if normalResp != nil {
		defer normalResp.Body.Close()
		c.analyzeResponse(result, normalResp, normalBody)
	}

	// Test 2: Trigger error responses for fingerprinting
	errorResp, errorBody := c.fingerprintErrorRequest(client)
	if errorResp != nil {
		defer errorResp.Body.Close()
		c.analyzeResponse(result, errorResp, errorBody)
	}

	// Test 3: Check for specific behaviors
	c.testProxyBehaviors(client, result)

	// Determine final proxy type based on accumulated indicators
	c.determineFinalProxySoftware(result)

	return result
}

// fingerprintNormalRequest makes a normal request to gather headers
func (c *Checker) fingerprintNormalRequest(client *http.Client) (*http.Response, []byte) {
	req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return nil, nil
	}

	req.Header.Set("User-Agent", c.config.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil
	}

	return resp, body
}

// fingerprintErrorRequest makes requests designed to trigger error pages
func (c *Checker) fingerprintErrorRequest(client *http.Client) (*http.Response, []byte) {
	// Send malformed request to trigger 400 error
	req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return nil, nil
	}

	// Add header that many proxies reject to trigger error page
	req.Header.Set("Bad Header:", "value")
	req.Header.Set("User-Agent", c.config.UserAgent)

	// Disable automatic redirect following
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Do(req)
	if err != nil {
		// Try another error trigger - invalid Host header
		req2, _ := http.NewRequest("GET", c.config.ValidationURL, nil)
		req2.Host = "/"
		resp, err = client.Do(req2)
		if err != nil {
			return nil, nil
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil
	}

	return resp, body
}

// analyzeResponse analyzes a response for proxy fingerprinting indicators
func (c *Checker) analyzeResponse(result *FingerprintResult, resp *http.Response, body []byte) {
	// Store all headers for analysis
	for name, values := range resp.Header {
		if len(values) > 0 {
			result.RawHeaders[name] = values[0]
		}
	}

	bodyStr := string(body)

	// Check against all signatures
	for _, sig := range fingerprintSignatures {
		matches := 0
		confidence := 0.0

		// Check headers
		for _, headerName := range sig.Headers {
			if value := resp.Header.Get(headerName); value != "" {
				matches++
				result.Indicators = append(result.Indicators, fmt.Sprintf("Header: %s=%s", headerName, value))

				// Check header patterns
				if pattern, exists := sig.HeaderPatterns[headerName]; exists {
					if pattern.MatchString(value) {
						confidence += sig.ConfidenceBoost * 0.5

						// Extract version if present
						if strings.EqualFold(headerName, "Server") {
							result.Version = extractVersion(value)
						}
					}
				} else {
					confidence += 0.1
				}
			}
		}

		// Check header patterns even without explicit header list
		for headerName, pattern := range sig.HeaderPatterns {
			if value := resp.Header.Get(headerName); value != "" && pattern.MatchString(value) {
				found := false
				for _, h := range sig.Headers {
					if strings.EqualFold(h, headerName) {
						found = true
						break
					}
				}
				if !found {
					matches++
					result.Indicators = append(result.Indicators, fmt.Sprintf("Pattern: %s matches %s", headerName, value))
					confidence += sig.ConfidenceBoost * 0.5
				}
			}
		}

		// Check error patterns in body
		for _, pattern := range sig.ErrorPatterns {
			if pattern.MatchString(bodyStr) {
				matches++
				result.Indicators = append(result.Indicators, fmt.Sprintf("Error pattern: %s", sig.ProxySoftware))
				result.ErrorPattern = pattern.String()
				confidence += sig.ConfidenceBoost * 0.7
				break
			}
		}

		// Update result if this signature has higher confidence
		if confidence > result.Confidence {
			result.Confidence = confidence
			result.ProxySoftware = sig.ProxySoftware
		}
	}
}

// testProxyBehaviors tests for specific proxy behaviors
func (c *Checker) testProxyBehaviors(client *http.Client, result *FingerprintResult) {
	// Test 1: Multiple Host headers (nginx/apache reject, haproxy accepts)
	req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return
	}
	req.Host = "test1.com"
	req.Header.Add("Host", "test2.com")

	resp, err := client.Do(req)
	if err == nil {
		if resp.StatusCode == 400 {
			result.Behaviors = append(result.Behaviors, "Rejects multiple Host headers (nginx/apache/traefik/caddy)")
			// Increase confidence for these types
			if result.ProxySoftware == ProxySoftwareNginx || result.ProxySoftware == ProxySoftwareApache ||
				result.ProxySoftware == ProxySoftwareTraefik || result.ProxySoftware == ProxySoftwareCaddy {
				result.Confidence += 0.1
			}
		} else if resp.StatusCode < 400 {
			result.Behaviors = append(result.Behaviors, "Accepts multiple Host headers (haproxy/envoy)")
			if result.ProxySoftware == ProxySoftwareHAProxy || result.ProxySoftware == ProxySoftwareEnvoy {
				result.Confidence += 0.1
			}
		}
		resp.Body.Close()
	}

	// Test 2: Underscore in header name (nginx rejects, apache/haproxy accept)
	req2, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return
	}
	req2.Header.Set("X_Custom_Header", "value")

	resp2, err := client.Do(req2)
	if err == nil {
		// Check if header was forwarded by looking at response
		if resp2.StatusCode < 400 {
			result.Behaviors = append(result.Behaviors, "Forwards underscore headers (apache/haproxy/traefik/envoy)")
			if result.ProxySoftware == ProxySoftwareApache || result.ProxySoftware == ProxySoftwareHAProxy ||
				result.ProxySoftware == ProxySoftwareTraefik || result.ProxySoftware == ProxySoftwareEnvoy {
				result.Confidence += 0.05
			}
		}
		resp2.Body.Close()
	}

	// Test 3: URL encoding behavior - %2f (encoded slash)
	testURL := strings.Replace(c.config.ValidationURL, "/", "/%2f", 1)
	req3, err := http.NewRequest("GET", testURL, nil)
	if err == nil {
		resp3, err := client.Do(req3)
		if err == nil {
			if resp3.StatusCode == 404 {
				result.Behaviors = append(result.Behaviors, "Rejects %2f in path (apache default)")
				if result.ProxySoftware == ProxySoftwareApache {
					result.Confidence += 0.1
				}
			}
			resp3.Body.Close()
		}
	}
}

// determineFinalProxySoftware determines the final proxy type based on all indicators
func (c *Checker) determineFinalProxySoftware(result *FingerprintResult) {
	// Normalize confidence to 0.0-1.0 range
	if result.Confidence > 1.0 {
		result.Confidence = 1.0
	}

	// If confidence is too low, mark as unknown
	if result.Confidence < 0.3 {
		result.ProxySoftware = ProxySoftwareUnknown
	}
}

// extractVersion extracts version information from a Server header
func extractVersion(serverHeader string) string {
	// Match version patterns like "nginx/1.15.3" or "Apache/2.4.34"
	versionRegex := regexp.MustCompile(`[\d.]+`)
	if match := versionRegex.FindString(serverHeader); match != "" {
		return match
	}
	return ""
}

// FingerprintFromURL performs fingerprinting using a direct URL instead of through proxy
func FingerprintFromURL(url string, timeout time.Duration, insecureSSL bool) (*FingerprintResult, error) {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecureSSL,
			},
		},
	}

	checker := &Checker{
		config: Config{
			ValidationURL: url,
			UserAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		},
	}

	return checker.FingerprintProxy(client, ""), nil
}
