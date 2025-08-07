package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// GeographicTester performs DNS and HTTP tests across different regions
type GeographicTester struct {
	poolManager *ProxyPoolManager
	dnsCache    *DNSCache
	config      RoundRobinConfig
	
	// HTTP client per region
	clients map[string]*http.Client
	mutex   sync.RWMutex
}

// GeoTestResult represents the result of geographic testing
type GeoTestResult struct {
	Domain                   string                 `json:"domain"`
	TestedAt                 time.Time             `json:"tested_at"`
	HasGeographicDifferences bool                  `json:"has_geographic_differences"`
	IsRoundRobin            bool                  `json:"is_round_robin"`
	Confidence              float64               `json:"confidence"`
	RegionResults           map[string]*RegionResult `json:"region_results"`
	Summary                 *TestSummary          `json:"summary"`
}

// RegionResult represents test results for a specific region
type RegionResult struct {
	Region      string        `json:"region"`
	ProxyUsed   string        `json:"proxy_used"`
	DNSResults  []DNSResult   `json:"dns_results"`
	HTTPResults []HTTPResult  `json:"http_results"`
	ResponseTime time.Duration `json:"response_time"`
	Success     bool          `json:"success"`
	Error       string        `json:"error,omitempty"`
}

// DNSResult represents a DNS lookup result
type DNSResult struct {
	QueryTime time.Time `json:"query_time"`
	IP        string    `json:"ip"`
	TTL       uint32    `json:"ttl"`
	Type      string    `json:"type"` // A, AAAA, CNAME, etc.
}

// HTTPResult represents an HTTP test result
type HTTPResult struct {
	RequestTime  time.Time         `json:"request_time"`
	StatusCode   int              `json:"status_code"`
	ResponseTime time.Duration    `json:"response_time"`
	Headers      map[string]string `json:"headers"`
	ServerHeader string           `json:"server_header"`
	ContentHash  string           `json:"content_hash"`
	ContentSize  int64            `json:"content_size"`
	RemoteAddr   string           `json:"remote_addr"`
}

// TestSummary provides a summary of the test results
type TestSummary struct {
	UniqueIPs        []string          `json:"unique_ips"`
	UniqueServers    []string          `json:"unique_servers"`
	ResponseTimeDiff time.Duration     `json:"response_time_diff"`
	ContentVariations map[string]int   `json:"content_variations"`
	GeographicSpread bool             `json:"geographic_spread"`
}

// NewGeographicTester creates a new geographic tester
func NewGeographicTester(poolManager *ProxyPoolManager, dnsCache *DNSCache, config RoundRobinConfig) *GeographicTester {
	return &GeographicTester{
		poolManager: poolManager,
		dnsCache:    dnsCache,
		config:      config,
		clients:     make(map[string]*http.Client),
	}
}

// TestDomain performs comprehensive geographic testing for a domain
func (gt *GeographicTester) TestDomain(domain string, regions []string) *GeoTestResult {
	result := &GeoTestResult{
		Domain:        domain,
		TestedAt:      time.Now(),
		RegionResults: make(map[string]*RegionResult),
	}
	
	// Test each region
	var wg sync.WaitGroup
	resultChan := make(chan *RegionResult, len(regions))
	
	for _, region := range regions {
		wg.Add(1)
		go func(r string) {
			defer wg.Done()
			regionResult := gt.testRegion(domain, r)
			resultChan <- regionResult
		}(region)
	}
	
	// Wait for all tests to complete
	wg.Wait()
	close(resultChan)
	
	// Collect results
	for regionResult := range resultChan {
		if regionResult != nil {
			result.RegionResults[regionResult.Region] = regionResult
		}
	}
	
	// Analyze results
	gt.analyzeResults(result)
	
	return result
}

// TestDomainsBatch performs batch testing of multiple domains
func (gt *GeographicTester) TestDomainsBatch(domains []string, regions []string) []*GeoTestResult {
	results := make([]*GeoTestResult, 0, len(domains))
	resultsChan := make(chan *GeoTestResult, len(domains))
	
	// Process domains concurrently
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // Limit concurrent tests
	
	for _, domain := range domains {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			result := gt.TestDomain(d, regions)
			resultsChan <- result
		}(domain)
	}
	
	// Wait for completion
	go func() {
		wg.Wait()
		close(resultsChan)
	}()
	
	// Collect results
	for result := range resultsChan {
		results = append(results, result)
	}
	
	return results
}

// testRegion tests a domain from a specific region
func (gt *GeographicTester) testRegion(domain, region string) *RegionResult {
	result := &RegionResult{
		Region:      region,
		DNSResults:  make([]DNSResult, 0),
		HTTPResults: make([]HTTPResult, 0),
	}
	
	start := time.Now()
	
	// Get proxy for region
	proxy := gt.poolManager.GetHealthyProxy(region)
	if proxy == nil {
		result.Error = fmt.Sprintf("No healthy proxy available for region %s", region)
		return result
	}
	
	result.ProxyUsed = proxy.URL
	
	// Get or create HTTP client for this region
	client := gt.getHTTPClient(region, proxy.URL)
	if client == nil {
		result.Error = "Failed to create HTTP client"
		return result
	}
	
	// Perform DNS lookups if configured
	if gt.config.Enabled {
		gt.performDNSTests(domain, result, client)
	}
	
	// Perform HTTP tests
	gt.performHTTPTests(domain, result, client)
	
	result.ResponseTime = time.Since(start)
	result.Success = result.Error == ""
	
	return result
}

// performDNSTests performs DNS resolution tests
func (gt *GeographicTester) performDNSTests(domain string, result *RegionResult, client *http.Client) {
	// Perform multiple DNS lookups to detect round-robin
	for i := 0; i < gt.config.MinSamples; i++ {
		ips, err := net.LookupIP(domain)
		if err != nil {
			if result.Error == "" {
				result.Error = fmt.Sprintf("DNS lookup failed: %v", err)
			}
			continue
		}
		
		for _, ip := range ips {
			dnsResult := DNSResult{
				QueryTime: time.Now(),
				IP:        ip.String(),
				Type:      "A",
			}
			
			if ip.To4() == nil {
				dnsResult.Type = "AAAA"
			}
			
			result.DNSResults = append(result.DNSResults, dnsResult)
		}
		
		// Add delay between samples
		if i < gt.config.MinSamples-1 {
			time.Sleep(gt.config.SampleInterval)
		}
	}
}

// performHTTPTests performs HTTP tests
func (gt *GeographicTester) performHTTPTests(domain string, result *RegionResult, client *http.Client) {
	// Test HTTP
	gt.performSingleHTTPTest(fmt.Sprintf("http://%s", domain), result, client)
	
	// Test HTTPS
	gt.performSingleHTTPTest(fmt.Sprintf("https://%s", domain), result, client)
}

// performSingleHTTPTest performs a single HTTP test
func (gt *GeographicTester) performSingleHTTPTest(url string, result *RegionResult, client *http.Client) {
	start := time.Now()
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		if result.Error == "" {
			result.Error = fmt.Sprintf("Failed to create request: %v", err)
		}
		return
	}
	
	// Add headers to identify the request
	req.Header.Set("User-Agent", "ProxyHawk-Geographic-Agent/1.0")
	req.Header.Set("X-ProxyHawk-Test", "geographic")
	
	// Perform request with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		if result.Error == "" {
			result.Error = fmt.Sprintf("HTTP request failed: %v", err)
		}
		return
	}
	defer resp.Body.Close()
	
	// Read response body for content analysis
	body := make([]byte, 1024) // Read first 1KB
	n, _ := resp.Body.Read(body)
	
	// Create HTTP result
	httpResult := HTTPResult{
		RequestTime:  start,
		StatusCode:   resp.StatusCode,
		ResponseTime: time.Since(start),
		Headers:      make(map[string]string),
		ServerHeader: resp.Header.Get("Server"),
		ContentSize:  int64(n),
		RemoteAddr:   "", // Will be populated if available
	}
	
	// Copy important headers
	importantHeaders := []string{"Server", "X-Cache", "CF-Cache-Status", "X-Served-By", "X-Cache-Hits"}
	for _, header := range importantHeaders {
		if value := resp.Header.Get(header); value != "" {
			httpResult.Headers[header] = value
		}
	}
	
	// Calculate content hash for comparison
	if n > 0 {
		httpResult.ContentHash = fmt.Sprintf("%x", body[:n])
	}
	
	result.HTTPResults = append(result.HTTPResults, httpResult)
}

// getHTTPClient gets or creates an HTTP client for a region
func (gt *GeographicTester) getHTTPClient(region, proxyURL string) *http.Client {
	gt.mutex.RLock()
	client, exists := gt.clients[region]
	gt.mutex.RUnlock()
	
	if exists {
		return client
	}
	
	// Create new client with proxy
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// This would use the proxy - simplified for now
			return net.Dial(network, addr)
		},
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	
	client = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
	
	gt.mutex.Lock()
	gt.clients[region] = client
	gt.mutex.Unlock()
	
	return client
}

// analyzeResults analyzes the test results to determine geographic differences
func (gt *GeographicTester) analyzeResults(result *GeoTestResult) {
	if len(result.RegionResults) < 2 {
		return
	}
	
	summary := &TestSummary{
		UniqueIPs:         make([]string, 0),
		UniqueServers:     make([]string, 0),
		ContentVariations: make(map[string]int),
	}
	
	ipSet := make(map[string]bool)
	serverSet := make(map[string]bool)
	contentHashes := make(map[string]int)
	
	var minResponseTime, maxResponseTime time.Duration
	firstResult := true
	
	// Analyze each region's results
	for _, regionResult := range result.RegionResults {
		if !regionResult.Success {
			continue
		}
		
		// Collect unique IPs from DNS results
		for _, dns := range regionResult.DNSResults {
			if !ipSet[dns.IP] {
				ipSet[dns.IP] = true
				summary.UniqueIPs = append(summary.UniqueIPs, dns.IP)
			}
		}
		
		// Collect unique servers from HTTP results
		for _, http := range regionResult.HTTPResults {
			if http.ServerHeader != "" && !serverSet[http.ServerHeader] {
				serverSet[http.ServerHeader] = true
				summary.UniqueServers = append(summary.UniqueServers, http.ServerHeader)
			}
			
			// Track content variations
			if http.ContentHash != "" {
				contentHashes[http.ContentHash]++
			}
			
			// Track response time differences
			if firstResult {
				minResponseTime = http.ResponseTime
				maxResponseTime = http.ResponseTime
				firstResult = false
			} else {
				if http.ResponseTime < minResponseTime {
					minResponseTime = http.ResponseTime
				}
				if http.ResponseTime > maxResponseTime {
					maxResponseTime = http.ResponseTime
				}
			}
		}
	}
	
	// Set content variations
	summary.ContentVariations = contentHashes
	summary.ResponseTimeDiff = maxResponseTime - minResponseTime
	
	// Determine if there are geographic differences
	hasIPDifferences := len(summary.UniqueIPs) > 1
	hasServerDifferences := len(summary.UniqueServers) > 1
	hasContentDifferences := len(summary.ContentVariations) > 1
	hasSignificantLatencyDiff := summary.ResponseTimeDiff > 500*time.Millisecond
	
	result.HasGeographicDifferences = hasIPDifferences || hasServerDifferences || hasContentDifferences || hasSignificantLatencyDiff
	summary.GeographicSpread = result.HasGeographicDifferences
	
	// Detect round-robin DNS
	result.IsRoundRobin = gt.detectRoundRobin(result.RegionResults)
	
	// Calculate confidence
	result.Confidence = gt.calculateConfidence(result)
	
	result.Summary = summary
}

// detectRoundRobin detects if the domain uses round-robin DNS
func (gt *GeographicTester) detectRoundRobin(regionResults map[string]*RegionResult) bool {
	if !gt.config.Enabled {
		return false
	}
	
	totalSamples := 0
	ipVariations := make(map[string]int)
	
	// Count IP variations across all regions
	for _, result := range regionResults {
		for _, dns := range result.DNSResults {
			ipVariations[dns.IP]++
			totalSamples++
		}
	}
	
	if totalSamples < gt.config.MinSamples || len(ipVariations) < 2 {
		return false
	}
	
	// Calculate distribution variance
	expectedPerIP := float64(totalSamples) / float64(len(ipVariations))
	variance := 0.0
	
	for _, count := range ipVariations {
		diff := float64(count) - expectedPerIP
		variance += diff * diff
	}
	variance /= float64(len(ipVariations))
	
	// If variance is low, it suggests round-robin behavior
	maxExpectedVariance := expectedPerIP * 0.3 // 30% tolerance
	return variance <= maxExpectedVariance
}

// calculateConfidence calculates confidence in the test results
func (gt *GeographicTester) calculateConfidence(result *GeoTestResult) float64 {
	confidence := 0.0
	totalRegions := len(result.RegionResults)
	successfulRegions := 0
	
	// Base confidence on successful tests
	for _, regionResult := range result.RegionResults {
		if regionResult.Success {
			successfulRegions++
		}
	}
	
	if totalRegions == 0 {
		return 0.0
	}
	
	// Success rate contributes up to 60% confidence
	successRate := float64(successfulRegions) / float64(totalRegions)
	confidence += successRate * 0.6
	
	// Number of samples contributes up to 20% confidence
	totalSamples := 0
	for _, regionResult := range result.RegionResults {
		totalSamples += len(regionResult.DNSResults) + len(regionResult.HTTPResults)
	}
	
	sampleScore := float64(totalSamples) / float64(gt.config.MinSamples*totalRegions*2) // DNS + HTTP per region
	if sampleScore > 1.0 {
		sampleScore = 1.0
	}
	confidence += sampleScore * 0.2
	
	// Clear differences contribute up to 20% confidence
	if result.HasGeographicDifferences {
		diffScore := 0.0
		if len(result.Summary.UniqueIPs) > 1 {
			diffScore += 0.5
		}
		if len(result.Summary.UniqueServers) > 1 {
			diffScore += 0.3
		}
		if len(result.Summary.ContentVariations) > 1 {
			diffScore += 0.2
		}
		
		if diffScore > 1.0 {
			diffScore = 1.0
		}
		confidence += diffScore * 0.2
	}
	
	return confidence
}