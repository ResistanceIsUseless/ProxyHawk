package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// FreeListsDiscoverer implements the Discoverer interface for free proxy lists
type FreeListsDiscoverer struct {
	httpClient *http.Client
	userAgent  string
	sources    []FreeListSource
}

// FreeListSource represents a free proxy list source
type FreeListSource struct {
	Name        string
	URL         string
	Format      string // "json", "text", "html"
	Parser      func([]byte) ([]ProxyCandidate, error)
	RateLimit   time.Duration
	LastAccess  time.Time
}

// GeonodeProxy represents a proxy from ProxyList.geonode.com
type GeonodeProxy struct {
	IP            string `json:"ip"`
	Port          string `json:"port"`
	Country       string `json:"country"`
	ResponseTime  int    `json:"responseTime"`
	Uptime        int    `json:"uptime"`
	LastChecked   string `json:"lastChecked"`
	Protocols     []string `json:"protocols"`
	Anonymity     string      `json:"anonymity"`
	SSL           interface{} `json:"https"` // Can be string or bool
	Google        interface{} `json:"google"` // Can be string or bool
	IPType        string `json:"ipType"`
	ISP           string `json:"isp"`
	ASN           string `json:"asn"`
	ASNName       string `json:"asnName"`
	OrganisationName string `json:"organisationName"`
	City          string `json:"city"`
	Region        string `json:"region"`
	Speed         int    `json:"speed"`
}

// GeonodeResponse represents the API response from ProxyList.geonode.com
type GeonodeResponse struct {
	Data  []GeonodeProxy `json:"data"`
	Total int           `json:"total"`
	Page  int           `json:"page"`
	Limit int           `json:"limit"`
}

// FreeProxyWorldEntry represents a proxy from FreeProxy.world
type FreeProxyWorldEntry struct {
	IP       string
	Port     int
	Country  string
	Protocol string
	Uptime   string
	Speed    string
}

// NewFreeListsDiscoverer creates a new free lists discoverer
func NewFreeListsDiscoverer() *FreeListsDiscoverer {
	f := &FreeListsDiscoverer{
		userAgent: "ProxyHawk/2.0 (https://github.com/ResistanceIsUseless/ProxyHawk)",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Initialize sources
	f.sources = []FreeListSource{
		{
			Name:      "proxylist-geonode",
			URL:       "https://proxylist.geonode.com/api/proxy-list",
			Format:    "json",
			Parser:    f.parseGeonodeResponse,
			RateLimit: 5 * time.Second, // Be respectful
		},
		{
			Name:      "freeproxy-world",
			URL:       "https://www.proxy-list.download/api/v1/get",
			Format:    "text",
			Parser:    f.parseFreeProxyWorldResponse,
			RateLimit: 3 * time.Second,
		},
		{
			Name:      "proxy-daily",
			URL:       "https://proxy-daily.com/",
			Format:    "html",
			Parser:    f.parseProxyDailyResponse,
			RateLimit: 10 * time.Second,
		},
	}

	return f
}

// Name returns the name of this discoverer
func (f *FreeListsDiscoverer) Name() string {
	return "freelists"
}

// IsConfigured checks if the discoverer is properly configured
func (f *FreeListsDiscoverer) IsConfigured() bool {
	return true // No API keys required for free lists
}

// Search performs a search query across free proxy lists
func (f *FreeListsDiscoverer) Search(query string, limit int) (*DiscoveryResult, error) {
	start := time.Now()
	result := &DiscoveryResult{
		Query:     query,
		Source:    f.Name(),
		Timestamp: start,
		Metadata:  make(map[string]interface{}),
	}

	var allCandidates []ProxyCandidate
	var allErrors []string

	// Search each source
	for _, source := range f.sources {
		// Respect rate limiting
		if time.Since(source.LastAccess) < source.RateLimit {
			time.Sleep(source.RateLimit - time.Since(source.LastAccess))
		}

		candidates, err := f.searchSource(source, query, limit)
		if err != nil {
			errMsg := fmt.Sprintf("%s: %v", source.Name, err)
			allErrors = append(allErrors, errMsg)
			continue
		}

		allCandidates = append(allCandidates, candidates...)
		source.LastAccess = time.Now()

		// Update metadata
		result.Metadata[source.Name+"_results"] = len(candidates)
	}

	// Apply basic filtering based on query
	filtered := f.filterByQuery(allCandidates, query)

	// Limit results
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	result.Duration = time.Since(start)
	result.Total = len(allCandidates)
	result.Filtered = len(filtered)
	result.Candidates = filtered
	result.Errors = allErrors

	return result, nil
}

// GetDetails gets detailed information about a specific IP (not supported for free lists)
func (f *FreeListsDiscoverer) GetDetails(ip string) (*ProxyCandidate, error) {
	return nil, fmt.Errorf("detailed IP lookup not supported for free proxy lists")
}

// GetRateLimit gets current rate limit status (not applicable for free lists)
func (f *FreeListsDiscoverer) GetRateLimit() (remaining int, resetTime time.Time, err error) {
	// Free lists don't have traditional rate limits, return unlimited
	return 999999, time.Now().Add(24*time.Hour), nil
}

// searchSource searches a specific free proxy list source
func (f *FreeListsDiscoverer) searchSource(source FreeListSource, query string, limit int) ([]ProxyCandidate, error) {
	// Build request URL with parameters
	reqURL := source.URL
	if source.Name == "proxylist-geonode" {
		params := url.Values{
			"limit": {strconv.Itoa(limit)},
			"page":  {"1"},
			"sort_by": {"lastChecked"},
			"sort_type": {"desc"},
		}
		
		// Add filters based on query
		if strings.Contains(strings.ToLower(query), "anonymous") {
			params.Set("anonymityLevel", "anonymous")
		}
		if strings.Contains(strings.ToLower(query), "https") {
			params.Set("protocols", "https")
		}
		if strings.Contains(strings.ToLower(query), "socks") {
			params.Set("protocols", "socks4,socks5")
		}
		
		reqURL += "?" + params.Encode()
	} else if source.Name == "freeproxy-world" {
		params := url.Values{
			"type": {"http"},
			"anon": {"yes"},
			"count": {strconv.Itoa(limit)},
		}
		reqURL += "?" + params.Encode()
	}

	// Make request
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from %s: %w", source.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", source.Name, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from %s: %w", source.Name, err)
	}

	// Parse response using source-specific parser
	candidates, err := source.Parser(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response from %s: %w", source.Name, err)
	}

	return candidates, nil
}

// parseGeonodeResponse parses the response from ProxyList.geonode.com
func (f *FreeListsDiscoverer) parseGeonodeResponse(body []byte) ([]ProxyCandidate, error) {
	var response GeonodeResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal geonode response: %w", err)
	}

	var candidates []ProxyCandidate
	for _, proxy := range response.Data {
		port, err := strconv.Atoi(proxy.Port)
		if err != nil {
			continue // Skip invalid ports
		}

		lastChecked, _ := time.Parse("2006-01-02 15:04:05", proxy.LastChecked)

		candidate := ProxyCandidate{
			IP:           proxy.IP,
			Port:         port,
			Protocol:     f.determineProtocolFromGeonode(proxy),
			Source:       "proxylist-geonode",
			LastSeen:     lastChecked,
			FirstSeen:    lastChecked,
			DiscoveryID:  fmt.Sprintf("geonode-%s-%d", proxy.IP, port),
			Country:      proxy.Country,
			City:         proxy.City,
			ASN:          proxy.ASN,
			ASNOrg:       proxy.ASNName,
			ISP:          proxy.ISP,
			TLSEnabled:   f.parseSSLFlag(proxy.SSL),
			Confidence:   f.calculateGeonodeScore(proxy),
		}

		// Set proxy type based on detected info
		if len(proxy.Protocols) > 0 {
			candidate.ProxyType = strings.Join(proxy.Protocols, ",")
		}

		candidates = append(candidates, candidate)
	}

	return candidates, nil
}

// parseFreeProxyWorldResponse parses the response from FreeProxy.world
func (f *FreeListsDiscoverer) parseFreeProxyWorldResponse(body []byte) ([]ProxyCandidate, error) {
	var candidates []ProxyCandidate
	text := string(body)
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Expected format: IP:PORT
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		port, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}

		candidate := ProxyCandidate{
			IP:          parts[0],
			Port:        port,
			Protocol:    "http", // FreeProxy.world typically provides HTTP proxies
			Source:      "freeproxy-world",
			LastSeen:    time.Now(),
			FirstSeen:   time.Now(),
			DiscoveryID: fmt.Sprintf("freeproxyworld-%s-%d", parts[0], port),
			Confidence:  0.3, // Lower confidence for free lists
		}

		candidates = append(candidates, candidate)
	}

	return candidates, nil
}

// parseProxyDailyResponse parses HTML response from proxy-daily.com
func (f *FreeListsDiscoverer) parseProxyDailyResponse(body []byte) ([]ProxyCandidate, error) {
	var candidates []ProxyCandidate
	text := string(body)

	// Use regex to extract IP:PORT patterns from HTML
	ipPortRegex := regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):(\d{1,5})`)
	matches := ipPortRegex.FindAllStringSubmatch(text, -1)

	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}

		ip := match[1]
		portStr := match[2]
		key := ip + ":" + portStr

		// Avoid duplicates
		if seen[key] {
			continue
		}
		seen[key] = true

		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			continue
		}

		candidate := ProxyCandidate{
			IP:          ip,
			Port:        port,
			Protocol:    f.guessProtocolFromPort(port),
			Source:      "proxy-daily",
			LastSeen:    time.Now(),
			FirstSeen:   time.Now(),
			DiscoveryID: fmt.Sprintf("proxydaily-%s-%d", ip, port),
			Confidence:  0.25, // Lower confidence for scraped data
		}

		candidates = append(candidates, candidate)
	}

	return candidates, nil
}

// determineProtocolFromGeonode determines protocol from Geonode proxy data
func (f *FreeListsDiscoverer) determineProtocolFromGeonode(proxy GeonodeProxy) string {
	for _, protocol := range proxy.Protocols {
		protocolLower := strings.ToLower(protocol)
		if protocolLower == "socks5" || protocolLower == "socks4" {
			return protocolLower
		}
		if protocolLower == "https" {
			return "https"
		}
	}
	return "http" // Default
}

// guessProtocolFromPort makes an educated guess about protocol from port
func (f *FreeListsDiscoverer) guessProtocolFromPort(port int) string {
	switch port {
	case 443, 8443:
		return "https"
	case 1080:
		return "socks5"
	case 1085:
		return "socks4"
	default:
		return "http"
	}
}

// parseSSLFlag parses SSL flag which can be string or bool
func (f *FreeListsDiscoverer) parseSSLFlag(sslFlag interface{}) bool {
	if sslFlag == nil {
		return false
	}
	
	switch v := sslFlag.(type) {
	case bool:
		return v
	case string:
		return v == "yes" || v == "true" || v == "1"
	default:
		return false
	}
}

// calculateGeonodeScore calculates confidence score for Geonode proxies
func (f *FreeListsDiscoverer) calculateGeonodeScore(proxy GeonodeProxy) float64 {
	score := 0.3 // Base score for free lists

	// Uptime scoring (higher is better)
	if proxy.Uptime > 80 {
		score += 0.3
	} else if proxy.Uptime > 50 {
		score += 0.2
	} else if proxy.Uptime > 20 {
		score += 0.1
	}

	// Response time scoring (lower is better)
	if proxy.ResponseTime < 1000 { // Less than 1 second
		score += 0.2
	} else if proxy.ResponseTime < 3000 { // Less than 3 seconds
		score += 0.1
	}

	// Speed scoring
	if proxy.Speed > 50 {
		score += 0.1
	}

	// Anonymity bonus
	if strings.ToLower(proxy.Anonymity) == "anonymous" || 
	   strings.ToLower(proxy.Anonymity) == "elite" {
		score += 0.1
	}

	// SSL support bonus
	if f.parseSSLFlag(proxy.SSL) {
		score += 0.1
	}

	// Recently checked bonus
	if lastChecked, err := time.Parse("2006-01-02 15:04:05", proxy.LastChecked); err == nil {
		if time.Since(lastChecked) < 24*time.Hour {
			score += 0.1
		}
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

// filterByQuery applies basic filtering based on search query
func (f *FreeListsDiscoverer) filterByQuery(candidates []ProxyCandidate, query string) []ProxyCandidate {
	if query == "" {
		return candidates
	}

	var filtered []ProxyCandidate
	queryLower := strings.ToLower(query)

	for _, candidate := range candidates {
		match := false

		// Protocol matching
		if strings.Contains(queryLower, "http") && strings.Contains(candidate.Protocol, "http") {
			match = true
		}
		if strings.Contains(queryLower, "socks") && strings.Contains(candidate.Protocol, "socks") {
			match = true
		}
		if strings.Contains(queryLower, "https") && candidate.Protocol == "https" {
			match = true
		}

		// Country matching
		if strings.Contains(queryLower, strings.ToLower(candidate.Country)) {
			match = true
		}

		// Port matching
		if strings.Contains(queryLower, "3128") && candidate.Port == 3128 {
			match = true
		}
		if strings.Contains(queryLower, "8080") && candidate.Port == 8080 {
			match = true
		}
		if strings.Contains(queryLower, "1080") && candidate.Port == 1080 {
			match = true
		}

		// Generic proxy terms
		if strings.Contains(queryLower, "proxy") || strings.Contains(queryLower, "anonymous") {
			match = true
		}

		// If no specific filters, include all
		if !strings.Contains(queryLower, "http") && 
		   !strings.Contains(queryLower, "socks") && 
		   !strings.Contains(queryLower, "3128") && 
		   !strings.Contains(queryLower, "8080") && 
		   !strings.Contains(queryLower, "1080") &&
		   candidate.Country == "" {
			match = true
		}

		if match {
			filtered = append(filtered, candidate)
		}
	}

	return filtered
}

// Common free proxy list queries
var FreeListProxyQueries = []string{
	"http proxy",           // Generic HTTP proxies
	"https proxy",          // HTTPS proxies
	"socks proxy",          // SOCKS proxies
	"anonymous proxy",      // Anonymous proxies
	"elite proxy",          // Elite anonymity proxies
	"US proxy",            // US-based proxies
	"European proxy",      // European proxies
	"fast proxy",          // High-speed proxies
	"port:3128",           // Common proxy port
	"port:8080",           // Alternative proxy port
	"port:1080",           // SOCKS port
}