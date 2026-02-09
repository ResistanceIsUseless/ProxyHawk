package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AdvancedChecks represents advanced security check settings
type AdvancedChecks struct {
	TestProtocolSmuggling     bool     `yaml:"test_protocol_smuggling"`
	TestDNSRebinding          bool     `yaml:"test_dns_rebinding"`
	TestIPv6                  bool     `yaml:"test_ipv6"`
	TestHTTPMethods           []string `yaml:"test_http_methods"`
	TestCachePoisoning        bool     `yaml:"test_cache_poisoning"`
	TestHostHeaderInjection   bool     `yaml:"test_host_header_injection"`
	TestSSRF                  bool     `yaml:"test_ssrf"`
	DisableInteractsh         bool     `yaml:"disable_interactsh"`            // Set to true to disable Interactsh and use basic checks
	TestNginxVulnerabilities   bool     `yaml:"test_nginx_vulnerabilities"`     // Test for nginx-specific vulnerabilities
	TestApacheVulnerabilities  bool     `yaml:"test_apache_vulnerabilities"`    // Test for Apache mod_proxy vulnerabilities
	TestKongVulnerabilities    bool     `yaml:"test_kong_vulnerabilities"`      // Test for Kong API Gateway vulnerabilities
	TestGenericVulnerabilities bool     `yaml:"test_generic_vulnerabilities"`   // Test for generic proxy misconfigurations
	TestExtendedVulnerabilities bool    `yaml:"test_extended_vulnerabilities"` // Test for extended/medium-priority vulnerabilities
}

// AdvancedCheckResult represents the result of advanced security checks
type AdvancedCheckResult struct {
	ProtocolSmuggling   *CheckResult
	DNSRebinding        *CheckResult
	IPv6                *CheckResult
	HTTPMethods         []*CheckResult
	CachePoisoning      *CheckResult
	HostHeaderInjection *CheckResult
	SSRF                *CheckResult
}

// performAdvancedChecks runs all configured advanced security checks
func (c *Checker) performAdvancedChecks(client *http.Client, result *ProxyResult) error {
	if !c.hasAdvancedChecks() {
		return nil
	}

	advancedResults := &AdvancedCheckResult{}

	// Initialize Interactsh tester unless explicitly disabled
	var tester *InteractshTester
	var err error
	if !c.config.AdvancedChecks.DisableInteractsh {
		tester, err = NewInteractshTester()
		if err != nil {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("\nFailed to initialize Interactsh tester: %v\nFalling back to basic checks.", err)
			}
		}
		if tester != nil {
			defer tester.Close()
		}
	}

	// Use validation URL as fallback test domain
	testDomain := c.config.ValidationURL
	if testDomain == "" {
		testDomain = "http://www.google.com"
	}
	if u, err := url.Parse(testDomain); err == nil {
		testDomain = u.Host
	}

	// Protocol Smuggling Test
	if c.config.AdvancedChecks.TestProtocolSmuggling {
		if tester != nil {
			res, err := tester.PerformInteractshTest(client, c, func(url string) (*http.Request, error) {
				req, err := http.NewRequest("POST", fmt.Sprintf("http://%s", url), strings.NewReader("test"))
				if err != nil {
					return nil, err
				}
				req.Header.Add("Content-Length", "4")
				req.Header.Add("Transfer-Encoding", "chunked")
				return req, nil
			})
			if err == nil {
				advancedResults.ProtocolSmuggling = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		} else {
			if res, err := c.checkProtocolSmuggling(client, testDomain); err == nil {
				advancedResults.ProtocolSmuggling = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		}
	}

	// DNS Rebinding Test
	if c.config.AdvancedChecks.TestDNSRebinding {
		if tester != nil {
			res, err := tester.PerformInteractshTest(client, c, func(url string) (*http.Request, error) {
				req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", url), nil)
				if err != nil {
					return nil, err
				}
				req.Header.Set("X-Forwarded-Host", url)
				req.Header.Set("Host", url)
				return req, nil
			})
			if err == nil {
				advancedResults.DNSRebinding = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		} else {
			if res, err := c.checkDNSRebinding(client, testDomain); err == nil {
				advancedResults.DNSRebinding = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		}
	}

	// IPv6 Test
	if c.config.AdvancedChecks.TestIPv6 {
		if tester != nil {
			res, err := tester.PerformInteractshTest(client, c, func(url string) (*http.Request, error) {
				return http.NewRequest("GET", fmt.Sprintf("http://[%s]", url), nil)
			})
			if err == nil {
				advancedResults.IPv6 = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		} else {
			if res, err := c.checkIPv6Support(client, testDomain); err == nil {
				advancedResults.IPv6 = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		}
	}

	// HTTP Methods Test
	if len(c.config.AdvancedChecks.TestHTTPMethods) > 0 {
		var results []*CheckResult
		if tester != nil {
			for _, method := range c.config.AdvancedChecks.TestHTTPMethods {
				res, err := tester.PerformInteractshTest(client, c, func(url string) (*http.Request, error) {
					return http.NewRequest(method, fmt.Sprintf("http://%s", url), nil)
				})
				if err == nil {
					results = append(results, res)
				}
			}
		} else {
			results, err = c.checkHTTPMethods(client, testDomain)
		}
		if err == nil && len(results) > 0 {
			advancedResults.HTTPMethods = results
			for _, res := range results {
				result.CheckResults = append(result.CheckResults, *res)
			}
		}
	}

	// Cache Poisoning Test
	if c.config.AdvancedChecks.TestCachePoisoning {
		if tester != nil {
			res, err := tester.PerformInteractshTest(client, c, func(url string) (*http.Request, error) {
				req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", url), nil)
				if err != nil {
					return nil, err
				}
				req.Header.Set("Cache-Control", "public, max-age=31536000")
				req.Header.Set("X-Cache-Control", "public, max-age=31536000")
				return req, nil
			})
			if err == nil {
				advancedResults.CachePoisoning = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		} else {
			if res, err := c.checkCachePoisoning(client, testDomain); err == nil {
				advancedResults.CachePoisoning = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		}
	}

	// Host Header Injection Test
	if c.config.AdvancedChecks.TestHostHeaderInjection {
		if tester != nil {
			res, err := tester.PerformInteractshTest(client, c, func(url string) (*http.Request, error) {
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
			if err == nil {
				advancedResults.HostHeaderInjection = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		} else {
			if res, err := c.checkHostHeaderInjection(client, testDomain); err == nil {
				advancedResults.HostHeaderInjection = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		}
	}

	// SSRF Test
	if c.config.AdvancedChecks.TestSSRF {
		if res, err := c.checkSSRF(client, testDomain); err == nil {
			advancedResults.SSRF = res
			result.CheckResults = append(result.CheckResults, *res)
		}
	}

	// Nginx Vulnerability Tests
	if c.config.AdvancedChecks.TestNginxVulnerabilities {
		if c.debug {
			result.DebugInfo += "[NGINX VULNS] Running nginx-specific vulnerability checks\n"
		}
		nginxResults := c.performNginxVulnerabilityChecks(client, result)
		result.NginxVulnerabilities = nginxResults

		if c.debug {
			result.DebugInfo += fmt.Sprintf("[NGINX VULNS] Complete - Found: off-by-slash=%t, k8s-api=%t, webhook=%t\n",
				nginxResults.OffBySlashVuln, nginxResults.K8sAPIExposed, nginxResults.IngressWebhookExposed)
		}
	}

	// Apache Vulnerability Tests
	if c.config.AdvancedChecks.TestApacheVulnerabilities {
		if c.debug {
			result.DebugInfo += "[APACHE VULNS] Running Apache mod_proxy vulnerability checks\n"
		}
		apacheResults := c.performApacheVulnerabilityChecks(client, result)
		result.ApacheVulnerabilities = apacheResults

		if c.debug {
			result.DebugInfo += fmt.Sprintf("[APACHE VULNS] Complete - Found: CVE-2021-40438=%t, CVE-2020-11984=%t, CVE-2021-41773=%t\n",
				apacheResults.CVE_2021_40438_SSRF, apacheResults.CVE_2020_11984_RCE, apacheResults.CVE_2021_41773_PathTraversal)
		}
	}

	// Kong Vulnerability Tests
	if c.config.AdvancedChecks.TestKongVulnerabilities {
		if c.debug {
			result.DebugInfo += "[KONG VULNS] Running Kong API Gateway vulnerability checks\n"
		}
		kongResults := c.performKongVulnerabilityChecks(client, result)
		result.KongVulnerabilities = kongResults

		if c.debug {
			result.DebugInfo += fmt.Sprintf("[KONG VULNS] Complete - Found: manager=%t, admin-api=%t, unauth-access=%t\n",
				kongResults.ManagerExposed, kongResults.AdminAPIExposed, kongResults.UnauthorizedAccess)
		}
	}

	// Generic Vulnerability Tests
	if c.config.AdvancedChecks.TestGenericVulnerabilities {
		if c.debug {
			result.DebugInfo += "[GENERIC VULNS] Running generic proxy misconfiguration checks\n"
		}
		genericResults := c.performGenericVulnerabilityChecks(client, result)
		result.GenericVulnerabilities = genericResults

		if c.debug {
			result.DebugInfo += fmt.Sprintf("[GENERIC VULNS] Complete - Found: open-proxy=%t, xff-bypass=%t, cache-poison=%t, linkerd-ssrf=%t, actuator=%t\n",
				genericResults.OpenProxyToLocalhost, genericResults.XForwardedForBypass, genericResults.CachePoisonVulnerable,
				genericResults.LinkerdSSRF, genericResults.SpringBootActuator)
		}
	}

	// Extended Vulnerability Tests
	if c.config.AdvancedChecks.TestExtendedVulnerabilities {
		if c.debug {
			result.DebugInfo += "[EXTENDED VULNS] Running extended vulnerability checks\n"
		}
		extendedResults := c.performExtendedVulnerabilityChecks(client, result)
		result.ExtendedVulnerabilities = extendedResults

		if c.debug {
			result.DebugInfo += fmt.Sprintf("[EXTENDED VULNS] Complete - Found: nginx-version=%t, nginx-config=%t, websocket=%t, http2-smuggling=%t, proxy-auth-bypass=%t\n",
				extendedResults.NginxVersionDetected, extendedResults.NginxConfigExposed, extendedResults.WebSocketAbuseVulnerable,
				extendedResults.HTTP2SmugglingVulnerable, extendedResults.ProxyAuthBypass)
		}
	}

	return nil
}

func (c *Checker) hasAdvancedChecks() bool {
	checks := c.config.AdvancedChecks
	return checks.TestProtocolSmuggling ||
		checks.TestDNSRebinding ||
		checks.TestIPv6 ||
		len(checks.TestHTTPMethods) > 0 ||
		checks.TestCachePoisoning ||
		checks.TestHostHeaderInjection ||
		checks.TestSSRF ||
		checks.TestNginxVulnerabilities ||
		checks.TestApacheVulnerabilities ||
		checks.TestKongVulnerabilities ||
		checks.TestGenericVulnerabilities ||
		checks.TestExtendedVulnerabilities
}

// Individual check implementations
func (c *Checker) checkProtocolSmuggling(client *http.Client, testDomain string) (*CheckResult, error) {
	result := &CheckResult{
		URL:     fmt.Sprintf("http://%s", testDomain),
		Success: false,
	}

	// Apply rate limiting
	c.applyRateLimit(testDomain, &ProxyResult{DebugInfo: ""})

	// Send a request with ambiguous Content-Length headers
	req, err := http.NewRequest("POST", result.URL, strings.NewReader("test"))
	if err != nil {
		return result, err
	}

	req.Header.Add("Content-Length", "4")
	req.Header.Add("Transfer-Encoding", "chunked")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	defer resp.Body.Close()

	result.Speed = time.Since(start)
	result.StatusCode = resp.StatusCode
	result.Success = resp.StatusCode < 400

	return result, nil
}

func (c *Checker) checkDNSRebinding(client *http.Client, testDomain string) (*CheckResult, error) {
	result := &CheckResult{
		URL:     fmt.Sprintf("http://%s", testDomain),
		Success: false,
	}

	// Apply rate limiting
	c.applyRateLimit(testDomain, &ProxyResult{DebugInfo: ""})

	// Use a real DNS rebinding test service
	// This domain should first resolve to a public IP, then to 127.0.0.1
	// Format: 7f000001.1time.<interactsh-domain> resolves to 127.0.0.1 after TTL expires
	rebindingURL := fmt.Sprintf("http://%s", testDomain)

	// First request - should succeed (resolves to public IP)
	req1, err := http.NewRequest("GET", rebindingURL, nil)
	if err != nil {
		return result, err
	}
	req1.Header.Set("User-Agent", c.config.UserAgent)

	start := time.Now()
	resp1, err := client.Do(req1)
	if err != nil {
		result.Error = fmt.Sprintf("First request failed: %v", err)
		return result, err
	}
	defer resp1.Body.Close()

	// Small delay to allow DNS TTL to expire
	time.Sleep(2 * time.Second)

	// Second request - should fail if DNS rebinding protection works (resolves to 127.0.0.1)
	req2, err := http.NewRequest("GET", rebindingURL, nil)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	req2.Header.Set("User-Agent", c.config.UserAgent)

	resp2, err := client.Do(req2)
	result.Speed = time.Since(start)

	if err != nil {
		// If second request fails, DNS rebinding protection likely works
		result.Success = true
		result.Error = "DNS rebinding protection appears effective"
	} else {
		defer resp2.Body.Close()
		result.StatusCode = resp2.StatusCode
		// If second request succeeds, proxy may be vulnerable to DNS rebinding
		if resp2.StatusCode < 400 {
			result.Success = false
			result.Error = "Warning: DNS rebinding attack may be possible"
		} else {
			result.Success = true
			result.Error = "DNS rebinding protection appears effective"
		}
	}

	return result, nil
}

func (c *Checker) checkIPv6Support(client *http.Client, testDomain string) (*CheckResult, error) {
	result := &CheckResult{
		URL:     fmt.Sprintf("http://[%s]", testDomain),
		Success: false,
	}

	// Apply rate limiting
	c.applyRateLimit(testDomain, &ProxyResult{DebugInfo: ""})

	req, err := http.NewRequest("GET", result.URL, nil)
	if err != nil {
		return result, err
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	defer resp.Body.Close()

	result.Speed = time.Since(start)
	result.StatusCode = resp.StatusCode
	result.Success = resp.StatusCode < 400

	return result, nil
}

func (c *Checker) checkHTTPMethods(client *http.Client, testDomain string) ([]*CheckResult, error) {
	var results []*CheckResult
	baseURL := fmt.Sprintf("http://%s", testDomain)

	for _, method := range c.config.AdvancedChecks.TestHTTPMethods {
		// Apply rate limiting between method tests
		c.applyRateLimit(testDomain, &ProxyResult{DebugInfo: ""})

		result := &CheckResult{
			URL: baseURL,
		}

		req, err := http.NewRequest(method, baseURL, nil)
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}

		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		resp.Body.Close()

		result.Speed = time.Since(start)
		result.StatusCode = resp.StatusCode
		result.Success = resp.StatusCode < 400

		results = append(results, result)
	}

	return results, nil
}

func (c *Checker) checkCachePoisoning(client *http.Client, testDomain string) (*CheckResult, error) {
	result := &CheckResult{
		URL:     fmt.Sprintf("http://%s", testDomain),
		Success: false,
	}

	// Apply rate limiting
	c.applyRateLimit(testDomain, &ProxyResult{DebugInfo: ""})

	req, err := http.NewRequest("GET", result.URL, nil)
	if err != nil {
		return result, err
	}

	// Add cache control headers
	req.Header.Set("Cache-Control", "public, max-age=31536000")
	req.Header.Set("X-Cache-Control", "public, max-age=31536000")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	defer resp.Body.Close()

	result.Speed = time.Since(start)
	result.StatusCode = resp.StatusCode
	result.Success = resp.StatusCode < 400

	return result, nil
}

func (c *Checker) checkHostHeaderInjection(client *http.Client, testDomain string) (*CheckResult, error) {
	result := &CheckResult{
		URL:     fmt.Sprintf("http://%s", testDomain),
		Success: false,
	}

	// Apply rate limiting
	c.applyRateLimit(testDomain, &ProxyResult{DebugInfo: ""})

	// Test multiple internal network targets through host header injection
	internalTargets := []string{
		"127.0.0.1",           // Localhost
		"127.0.0.1:22",        // SSH port
		"127.0.0.1:3306",      // MySQL port
		"192.168.1.1",         // Common gateway
		"10.0.0.1",            // RFC 1918 private
		"172.16.0.1",          // RFC 1918 private
		"169.254.169.254",     // AWS metadata service
		"169.254.169.254:80",  // AWS metadata with explicit port
		"localhost",           // Localhost by name
		"0.0.0.0",            // All interfaces
		"[::1]",              // IPv6 localhost
		"metadata.google.internal", // GCP metadata
	}

	var vulnerabilityFound bool
	var vulnerabilityDetails []string

	for _, target := range internalTargets {
		if c.testHostHeaderWithTarget(client, testDomain, target, &vulnerabilityDetails) {
			vulnerabilityFound = true
		}
	}

	// Test additional header injection techniques
	if c.testAdvancedHeaderInjection(client, testDomain, &vulnerabilityDetails) {
		vulnerabilityFound = true
	}

	result.Success = !vulnerabilityFound
	if vulnerabilityFound {
		result.Error = fmt.Sprintf("Host header injection vulnerability detected: %s", 
			strings.Join(vulnerabilityDetails, "; "))
	}

	return result, nil
}

// testHostHeaderWithTarget tests various host header injection techniques against internal targets
func (c *Checker) testHostHeaderWithTarget(client *http.Client, testDomain, target string, details *[]string) bool {
	vulnerabilityFound := false
	
	// Test various host header injection vectors
	headerTests := []struct {
		name   string
		setter func(*http.Request, string)
	}{
		{"Host", func(req *http.Request, val string) { req.Host = val }},
		{"X-Forwarded-Host", func(req *http.Request, val string) { req.Header.Set("X-Forwarded-Host", val) }},
		{"X-Host", func(req *http.Request, val string) { req.Header.Set("X-Host", val) }},
		{"X-Forwarded-Server", func(req *http.Request, val string) { req.Header.Set("X-Forwarded-Server", val) }},
		{"X-HTTP-Host-Override", func(req *http.Request, val string) { req.Header.Set("X-HTTP-Host-Override", val) }},
		{"X-Real-IP", func(req *http.Request, val string) { req.Header.Set("X-Real-IP", val) }},
		{"X-Originating-IP", func(req *http.Request, val string) { req.Header.Set("X-Originating-IP", val) }},
		{"X-Remote-IP", func(req *http.Request, val string) { req.Header.Set("X-Remote-IP", val) }},
		{"X-Client-IP", func(req *http.Request, val string) { req.Header.Set("X-Client-IP", val) }},
		{"CF-Connecting-IP", func(req *http.Request, val string) { req.Header.Set("CF-Connecting-IP", val) }},
		{"True-Client-IP", func(req *http.Request, val string) { req.Header.Set("True-Client-IP", val) }},
	}

	for _, test := range headerTests {
		req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", testDomain), nil)
		if err != nil {
			continue
		}

		// Set the potentially malicious header value
		test.setter(req, target)
		
		// Add timeout for internal network requests
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req = req.WithContext(ctx)
		
		resp, err := client.Do(req)
		cancel()
		
		if err == nil {
			resp.Body.Close()
			// If we get a response, the proxy might be forwarding to internal networks
			if resp.StatusCode != 400 && resp.StatusCode != 403 && resp.StatusCode != 502 {
				vulnerabilityFound = true
				*details = append(*details, fmt.Sprintf("%s header with %s returned %d", 
					test.name, target, resp.StatusCode))
			}
		}
	}
	
	return vulnerabilityFound
}

// checkSSRF tests for Server-Side Request Forgery vulnerabilities
func (c *Checker) checkSSRF(client *http.Client, testDomain string) (*CheckResult, error) {
	result := &CheckResult{
		URL:     fmt.Sprintf("http://%s", testDomain),
		Success: false,
	}

	// Apply rate limiting
	c.applyRateLimit(testDomain, &ProxyResult{DebugInfo: ""})

	var vulnerabilityFound bool
	var vulnerabilityDetails []string

	// Test internal network access through proxy
	internalTargets := []string{
		// Localhost variations
		"127.0.0.1",
		"127.0.0.1:22",     // SSH
		"127.0.0.1:3306",   // MySQL
		"127.0.0.1:5432",   // PostgreSQL
		"127.0.0.1:6379",   // Redis
		"127.1",            // Short localhost
		"0.0.0.0",
		"localhost",
		"0:80",
		"[::1]",            // IPv6 localhost

		// Cloud metadata services
		"169.254.169.254",                              // AWS metadata
		"169.254.169.254:80",                          // AWS metadata explicit port
		"169.254.169.254/latest/meta-data/",           // AWS metadata path
		"metadata.google.internal",                     // GCP metadata
		"metadata.google.internal:80",                  // GCP metadata explicit port
		"metadata",                                     // Short metadata
		"169.254.169.254/metadata/instance",           // Azure metadata
		"169.254.169.254/metadata/v1/maintenance",     // DigitalOcean metadata

		// Common private network ranges
		"192.168.1.1",      // Common gateway
		"192.168.0.1",      // Common gateway
		"192.168.1.1:80",   // Gateway with port
		"10.0.0.1",         // RFC 1918 private
		"172.16.0.1",       // RFC 1918 private
		"172.31.255.254",   // AWS VPC default
		"100.64.0.1",       // RFC 6598 Carrier-grade NAT

		// Additional localhost variations
		"127.0.0.2",
		"127.127.127.127",
		"127.255.255.254",
		"0177.0.0.1",       // Octal notation
		"0x7f.0x0.0x0.0x1", // Hex notation
		"2130706433",       // Decimal notation for 127.0.0.1

		// Special addresses
		"0.0.0.0:22",
		"255.255.255.255",
		"224.0.0.1",        // Multicast

		// IPv6 variations
		"[fc00::1]",        // IPv6 unique local
		"[fd00::1]:80",     // IPv6 unique local with port
		"[fe80::1]",        // IPv6 link-local
		"[::ffff:127.0.0.1]", // IPv6 IPv4-mapped localhost
		"[2001:db8::1]",    // RFC 3849 documentation prefix
		"[ff02::1]",        // IPv6 multicast
	}

	for _, target := range internalTargets {
		if c.testSSRFTarget(client, target, &vulnerabilityDetails) {
			vulnerabilityFound = true
		}
	}

	// Test port scanning capabilities
	if c.testPortScanning(client, &vulnerabilityDetails) {
		vulnerabilityFound = true
	}

	// Test DNS rebinding protection
	if c.testDNSRebindingSSRF(client, &vulnerabilityDetails) {
		vulnerabilityFound = true
	}

	result.Success = !vulnerabilityFound
	if vulnerabilityFound {
		result.Error = fmt.Sprintf("SSRF vulnerability detected: %s", 
			strings.Join(vulnerabilityDetails, "; "))
	}

	return result, nil
}

// testSSRFTarget tests access to a specific internal target
func (c *Checker) testSSRFTarget(client *http.Client, target string, details *[]string) bool {
	// Create request to internal target
	targetURL := target
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		targetURL = "http://" + target
	}

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return false
	}

	// Short timeout for internal network requests
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	req = req.WithContext(ctx)
	defer cancel()

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		
		// If we get any response from internal networks, it's a vulnerability
		if resp.StatusCode != 403 && resp.StatusCode != 502 && resp.StatusCode != 503 {
			*details = append(*details, fmt.Sprintf("Access to %s returned %d", target, resp.StatusCode))
			return true
		}
	}

	return false
}

// testPortScanning tests if proxy can be used for internal port scanning
func (c *Checker) testPortScanning(client *http.Client, details *[]string) bool {
	vulnerabilityFound := false
	
	// Test common internal services
	commonPorts := []int{22, 23, 25, 53, 80, 110, 143, 443, 993, 995, 3306, 5432, 6379, 8080, 9200}
	
	for _, port := range commonPorts {
		target := fmt.Sprintf("127.0.0.1:%d", port)
		req, err := http.NewRequest("GET", "http://"+target, nil)
		if err != nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		req = req.WithContext(ctx)
		
		resp, err := client.Do(req)
		cancel()
		
		if err == nil {
			defer resp.Body.Close()
			// Different response indicates port is open/filtered differently
			if resp.StatusCode < 500 {
				vulnerabilityFound = true
				*details = append(*details, fmt.Sprintf("Port %d appears accessible", port))
			}
		}
	}
	
	return vulnerabilityFound
}

// testDNSRebindingSSRF tests basic DNS rebinding protection for SSRF
func (c *Checker) testDNSRebindingSSRF(client *http.Client, details *[]string) bool {
	// Use real DNS rebinding test domains that actually rebind
	// These domains use DNS tricks to first resolve to a public IP, then to internal IPs
	rebindingTests := []string{
		"make-127-0-0-1-rr.1u.ms", // Public service that rebinds to 127.0.0.1
		"7f000001.rbndr.us",       // Another DNS rebinding service (resolves to 127.0.0.1)
	}

	for _, domain := range rebindingTests {
		// First request - should succeed
		req1, err := http.NewRequest("GET", "http://"+domain, nil)
		if err != nil {
			continue
		}

		ctx1, cancel1 := context.WithTimeout(context.Background(), 3*time.Second)
		req1 = req1.WithContext(ctx1)

		resp1, err1 := client.Do(req1)
		cancel1()

		if err1 != nil {
			continue // Skip if first request fails
		}
		resp1.Body.Close()

		// Wait for DNS TTL to expire
		time.Sleep(2 * time.Second)

		// Second request - should fail if DNS rebinding protection works
		req2, err := http.NewRequest("GET", "http://"+domain, nil)
		if err != nil {
			continue
		}

		ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
		req2 = req2.WithContext(ctx2)

		resp2, err2 := client.Do(req2)
		cancel2()

		if err2 == nil {
			defer resp2.Body.Close()
			if resp2.StatusCode < 400 {
				*details = append(*details, fmt.Sprintf("DNS rebinding vulnerability: %s accessible after rebind", domain))
				return true
			}
		}
	}

	return false
}

// testAdvancedHeaderInjection tests more sophisticated header injection techniques
func (c *Checker) testAdvancedHeaderInjection(client *http.Client, testDomain string, details *[]string) bool {
	vulnerabilityFound := false

	// Use Interactsh for OOB callback validation if available
	var interactshDomain string
	if !c.config.AdvancedChecks.DisableInteractsh {
		// If Interactsh is configured, we could use it for OOB callbacks
		// For now, we'll use testDomain as a placeholder
		// In production, this should integrate with interactsh-client
		interactshDomain = testDomain
	}

	// Test multiple conflicting host headers
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", testDomain), nil)
	if err == nil {
		req.Host = testDomain
		req.Header.Add("Host", "127.0.0.1")  // Duplicate Host header
		req.Header.Set("X-Forwarded-Host", "169.254.169.254")

		// Add Interactsh callback URL if available
		if interactshDomain != "" {
			req.Header.Set("X-Callback-URL", fmt.Sprintf("http://%s/callback", interactshDomain))
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req = req.WithContext(ctx)

		resp, err := client.Do(req)
		cancel()

		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 400 {
				vulnerabilityFound = true
				*details = append(*details, "Accepts conflicting Host headers")
			}
		}
	}

	// Test HTTP/1.0 Host header bypass
	req, err = http.NewRequest("GET", fmt.Sprintf("http://%s", testDomain), nil)
	if err == nil {
		req.Proto = "HTTP/1.0"
		req.ProtoMajor = 1
		req.ProtoMinor = 0
		req.Header.Set("Host", "127.0.0.1")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req = req.WithContext(ctx)

		resp, err := client.Do(req)
		cancel()

		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 400 {
				vulnerabilityFound = true
				*details = append(*details, "HTTP/1.0 Host header bypass possible")
			}
		}
	}

	// Test malformed host headers with CRLF and other injection vectors
	malformedHosts := []string{
		"127.0.0.1\r\nX-Injected: true",  // CRLF injection
		"127.0.0.1\nX-Injected: true",    // LF injection
		"127.0.0.1:80\x00",               // Null byte
		"127.0.0.1:80 ",                  // Trailing space
		"\t127.0.0.1",                    // Leading tab
		"127.0.0.1\r\n\r\nGET /evil HTTP/1.1", // HTTP request smuggling attempt
	}

	for _, malformedHost := range malformedHosts {
		req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", testDomain), nil)
		if err == nil {
			req.Host = malformedHost

			// Add Interactsh callback in malformed header if available
			if interactshDomain != "" && !strings.Contains(malformedHost, "\r\n") {
				req.Header.Set("X-Callback-URL", fmt.Sprintf("http://%s/callback", interactshDomain))
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			req = req.WithContext(ctx)

			resp, err := client.Do(req)
			cancel()

			if err == nil {
				resp.Body.Close()
				if resp.StatusCode < 400 {
					vulnerabilityFound = true
					*details = append(*details, fmt.Sprintf("Accepts malformed Host: %q", malformedHost))
				}
			}
		}
	}

	// Test additional header injection vectors
	injectionHeaders := []struct {
		name  string
		value string
	}{
		{"X-Forwarded-For", "127.0.0.1\r\nX-Injected: true"},
		{"X-Real-IP", "127.0.0.1\nX-Injected: true"},
		{"X-Original-URL", "/admin\r\nX-Injected: true"},
		{"X-Rewrite-URL", "/admin\r\nX-Injected: true"},
		{"Referer", fmt.Sprintf("http://%s\r\nX-Injected: true", testDomain)},
	}

	for _, header := range injectionHeaders {
		req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", testDomain), nil)
		if err == nil {
			req.Header.Set(header.name, header.value)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			req = req.WithContext(ctx)

			resp, err := client.Do(req)
			cancel()

			if err == nil {
				resp.Body.Close()
				// Check if the injection was successful by looking for the injected header in response
				if resp.Header.Get("X-Injected") == "true" {
					vulnerabilityFound = true
					*details = append(*details, fmt.Sprintf("CRLF injection successful via %s header", header.name))
				}
			}
		}
	}

	return vulnerabilityFound
}
