package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/errors"
)

// CensysDiscoverer implements the Discoverer interface for Censys
type CensysDiscoverer struct {
	apiID      string
	apiSecret  string
	httpClient *http.Client
	baseURL    string
	userAgent  string
}

// CensysResult represents a single result from Censys search
type CensysResult struct {
	IP           string                 `json:"ip"`
	Port         int                    `json:"port"`
	Protocol     string                 `json:"protocol"`
	Services     []CensysService        `json:"services"`
	Location     CensysLocation         `json:"location"`
	ASN          CensysASN              `json:"autonomous_system"`
	LastUpdated  string                 `json:"last_updated_at"`
	Labels       []string               `json:"labels"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// CensysService represents a service detected by Censys
type CensysService struct {
	Port            int                    `json:"port"`
	ServiceName     string                 `json:"service_name"`
	TransportProto  string                 `json:"transport_protocol"`
	Certificate     *CensysCertificate     `json:"certificate,omitempty"`
	HTTP            *CensysHTTP            `json:"http,omitempty"`
	Banner          string                 `json:"banner,omitempty"`
	Software        []CensysSoftware       `json:"software,omitempty"`
	ExtendedService map[string]interface{} `json:"extended_service,omitempty"`
}

// CensysHTTP represents HTTP-specific data from Censys
type CensysHTTP struct {
	Request  CensysHTTPRequest  `json:"request"`
	Response CensysHTTPResponse `json:"response"`
}

// CensysHTTPRequest represents the HTTP request data
type CensysHTTPRequest struct {
	Method  string            `json:"method"`
	URI     string            `json:"uri"`
	Headers map[string]string `json:"headers"`
}

// CensysHTTPResponse represents the HTTP response data
type CensysHTTPResponse struct {
	StatusCode    int               `json:"status_code"`
	StatusReason  string            `json:"status_reason"`
	Headers       map[string]string `json:"headers"`
	Body          string            `json:"body"`
	BodyHash      string            `json:"body_hash"`
	HTMLTitle     string            `json:"html_title"`
	HTMLTags      []string          `json:"html_tags"`
}

// CensysCertificate represents TLS certificate data
type CensysCertificate struct {
	FingerprintSHA256 string   `json:"fingerprint_sha256"`
	Names             []string `json:"names"`
	PubkeyAlgorithm   string   `json:"pubkey_algorithm"`
	SignatureAlgo     string   `json:"signature_algorithm"`
}

// CensysSoftware represents detected software
type CensysSoftware struct {
	Product  string `json:"product"`
	Vendor   string `json:"vendor"`
	Version  string `json:"version"`
	Source   string `json:"source"`
}

// CensysLocation represents location data from Censys
type CensysLocation struct {
	City            string  `json:"city"`
	Province        string  `json:"province"`
	PostalCode      string  `json:"postal_code"`
	CountryCode     string  `json:"country_code"`
	Country         string  `json:"country"`
	Continent       string  `json:"continent"`
	Coordinates     [2]float64 `json:"coordinates"`
	RegisteredCountry string `json:"registered_country"`
	Timezone        string  `json:"timezone"`
}

// CensysASN represents autonomous system information
type CensysASN struct {
	ASN         int    `json:"asn"`
	Description string `json:"description"`
	BGPPrefix   string `json:"bgp_prefix"`
	Name        string `json:"name"`
	CountryCode string `json:"country_code"`
}

// CensysSearchResponse represents the full response from Censys search API
type CensysSearchResponse struct {
	Code   string         `json:"code"`
	Status string         `json:"status"`
	Result CensysSearchResult `json:"result"`
}

// CensysSearchResult contains the actual search results
type CensysSearchResult struct {
	Query      string         `json:"query"`
	Total      int            `json:"total"`
	Duration   int            `json:"duration_ms"`
	Hits       []CensysResult `json:"hits"`
	Links      CensysLinks    `json:"links"`
}

// CensysLinks contains pagination links
type CensysLinks struct {
	Next string `json:"next"`
	Prev string `json:"prev"`
}

// NewCensysDiscoverer creates a new Censys discoverer instance
func NewCensysDiscoverer(apiID, apiSecret string) *CensysDiscoverer {
	return &CensysDiscoverer{
		apiID:     apiID,
		apiSecret: apiSecret,
		baseURL:   "https://search.censys.io/api/v2",
		userAgent: "ProxyHawk/2.0 (https://github.com/ResistanceIsUseless/ProxyHawk)",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the name of this discoverer
func (c *CensysDiscoverer) Name() string {
	return "censys"
}

// IsConfigured checks if the discoverer is properly configured
func (c *CensysDiscoverer) IsConfigured() bool {
	return c.apiID != "" && c.apiSecret != ""
}

// Search performs a search query on Censys
func (c *CensysDiscoverer) Search(query string, limit int) (*DiscoveryResult, error) {
	if !c.IsConfigured() {
		return nil, errors.NewConfigError(errors.ErrorConfigNotFound, "Censys API credentials not configured", nil)
	}

	start := time.Now()
	result := &DiscoveryResult{
		Query:     query,
		Source:    c.Name(),
		Timestamp: start,
		Metadata:  make(map[string]interface{}),
	}

	// Build search request - try the newer API endpoint
	searchURL := fmt.Sprintf("%s/hosts/search", c.baseURL)
	
	requestBody := map[string]interface{}{
		"q":        query,
		"per_page": limit,
		"virtual_hosts": "EXCLUDE", // Focus on actual services
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Make request
	req, err := http.NewRequest("POST", searchURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	req.SetBasicAuth(c.apiID, c.apiSecret)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("censys API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var searchResp CensysSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if searchResp.Code != "200" {
		return nil, fmt.Errorf("censys search failed: %s", searchResp.Status)
	}

	// Convert Censys results to ProxyCandidate
	candidates := make([]ProxyCandidate, 0, len(searchResp.Result.Hits))
	for _, hit := range searchResp.Result.Hits {
		proxyCandidates := c.convertToCandidate(hit)
		candidates = append(candidates, proxyCandidates...)
	}

	result.Duration = time.Since(start)
	result.Total = searchResp.Result.Total
	result.Filtered = len(candidates)
	result.Candidates = candidates
	result.Metadata["censys_total"] = searchResp.Result.Total
	result.Metadata["censys_duration_ms"] = searchResp.Result.Duration

	return result, nil
}

// GetDetails gets detailed information about a specific IP
func (c *CensysDiscoverer) GetDetails(ip string) (*ProxyCandidate, error) {
	if !c.IsConfigured() {
		return nil, errors.NewConfigError(errors.ErrorConfigNotFound, "Censys API credentials not configured", nil)
	}

	// Build host view URL
	hostURL := fmt.Sprintf("%s/hosts/%s", c.baseURL, ip)

	// Make request
	req, err := http.NewRequest("GET", hostURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.SetBasicAuth(c.apiID, c.apiSecret)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get host details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("censys API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var hostResp struct {
		Code   string       `json:"code"`
		Status string       `json:"status"`
		Result CensysResult `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&hostResp); err != nil {
		return nil, fmt.Errorf("failed to decode host details: %w", err)
	}

	if hostResp.Code != "200" {
		return nil, fmt.Errorf("censys host lookup failed: %s", hostResp.Status)
	}

	// Find the best proxy-related service
	candidates := c.convertToCandidate(hostResp.Result)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no proxy-related services found for IP %s", ip)
	}

	// Return the highest scoring candidate
	bestCandidate := &candidates[0]
	bestScore := candidates[0].Confidence
	for i := 1; i < len(candidates); i++ {
		if candidates[i].Confidence > bestScore {
			bestCandidate = &candidates[i]
			bestScore = candidates[i].Confidence
		}
	}

	return bestCandidate, nil
}

// GetRateLimit gets current API rate limit status for Censys
func (c *CensysDiscoverer) GetRateLimit() (remaining int, resetTime time.Time, err error) {
	// Censys doesn't provide direct rate limit info in the API
	// They have different limits per plan, but no way to query current usage
	// Return a reasonable estimate based on common limits
	return 1000, time.Now().Add(24*time.Hour), nil
}

// convertToCandidate converts a Censys result to ProxyCandidate(s)
func (c *CensysDiscoverer) convertToCandidate(result CensysResult) []ProxyCandidate {
	var candidates []ProxyCandidate

	// Parse timestamp
	lastSeen, _ := time.Parse(time.RFC3339, result.LastUpdated)

	// Analyze each service for proxy indicators
	for _, service := range result.Services {
		if score := c.scoreProxyService(service); score > 0.1 {
			candidate := ProxyCandidate{
				IP:           result.IP,
				Port:         service.Port,
				Protocol:     c.inferProtocol(service),
				Source:       c.Name(),
				LastSeen:     lastSeen,
				FirstSeen:    lastSeen,
				DiscoveryID:  fmt.Sprintf("censys-%s-%d", result.IP, service.Port),
				Country:      result.Location.CountryCode,
				City:         result.Location.City,
				ASN:          fmt.Sprintf("AS%d", result.ASN.ASN),
				ASNOrg:       result.ASN.Name,
				ISP:          result.ASN.Description,
				ServerHeader: c.extractServerHeader(service),
				ResponseSize: int64(len(service.Banner)),
				TLSEnabled:   service.Certificate != nil,
				ProxyHeaders: c.extractProxyHeaders(service),
				ProxyType:    c.inferProxyType(service),
				Confidence:   score,
			}

			// Add HTTP-specific data if available
			if service.HTTP != nil {
				candidate.Headers = service.HTTP.Response.Headers
				candidate.ResponseSize = int64(len(service.HTTP.Response.Body))
				candidate.AuthRequired = c.checkAuthRequired(service.HTTP.Response)
			}

			// Extract open ports from all services
			var ports []int
			for _, svc := range result.Services {
				ports = append(ports, svc.Port)
			}
			candidate.OpenPorts = ports

			// Add threat tags if any labels indicate suspicious activity
			for _, label := range result.Labels {
				if strings.Contains(strings.ToLower(label), "malware") ||
					strings.Contains(strings.ToLower(label), "botnet") ||
					strings.Contains(strings.ToLower(label), "suspicious") {
					candidate.ThreatTags = append(candidate.ThreatTags, label)
				}
			}

			candidates = append(candidates, candidate)
		}
	}

	return candidates
}

// inferProtocol infers the proxy protocol from Censys service data
func (c *CensysDiscoverer) inferProtocol(service CensysService) string {
	serviceName := strings.ToLower(service.ServiceName)
	banner := strings.ToLower(service.Banner)
	port := service.Port

	// Check for SOCKS indicators
	if strings.Contains(serviceName, "socks") || strings.Contains(banner, "socks") {
		if port == 1080 || strings.Contains(banner, "socks5") {
			return "socks5"
		}
		return "socks4"
	}

	// Check for HTTPS/TLS
	if port == 443 || port == 8443 || service.Certificate != nil {
		return "https"
	}

	// Check service name and common proxy ports
	if serviceName == "http" || port == 3128 || port == 8080 || port == 8118 {
		return "http"
	}

	// Check transport protocol
	if service.TransportProto == "tcp" {
		if strings.Contains(banner, "http") || (service.HTTP != nil) {
			return "http"
		}
	}

	return "http" // Default assumption
}

// extractServerHeader extracts the server header from Censys service data
func (c *CensysDiscoverer) extractServerHeader(service CensysService) string {
	if service.HTTP != nil {
		if server, ok := service.HTTP.Response.Headers["server"]; ok {
			return server
		}
		if server, ok := service.HTTP.Response.Headers["Server"]; ok {
			return server
		}
	}

	// Check software detection
	for _, software := range service.Software {
		if software.Product != "" {
			server := software.Product
			if software.Version != "" {
				server += "/" + software.Version
			}
			return server
		}
	}

	return ""
}

// extractProxyHeaders extracts proxy-related headers from Censys service data
func (c *CensysDiscoverer) extractProxyHeaders(service CensysService) []string {
	var proxyHeaders []string

	if service.HTTP != nil {
		proxyHeaderNames := []string{
			"via", "x-forwarded-for", "x-forwarded-proto", "x-forwarded-host",
			"x-real-ip", "x-proxy-authorization", "proxy-authorization",
			"x-cache", "x-cache-lookup", "x-served-by", "x-squid-error",
		}

		for _, headerName := range proxyHeaderNames {
			if _, exists := service.HTTP.Response.Headers[headerName]; exists {
				proxyHeaders = append(proxyHeaders, headerName)
			}
			if _, exists := service.HTTP.Response.Headers[strings.Title(headerName)]; exists {
				proxyHeaders = append(proxyHeaders, headerName)
			}
		}
	}

	// Check banner for proxy headers
	banner := strings.ToLower(service.Banner)
	if strings.Contains(banner, "via:") {
		proxyHeaders = append(proxyHeaders, "via")
	}
	if strings.Contains(banner, "x-forwarded") {
		proxyHeaders = append(proxyHeaders, "x-forwarded-for")
	}

	return proxyHeaders
}

// inferProxyType infers the proxy software type from Censys service data
func (c *CensysDiscoverer) inferProxyType(service CensysService) string {
	banner := strings.ToLower(service.Banner)
	serviceName := strings.ToLower(service.ServiceName)

	// Check software detection first
	for _, software := range service.Software {
		product := strings.ToLower(software.Product)
		if strings.Contains(product, "squid") {
			return "squid"
		}
		if strings.Contains(product, "nginx") {
			return "nginx"
		}
		if strings.Contains(product, "apache") {
			return "apache"
		}
		if strings.Contains(product, "tinyproxy") {
			return "tinyproxy"
		}
		if strings.Contains(product, "privoxy") {
			return "privoxy"
		}
	}

	// Check server header
	server := c.extractServerHeader(service)
	if server != "" {
		serverLower := strings.ToLower(server)
		if strings.Contains(serverLower, "squid") {
			return "squid"
		}
		if strings.Contains(serverLower, "nginx") {
			return "nginx"
		}
		if strings.Contains(serverLower, "apache") {
			return "apache"
		}
	}

	// Check banner and service name
	if strings.Contains(banner, "squid") || strings.Contains(serviceName, "squid") {
		return "squid"
	}
	if strings.Contains(banner, "nginx") || strings.Contains(serviceName, "nginx") {
		return "nginx"
	}
	if strings.Contains(banner, "apache") || strings.Contains(serviceName, "apache") {
		return "apache"
	}
	if strings.Contains(banner, "tinyproxy") {
		return "tinyproxy"
	}

	return "unknown"
}

// checkAuthRequired checks if the service requires authentication
func (c *CensysDiscoverer) checkAuthRequired(response CensysHTTPResponse) bool {
	if response.StatusCode == 401 || response.StatusCode == 407 {
		return true
	}

	for header, value := range response.Headers {
		headerLower := strings.ToLower(header)
		valueLower := strings.ToLower(value)
		if headerLower == "proxy-authenticate" || headerLower == "www-authenticate" {
			return true
		}
		if strings.Contains(valueLower, "proxy-authenticate") {
			return true
		}
	}

	bodyLower := strings.ToLower(response.Body)
	return strings.Contains(bodyLower, "proxy-authenticate") ||
		strings.Contains(bodyLower, "401 unauthorized") ||
		strings.Contains(bodyLower, "407 proxy authentication required")
}

// scoreProxyService scores how likely this service is to be a working proxy
func (c *CensysDiscoverer) scoreProxyService(service CensysService) float64 {
	score := 0.0
	banner := strings.ToLower(service.Banner)
	serviceName := strings.ToLower(service.ServiceName)

	// Port-based scoring
	switch service.Port {
	case 3128, 8080: // Common proxy ports
		score += 0.4
	case 1080: // SOCKS port
		score += 0.3
	case 8118, 9050: // Privoxy, Tor
		score += 0.2
	case 80, 8000, 8001, 8008: // HTTP ports that might be proxies
		score += 0.1
	default:
		if service.Port > 1024 && service.Port < 65535 {
			score += 0.05
		}
	}

	// Service name scoring
	if strings.Contains(serviceName, "proxy") {
		score += 0.3
	}
	if serviceName == "http" && (service.Port == 3128 || service.Port == 8080) {
		score += 0.2
	}

	// Banner content scoring
	if strings.Contains(banner, "proxy") {
		score += 0.3
	}
	if strings.Contains(banner, "squid") {
		score += 0.3
	}
	if strings.Contains(banner, "via:") || strings.Contains(banner, "x-forwarded") {
		score += 0.2
	}
	if strings.Contains(banner, "x-cache") {
		score += 0.2
	}
	if strings.Contains(banner, "connect") && strings.Contains(banner, "tunnel") {
		score += 0.2
	}

	// Software detection scoring
	for _, software := range service.Software {
		product := strings.ToLower(software.Product)
		if strings.Contains(product, "squid") || strings.Contains(product, "proxy") {
			score += 0.3
		}
		if strings.Contains(product, "nginx") || strings.Contains(product, "apache") {
			score += 0.1 // Could be reverse proxy
		}
	}

	// HTTP response scoring
	if service.HTTP != nil {
		response := service.HTTP.Response
		
		// Check for proxy-specific headers
		for header := range response.Headers {
			headerLower := strings.ToLower(header)
			if strings.Contains(headerLower, "proxy") || 
			   strings.Contains(headerLower, "x-forwarded") ||
			   strings.Contains(headerLower, "via") ||
			   strings.Contains(headerLower, "x-cache") {
				score += 0.1
			}
		}

		// Status code indicators
		if response.StatusCode == 407 { // Proxy Authentication Required
			score += 0.2
		}
	}

	// Negative indicators
	if strings.Contains(banner, "404 not found") {
		score -= 0.2
	}
	if strings.Contains(banner, "connection refused") {
		score -= 0.3
	}
	if strings.Contains(banner, "access denied") {
		score -= 0.1
	}

	// Ensure score is between 0 and 1
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}

// Common Censys search queries for finding proxies
var CensysProxyQueries = []string{
	"services.service_name: http and services.port: 3128",           // HTTP proxies on common port
	"services.service_name: http and services.port: 8080",           // HTTP proxies on alt port
	"services.port: 1080 and services.service_name: socks",          // SOCKS proxies
	"services.http.response.headers.server: squid",                  // Squid proxy servers
	"services.http.response.headers.via: *",                         // Proxies with Via header
	"services.http.response.headers.x_cache: *",                     // Caching proxies
	"services.http.response.body: \"proxy\"",                        // Generic proxy search in body
	"services.http.response.status_code: 407",                       // Proxy auth required
	"services.software.product: squid",                              // Squid software detection
	"services.software.product: nginx and services.port: (3128 or 8080)", // Nginx reverse proxies
	"services.banner: \"tinyproxy\"",                                 // TinyProxy
	"services.banner: \"privoxy\"",                                   // Privoxy
	"services.http.response.body: \"connect\" and services.http.response.body: \"tunnel\"", // CONNECT method
}