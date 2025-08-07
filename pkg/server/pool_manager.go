package server

import (
	"context"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ProxyPoolManager manages regional proxy pools
type ProxyPoolManager struct {
	regions map[string]*RegionPool
	strategy SelectionStrategy
	
	// Health checking
	healthChecker *HealthChecker
	healthCancel  context.CancelFunc
	
	// Round-robin state
	roundRobinMutex sync.Mutex
	regionOrder     []string
	currentIndex    int
	
	mutex sync.RWMutex
}

// RegionPool represents a pool of proxies in a specific region
type RegionPool struct {
	Name    string
	Proxies []*ProxyInfo
	
	// Selection state
	mutex         sync.RWMutex
	currentIndex  int
	lastUsed      time.Time
}

// ProxyInfo represents a single proxy with health information
type ProxyInfo struct {
	URL            string
	Weight         int
	HealthCheckURL string
	
	// Proxy chaining support
	Chain          []string      // Proxy chain URLs (if URL is empty, use chain)
	ChainTimeout   time.Duration // Timeout for chain establishment
	RetryOnFailure bool          // Retry with different chain on failure
	
	// Health status
	IsHealthy      bool
	LastHealthCheck time.Time
	FailureCount   int
	SuccessCount   int
	ResponseTime   time.Duration
	
	// Usage statistics
	TotalRequests   int64
	SuccessfulRequests int64
	
	mutex sync.RWMutex
}

// HealthChecker performs health checks on proxies
type HealthChecker struct {
	config   HealthCheckConfig
	client   *http.Client
	logger   Logger
	stopChan chan struct{}
}

// NewProxyPoolManager creates a new proxy pool manager
func NewProxyPoolManager(regions map[string]*RegionConfig, strategy SelectionStrategy) *ProxyPoolManager {
	manager := &ProxyPoolManager{
		regions:  make(map[string]*RegionPool),
		strategy: strategy,
	}
	
	// Initialize region pools
	for name, config := range regions {
		pool := &RegionPool{
			Name:    name,
			Proxies: make([]*ProxyInfo, 0, len(config.Proxies)),
		}
		
		// Add proxies to pool
		for _, proxyConfig := range config.Proxies {
			proxyInfo := &ProxyInfo{
				URL:            proxyConfig.URL,
				Weight:         proxyConfig.Weight,
				HealthCheckURL: proxyConfig.HealthCheckURL,
				Chain:          proxyConfig.Chain,
				ChainTimeout:   proxyConfig.ChainTimeout,
				RetryOnFailure: proxyConfig.RetryOnFailure,
				IsHealthy:      true, // Assume healthy initially
			}
			
			// Set default chain timeout if not specified
			if proxyInfo.ChainTimeout == 0 && len(proxyInfo.Chain) > 0 {
				proxyInfo.ChainTimeout = 30 * time.Second
			}
			
			pool.Proxies = append(pool.Proxies, proxyInfo)
		}
		
		manager.regions[name] = pool
		manager.regionOrder = append(manager.regionOrder, name)
	}
	
	return manager
}

// GetProxy gets a proxy from the specified region
func (pm *ProxyPoolManager) GetProxy(region string) *ProxyInfo {
	pm.mutex.RLock()
	pool, exists := pm.regions[region]
	pm.mutex.RUnlock()
	
	if !exists {
		// Fall back to default region or any available
		return pm.getAnyProxy()
	}
	
	return pm.selectFromPool(pool)
}

// GetHealthyProxy gets a healthy proxy from the specified region
func (pm *ProxyPoolManager) GetHealthyProxy(region string) *ProxyInfo {
	pm.mutex.RLock()
	pool, exists := pm.regions[region]
	pm.mutex.RUnlock()
	
	if !exists {
		return pm.getAnyHealthyProxy()
	}
	
	// Get healthy proxies from pool
	pool.mutex.RLock()
	healthyProxies := make([]*ProxyInfo, 0)
	for _, proxy := range pool.Proxies {
		proxy.mutex.RLock()
		if proxy.IsHealthy {
			healthyProxies = append(healthyProxies, proxy)
		}
		proxy.mutex.RUnlock()
	}
	pool.mutex.RUnlock()
	
	if len(healthyProxies) == 0 {
		return nil
	}
	
	// Select from healthy proxies
	return pm.selectFromProxies(healthyProxies)
}

// GetNextRegion gets the next region in round-robin order
func (pm *ProxyPoolManager) GetNextRegion() string {
	pm.roundRobinMutex.Lock()
	defer pm.roundRobinMutex.Unlock()
	
	if len(pm.regionOrder) == 0 {
		return ""
	}
	
	region := pm.regionOrder[pm.currentIndex]
	pm.currentIndex = (pm.currentIndex + 1) % len(pm.regionOrder)
	
	return region
}

// selectFromPool selects a proxy from a pool based on strategy
func (pm *ProxyPoolManager) selectFromPool(pool *RegionPool) *ProxyInfo {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()
	
	if len(pool.Proxies) == 0 {
		return nil
	}
	
	return pm.selectFromProxies(pool.Proxies)
}

// selectFromProxies selects a proxy from a list based on strategy
func (pm *ProxyPoolManager) selectFromProxies(proxies []*ProxyInfo) *ProxyInfo {
	if len(proxies) == 0 {
		return nil
	}
	
	switch pm.strategy {
	case StrategyRoundRobin:
		return pm.selectRoundRobin(proxies)
	case StrategyRandom:
		return pm.selectRandom(proxies)
	case StrategyWeighted:
		return pm.selectWeighted(proxies)
	case StrategySmart:
		return pm.selectSmart(proxies)
	default:
		return proxies[0]
	}
}

// selectRoundRobin selects proxy using round-robin
func (pm *ProxyPoolManager) selectRoundRobin(proxies []*ProxyInfo) *ProxyInfo {
	// Simple round-robin based on current time
	index := int(time.Now().UnixNano()) % len(proxies)
	return proxies[index]
}

// selectRandom selects proxy randomly
func (pm *ProxyPoolManager) selectRandom(proxies []*ProxyInfo) *ProxyInfo {
	index := rand.Intn(len(proxies))
	return proxies[index]
}

// selectWeighted selects proxy based on weights
func (pm *ProxyPoolManager) selectWeighted(proxies []*ProxyInfo) *ProxyInfo {
	// Calculate total weight
	totalWeight := 0
	for _, proxy := range proxies {
		proxy.mutex.RLock()
		totalWeight += proxy.Weight
		proxy.mutex.RUnlock()
	}
	
	if totalWeight == 0 {
		return pm.selectRandom(proxies)
	}
	
	// Select based on weight
	target := rand.Intn(totalWeight)
	current := 0
	
	for _, proxy := range proxies {
		proxy.mutex.RLock()
		current += proxy.Weight
		proxy.mutex.RUnlock()
		
		if current > target {
			return proxy
		}
	}
	
	return proxies[len(proxies)-1]
}

// selectSmart selects proxy based on performance and health
func (pm *ProxyPoolManager) selectSmart(proxies []*ProxyInfo) *ProxyInfo {
	var bestProxy *ProxyInfo
	bestScore := -1.0
	
	for _, proxy := range proxies {
		proxy.mutex.RLock()
		
		score := 0.0
		
		// Health score (0-40 points)
		if proxy.IsHealthy {
			score += 40.0
		}
		
		// Success rate score (0-30 points)
		if proxy.TotalRequests > 0 {
			successRate := float64(proxy.SuccessfulRequests) / float64(proxy.TotalRequests)
			score += successRate * 30.0
		} else {
			score += 25.0 // Neutral score for untested proxies
		}
		
		// Response time score (0-20 points)
		if proxy.ResponseTime > 0 {
			// Better response time = higher score
			responseScore := 20.0 - (proxy.ResponseTime.Seconds() * 2.0)
			if responseScore < 0 {
				responseScore = 0
			}
			score += responseScore
		} else {
			score += 15.0 // Neutral score
		}
		
		// Weight bonus (0-10 points)
		score += float64(proxy.Weight) / 10.0 * 10.0
		
		proxy.mutex.RUnlock()
		
		if score > bestScore {
			bestScore = score
			bestProxy = proxy
		}
	}
	
	if bestProxy == nil {
		return proxies[0]
	}
	
	return bestProxy
}

// getAnyProxy gets any available proxy from any region
func (pm *ProxyPoolManager) getAnyProxy() *ProxyInfo {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	for _, pool := range pm.regions {
		if proxy := pm.selectFromPool(pool); proxy != nil {
			return proxy
		}
	}
	
	return nil
}

// getAnyHealthyProxy gets any healthy proxy from any region
func (pm *ProxyPoolManager) getAnyHealthyProxy() *ProxyInfo {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	for _, pool := range pm.regions {
		pool.mutex.RLock()
		for _, proxy := range pool.Proxies {
			proxy.mutex.RLock()
			if proxy.IsHealthy {
				proxy.mutex.RUnlock()
				pool.mutex.RUnlock()
				return proxy
			}
			proxy.mutex.RUnlock()
		}
		pool.mutex.RUnlock()
	}
	
	return nil
}

// StartHealthChecking starts health checking for all proxies
func (pm *ProxyPoolManager) StartHealthChecking(config HealthCheckConfig) {
	if !config.Enabled {
		return
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	pm.healthCancel = cancel
	
	pm.healthChecker = &HealthChecker{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		stopChan: make(chan struct{}),
	}
	
	// Start health check goroutine
	go pm.runHealthChecks(ctx)
}

// StopHealthChecking stops health checking
func (pm *ProxyPoolManager) StopHealthChecking() {
	if pm.healthCancel != nil {
		pm.healthCancel()
	}
}

// runHealthChecks runs periodic health checks
func (pm *ProxyPoolManager) runHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(pm.healthChecker.config.Interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.performHealthChecks()
		}
	}
}

// performHealthChecks performs health checks on all proxies
func (pm *ProxyPoolManager) performHealthChecks() {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	var wg sync.WaitGroup
	
	for _, pool := range pm.regions {
		pool.mutex.RLock()
		for _, proxy := range pool.Proxies {
			wg.Add(1)
			go func(p *ProxyInfo) {
				defer wg.Done()
				pm.checkProxyHealth(p)
			}(proxy)
		}
		pool.mutex.RUnlock()
	}
	
	wg.Wait()
}

// checkProxyHealth checks the health of a single proxy
func (pm *ProxyPoolManager) checkProxyHealth(proxy *ProxyInfo) {
	proxy.mutex.Lock()
	defer proxy.mutex.Unlock()
	
	start := time.Now()
	
	// Use health check URL or a default test URL
	testURL := proxy.HealthCheckURL
	if testURL == "" {
		testURL = "http://httpbin.org/ip"
	}
	
	// Create proxy client
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		proxy.FailureCount++
		proxy.IsHealthy = false
		return
	}
	
	client := &http.Client{
		Timeout: pm.healthChecker.config.Timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}
	
	// Perform health check
	resp, err := client.Get(testURL)
	if err != nil {
		proxy.FailureCount++
	} else {
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			proxy.SuccessCount++
			proxy.ResponseTime = time.Since(start)
		} else {
			proxy.FailureCount++
		}
	}
	
	proxy.LastHealthCheck = time.Now()
	
	// Update health status based on thresholds
	if proxy.FailureCount >= pm.healthChecker.config.FailureThreshold {
		proxy.IsHealthy = false
	} else if proxy.SuccessCount >= pm.healthChecker.config.SuccessThreshold {
		proxy.IsHealthy = true
	}
}

// RecordProxyUsage records usage statistics for a proxy
func (pm *ProxyPoolManager) RecordProxyUsage(proxyURL string, successful bool) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	// Find the proxy
	for _, pool := range pm.regions {
		pool.mutex.RLock()
		for _, proxy := range pool.Proxies {
			if proxy.URL == proxyURL {
				proxy.mutex.Lock()
				proxy.TotalRequests++
				if successful {
					proxy.SuccessfulRequests++
				}
				proxy.mutex.Unlock()
				pool.mutex.RUnlock()
				return
			}
		}
		pool.mutex.RUnlock()
	}
}

// GetStats returns statistics about the proxy pools
func (pm *ProxyPoolManager) GetStats() map[string]interface{} {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	stats := make(map[string]interface{})
	regionStats := make(map[string]interface{})
	
	totalProxies := 0
	healthyProxies := 0
	
	for name, pool := range pm.regions {
		pool.mutex.RLock()
		
		poolStats := map[string]interface{}{
			"total_proxies": len(pool.Proxies),
			"healthy_proxies": 0,
			"proxies": make([]map[string]interface{}, 0, len(pool.Proxies)),
		}
		
		healthyInPool := 0
		for _, proxy := range pool.Proxies {
			proxy.mutex.RLock()
			
			if proxy.IsHealthy {
				healthyInPool++
			}
			
			proxyStats := map[string]interface{}{
				"url":                  proxy.URL,
				"chain":               proxy.Chain,
				"chain_timeout":       proxy.ChainTimeout.String(),
				"retry_on_failure":    proxy.RetryOnFailure,
				"healthy":             proxy.IsHealthy,
				"total_requests":      proxy.TotalRequests,
				"successful_requests": proxy.SuccessfulRequests,
				"failure_count":       proxy.FailureCount,
				"success_count":       proxy.SuccessCount,
				"response_time":       proxy.ResponseTime.String(),
				"last_health_check":   proxy.LastHealthCheck,
			}
			
			poolStats["proxies"] = append(poolStats["proxies"].([]map[string]interface{}), proxyStats)
			proxy.mutex.RUnlock()
		}
		
		poolStats["healthy_proxies"] = healthyInPool
		regionStats[name] = poolStats
		
		totalProxies += len(pool.Proxies)
		healthyProxies += healthyInPool
		
		pool.mutex.RUnlock()
	}
	
	stats["total_proxies"] = totalProxies
	stats["healthy_proxies"] = healthyProxies
	stats["total_regions"] = len(pm.regions)
	stats["selection_strategy"] = string(pm.strategy)
	stats["regions"] = regionStats
	
	return stats
}