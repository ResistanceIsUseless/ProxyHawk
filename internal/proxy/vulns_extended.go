package proxy

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// ExtendedVulnResult contains results from extended/medium-priority vulnerability checks
type ExtendedVulnResult struct {
	// Nginx extended checks
	NginxVersionDetected       bool     `json:"nginx_version_detected"`
	NginxVersion               string   `json:"nginx_version,omitempty"`
	NginxConfigExposed         bool     `json:"nginx_config_exposed"`
	NginxConfigPaths           []string `json:"nginx_config_paths,omitempty"`
	NginxProxyCacheBypass      bool     `json:"nginx_proxy_cache_bypass"`
	NginxSubrequestAuthBypass  bool     `json:"nginx_subrequest_auth_bypass"`
	WebSocketAbuseVulnerable   bool     `json:"websocket_abuse_vulnerable"`
	WebSocketIssues            []string `json:"websocket_issues,omitempty"`

	// HTTP/2 checks
	HTTP2SmugglingVulnerable bool     `json:"http2_smuggling_vulnerable"`
	HTTP2SmugglingVectors    []string `json:"http2_smuggling_vectors,omitempty"`

	// Authentication checks
	ProxyAuthBypass          bool     `json:"proxy_auth_bypass"`
	ProxyAuthBypassMethods   []string `json:"proxy_auth_bypass_methods,omitempty"`

	// Apache extended checks
	ApacheServerStatusExposed bool     `json:"apache_server_status_exposed"`
	ServerStatusPath          string   `json:"server_status_path,omitempty"`
	CGIScriptExposed          bool     `json:"cgi_script_exposed"`
	CGIScriptPaths            []string `json:"cgi_script_paths,omitempty"`
	ApacheCVE_2019_10092      bool     `json:"apache_cve_2019_10092"` // XSS in error page
	ApacheModRewriteSSRF      bool     `json:"apache_mod_rewrite_ssrf"`
	ApacheHtaccessOverride    bool     `json:"apache_htaccess_override"`
}

// testNginxVersionDetection performs precise nginx version fingerprinting
func (c *Checker) testNginxVersionDetection(client *http.Client, result *ProxyResult) (bool, string) {
	if c.debug {
		result.DebugInfo += "[NGINX VERSION] Performing precise nginx version fingerprinting\n"
	}

	var detectedVersion string

	// Method 1: Server header
	req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return false, ""
	}
	req.Header.Set("User-Agent", c.config.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()

	serverHeader := resp.Header.Get("Server")
	if serverHeader != "" {
		// Match nginx version patterns: nginx/1.18.0, nginx/1.21.6
		versionRegex := regexp.MustCompile(`nginx/([\d.]+)`)
		if matches := versionRegex.FindStringSubmatch(serverHeader); len(matches) > 1 {
			detectedVersion = matches[1]
			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [INFO] Nginx version from Server header: %s\n", detectedVersion)
			}
			return true, detectedVersion
		}
	}

	// Method 2: Error page version disclosure
	errorReq, err := http.NewRequest("GET", c.config.ValidationURL+"/nonexistent-"+fmt.Sprintf("%d", 12345), nil)
	if err != nil {
		return false, ""
	}
	errorReq.Header.Set("User-Agent", c.config.UserAgent)

	errorResp, err := client.Do(errorReq)
	if err != nil {
		return false, ""
	}

	body, _ := io.ReadAll(errorResp.Body)
	errorResp.Body.Close()

	bodyStr := string(body)

	// Check for version in error page HTML
	// Pattern: <center>nginx/1.18.0</center>
	htmlVersionRegex := regexp.MustCompile(`nginx/([\d.]+)`)
	if matches := htmlVersionRegex.FindStringSubmatch(bodyStr); len(matches) > 1 {
		detectedVersion = matches[1]
		if c.debug {
			result.DebugInfo += fmt.Sprintf("  [INFO] Nginx version from error page: %s\n", detectedVersion)
		}
		return true, detectedVersion
	}

	// Method 3: Specific error behavior fingerprinting (version-specific responses)
	// Different nginx versions handle certain requests differently
	testReq, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return false, ""
	}
	testReq.Header.Set("User-Agent", c.config.UserAgent)
	testReq.Header.Set("Host", "test\r\nX-Injected: true")

	testResp, err := client.Do(testReq)
	if err == nil {
		defer testResp.Body.Close()
		// Nginx < 1.20.0 had different behavior with malformed headers
		if testResp.StatusCode == 400 {
			if c.debug {
				result.DebugInfo += "  [INFO] Nginx likely version 1.20.0+ (improved header validation)\n"
			}
		}
	}

	return detectedVersion != "", detectedVersion
}

// testNginxConfigExposure tests for exposed nginx configuration files
func (c *Checker) testNginxConfigExposure(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[NGINX CONFIG] Testing for exposed nginx configuration files\n"
	}

	exposedPaths := []string{}

	configPaths := []string{
		"/nginx.conf",
		"/conf/nginx.conf",
		"/etc/nginx/nginx.conf",
		"/usr/local/nginx/conf/nginx.conf",
		"/etc/nginx/sites-enabled/default",
		"/etc/nginx/sites-available/default",
		"/etc/nginx/conf.d/default.conf",
		"/../nginx.conf",
		"/../conf/nginx.conf",
		"/../etc/nginx/nginx.conf",
		"/nginx/nginx.conf",
		"/config/nginx.conf",
	}

	for _, path := range configPaths {
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

		if resp.StatusCode == 200 {
			bodyStr := strings.ToLower(string(body))

			// Check for nginx config indicators
			if strings.Contains(bodyStr, "server {") || strings.Contains(bodyStr, "http {") ||
				strings.Contains(bodyStr, "location") || strings.Contains(bodyStr, "proxy_pass") ||
				strings.Contains(bodyStr, "listen") || strings.Contains(bodyStr, "server_name") {
				exposedPaths = append(exposedPaths, path)
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] Nginx configuration exposed at: %s\n", path)
				}
			}
		}
	}

	return len(exposedPaths) > 0, exposedPaths
}

// testWebSocketAbuseVulnerabilities tests for WebSocket protocol upgrade abuse
func (c *Checker) testWebSocketAbuseVulnerabilities(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[WEBSOCKET] Testing for WebSocket upgrade abuse vulnerabilities\n"
	}

	issues := []string{}

	// Test 1: WebSocket upgrade without proper origin validation
	wsReq, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return false, nil
	}

	wsReq.Header.Set("User-Agent", c.config.UserAgent)
	wsReq.Header.Set("Upgrade", "websocket")
	wsReq.Header.Set("Connection", "Upgrade")
	wsReq.Header.Set("Sec-WebSocket-Version", "13")
	wsReq.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	wsReq.Header.Set("Origin", "http://evil.example.com")

	wsResp, err := client.Do(wsReq)
	if err == nil {
		defer wsResp.Body.Close()

		// Check if upgrade was accepted from evil origin
		if wsResp.StatusCode == 101 {
			issues = append(issues, "WebSocket upgrade accepted from arbitrary origin")
			if c.debug {
				result.DebugInfo += "  [HIGH] WebSocket upgrade lacks origin validation\n"
			}
		}
	}

	// Test 2: WebSocket upgrade with protocol smuggling
	smuggleReq, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return len(issues) > 0, issues
	}

	smuggleReq.Header.Set("User-Agent", c.config.UserAgent)
	smuggleReq.Header.Set("Upgrade", "websocket")
	smuggleReq.Header.Set("Connection", "Upgrade, Keep-Alive")
	smuggleReq.Header.Set("Sec-WebSocket-Version", "13")
	smuggleReq.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	smuggleReq.Header.Set("Content-Length", "100")

	smuggleResp, err := client.Do(smuggleReq)
	if err == nil {
		defer smuggleResp.Body.Close()

		if smuggleResp.StatusCode == 101 {
			issues = append(issues, "WebSocket upgrade with Content-Length (potential smuggling)")
			if c.debug {
				result.DebugInfo += "  [HIGH] WebSocket upgrade accepts Content-Length header\n"
			}
		}
	}

	// Test 3: Malformed WebSocket upgrade
	malformedReq, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return len(issues) > 0, issues
	}

	malformedReq.Header.Set("User-Agent", c.config.UserAgent)
	malformedReq.Header.Set("Upgrade", "websocket, http/2.0")
	malformedReq.Header.Set("Connection", "Upgrade")

	malformedResp, err := client.Do(malformedReq)
	if err == nil {
		defer malformedResp.Body.Close()

		if malformedResp.StatusCode < 400 {
			issues = append(issues, "Malformed WebSocket upgrade accepted")
			if c.debug {
				result.DebugInfo += "  [MEDIUM] Accepts malformed WebSocket upgrade headers\n"
			}
		}
	}

	// Test 4: Cross-site WebSocket hijacking via Origin manipulation
	hijackReq, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return len(issues) > 0, issues
	}

	hijackReq.Header.Set("User-Agent", c.config.UserAgent)
	hijackReq.Header.Set("Upgrade", "websocket")
	hijackReq.Header.Set("Connection", "Upgrade")
	hijackReq.Header.Set("Sec-WebSocket-Version", "13")
	hijackReq.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	hijackReq.Header.Set("Origin", "null")

	hijackResp, err := client.Do(hijackReq)
	if err == nil {
		defer hijackResp.Body.Close()

		if hijackResp.StatusCode == 101 {
			issues = append(issues, "WebSocket upgrade accepts 'null' origin (CSWSH vulnerability)")
			if c.debug {
				result.DebugInfo += "  [HIGH] Cross-Site WebSocket Hijacking possible (null origin)\n"
			}
		}
	}

	return len(issues) > 0, issues
}

// testHTTP2RequestSmuggling tests for HTTP/2 request smuggling vulnerabilities
func (c *Checker) testHTTP2RequestSmuggling(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[HTTP/2 SMUGGLING] Testing for HTTP/2 request smuggling vulnerabilities\n"
	}

	vectors := []string{}

	// Test 1: Content-Length vs Transfer-Encoding in HTTP/2 downgrade
	// HTTP/2 doesn't support Transfer-Encoding, but downgrade to HTTP/1.1 might be vulnerable
	req1, err := http.NewRequest("POST", c.config.ValidationURL, strings.NewReader("malicious=payload"))
	if err != nil {
		return false, nil
	}

	req1.Header.Set("User-Agent", c.config.UserAgent)
	req1.Header.Set("Content-Length", "17")
	req1.Header.Set("Transfer-Encoding", "chunked")

	resp1, err := client.Do(req1)
	if err == nil {
		defer resp1.Body.Close()

		// If accepted, might be vulnerable during HTTP/2 to HTTP/1.1 downgrade
		if resp1.StatusCode < 500 {
			vectors = append(vectors, "Accepts both Content-Length and Transfer-Encoding")
			if c.debug {
				result.DebugInfo += "  [HIGH] Potential HTTP/2 CL-TE smuggling on downgrade\n"
			}
		}
	}

	// Test 2: HTTP/2 pseudo-headers injection
	req2, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return len(vectors) > 0, vectors
	}

	req2.Header.Set("User-Agent", c.config.UserAgent)
	req2.Header.Set(":path", "/admin")
	req2.Header.Set(":method", "DELETE")

	resp2, err := client.Do(req2)
	if err == nil {
		defer resp2.Body.Close()

		if resp2.StatusCode < 500 {
			vectors = append(vectors, "Accepts HTTP/2 pseudo-headers in HTTP/1.1")
			if c.debug {
				result.DebugInfo += "  [MEDIUM] HTTP/2 pseudo-header injection possible\n"
			}
		}
	}

	// Test 3: CRLF injection via HTTP/2 binary headers
	// HTTP/2 allows binary headers, which might bypass CRLF sanitization
	req3, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return len(vectors) > 0, vectors
	}

	req3.Header.Set("User-Agent", c.config.UserAgent)
	req3.Header.Set("X-Custom", "value\r\nX-Injected: true")

	resp3, err := client.Do(req3)
	if err == nil {
		defer resp3.Body.Close()

		// Check if injected header appears in response
		if resp3.Header.Get("X-Injected") == "true" {
			vectors = append(vectors, "CRLF injection via HTTP/2 headers")
			if c.debug {
				result.DebugInfo += "  [CRITICAL] CRLF injection successful via HTTP/2\n"
			}
		}
	}

	// Test 4: HTTP/2 connection coalescing abuse
	req4, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return len(vectors) > 0, vectors
	}

	req4.Header.Set("User-Agent", c.config.UserAgent)
	req4.Host = "evil.example.com"

	resp4, err := client.Do(req4)
	if err == nil {
		defer resp4.Body.Close()

		if resp4.StatusCode == 200 {
			vectors = append(vectors, "HTTP/2 connection coalescing with arbitrary Host")
			if c.debug {
				result.DebugInfo += "  [HIGH] HTTP/2 connection reuse for different hosts\n"
			}
		}
	}

	return len(vectors) > 0, vectors
}

// testProxyAuthenticationBypass tests for proxy authentication bypass vulnerabilities
func (c *Checker) testProxyAuthenticationBypass(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[PROXY AUTH] Testing for proxy authentication bypass vulnerabilities\n"
	}

	bypassMethods := []string{}

	// Test 1: Empty/malformed Proxy-Authorization header
	req1, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return false, nil
	}

	req1.Header.Set("User-Agent", c.config.UserAgent)
	req1.Header.Set("Proxy-Authorization", "")

	resp1, err := client.Do(req1)
	if err == nil {
		defer resp1.Body.Close()

		if resp1.StatusCode != 407 && resp1.StatusCode < 400 {
			bypassMethods = append(bypassMethods, "Empty Proxy-Authorization header bypasses authentication")
			if c.debug {
				result.DebugInfo += "  [CRITICAL] Empty Proxy-Authorization bypasses auth\n"
			}
		}
	}

	// Test 2: Malformed Basic auth
	req2, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return len(bypassMethods) > 0, bypassMethods
	}

	req2.Header.Set("User-Agent", c.config.UserAgent)
	req2.Header.Set("Proxy-Authorization", "Basic invalid")

	resp2, err := client.Do(req2)
	if err == nil {
		defer resp2.Body.Close()

		if resp2.StatusCode != 407 && resp2.StatusCode < 400 {
			bypassMethods = append(bypassMethods, "Malformed Basic auth bypasses authentication")
			if c.debug {
				result.DebugInfo += "  [CRITICAL] Malformed Basic auth accepted\n"
			}
		}
	}

	// Test 3: SQL injection in credentials
	sqlPayload := "admin' OR '1'='1"
	encodedPayload := base64.StdEncoding.EncodeToString([]byte(sqlPayload + ":password"))

	req3, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return len(bypassMethods) > 0, bypassMethods
	}

	req3.Header.Set("User-Agent", c.config.UserAgent)
	req3.Header.Set("Proxy-Authorization", "Basic "+encodedPayload)

	resp3, err := client.Do(req3)
	if err == nil {
		defer resp3.Body.Close()

		if resp3.StatusCode != 407 && resp3.StatusCode < 400 {
			bypassMethods = append(bypassMethods, "SQL injection in Proxy-Authorization credentials")
			if c.debug {
				result.DebugInfo += "  [CRITICAL] SQL injection in auth credentials\n"
			}
		}
	}

	// Test 4: Multiple Proxy-Authorization headers
	req4, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return len(bypassMethods) > 0, bypassMethods
	}

	req4.Header.Set("User-Agent", c.config.UserAgent)
	req4.Header.Add("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("user1:pass1")))
	req4.Header.Add("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:admin")))

	resp4, err := client.Do(req4)
	if err == nil {
		defer resp4.Body.Close()

		if resp4.StatusCode != 407 && resp4.StatusCode < 400 {
			bypassMethods = append(bypassMethods, "Multiple Proxy-Authorization headers cause confusion")
			if c.debug {
				result.DebugInfo += "  [HIGH] Multiple auth headers accepted\n"
			}
		}
	}

	// Test 5: Proxy-Connection header bypass
	req5, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return len(bypassMethods) > 0, bypassMethods
	}

	req5.Header.Set("User-Agent", c.config.UserAgent)
	req5.Header.Set("Proxy-Connection", "keep-alive")

	resp5, err := client.Do(req5)
	if err == nil {
		defer resp5.Body.Close()

		if resp5.StatusCode != 407 && resp5.StatusCode < 400 {
			bypassMethods = append(bypassMethods, "Proxy-Connection header bypasses authentication")
			if c.debug {
				result.DebugInfo += "  [HIGH] Proxy-Connection header bypass\n"
			}
		}
	}

	return len(bypassMethods) > 0, bypassMethods
}

// testApacheServerStatus tests for exposed Apache server-status page
func (c *Checker) testApacheServerStatus(client *http.Client, result *ProxyResult) (bool, string) {
	if c.debug {
		result.DebugInfo += "[APACHE STATUS] Testing for exposed Apache server-status page\n"
	}

	statusPaths := []string{
		"/server-status",
		"/server-status?auto",
		"/server-status?refresh=5",
		"/status",
		"/apache-status",
		"/server-info",
	}

	for _, path := range statusPaths {
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

		if resp.StatusCode == 200 {
			bodyStr := strings.ToLower(string(body))

			// Check for Apache server-status indicators
			if strings.Contains(bodyStr, "apache server status") ||
				strings.Contains(bodyStr, "server version") ||
				strings.Contains(bodyStr, "server mpm") ||
				strings.Contains(bodyStr, "current time") ||
				strings.Contains(bodyStr, "restart time") ||
				strings.Contains(bodyStr, "scoreboard") {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [MEDIUM] Apache server-status exposed at: %s\n", path)
				}
				return true, path
			}
		}
	}

	return false, ""
}

// testCGIScriptExposure tests for exposed CGI scripts
func (c *Checker) testCGIScriptExposure(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[CGI SCRIPTS] Testing for exposed CGI scripts\n"
	}

	exposedScripts := []string{}

	cgiPaths := []string{
		"/cgi-bin/test",
		"/cgi-bin/printenv",
		"/cgi-bin/test-cgi",
		"/cgi-bin/php",
		"/cgi-bin/php5",
		"/cgi-bin/php-cgi",
		"/cgi-bin/status",
		"/cgi-bin/test.cgi",
		"/cgi-bin/test.sh",
		"/cgi-bin/test.pl",
		"/cgi-bin/test.py",
		"/cgi-bin/admin.cgi",
	}

	for _, path := range cgiPaths {
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

		if resp.StatusCode == 200 {
			bodyStr := strings.ToLower(string(body))

			// Check for CGI script indicators
			if strings.Contains(bodyStr, "content-type:") ||
				strings.Contains(bodyStr, "cgi") ||
				strings.Contains(bodyStr, "environment") ||
				strings.Contains(bodyStr, "server_software") ||
				strings.Contains(bodyStr, "script_name") {
				exposedScripts = append(exposedScripts, path)
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [HIGH] CGI script accessible at: %s\n", path)
				}
			}
		}
	}

	return len(exposedScripts) > 0, exposedScripts
}

// testNginxProxyCacheBypass tests for nginx proxy cache key manipulation
func (c *Checker) testNginxProxyCacheBypass(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[NGINX CACHE] Testing for proxy cache bypass via key manipulation\n"
	}

	// Test 1: Cache key manipulation via Vary header
	baselineReq, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return false
	}
	baselineReq.Header.Set("User-Agent", c.config.UserAgent)

	baselineResp, err := client.Do(baselineReq)
	if err != nil {
		return false
	}
	baselineBody, _ := io.ReadAll(baselineResp.Body)
	baselineResp.Body.Close()

	// Test with cache-busting headers that might not be in cache key
	testReq, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return false
	}
	testReq.Header.Set("User-Agent", c.config.UserAgent)
	testReq.Header.Set("X-Cache-Buster", "test123")
	testReq.Header.Set("Range", "bytes=0-0")

	testResp, err := client.Do(testReq)
	if err != nil {
		return false
	}
	testBody, _ := io.ReadAll(testResp.Body)
	testResp.Body.Close()

	// If we get different content, cache might be bypassable
	if len(baselineBody) != len(testBody) {
		if c.debug {
			result.DebugInfo += "  [MEDIUM] Nginx cache bypass possible via unkeyed headers\n"
		}
		return true
	}

	// Test 2: Query string vs no query string (cache key manipulation)
	queryReq, _ := http.NewRequest("GET", c.config.ValidationURL+"?nocache=1", nil)
	queryReq.Header.Set("User-Agent", c.config.UserAgent)

	queryResp, err := client.Do(queryReq)
	if err == nil {
		defer queryResp.Body.Close()
		// Check for cache status headers
		xCacheStatus := queryResp.Header.Get("X-Cache-Status")
		if xCacheStatus == "BYPASS" || xCacheStatus == "MISS" {
			if c.debug {
				result.DebugInfo += "  [MEDIUM] Nginx cache bypass via query string manipulation\n"
			}
			return true
		}
	}

	return false
}

// testNginxSubrequestAuthBypass tests for nginx auth_request module bypass
func (c *Checker) testNginxSubrequestAuthBypass(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[NGINX AUTH] Testing for auth_request subrequest bypass\n"
	}

	protectedPaths := []string{
		"/admin",
		"/api/admin",
		"/protected",
		"/internal",
		"/management",
	}

	for _, path := range protectedPaths {
		// Test 1: Original request with X-Original-URI manipulation
		req1, err := http.NewRequest("GET", c.config.ValidationURL+path, nil)
		if err != nil {
			continue
		}
		req1.Header.Set("User-Agent", c.config.UserAgent)
		req1.Header.Set("X-Original-URI", "/public")
		req1.Header.Set("X-Original-URL", "/public")

		resp1, err := client.Do(req1)
		if err != nil {
			continue
		}
		resp1.Body.Close()

		if resp1.StatusCode == 200 {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [HIGH] Auth bypass via X-Original-URI at: %s\n", path)
			}
			return true
		}

		// Test 2: Request with subrequest error codes
		req2, _ := http.NewRequest("GET", c.config.ValidationURL+path, nil)
		req2.Header.Set("User-Agent", c.config.UserAgent)
		req2.Header.Set("X-Accel-Redirect", "/public")

		resp2, err := client.Do(req2)
		if err == nil {
			resp2.Body.Close()
			if resp2.StatusCode == 200 {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [HIGH] Auth bypass via X-Accel-Redirect at: %s\n", path)
				}
				return true
			}
		}
	}

	return false
}

// testApacheCVE_2019_10092 tests for XSS in Apache mod_proxy error page
func (c *Checker) testApacheCVE_2019_10092(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[APACHE CVE-2019-10092] Testing for XSS in mod_proxy error page\n"
	}

	// CVE-2019-10092: XSS in error pages when using mod_proxy_ftp
	xssPayload := "<script>alert(1)</script>"
	testURL := c.config.ValidationURL + "/ftp://test" + xssPayload

	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", c.config.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return false
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	bodyStr := string(body)

	// Check if XSS payload is reflected without encoding
	if strings.Contains(bodyStr, xssPayload) && !strings.Contains(bodyStr, "&lt;script&gt;") {
		if c.debug {
			result.DebugInfo += "  [MEDIUM] XSS in Apache mod_proxy error page (CVE-2019-10092)\n"
		}
		return true
	}

	return false
}

// testApacheModRewriteSSRF tests for mod_rewrite-based SSRF
func (c *Checker) testApacheModRewriteSSRF(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[APACHE MOD_REWRITE] Testing for SSRF via RewriteRule\n"
	}

	// Test various mod_rewrite SSRF vectors
	ssrfVectors := []string{
		"http://127.0.0.1:22",
		"http://localhost:3306",
		"http://169.254.169.254/latest/meta-data/",
		"http://metadata.google.internal/computeMetadata/v1/",
		"file:///etc/passwd",
		"gopher://127.0.0.1:6379/_",
	}

	for _, vector := range ssrfVectors {
		// Try to trigger SSRF via various parameters
		testPaths := []string{
			"/?url=" + vector,
			"/?redirect=" + vector,
			"/?proxy=" + vector,
			"/proxy?url=" + vector,
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

			// Check for SSRF indicators
			if strings.Contains(bodyStr, "ssh-") || // SSH banner
				strings.Contains(bodyStr, "mysql") || // MySQL
				strings.Contains(bodyStr, "instance-id") || // AWS metadata
				strings.Contains(bodyStr, "root:x:0:0") { // /etc/passwd
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] mod_rewrite SSRF to: %s\n", vector)
				}
				return true
			}
		}
	}

	return false
}

// testApacheHtaccessOverride tests for htaccess directory access control bypass
func (c *Checker) testApacheHtaccessOverride(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[APACHE HTACCESS] Testing for .htaccess override bypass\n"
	}

	sensitivePaths := []string{
		"/.htaccess",
		"/.htpasswd",
		"/admin/.htaccess",
		"/wp-admin/.htaccess",
		"/config/.htaccess",
	}

	for _, path := range sensitivePaths {
		// Test 1: Direct access
		req1, err := http.NewRequest("GET", c.config.ValidationURL+path, nil)
		if err != nil {
			continue
		}
		req1.Header.Set("User-Agent", c.config.UserAgent)

		resp1, err := client.Do(req1)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp1.Body)
		resp1.Body.Close()

		if resp1.StatusCode == 200 {
			bodyStr := strings.ToLower(string(body))
			// Check for .htaccess content indicators
			if strings.Contains(bodyStr, "authtype") ||
				strings.Contains(bodyStr, "require valid-user") ||
				strings.Contains(bodyStr, "deny from") ||
				strings.Contains(bodyStr, "rewriterule") {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [HIGH] .htaccess file exposed: %s\n", path)
				}
				return true
			}
		}

		// Test 2: Bypass via encoding
		encodedPath := strings.ReplaceAll(path, "/", "%2f")
		req2, _ := http.NewRequest("GET", c.config.ValidationURL+encodedPath, nil)
		req2.Header.Set("User-Agent", c.config.UserAgent)

		resp2, err := client.Do(req2)
		if err == nil {
			body2, _ := io.ReadAll(resp2.Body)
			resp2.Body.Close()

			if resp2.StatusCode == 200 && len(body2) > 0 {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [HIGH] .htaccess bypass via encoding: %s\n", path)
				}
				return true
			}
		}
	}

	return false
}

// performExtendedVulnerabilityChecks runs all extended/medium-priority vulnerability checks
func (c *Checker) performExtendedVulnerabilityChecks(client *http.Client, result *ProxyResult) *ExtendedVulnResult {
	extendedResult := &ExtendedVulnResult{}

	// Nginx extended checks
	extendedResult.NginxVersionDetected, extendedResult.NginxVersion = c.testNginxVersionDetection(client, result)
	extendedResult.NginxConfigExposed, extendedResult.NginxConfigPaths = c.testNginxConfigExposure(client, result)
	extendedResult.NginxProxyCacheBypass = c.testNginxProxyCacheBypass(client, result)
	extendedResult.NginxSubrequestAuthBypass = c.testNginxSubrequestAuthBypass(client, result)

	// WebSocket checks
	extendedResult.WebSocketAbuseVulnerable, extendedResult.WebSocketIssues = c.testWebSocketAbuseVulnerabilities(client, result)

	// HTTP/2 checks
	extendedResult.HTTP2SmugglingVulnerable, extendedResult.HTTP2SmugglingVectors = c.testHTTP2RequestSmuggling(client, result)

	// Authentication checks
	extendedResult.ProxyAuthBypass, extendedResult.ProxyAuthBypassMethods = c.testProxyAuthenticationBypass(client, result)

	// Apache extended checks
	extendedResult.ApacheServerStatusExposed, extendedResult.ServerStatusPath = c.testApacheServerStatus(client, result)
	extendedResult.CGIScriptExposed, extendedResult.CGIScriptPaths = c.testCGIScriptExposure(client, result)
	extendedResult.ApacheCVE_2019_10092 = c.testApacheCVE_2019_10092(client, result)
	extendedResult.ApacheModRewriteSSRF = c.testApacheModRewriteSSRF(client, result)
	extendedResult.ApacheHtaccessOverride = c.testApacheHtaccessOverride(client, result)

	return extendedResult
}
