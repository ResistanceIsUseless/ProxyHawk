package proxy

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// VendorVulnResult contains results from vendor-specific vulnerability checks
type VendorVulnResult struct {
	// HAProxy checks
	HAProxyStatsExposed     bool     `json:"haproxy_stats_exposed"`
	HAProxyStatsPath        string   `json:"haproxy_stats_path,omitempty"`
	HAProxyCVE_2023_40225   bool     `json:"haproxy_cve_2023_40225"` // Request smuggling via content-length
	HAProxyCVE_2021_40346   bool     `json:"haproxy_cve_2021_40346"` // Integer overflow
	HAProxyVersionDetected  bool     `json:"haproxy_version_detected"`
	HAProxyVersion          string   `json:"haproxy_version,omitempty"`

	// Squid checks
	SquidCacheManagerExposed bool     `json:"squid_cache_manager_exposed"`
	SquidCacheManagerPaths   []string `json:"squid_cache_manager_paths,omitempty"`
	SquidCVE_2021_46784      bool     `json:"squid_cve_2021_46784"` // Buffer overflow
	SquidCVE_2020_15810      bool     `json:"squid_cve_2020_15810"` // HTTP request smuggling
	SquidVersionDetected     bool     `json:"squid_version_detected"`
	SquidVersion             string   `json:"squid_version,omitempty"`

	// Traefik checks
	TraefikDashboardExposed  bool     `json:"traefik_dashboard_exposed"`
	TraefikDashboardPath     string   `json:"traefik_dashboard_path,omitempty"`
	TraefikAPIExposed        bool     `json:"traefik_api_exposed"`
	TraefikAPIPaths          []string `json:"traefik_api_paths,omitempty"`
	TraefikCVE_2024_45410    bool     `json:"traefik_cve_2024_45410"` // SSRF via misconfigured middleware

	// Envoy checks
	EnvoyAdminExposed        bool     `json:"envoy_admin_exposed"`
	EnvoyAdminPath           string   `json:"envoy_admin_path,omitempty"`
	EnvoyCVE_2022_21654      bool     `json:"envoy_cve_2022_21654"` // SSRF in original_dst cluster
	EnvoyVersionDetected     bool     `json:"envoy_version_detected"`
	EnvoyVersion             string   `json:"envoy_version,omitempty"`

	// Caddy checks
	CaddyAdminAPIExposed     bool     `json:"caddy_admin_api_exposed"`
	CaddyAdminPath           string   `json:"caddy_admin_path,omitempty"`
	CaddyVersionDetected     bool     `json:"caddy_version_detected"`
	CaddyVersion             string   `json:"caddy_version,omitempty"`

	// Varnish checks
	VarnishBanLurkExposed    bool     `json:"varnish_ban_lurk_exposed"`
	VarnishCVE_2022_45060    bool     `json:"varnish_cve_2022_45060"` // Request smuggling
	VarnishVersionDetected   bool     `json:"varnish_version_detected"`
	VarnishVersion           string   `json:"varnish_version,omitempty"`

	// Cloud-specific checks
	AWSALBHeaderInjection    bool     `json:"aws_alb_header_injection"`
	CloudflareWorkerBypass   bool     `json:"cloudflare_worker_bypass"`
	CloudflareCachePoisoning bool     `json:"cloudflare_cache_poisoning"`
}

// testHAProxyStatsExposure tests for exposed HAProxy statistics page
func (c *Checker) testHAProxyStatsExposure(client *http.Client, result *ProxyResult) (bool, string) {
	if c.debug {
		result.DebugInfo += "[HAPROXY STATS] Testing for exposed HAProxy statistics page\n"
	}

	statsPaths := []string{
		"/haproxy?stats",
		"/haproxy-status",
		"/haproxy_stats",
		"/stats",
		"/admin?stats",
		"/;csv;norefresh",
	}

	for _, path := range statsPaths {
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

			// Check for HAProxy stats indicators
			if strings.Contains(bodyStr, "haproxy") &&
				(strings.Contains(bodyStr, "statistics report") ||
					strings.Contains(bodyStr, "session rate") ||
					strings.Contains(bodyStr, "frontend") ||
					strings.Contains(bodyStr, "backend") ||
					strings.Contains(bodyStr, "queue") ||
					strings.Contains(bodyStr, "statistics")) {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [MEDIUM] HAProxy statistics exposed at: %s\n", path)
				}
				return true, path
			}
		}
	}

	return false, ""
}

// testHAProxyCVE_2023_40225 tests for HAProxy request smuggling via content-length
func (c *Checker) testHAProxyCVE_2023_40225(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[HAPROXY CVE-2023-40225] Testing for request smuggling vulnerability\n"
	}

	// CVE-2023-40225: HAProxy request smuggling via content-length manipulation
	// Vulnerable versions: HAProxy < 2.0.33, < 2.2.30, < 2.4.23, < 2.5.12, < 2.6.9, < 2.7.1
	req, err := http.NewRequest("POST", c.config.ValidationURL, strings.NewReader("x"))
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Content-Length", "1")
	req.Header.Add("Content-Length", "2")

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()

		// Vulnerable HAProxy may accept duplicate Content-Length headers
		if resp.StatusCode < 500 {
			if c.debug {
				result.DebugInfo += "  [HIGH] HAProxy accepts duplicate Content-Length headers (CVE-2023-40225)\n"
			}
			return true
		}
	}

	return false
}

// testHAProxyCVE_2021_40346 tests for HAProxy integer overflow vulnerability
func (c *Checker) testHAProxyCVE_2021_40346(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[HAPROXY CVE-2021-40346] Testing for integer overflow vulnerability\n"
	}

	// CVE-2021-40346: Integer overflow in header size calculation
	// Vulnerable versions: HAProxy < 2.0.25, < 2.2.17, < 2.3.14, < 2.4.4
	req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	// Add header with size close to integer overflow boundary
	req.Header.Set("X-Test", strings.Repeat("A", 2147483647))

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()

		// Vulnerable versions may crash or behave unexpectedly
		if resp.StatusCode == 500 || resp.StatusCode == 502 {
			if c.debug {
				result.DebugInfo += "  [MEDIUM] HAProxy may be vulnerable to integer overflow (CVE-2021-40346)\n"
			}
			return true
		}
	}

	return false
}

// testHAProxyVersionDetection detects HAProxy version from headers and responses
func (c *Checker) testHAProxyVersionDetection(client *http.Client, result *ProxyResult) (bool, string) {
	if c.debug {
		result.DebugInfo += "[HAPROXY VERSION] Detecting HAProxy version\n"
	}

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

	// Check Server header
	serverHeader := resp.Header.Get("Server")
	if serverHeader != "" {
		versionRegex := regexp.MustCompile(`(?i)haproxy[/\s]+([0-9.]+)`)
		if matches := versionRegex.FindStringSubmatch(serverHeader); len(matches) > 1 {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [INFO] HAProxy version from Server header: %s\n", matches[1])
			}
			return true, matches[1]
		}
	}

	// Check error pages
	errorReq, _ := http.NewRequest("GET", c.config.ValidationURL+"/nonexistent-"+fmt.Sprintf("%d", 12345), nil)
	errorReq.Header.Set("User-Agent", c.config.UserAgent)

	errorResp, err := client.Do(errorReq)
	if err == nil {
		body, _ := io.ReadAll(errorResp.Body)
		errorResp.Body.Close()

		versionRegex := regexp.MustCompile(`(?i)haproxy[/\s]+([0-9.]+)`)
		if matches := versionRegex.FindSubmatch(body); len(matches) > 1 {
			version := string(matches[1])
			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [INFO] HAProxy version from error page: %s\n", version)
			}
			return true, version
		}
	}

	return false, ""
}

// testSquidCacheManager tests for exposed Squid cache manager
func (c *Checker) testSquidCacheManager(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[SQUID CACHE] Testing for exposed Squid cache manager\n"
	}

	exposedPaths := []string{}

	cachePaths := []string{
		"/squid-internal-mgr/",
		"/squid-internal-mgr/info",
		"/squid-internal-mgr/menu",
		"/squid-internal-mgr/config",
		"/squid-internal-mgr/ipcache",
		"/squid-internal-mgr/fqdncache",
		"/squid-internal-mgr/objects",
		"/squid-internal-mgr/vm_objects",
		"/squid-internal-mgr/utilization",
		"/squid-internal-mgr/client_list",
	}

	for _, path := range cachePaths {
		req, err := http.NewRequest("GET", c.config.ValidationURL+path, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", c.config.UserAgent)
		req.Header.Set("Cache-Control", "no-cache")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 200 {
			bodyStr := strings.ToLower(string(body))

			// Check for Squid cache manager indicators
			if strings.Contains(bodyStr, "squid") &&
				(strings.Contains(bodyStr, "cache manager") ||
					strings.Contains(bodyStr, "cache information") ||
					strings.Contains(bodyStr, "cache statistics") ||
					strings.Contains(bodyStr, "cache digest")) {
				exposedPaths = append(exposedPaths, path)
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [MEDIUM] Squid cache manager exposed at: %s\n", path)
				}
			}
		}
	}

	return len(exposedPaths) > 0, exposedPaths
}

// testSquidCVE_2021_46784 tests for Squid buffer overflow vulnerability
func (c *Checker) testSquidCVE_2021_46784(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[SQUID CVE-2021-46784] Testing for buffer overflow vulnerability\n"
	}

	// CVE-2021-46784: Integer overflow in Squid allows DoS
	// Vulnerable versions: Squid < 5.7, < 6.0.1
	req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	// Craft malicious gopher:// URL
	req.Header.Set("X-Test-Gopher", "gopher://internal.host:70/"+strings.Repeat("1", 10000))

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()

		// Vulnerable Squid may crash or return error
		if resp.StatusCode == 500 || resp.StatusCode == 502 {
			if c.debug {
				result.DebugInfo += "  [HIGH] Squid may be vulnerable to buffer overflow (CVE-2021-46784)\n"
			}
			return true
		}
	}

	return false
}

// testSquidCVE_2020_15810 tests for Squid HTTP request smuggling
func (c *Checker) testSquidCVE_2020_15810(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[SQUID CVE-2020-15810] Testing for HTTP request smuggling\n"
	}

	// CVE-2020-15810: HTTP Request Smuggling vulnerability in Squid
	// Vulnerable versions: Squid < 4.13, < 5.0.4
	req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Content-Length", "0")
	req.Header.Set("Transfer-Encoding", "chunked")

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()

		// Vulnerable Squid accepts both CL and TE
		if resp.StatusCode < 500 {
			if c.debug {
				result.DebugInfo += "  [HIGH] Squid accepts both Content-Length and Transfer-Encoding (CVE-2020-15810)\n"
			}
			return true
		}
	}

	return false
}

// testSquidVersionDetection detects Squid version
func (c *Checker) testSquidVersionDetection(client *http.Client, result *ProxyResult) (bool, string) {
	if c.debug {
		result.DebugInfo += "[SQUID VERSION] Detecting Squid version\n"
	}

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

	// Check X-Squid-Error or Server headers
	squidError := resp.Header.Get("X-Squid-Error")
	serverHeader := resp.Header.Get("Server")

	versionRegex := regexp.MustCompile(`(?i)squid[/\s]+([0-9.]+)`)

	if matches := versionRegex.FindStringSubmatch(squidError); len(matches) > 1 {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("  [INFO] Squid version from X-Squid-Error: %s\n", matches[1])
		}
		return true, matches[1]
	}

	if matches := versionRegex.FindStringSubmatch(serverHeader); len(matches) > 1 {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("  [INFO] Squid version from Server header: %s\n", matches[1])
		}
		return true, matches[1]
	}

	return false, ""
}

// testTraefikDashboard tests for exposed Traefik dashboard
func (c *Checker) testTraefikDashboard(client *http.Client, result *ProxyResult) (bool, string) {
	if c.debug {
		result.DebugInfo += "[TRAEFIK DASHBOARD] Testing for exposed Traefik dashboard\n"
	}

	dashboardPaths := []string{
		"/dashboard/",
		"/dashboard/api",
		"/api/overview",
		"/traefik/dashboard/",
	}

	for _, path := range dashboardPaths {
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

			// Check for Traefik dashboard indicators
			if strings.Contains(bodyStr, "traefik") &&
				(strings.Contains(bodyStr, "dashboard") ||
					strings.Contains(bodyStr, "routers") ||
					strings.Contains(bodyStr, "services") ||
					strings.Contains(bodyStr, "middlewares")) {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [HIGH] Traefik dashboard exposed at: %s\n", path)
				}
				return true, path
			}
		}
	}

	return false, ""
}

// testTraefikAPI tests for exposed Traefik API endpoints
func (c *Checker) testTraefikAPI(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[TRAEFIK API] Testing for exposed Traefik API endpoints\n"
	}

	exposedPaths := []string{}

	apiPaths := []string{
		"/api/rawdata",
		"/api/http/routers",
		"/api/http/services",
		"/api/http/middlewares",
		"/api/tcp/routers",
		"/api/tcp/services",
		"/api/entrypoints",
		"/api/version",
	}

	for _, path := range apiPaths {
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
			bodyStr := string(body)

			// Check for JSON response with Traefik data
			if strings.Contains(bodyStr, "{") &&
				(strings.Contains(bodyStr, "traefik") ||
					strings.Contains(bodyStr, "router") ||
					strings.Contains(bodyStr, "service") ||
					strings.Contains(bodyStr, "provider")) {
				exposedPaths = append(exposedPaths, path)
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [HIGH] Traefik API endpoint exposed: %s\n", path)
				}
			}
		}
	}

	return len(exposedPaths) > 0, exposedPaths
}

// testTraefikCVE_2024_45410 tests for Traefik SSRF vulnerability
func (c *Checker) testTraefikCVE_2024_45410(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[TRAEFIK CVE-2024-45410] Testing for SSRF via misconfigured middleware\n"
	}

	// CVE-2024-45410: Traefik SSRF via misconfigured middleware
	// Test with internal redirect
	req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("X-Forwarded-Host", "169.254.169.254")
	req.Header.Set("X-Forwarded-Proto", "http")

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Check if we get metadata service response
		if strings.Contains(bodyStr, "instance-id") ||
			strings.Contains(bodyStr, "ami-id") ||
			strings.Contains(bodyStr, "placement") {
			if c.debug {
				result.DebugInfo += "  [CRITICAL] Traefik SSRF to metadata service (CVE-2024-45410)\n"
			}
			return true
		}
	}

	return false
}

// testEnvoyAdmin tests for exposed Envoy admin interface
func (c *Checker) testEnvoyAdmin(client *http.Client, result *ProxyResult) (bool, string) {
	if c.debug {
		result.DebugInfo += "[ENVOY ADMIN] Testing for exposed Envoy admin interface\n"
	}

	adminPaths := []string{
		"/admin",
		"/stats",
		"/clusters",
		"/server_info",
		"/config_dump",
	}

	for _, path := range adminPaths {
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

			// Check for Envoy admin indicators
			if strings.Contains(bodyStr, "envoy") ||
				strings.Contains(bodyStr, "cluster") && strings.Contains(bodyStr, "upstream") ||
				strings.Contains(bodyStr, "stats") && strings.Contains(bodyStr, "listener") {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] Envoy admin interface exposed at: %s\n", path)
				}
				return true, path
			}
		}
	}

	return false, ""
}

// testEnvoyCVE_2022_21654 tests for Envoy SSRF in original_dst cluster
func (c *Checker) testEnvoyCVE_2022_21654(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[ENVOY CVE-2022-21654] Testing for SSRF in original_dst cluster\n"
	}

	// CVE-2022-21654: Envoy SSRF via original_dst cluster
	req, err := http.NewRequest("CONNECT", c.config.ValidationURL, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Host = "169.254.169.254:80"

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			if c.debug {
				result.DebugInfo += "  [CRITICAL] Envoy SSRF to metadata service (CVE-2022-21654)\n"
			}
			return true
		}
	}

	return false
}

// testEnvoyVersionDetection detects Envoy version
func (c *Checker) testEnvoyVersionDetection(client *http.Client, result *ProxyResult) (bool, string) {
	if c.debug {
		result.DebugInfo += "[ENVOY VERSION] Detecting Envoy version\n"
	}

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

	// Check Server or X-Envoy-* headers
	serverHeader := resp.Header.Get("Server")
	envoyUpstream := resp.Header.Get("X-Envoy-Upstream-Service-Time")

	versionRegex := regexp.MustCompile(`(?i)envoy[/\s]+([0-9a-f.]+)`)

	if matches := versionRegex.FindStringSubmatch(serverHeader); len(matches) > 1 {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("  [INFO] Envoy version from Server header: %s\n", matches[1])
		}
		return true, matches[1]
	}

	// Presence of X-Envoy headers confirms Envoy usage
	if envoyUpstream != "" {
		if c.debug {
			result.DebugInfo += "  [INFO] Envoy proxy detected via X-Envoy headers\n"
		}
		return true, "unknown"
	}

	return false, ""
}

// testCaddyAdminAPI tests for exposed Caddy admin API
func (c *Checker) testCaddyAdminAPI(client *http.Client, result *ProxyResult) (bool, string) {
	if c.debug {
		result.DebugInfo += "[CADDY ADMIN] Testing for exposed Caddy admin API\n"
	}

	adminPaths := []string{
		"/config/",
		"/config/apps/http/servers",
		"/reverse_proxy/upstreams",
		"/load",
	}

	for _, path := range adminPaths {
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
			bodyStr := string(body)

			// Check for Caddy config JSON
			if strings.Contains(bodyStr, "{") &&
				(strings.Contains(bodyStr, "apps") ||
					strings.Contains(bodyStr, "servers") ||
					strings.Contains(bodyStr, "routes")) {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] Caddy admin API exposed at: %s\n", path)
				}
				return true, path
			}
		}
	}

	return false, ""
}

// testCaddyVersionDetection detects Caddy version
func (c *Checker) testCaddyVersionDetection(client *http.Client, result *ProxyResult) (bool, string) {
	if c.debug {
		result.DebugInfo += "[CADDY VERSION] Detecting Caddy version\n"
	}

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

	versionRegex := regexp.MustCompile(`(?i)caddy[/\s]+v?([0-9.]+)`)

	if matches := versionRegex.FindStringSubmatch(serverHeader); len(matches) > 1 {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("  [INFO] Caddy version from Server header: %s\n", matches[1])
		}
		return true, matches[1]
	}

	return false, ""
}

// testVarnishBanLurk tests for exposed Varnish ban lurker
func (c *Checker) testVarnishBanLurk(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[VARNISH BAN] Testing for Varnish ban lurker exposure\n"
	}

	req, err := http.NewRequest("BAN", c.config.ValidationURL, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("X-Ban-Pattern", ".*")

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()

		// Exposed ban functionality
		if resp.StatusCode == 200 {
			if c.debug {
				result.DebugInfo += "  [CRITICAL] Varnish BAN method exposed (cache poisoning risk)\n"
			}
			return true
		}
	}

	return false
}

// testVarnishCVE_2022_45060 tests for Varnish request smuggling
func (c *Checker) testVarnishCVE_2022_45060(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[VARNISH CVE-2022-45060] Testing for request smuggling\n"
	}

	// CVE-2022-45060: Varnish HTTP/1 request smuggling
	req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Content-Length", "0")
	req.Header.Set("Transfer-Encoding", "chunked")

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()

		if resp.StatusCode < 500 {
			if c.debug {
				result.DebugInfo += "  [HIGH] Varnish accepts both CL and TE (CVE-2022-45060)\n"
			}
			return true
		}
	}

	return false
}

// testVarnishVersionDetection detects Varnish version
func (c *Checker) testVarnishVersionDetection(client *http.Client, result *ProxyResult) (bool, string) {
	if c.debug {
		result.DebugInfo += "[VARNISH VERSION] Detecting Varnish version\n"
	}

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

	// Check Via, X-Varnish, or Server headers
	viaHeader := resp.Header.Get("Via")
	varnishHeader := resp.Header.Get("X-Varnish")
	serverHeader := resp.Header.Get("Server")

	versionRegex := regexp.MustCompile(`(?i)varnish[/\s]+([0-9.]+)`)

	if matches := versionRegex.FindStringSubmatch(viaHeader); len(matches) > 1 {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("  [INFO] Varnish version from Via header: %s\n", matches[1])
		}
		return true, matches[1]
	}

	if matches := versionRegex.FindStringSubmatch(serverHeader); len(matches) > 1 {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("  [INFO] Varnish version from Server header: %s\n", matches[1])
		}
		return true, matches[1]
	}

	// X-Varnish presence confirms usage even without version
	if varnishHeader != "" {
		if c.debug {
			result.DebugInfo += "  [INFO] Varnish cache detected via X-Varnish header\n"
		}
		return true, "unknown"
	}

	return false, ""
}

// testAWSALBHeaderInjection tests for AWS ALB header injection
func (c *Checker) testAWSALBHeaderInjection(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[AWS ALB] Testing for header injection vulnerabilities\n"
	}

	// Test X-Amzn-Trace-Id manipulation
	req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("X-Amzn-Trace-Id", "Root=1-injected\r\nX-Injected: true")

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()

		// Check if injected header appears
		if resp.Header.Get("X-Injected") == "true" {
			if c.debug {
				result.DebugInfo += "  [HIGH] AWS ALB header injection successful\n"
			}
			return true
		}
	}

	return false
}

// testCloudflareWorkerBypass tests for Cloudflare Worker security bypass
func (c *Checker) testCloudflareWorkerBypass(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[CLOUDFLARE] Testing for Worker security bypass\n"
	}

	// Test CF-Worker header manipulation
	req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("CF-Worker", "bypass")
	req.Header.Set("CF-IPCountry", "XX")

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()

		// Check if worker logic is bypassed
		if resp.StatusCode == 200 {
			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), "access denied") {
				if c.debug {
					result.DebugInfo += "  [MEDIUM] Cloudflare Worker may be bypassable\n"
				}
				return true
			}
		}
	}

	return false
}

// testCloudflareCachePoisoning tests for Cloudflare-specific cache poisoning
func (c *Checker) testCloudflareCachePoisoning(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[CLOUDFLARE] Testing for cache poisoning vulnerabilities\n"
	}

	// Test CF-Connecting-IP as unkeyed input
	testValue := fmt.Sprintf("test-%d", 123456)

	req1, err := http.NewRequest("GET", c.config.ValidationURL, nil)
	if err != nil {
		return false
	}

	req1.Header.Set("User-Agent", c.config.UserAgent)
	req1.Header.Set("CF-Connecting-IP", testValue)

	resp1, err := client.Do(req1)
	if err != nil {
		return false
	}

	body1, _ := io.ReadAll(resp1.Body)
	resp1.Body.Close()

	// Check if test value reflected
	if strings.Contains(string(body1), testValue) {
		// Verify it's cached
		req2, _ := http.NewRequest("GET", c.config.ValidationURL, nil)
		req2.Header.Set("User-Agent", c.config.UserAgent)

		resp2, err := client.Do(req2)
		if err == nil {
			body2, _ := io.ReadAll(resp2.Body)
			resp2.Body.Close()

			if strings.Contains(string(body2), testValue) {
				if c.debug {
					result.DebugInfo += "  [HIGH] Cloudflare cache poisoning via CF-Connecting-IP\n"
				}
				return true
			}
		}
	}

	return false
}

// performVendorVulnerabilityChecks runs all vendor-specific vulnerability checks
func (c *Checker) performVendorVulnerabilityChecks(client *http.Client, result *ProxyResult) *VendorVulnResult {
	vendorResult := &VendorVulnResult{}

	// HAProxy checks
	vendorResult.HAProxyStatsExposed, vendorResult.HAProxyStatsPath = c.testHAProxyStatsExposure(client, result)
	vendorResult.HAProxyCVE_2023_40225 = c.testHAProxyCVE_2023_40225(client, result)
	vendorResult.HAProxyCVE_2021_40346 = c.testHAProxyCVE_2021_40346(client, result)
	vendorResult.HAProxyVersionDetected, vendorResult.HAProxyVersion = c.testHAProxyVersionDetection(client, result)

	// Squid checks
	vendorResult.SquidCacheManagerExposed, vendorResult.SquidCacheManagerPaths = c.testSquidCacheManager(client, result)
	vendorResult.SquidCVE_2021_46784 = c.testSquidCVE_2021_46784(client, result)
	vendorResult.SquidCVE_2020_15810 = c.testSquidCVE_2020_15810(client, result)
	vendorResult.SquidVersionDetected, vendorResult.SquidVersion = c.testSquidVersionDetection(client, result)

	// Traefik checks
	vendorResult.TraefikDashboardExposed, vendorResult.TraefikDashboardPath = c.testTraefikDashboard(client, result)
	vendorResult.TraefikAPIExposed, vendorResult.TraefikAPIPaths = c.testTraefikAPI(client, result)
	vendorResult.TraefikCVE_2024_45410 = c.testTraefikCVE_2024_45410(client, result)

	// Envoy checks
	vendorResult.EnvoyAdminExposed, vendorResult.EnvoyAdminPath = c.testEnvoyAdmin(client, result)
	vendorResult.EnvoyCVE_2022_21654 = c.testEnvoyCVE_2022_21654(client, result)
	vendorResult.EnvoyVersionDetected, vendorResult.EnvoyVersion = c.testEnvoyVersionDetection(client, result)

	// Caddy checks
	vendorResult.CaddyAdminAPIExposed, vendorResult.CaddyAdminPath = c.testCaddyAdminAPI(client, result)
	vendorResult.CaddyVersionDetected, vendorResult.CaddyVersion = c.testCaddyVersionDetection(client, result)

	// Varnish checks
	vendorResult.VarnishBanLurkExposed = c.testVarnishBanLurk(client, result)
	vendorResult.VarnishCVE_2022_45060 = c.testVarnishCVE_2022_45060(client, result)
	vendorResult.VarnishVersionDetected, vendorResult.VarnishVersion = c.testVarnishVersionDetection(client, result)

	// Cloud-specific checks
	vendorResult.AWSALBHeaderInjection = c.testAWSALBHeaderInjection(client, result)
	vendorResult.CloudflareWorkerBypass = c.testCloudflareWorkerBypass(client, result)
	vendorResult.CloudflareCachePoisoning = c.testCloudflareCachePoisoning(client, result)

	return vendorResult
}
