package proxy

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PathFingerprintResult contains results from path-based fingerprinting
type PathFingerprintResult struct {
	TargetURL         string                   `json:"target_url"`
	Timestamp         time.Time                `json:"timestamp"`
	PathResults       map[string]*PathResponse `json:"path_results"`
	HeaderDifferences []HeaderDifference       `json:"header_differences"`
	DetectedBackends  []BackendDetection       `json:"detected_backends"`
	RoutingPatterns   []RoutingPattern         `json:"routing_patterns"`
	ProxySoftware     ProxySoftware            `json:"proxy_software"`
	Confidence        float64                  `json:"confidence"`
}

// PathResponse contains the response for a specific path
type PathResponse struct {
	Path           string            `json:"path"`
	StatusCode     int               `json:"status_code"`
	Headers        map[string]string `json:"headers"`
	BodyLength     int               `json:"body_length"`
	ResponseTime   time.Duration     `json:"response_time"`
	ServerHeader   string            `json:"server_header"`
	Error          string            `json:"error,omitempty"`
	BodyPreview    string            `json:"body_preview,omitempty"` // First 500 chars
}

// HeaderDifference represents a difference in headers between paths
type HeaderDifference struct {
	HeaderName string   `json:"header_name"`
	Paths      []string `json:"paths"`
	Values     []string `json:"values"`
	Suspicious bool     `json:"suspicious"` // True if indicates backend routing
}

// BackendDetection represents a detected backend service
type BackendDetection struct {
	Path       string   `json:"path"`
	Backend    string   `json:"backend"`
	Indicators []string `json:"indicators"`
	Confidence float64  `json:"confidence"`
}

// RoutingPattern represents a detected routing/rewrite pattern
type RoutingPattern struct {
	Pattern     string   `json:"pattern"`
	Description string   `json:"description"`
	Paths       []string `json:"paths"`
	Evidence    []string `json:"evidence"`
}

// Common paths to test for backend detection
var defaultTestPaths = []string{
	"/",
	"/admin",
	"/api",
	"/v1",
	"/v2",
	"/api/v1",
	"/api/v2",
	"/health",
	"/status",
	"/metrics",
	"/internal",
	"/debug",
	"/console",
	"/dashboard",
	"/management",
	"/actuator",
	"/swagger",
	"/graphql",
	"/ws",
	"/websocket",
}

// PathFingerprint performs path-based fingerprinting on a target
func PathFingerprint(baseURL string, timeout time.Duration, insecureSSL bool, customPaths []string) (*PathFingerprintResult, error) {
	result := &PathFingerprintResult{
		TargetURL:         baseURL,
		Timestamp:         time.Now(),
		PathResults:       make(map[string]*PathResponse),
		HeaderDifferences: []HeaderDifference{},
		DetectedBackends:  []BackendDetection{},
		RoutingPatterns:   []RoutingPattern{},
		ProxySoftware:     ProxySoftwareUnknown,
		Confidence:        0.0,
	}

	// Use custom paths if provided, otherwise use defaults
	testPaths := defaultTestPaths
	if len(customPaths) > 0 {
		testPaths = customPaths
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecureSSL,
			},
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Don't follow redirects - we want to see them
			return http.ErrUseLastResponse
		},
	}

	// Test each path
	for _, path := range testPaths {
		testURL := strings.TrimRight(baseURL, "/") + path
		pathResp := testPath(client, testURL, path)
		result.PathResults[path] = pathResp
	}

	// Analyze results
	analyzeHeaderDifferences(result)
	detectBackends(result)
	detectRoutingPatterns(result)
	detectProxySoftware(result)

	return result, nil
}

// testPath makes a request to a specific path and records the response
func testPath(client *http.Client, url string, path string) *PathResponse {
	resp := &PathResponse{
		Path:    path,
		Headers: make(map[string]string),
	}

	startTime := time.Now()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		resp.Error = err.Error()
		return resp
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")

	httpResp, err := client.Do(req)
	resp.ResponseTime = time.Since(startTime)

	if err != nil {
		resp.Error = err.Error()
		return resp
	}
	defer httpResp.Body.Close()

	// Record status code
	resp.StatusCode = httpResp.StatusCode

	// Record all headers
	for name, values := range httpResp.Header {
		if len(values) > 0 {
			resp.Headers[name] = values[0]
		}
	}

	// Extract Server header specifically
	resp.ServerHeader = httpResp.Header.Get("Server")

	// Read body
	body, err := io.ReadAll(httpResp.Body)
	if err == nil {
		resp.BodyLength = len(body)
		// Store preview (first 500 chars)
		if len(body) > 500 {
			resp.BodyPreview = string(body[:500])
		} else {
			resp.BodyPreview = string(body)
		}
	}

	return resp
}

// analyzeHeaderDifferences finds headers that differ between paths
func analyzeHeaderDifferences(result *PathFingerprintResult) {
	// Track all unique header names
	allHeaders := make(map[string]bool)
	for _, pathResp := range result.PathResults {
		for header := range pathResp.Headers {
			allHeaders[header] = true
		}
	}

	// Check each header across all paths
	for headerName := range allHeaders {
		values := make(map[string][]string)

		for path, pathResp := range result.PathResults {
			if val, exists := pathResp.Headers[headerName]; exists {
				values[val] = append(values[val], path)
			}
		}

		// If header has different values across paths, record it
		if len(values) > 1 {
			diff := HeaderDifference{
				HeaderName: headerName,
				Paths:      []string{},
				Values:     []string{},
				Suspicious: isSuspiciousHeader(headerName),
			}

			for value, paths := range values {
				diff.Values = append(diff.Values, value)
				diff.Paths = append(diff.Paths, paths...)
			}

			result.HeaderDifferences = append(result.HeaderDifferences, diff)
		}
	}
}

// isSuspiciousHeader determines if a header difference is suspicious (indicates backend routing)
func isSuspiciousHeader(headerName string) bool {
	suspiciousHeaders := []string{
		"Server",
		"X-Powered-By",
		"X-AspNet-Version",
		"X-AspNetMvc-Version",
		"X-Runtime",
		"X-Version",
		"X-Backend-Server",
		"X-Served-By",
		"X-Upstream-Address",
		"Via",
	}

	headerLower := strings.ToLower(headerName)
	for _, suspicious := range suspiciousHeaders {
		if strings.EqualFold(headerLower, strings.ToLower(suspicious)) {
			return true
		}
	}
	return false
}

// detectBackends attempts to identify different backend services
func detectBackends(result *PathFingerprintResult) {
	for path, pathResp := range result.PathResults {
		if pathResp.Error != "" || pathResp.StatusCode == 0 {
			continue
		}

		backend := BackendDetection{
			Path:       path,
			Indicators: []string{},
			Confidence: 0.0,
		}

		// Check Server header
		if pathResp.ServerHeader != "" {
			backend.Backend = pathResp.ServerHeader
			backend.Indicators = append(backend.Indicators, fmt.Sprintf("Server header: %s", pathResp.ServerHeader))
			backend.Confidence += 0.5
		}

		// Check for common framework headers
		if xpb := pathResp.Headers["X-Powered-By"]; xpb != "" {
			backend.Indicators = append(backend.Indicators, fmt.Sprintf("X-Powered-By: %s", xpb))
			backend.Confidence += 0.3
		}

		// Check body content for framework indicators
		bodyLower := strings.ToLower(pathResp.BodyPreview)
		frameworks := map[string][]string{
			"Django":     {"django", "csrfmiddlewaretoken"},
			"Flask":      {"werkzeug"},
			"Express":    {"express"},
			"Spring":     {"whitelabel error page", "spring"},
			"Laravel":    {"laravel"},
			"Rails":      {"rails"},
			"ASP.NET":    {"asp.net", "webforms", "__viewstate"},
			"Kubernetes": {"kubernetes", "k8s"},
		}

		for framework, keywords := range frameworks {
			for _, keyword := range keywords {
				if strings.Contains(bodyLower, keyword) {
					backend.Backend = framework
					backend.Indicators = append(backend.Indicators, fmt.Sprintf("Body contains: %s", keyword))
					backend.Confidence += 0.2
					break
				}
			}
		}

		// Only add if we detected something
		if backend.Confidence > 0 {
			result.DetectedBackends = append(result.DetectedBackends, backend)
		}
	}
}

// detectRoutingPatterns identifies routing/rewrite patterns
func detectRoutingPatterns(result *PathFingerprintResult) {
	// Pattern 1: Different Server headers for different paths
	serverHeaders := make(map[string][]string)
	for path, pathResp := range result.PathResults {
		if pathResp.ServerHeader != "" {
			serverHeaders[pathResp.ServerHeader] = append(serverHeaders[pathResp.ServerHeader], path)
		}
	}

	if len(serverHeaders) > 1 {
		evidence := []string{}
		for server, paths := range serverHeaders {
			evidence = append(evidence, fmt.Sprintf("%s: %v", server, paths))
		}

		result.RoutingPatterns = append(result.RoutingPatterns, RoutingPattern{
			Pattern:     "Multi-Backend Routing",
			Description: "Different Server headers detected across paths, indicating request routing to different backend services",
			Paths:       []string{},
			Evidence:    evidence,
		})
	}

	// Pattern 2: API versioning (v1, v2)
	apiPaths := []string{}
	for path := range result.PathResults {
		if strings.Contains(path, "/api") || strings.Contains(path, "/v1") || strings.Contains(path, "/v2") {
			apiPaths = append(apiPaths, path)
		}
	}

	if len(apiPaths) > 0 {
		result.RoutingPatterns = append(result.RoutingPatterns, RoutingPattern{
			Pattern:     "API Versioning",
			Description: "API paths detected, likely using versioned endpoints",
			Paths:       apiPaths,
			Evidence:    []string{fmt.Sprintf("Found %d API-related paths", len(apiPaths))},
		})
	}

	// Pattern 3: Admin/Management interfaces
	adminPaths := []string{}
	for path, pathResp := range result.PathResults {
		if (strings.Contains(path, "/admin") || strings.Contains(path, "/dashboard") ||
			strings.Contains(path, "/management") || strings.Contains(path, "/console")) &&
			pathResp.StatusCode < 500 && pathResp.Error == "" {
			adminPaths = append(adminPaths, path)
		}
	}

	if len(adminPaths) > 0 {
		result.RoutingPatterns = append(result.RoutingPatterns, RoutingPattern{
			Pattern:     "Admin Interface",
			Description: "Administrative or management endpoints detected",
			Paths:       adminPaths,
			Evidence:    []string{fmt.Sprintf("Found %d admin-related paths accessible", len(adminPaths))},
		})
	}

	// Pattern 4: Status/Health endpoints
	healthPaths := []string{}
	for path, pathResp := range result.PathResults {
		if (strings.Contains(path, "/health") || strings.Contains(path, "/status") ||
			strings.Contains(path, "/metrics") || strings.Contains(path, "/actuator")) &&
			pathResp.StatusCode == 200 {
			healthPaths = append(healthPaths, path)
		}
	}

	if len(healthPaths) > 0 {
		result.RoutingPatterns = append(result.RoutingPatterns, RoutingPattern{
			Pattern:     "Health/Metrics Endpoints",
			Description: "Health check or metrics endpoints detected, may expose internal information",
			Paths:       healthPaths,
			Evidence:    []string{fmt.Sprintf("Found %d health/metrics paths returning 200 OK", len(healthPaths))},
		})
	}
}

// detectProxySoftware determines the proxy software from path fingerprinting
func detectProxySoftware(result *PathFingerprintResult) {
	// Analyze the root path response for initial detection
	rootResp, exists := result.PathResults["/"]
	if !exists || rootResp.Error != "" {
		return
	}

	// Check Server header first
	serverHeader := rootResp.ServerHeader
	if serverHeader != "" {
		// Map server headers to proxy software
		serverLower := strings.ToLower(serverHeader)
		switch {
		case strings.Contains(serverLower, "nginx"):
			result.ProxySoftware = ProxySoftwareNginx
			result.Confidence = 0.9
			return
		case strings.Contains(serverLower, "apache"):
			result.ProxySoftware = ProxySoftwareApache
			result.Confidence = 0.9
			return
		case strings.Contains(serverLower, "haproxy"):
			result.ProxySoftware = ProxySoftwareHAProxy
			result.Confidence = 0.9
			return
		case strings.Contains(serverLower, "envoy"):
			result.ProxySoftware = ProxySoftwareEnvoy
			result.Confidence = 0.9
			return
		case strings.Contains(serverLower, "traefik"):
			result.ProxySoftware = ProxySoftwareTraefik
			result.Confidence = 0.9
			return
		case strings.Contains(serverLower, "caddy"):
			result.ProxySoftware = ProxySoftwareCaddy
			result.Confidence = 0.9
			return
		case strings.Contains(serverLower, "kong"):
			result.ProxySoftware = ProxySoftwareKong
			result.Confidence = 0.9
			return
		case strings.Contains(serverLower, "varnish"):
			result.ProxySoftware = ProxySoftwareVarnish
			result.Confidence = 0.9
			return
		case strings.Contains(serverLower, "squid"):
			result.ProxySoftware = ProxySoftwareSquid
			result.Confidence = 0.9
			return
		case strings.Contains(serverLower, "cloudflare"):
			result.ProxySoftware = ProxySoftwareCloudflare
			result.Confidence = 0.95
			return
		}
	}

	// Check for special headers that indicate proxy type
	if rootResp.Headers["X-Varnish"] != "" || rootResp.Headers["Via"] != "" && strings.Contains(strings.ToLower(rootResp.Headers["Via"]), "varnish") {
		result.ProxySoftware = ProxySoftwareVarnish
		result.Confidence = 0.95
		return
	}

	if rootResp.Headers["X-Kong-Upstream-Latency"] != "" || rootResp.Headers["X-Kong-Proxy-Latency"] != "" {
		result.ProxySoftware = ProxySoftwareKong
		result.Confidence = 0.95
		return
	}

	if rootResp.Headers["CF-Ray"] != "" {
		result.ProxySoftware = ProxySoftwareCloudflare
		result.Confidence = 0.99
		return
	}

	if rootResp.Headers["X-Amz-Cf-Id"] != "" || rootResp.Headers["X-Amzn-RequestId"] != "" {
		result.ProxySoftware = ProxySoftwareAWS
		result.Confidence = 0.95
		return
	}

	if rootResp.Headers["x-envoy-upstream-service-time"] != "" {
		result.ProxySoftware = ProxySoftwareEnvoy
		result.Confidence = 0.95
		return
	}

	// If Server header is missing or inconclusive, check body content
	// Many servers hide the Server header but reveal it in error pages
	if rootResp.BodyPreview != "" {
		bodyLower := strings.ToLower(rootResp.BodyPreview)

		// Check for Nginx
		if strings.Contains(bodyLower, "<hr><center>nginx</center>") ||
		   strings.Contains(bodyLower, "<center>nginx</center>") {
			result.ProxySoftware = ProxySoftwareNginx
			result.Confidence = 0.85 // Slightly lower confidence from body vs header
			return
		}

		// Check for Apache
		if strings.Contains(bodyLower, "<title>403 forbidden</title>") &&
		   strings.Contains(bodyLower, "apache") {
			result.ProxySoftware = ProxySoftwareApache
			result.Confidence = 0.8
			return
		}

		// Check for HAProxy
		if strings.Contains(bodyLower, "400 bad request") &&
		   strings.Contains(bodyLower, "your browser sent an invalid request") {
			result.ProxySoftware = ProxySoftwareHAProxy
			result.Confidence = 0.7
			return
		}

		// Check for other proxies by body patterns
		if strings.Contains(bodyLower, "varnish") {
			result.ProxySoftware = ProxySoftwareVarnish
			result.Confidence = 0.75
			return
		}

		if strings.Contains(bodyLower, "squid") {
			result.ProxySoftware = ProxySoftwareSquid
			result.Confidence = 0.75
			return
		}
	}
}
