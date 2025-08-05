package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/errors"
)

// ShodanDiscoverer implements the Discoverer interface for Shodan
type ShodanDiscoverer struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
	userAgent  string
}

// ShodanResult represents a single result from Shodan search
type ShodanResult struct {
	IP        string            `json:"ip_str"`
	Port      int               `json:"port"`
	Protocol  string            `json:"transport"`
	Timestamp string            `json:"timestamp"`
	Data      string            `json:"data"`
	Hostnames []string          `json:"hostnames"`
	Location  ShodanLocation    `json:"location"`
	ASN       string            `json:"asn"`
	ISP       string            `json:"isp"`
	Org       string            `json:"org"`
	Headers   interface{} `json:"http,omitempty"` // Can be map or number
	Product   string            `json:"product,omitempty"`
	Version   string            `json:"version,omitempty"`
	OS        string            `json:"os,omitempty"`
	CPE       []string          `json:"cpe,omitempty"`
	Hash      int64             `json:"hash"`
}

// ShodanLocation represents location data from Shodan
type ShodanLocation struct {
	City        string  `json:"city"`
	RegionCode  string  `json:"region_code"`
	AreaCode    int     `json:"area_code"`
	Longitude   float64 `json:"longitude"`
	CountryCode string  `json:"country_code"`
	CountryName string  `json:"country_name"`
	Postal      string  `json:"postal_code"`
	DMACode     int     `json:"dma_code"`
	Latitude    float64 `json:"latitude"`
}

// ShodanSearchResponse represents the full response from Shodan search API
type ShodanSearchResponse struct {
	Matches []ShodanResult `json:"matches"`
	Total   int            `json:"total"`
	Facets  interface{}    `json:"facets,omitempty"`
}

// ShodanHostInfo represents detailed host information from Shodan
type ShodanHostInfo struct {
	IP           string         `json:"ip_str"`
	Hostnames    []string       `json:"hostnames"`
	Country      string         `json:"country_code"`
	City         string         `json:"city"`
	ISP          string         `json:"isp"`
	ASN          string         `json:"asn"`
	Org          string         `json:"org"`
	Data         []ShodanResult `json:"data"`
	OS           string         `json:"os"`
	Ports        []int          `json:"ports"`
	Vulns        []string       `json:"vulns,omitempty"`
	LastUpdate   string         `json:"last_update"`
	Tags         []string       `json:"tags,omitempty"`
}

// ShodanAPIInfo represents API plan information
type ShodanAPIInfo struct {
	QueryCredits int  `json:"query_credits"`
	ScanCredits  int  `json:"scan_credits"`
	Telnet       bool `json:"telnet"`
	Plan         string `json:"plan"`
	HTTPS        bool `json:"https"`
	Unlocked     bool `json:"unlocked"`
}

// NewShodanDiscoverer creates a new Shodan discoverer instance
func NewShodanDiscoverer(apiKey string) *ShodanDiscoverer {
	return &ShodanDiscoverer{
		apiKey:    apiKey,
		baseURL:   "https://api.shodan.io",
		userAgent: "ProxyHawk/2.0 (https://github.com/ResistanceIsUseless/ProxyHawk)",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the name of this discoverer
func (s *ShodanDiscoverer) Name() string {
	return "shodan"
}

// IsConfigured checks if the discoverer is properly configured
func (s *ShodanDiscoverer) IsConfigured() bool {
	return s.apiKey != ""
}

// Search performs a search query on Shodan
func (s *ShodanDiscoverer) Search(query string, limit int) (*DiscoveryResult, error) {
	if !s.IsConfigured() {
		return nil, errors.NewConfigError(errors.ErrorConfigNotFound, "Shodan API key not configured", nil)
	}

	start := time.Now()
	result := &DiscoveryResult{
		Query:     query,
		Source:    s.Name(),
		Timestamp: start,
		Metadata:  make(map[string]interface{}),
	}

	// Build search URL
	searchURL := fmt.Sprintf("%s/shodan/host/search", s.baseURL)
	params := url.Values{
		"key":   {s.apiKey},
		"query": {query},
		"limit": {strconv.Itoa(limit)},
	}

	fullURL := fmt.Sprintf("%s?%s", searchURL, params.Encode())

	// Make request
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", s.userAgent)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("shodan API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var searchResp ShodanSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert Shodan results to ProxyCandidate
	candidates := make([]ProxyCandidate, 0, len(searchResp.Matches))
	for _, match := range searchResp.Matches {
		candidate := s.convertToCandidate(match)
		if candidate != nil {
			candidates = append(candidates, *candidate)
		}
	}

	result.Duration = time.Since(start)
	result.Total = searchResp.Total
	result.Filtered = len(candidates)
	result.Candidates = candidates
	result.Metadata["shodan_total"] = searchResp.Total
	result.Metadata["query_credits_used"] = 1

	return result, nil
}

// GetDetails gets detailed information about a specific IP
func (s *ShodanDiscoverer) GetDetails(ip string) (*ProxyCandidate, error) {
	if !s.IsConfigured() {
		return nil, errors.NewConfigError(errors.ErrorConfigNotFound, "Shodan API key not configured", nil)
	}

	// Build host info URL
	hostURL := fmt.Sprintf("%s/shodan/host/%s", s.baseURL, ip)
	params := url.Values{
		"key": {s.apiKey},
	}

	fullURL := fmt.Sprintf("%s?%s", hostURL, params.Encode())

	// Make request
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", s.userAgent)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get host details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("shodan API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var hostInfo ShodanHostInfo
	if err := json.NewDecoder(resp.Body).Decode(&hostInfo); err != nil {
		return nil, fmt.Errorf("failed to decode host info: %w", err)
	}

	// Find the best proxy-related service
	var bestMatch *ShodanResult
	var bestScore float64

	for _, service := range hostInfo.Data {
		score := s.scoreProxyCandidate(service)
		if score > bestScore {
			bestScore = score
			bestMatch = &service
		}
	}

	if bestMatch == nil {
		return nil, fmt.Errorf("no proxy-related services found for IP %s", ip)
	}

	candidate := s.convertToCandidate(*bestMatch)
	if candidate != nil {
		// Add extra details from host info
		candidate.OpenPorts = hostInfo.Ports
		if len(hostInfo.Vulns) > 0 {
			candidate.ThreatTags = append(candidate.ThreatTags, "vulnerabilities")
		}
		if len(hostInfo.Tags) > 0 {
			candidate.ThreatTags = append(candidate.ThreatTags, hostInfo.Tags...)
		}
	}

	return candidate, nil
}

// GetRateLimit gets current API rate limit status
func (s *ShodanDiscoverer) GetRateLimit() (remaining int, resetTime time.Time, err error) {
	if !s.IsConfigured() {
		return 0, time.Time{}, errors.NewConfigError(errors.ErrorConfigNotFound, "Shodan API key not configured", nil)
	}

	// Get API info
	infoURL := fmt.Sprintf("%s/api-info?key=%s", s.baseURL, s.apiKey)

	req, err := http.NewRequest("GET", infoURL, nil)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", s.userAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("failed to get API info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, time.Time{}, fmt.Errorf("shodan API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiInfo ShodanAPIInfo
	if err := json.NewDecoder(resp.Body).Decode(&apiInfo); err != nil {
		return 0, time.Time{}, fmt.Errorf("failed to decode API info: %w", err)
	}

	// Shodan doesn't provide exact reset time, so estimate next month
	now := time.Now()
	nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())

	return apiInfo.QueryCredits, nextMonth, nil
}

// convertToCandidate converts a Shodan result to a ProxyCandidate
func (s *ShodanDiscoverer) convertToCandidate(result ShodanResult) *ProxyCandidate {
	// Parse timestamp
	lastSeen, _ := time.Parse("2006-01-02T15:04:05.000000", result.Timestamp)

	candidate := &ProxyCandidate{
		IP:           result.IP,
		Port:         result.Port,
		Protocol:     s.inferProtocol(result),
		Source:       s.Name(),
		LastSeen:     lastSeen,
		FirstSeen:    lastSeen,
		DiscoveryID:  fmt.Sprintf("shodan-%d", result.Hash),
		Country:      result.Location.CountryCode,
		City:         result.Location.City,
		ASN:          result.ASN,
		ASNOrg:       result.Org,
		ISP:          result.ISP,
		ServerHeader: s.extractServerHeader(result),
		ResponseSize: int64(len(result.Data)),
		TLSEnabled:   result.Port == 443 || result.Port == 8443 || strings.Contains(result.Data, "SSL"),
		Headers:      s.extractHeaders(result),
		ProxyHeaders: s.extractProxyHeaders(result),
		ProxyType:    s.inferProxyType(result),
		Confidence:   s.scoreProxyCandidate(result),
	}

	// Set hostname if available
	if len(result.Hostnames) > 0 {
		candidate.Hostname = result.Hostnames[0]
	}

	// Check for authentication requirements
	candidate.AuthRequired = strings.Contains(strings.ToLower(result.Data), "proxy-authenticate") ||
		strings.Contains(strings.ToLower(result.Data), "401 unauthorized")

	return candidate
}

// inferProtocol infers the proxy protocol from Shodan data
func (s *ShodanDiscoverer) inferProtocol(result ShodanResult) string {
	port := result.Port
	data := strings.ToLower(result.Data)

	// Check for SOCKS indicators
	if strings.Contains(data, "socks") {
		if port == 1080 || strings.Contains(data, "socks5") {
			return "socks5"
		}
		return "socks4"
	}

	// Check for HTTPS/TLS
	if port == 443 || port == 8443 || strings.Contains(data, "ssl") || strings.Contains(data, "tls") {
		return "https"
	}

	// Default to HTTP for common proxy ports
	if port == 3128 || port == 8080 || port == 8118 || port == 9050 {
		return "http"
	}

	// Check protocol from transport
	if result.Protocol == "tcp" {
		if strings.Contains(data, "http") {
			return "http"
		}
	}

	return "http" // Default assumption
}

// extractHeaders converts Shodan headers to map[string]string
func (s *ShodanDiscoverer) extractHeaders(result ShodanResult) map[string]string {
	headers := make(map[string]string)
	
	if result.Headers != nil {
		if headerMap, ok := result.Headers.(map[string]interface{}); ok {
			for key, value := range headerMap {
				if strValue, ok := value.(string); ok {
					headers[key] = strValue
				}
			}
		}
	}
	
	return headers
}

// extractServerHeader extracts the server header from Shodan data
func (s *ShodanDiscoverer) extractServerHeader(result ShodanResult) string {
	headers := s.extractHeaders(result)
	if server, ok := headers["server"]; ok {
		return server
	}

	// Try to extract from raw data
	lines := strings.Split(result.Data, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "server:") {
			return strings.TrimSpace(line[7:])
		}
	}

	// Use product/version if available
	if result.Product != "" {
		server := result.Product
		if result.Version != "" {
			server += "/" + result.Version
		}
		return server
	}

	return ""
}

// extractProxyHeaders extracts proxy-related headers from Shodan data
func (s *ShodanDiscoverer) extractProxyHeaders(result ShodanResult) []string {
	var proxyHeaders []string
	data := strings.ToLower(result.Data)

	// Look for common proxy headers
	proxyHeaderNames := []string{
		"via", "x-forwarded-for", "x-forwarded-proto", "x-forwarded-host",
		"x-real-ip", "x-proxy-authorization", "proxy-authorization",
		"x-cache", "x-cache-lookup", "x-served-by", "x-squid-error",
	}

	for _, headerName := range proxyHeaderNames {
		if strings.Contains(data, headerName+":") {
			proxyHeaders = append(proxyHeaders, headerName)
		}
	}

	return proxyHeaders
}

// inferProxyType infers the proxy software type from Shodan data
func (s *ShodanDiscoverer) inferProxyType(result ShodanResult) string {
	data := strings.ToLower(result.Data)
	product := strings.ToLower(result.Product)

	// Check product first
	if product != "" {
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
	server := s.extractServerHeader(result)
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

	// Check raw data for indicators
	if strings.Contains(data, "squid") {
		return "squid"
	}
	if strings.Contains(data, "x-cache") || strings.Contains(data, "x-squid") {
		return "squid"
	}
	if strings.Contains(data, "nginx") {
		return "nginx"
	}
	if strings.Contains(data, "apache") {
		return "apache"
	}
	if strings.Contains(data, "tinyproxy") {
		return "tinyproxy"
	}

	return "unknown"
}

// scoreProxyCandidate scores how likely this result is to be a working proxy
func (s *ShodanDiscoverer) scoreProxyCandidate(result ShodanResult) float64 {
	score := 0.0
	data := strings.ToLower(result.Data)

	// Port-based scoring
	switch result.Port {
	case 3128, 8080: // Common proxy ports
		score += 0.4
	case 1080: // SOCKS port
		score += 0.3
	case 8118, 9050: // Privoxy, Tor
		score += 0.2
	case 80, 8000, 8001, 8008: // HTTP ports that might be proxies
		score += 0.1
	default:
		if result.Port > 1024 && result.Port < 65535 {
			score += 0.05 // Non-standard ports get small bonus
		}
	}

	// Content-based scoring
	if strings.Contains(data, "proxy") {
		score += 0.3
	}
	if strings.Contains(data, "squid") {
		score += 0.3
	}
	if strings.Contains(data, "via:") || strings.Contains(data, "x-forwarded") {
		score += 0.2
	}
	if strings.Contains(data, "x-cache") {
		score += 0.2
	}
	if strings.Contains(data, "connect") && strings.Contains(data, "tunnel") {
		score += 0.2
	}
	if strings.Contains(data, "proxy-authorization") || strings.Contains(data, "proxy-authenticate") {
		score += 0.1
	}

	// Product-based scoring
	product := strings.ToLower(result.Product)
	if strings.Contains(product, "squid") || strings.Contains(product, "proxy") {
		score += 0.3
	}
	if strings.Contains(product, "nginx") || strings.Contains(product, "apache") {
		score += 0.1 // Could be reverse proxy
	}

	// Negative indicators
	if strings.Contains(data, "404 not found") {
		score -= 0.2
	}
	if strings.Contains(data, "connection refused") {
		score -= 0.3
	}
	if strings.Contains(data, "access denied") {
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

// Common Shodan search queries for finding proxies
var ShodanProxyQueries = []string{
	"Server: squid",                    // Squid proxy servers
	"\"Via:\" port:3128",              // Common proxy port with Via header
	"\"X-Cache\" port:8080",           // Caching proxies
	"\"proxy\" port:3128,8080,1080",   // Generic proxy search
	"\"HTTP/1.1 200 Connection established\"", // CONNECT method responses
	"\"Proxy-Authorization\"",          // Authenticated proxies
	"product:squid",                    // Squid product
	"port:1080 socks",                 // SOCKS proxies
	"\"tinyproxy\"",                   // TinyProxy
	"\"Privoxy\"",                     // Privoxy
	"nginx \"proxy_pass\"",            // Nginx reverse proxies
	"apache \"ProxyPass\"",            // Apache reverse proxies
}