package discovery

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// WebScraperDiscoverer implements web scraping for proxy discovery
type WebScraperDiscoverer struct {
	httpClient *http.Client
	userAgent  string
	sources    []ScrapingTarget
}

// ScrapingTarget represents a web scraping target
type ScrapingTarget struct {
	Name        string
	URL         string
	Parser      func([]byte) ([]ProxyCandidate, error)
	RateLimit   time.Duration
	LastAccess  time.Time
	MaxRetries  int
}

// NewWebScraperDiscoverer creates a new web scraper discoverer
func NewWebScraperDiscoverer() *WebScraperDiscoverer {
	w := &WebScraperDiscoverer{
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Initialize scraping targets
	w.sources = []ScrapingTarget{
		{
			Name:       "spysproxy",
			URL:        "http://spys.one/en/free-proxy-list/",
			Parser:     w.parseSpysProxyResponse,
			RateLimit:  15 * time.Second,
			MaxRetries: 2,
		},
		{
			Name:       "proxyscrape",
			URL:        "https://api.proxyscrape.com/v2/?request=get&protocol=http&timeout=10000&country=all&ssl=all&anonymity=all",
			Parser:     w.parseProxyScrapeResponse,
			RateLimit:  10 * time.Second,
			MaxRetries: 3,
		},
		{
			Name:       "freeproxylist",
			URL:        "https://free-proxy-list.net/",
			Parser:     w.parseFreeProxyListResponse,
			RateLimit:  20 * time.Second,
			MaxRetries: 2,
		},
		{
			Name:       "proxylistplus",
			URL:        "https://list.proxylistplus.com/Fresh-HTTP-Proxy-List-1",
			Parser:     w.parseProxyListPlusResponse,
			RateLimit:  12 * time.Second,
			MaxRetries: 2,
		},
	}

	return w
}

// Name returns the name of this discoverer
func (w *WebScraperDiscoverer) Name() string {
	return "webscraper"
}

// IsConfigured checks if the discoverer is properly configured
func (w *WebScraperDiscoverer) IsConfigured() bool {
	return true // No configuration required
}

// Search performs web scraping across configured targets
func (w *WebScraperDiscoverer) Search(query string, limit int) (*DiscoveryResult, error) {
	start := time.Now()
	result := &DiscoveryResult{
		Query:     query,
		Source:    w.Name(),
		Timestamp: start,
		Metadata:  make(map[string]interface{}),
	}

	var allCandidates []ProxyCandidate
	var allErrors []string

	// Scrape each target
	for _, target := range w.sources {
		// Respect rate limiting
		if time.Since(target.LastAccess) < target.RateLimit {
			time.Sleep(target.RateLimit - time.Since(target.LastAccess))
		}

		candidates, err := w.scrapeTarget(target)
		if err != nil {
			errMsg := fmt.Sprintf("%s: %v", target.Name, err)
			allErrors = append(allErrors, errMsg)
			continue
		}

		allCandidates = append(allCandidates, candidates...)
		target.LastAccess = time.Now()

		// Update metadata
		result.Metadata[target.Name+"_results"] = len(candidates)
	}

	// Apply filtering based on query
	filtered := w.filterByQuery(allCandidates, query)

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

// GetDetails gets detailed information about a specific IP (not supported)
func (w *WebScraperDiscoverer) GetDetails(ip string) (*ProxyCandidate, error) {
	return nil, fmt.Errorf("detailed IP lookup not supported for web scraping")
}

// GetRateLimit gets current rate limit status (not applicable for web scraping)
func (w *WebScraperDiscoverer) GetRateLimit() (remaining int, resetTime time.Time, err error) {
	// Web scrapers have built-in rate limiting but no API limits
	return 999999, time.Now().Add(24*time.Hour), nil
}

// scrapeTarget scrapes a specific target with retry logic
func (w *WebScraperDiscoverer) scrapeTarget(target ScrapingTarget) ([]ProxyCandidate, error) {
	var lastErr error
	
	for attempt := 0; attempt <= target.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(attempt) * 2 * time.Second
			time.Sleep(backoff)
		}

		candidates, err := w.attemptScrape(target)
		if err == nil {
			return candidates, nil
		}
		
		lastErr = err
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", target.MaxRetries+1, lastErr)
}

// attemptScrape performs a single scraping attempt
func (w *WebScraperDiscoverer) attemptScrape(target ScrapingTarget) ([]ProxyCandidate, error) {
	req, err := http.NewRequest("GET", target.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers to mimic a real browser
	req.Header.Set("User-Agent", w.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "no-cache")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from %s: %w", target.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", target.Name, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from %s: %w", target.Name, err)
	}

	// Parse using target-specific parser
	candidates, err := target.Parser(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response from %s: %w", target.Name, err)
	}

	return candidates, nil
}

// parseSpysProxyResponse parses HTML from spys.one
func (w *WebScraperDiscoverer) parseSpysProxyResponse(body []byte) ([]ProxyCandidate, error) {
	var candidates []ProxyCandidate
	text := string(body)

	// Look for proxy entries in HTML tables
	// Spys.one uses a specific format with IP:PORT in table cells
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
			Protocol:    w.guessProtocolFromPort(port),
			Source:      "spysproxy",
			LastSeen:    time.Now(),
			FirstSeen:   time.Now(),
			DiscoveryID: fmt.Sprintf("spysproxy-%s-%d", ip, port),
			Confidence:  0.4, // Medium confidence
		}

		candidates = append(candidates, candidate)
	}

	return candidates, nil
}

// parseProxyScrapeResponse parses text response from ProxyScrape API
func (w *WebScraperDiscoverer) parseProxyScrapeResponse(body []byte) ([]ProxyCandidate, error) {
	var candidates []ProxyCandidate
	text := string(body)
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Expected format: IP:PORT
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		port, err := strconv.Atoi(parts[1])
		if err != nil || port < 1 || port > 65535 {
			continue
		}

		candidate := ProxyCandidate{
			IP:          parts[0],
			Port:        port,
			Protocol:    "http", // ProxyScrape typically provides HTTP
			Source:      "proxyscrape",
			LastSeen:    time.Now(),
			FirstSeen:   time.Now(),
			DiscoveryID: fmt.Sprintf("proxyscrape-%s-%d", parts[0], port),
			Confidence:  0.5, // Higher confidence for API
		}

		candidates = append(candidates, candidate)
	}

	return candidates, nil
}

// parseFreeProxyListResponse parses HTML from free-proxy-list.net
func (w *WebScraperDiscoverer) parseFreeProxyListResponse(body []byte) ([]ProxyCandidate, error) {
	var candidates []ProxyCandidate
	text := string(body)

	// Extract table rows with proxy data
	// Look for IP addresses followed by ports in table format
	tableRegex := regexp.MustCompile(`<tr><td>(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})</td><td>(\d{1,5})</td>`)
	matches := tableRegex.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) != 3 {
			continue
		}

		ip := match[1]
		portStr := match[2]

		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			continue
		}

		candidate := ProxyCandidate{
			IP:          ip,
			Port:        port,
			Protocol:    w.guessProtocolFromPort(port),
			Source:      "freeproxylist",
			LastSeen:    time.Now(),
			FirstSeen:   time.Now(),
			DiscoveryID: fmt.Sprintf("freeproxylist-%s-%d", ip, port),
			Confidence:  0.35, // Medium-low confidence
		}

		// Try to extract additional info from the same table row
		if strings.Contains(text, "yes</td>") { // HTTPS support
			candidate.TLSEnabled = true
			candidate.Protocol = "https"
		}

		candidates = append(candidates, candidate)
	}

	return candidates, nil
}

// parseProxyListPlusResponse parses HTML from proxylistplus.com
func (w *WebScraperDiscoverer) parseProxyListPlusResponse(body []byte) ([]ProxyCandidate, error) {
	var candidates []ProxyCandidate
	text := string(body)

	// Look for IP:PORT patterns in various HTML structures
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
			Protocol:    w.guessProtocolFromPort(port),
			Source:      "proxylistplus",
			LastSeen:    time.Now(),
			FirstSeen:   time.Now(),
			DiscoveryID: fmt.Sprintf("proxylistplus-%s-%d", ip, port),
			Confidence:  0.3, // Lower confidence
		}

		candidates = append(candidates, candidate)
	}

	return candidates, nil
}

// guessProtocolFromPort makes an educated guess about protocol from port
func (w *WebScraperDiscoverer) guessProtocolFromPort(port int) string {
	switch port {
	case 443, 8443:
		return "https"
	case 1080:
		return "socks5"
	case 1085:
		return "socks4"
	case 3128, 8080:
		return "http"
	default:
		return "http"
	}
}

// filterByQuery applies basic filtering based on search query
func (w *WebScraperDiscoverer) filterByQuery(candidates []ProxyCandidate, query string) []ProxyCandidate {
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

		// Generic terms
		if strings.Contains(queryLower, "proxy") || strings.Contains(queryLower, "anonymous") {
			match = true
		}

		// Default inclusion for broad queries
		if !strings.Contains(queryLower, "http") && 
		   !strings.Contains(queryLower, "socks") && 
		   !strings.Contains(queryLower, "3128") && 
		   !strings.Contains(queryLower, "8080") && 
		   !strings.Contains(queryLower, "1080") {
			match = true
		}

		if match {
			filtered = append(filtered, candidate)
		}
	}

	return filtered
}

// Common web scraping queries
var WebScraperProxyQueries = []string{
	"proxy",               // Generic proxy search
	"http proxy",          // HTTP proxies
	"https proxy",         // HTTPS proxies
	"socks proxy",         // SOCKS proxies
	"anonymous proxy",     // Anonymous proxies
	"elite proxy",         // Elite proxies
	"fresh proxy",         // Recently updated proxies
	"working proxy",       // Active proxies
	"fast proxy",          // High-speed proxies
	"free proxy",          // Free public proxies
}