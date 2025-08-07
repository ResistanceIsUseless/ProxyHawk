package server

import (
	"sync"
	"time"
)

// DNSCache provides caching for DNS and geographic test results
type DNSCache struct {
	cache     map[string]*CacheEntry
	mutex     sync.RWMutex
	config    CacheConfig
	
	// Cleanup ticker
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// CacheEntry represents a cached entry
type CacheEntry struct {
	Key       string
	Value     interface{}
	ExpiresAt time.Time
	CreatedAt time.Time
	AccessCount int64
	LastAccess  time.Time
}

// CacheStats provides cache statistics
type CacheStats struct {
	TotalEntries   int           `json:"total_entries"`
	HitCount       int64         `json:"hit_count"`
	MissCount      int64         `json:"miss_count"`
	HitRate        float64       `json:"hit_rate"`
	OldestEntry    time.Time     `json:"oldest_entry"`
	NewestEntry    time.Time     `json:"newest_entry"`
	MemoryUsage    int64         `json:"memory_usage_bytes"`
	AverageAge     time.Duration `json:"average_age"`
}

// NewDNSCache creates a new DNS cache
func NewDNSCache(config CacheConfig) *DNSCache {
	cache := &DNSCache{
		cache:       make(map[string]*CacheEntry),
		config:      config,
		stopCleanup: make(chan struct{}),
	}
	
	// Start cleanup goroutine if cache is enabled
	if config.Enabled && config.TTL > 0 {
		cache.cleanupTicker = time.NewTicker(config.TTL / 2) // Cleanup at half TTL interval
		go cache.runCleanup()
	}
	
	return cache
}

// Get retrieves a value from the cache
func (dc *DNSCache) Get(key string) interface{} {
	if !dc.config.Enabled {
		return nil
	}
	
	dc.mutex.RLock()
	entry, exists := dc.cache[key]
	dc.mutex.RUnlock()
	
	if !exists {
		return nil
	}
	
	// Check if entry has expired
	if time.Now().After(entry.ExpiresAt) {
		dc.mutex.Lock()
		delete(dc.cache, key)
		dc.mutex.Unlock()
		return nil
	}
	
	// Update access statistics
	dc.mutex.Lock()
	entry.AccessCount++
	entry.LastAccess = time.Now()
	dc.mutex.Unlock()
	
	return entry.Value
}

// Set stores a value in the cache
func (dc *DNSCache) Set(key string, value interface{}) {
	if !dc.config.Enabled {
		return
	}
	
	dc.mutex.Lock()
	defer dc.mutex.Unlock()
	
	// Check if we need to evict entries due to size limit
	if dc.config.MaxEntries > 0 && len(dc.cache) >= dc.config.MaxEntries {
		dc.evictLRU()
	}
	
	// Create new entry
	entry := &CacheEntry{
		Key:         key,
		Value:       value,
		ExpiresAt:   time.Now().Add(dc.config.TTL),
		CreatedAt:   time.Now(),
		AccessCount: 0,
		LastAccess:  time.Now(),
	}
	
	dc.cache[key] = entry
}

// Delete removes a value from the cache
func (dc *DNSCache) Delete(key string) {
	if !dc.config.Enabled {
		return
	}
	
	dc.mutex.Lock()
	delete(dc.cache, key)
	dc.mutex.Unlock()
}

// Clear removes all entries from the cache
func (dc *DNSCache) Clear() {
	dc.mutex.Lock()
	dc.cache = make(map[string]*CacheEntry)
	dc.mutex.Unlock()
}

// Size returns the number of entries in the cache
func (dc *DNSCache) Size() int {
	dc.mutex.RLock()
	size := len(dc.cache)
	dc.mutex.RUnlock()
	return size
}

// Keys returns all keys in the cache
func (dc *DNSCache) Keys() []string {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()
	
	keys := make([]string, 0, len(dc.cache))
	for key := range dc.cache {
		keys = append(keys, key)
	}
	
	return keys
}

// GetMultiple retrieves multiple values from the cache
func (dc *DNSCache) GetMultiple(keys []string) map[string]interface{} {
	result := make(map[string]interface{})
	
	for _, key := range keys {
		if value := dc.Get(key); value != nil {
			result[key] = value
		}
	}
	
	return result
}

// SetMultiple stores multiple values in the cache
func (dc *DNSCache) SetMultiple(entries map[string]interface{}) {
	for key, value := range entries {
		dc.Set(key, value)
	}
}

// GetStats returns cache statistics
func (dc *DNSCache) GetStats() *CacheStats {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()
	
	stats := &CacheStats{
		TotalEntries: len(dc.cache),
	}
	
	if len(dc.cache) == 0 {
		return stats
	}
	
	var totalHits, totalMisses int64
	var oldestTime, newestTime time.Time
	var totalAge time.Duration
	var memoryUsage int64
	
	now := time.Now()
	first := true
	
	for _, entry := range dc.cache {
		totalHits += entry.AccessCount
		
		if first {
			oldestTime = entry.CreatedAt
			newestTime = entry.CreatedAt
			first = false
		} else {
			if entry.CreatedAt.Before(oldestTime) {
				oldestTime = entry.CreatedAt
			}
			if entry.CreatedAt.After(newestTime) {
				newestTime = entry.CreatedAt
			}
		}
		
		totalAge += now.Sub(entry.CreatedAt)
		memoryUsage += dc.estimateEntrySize(entry)
	}
	
	stats.HitCount = totalHits
	stats.MissCount = totalMisses
	if totalHits+totalMisses > 0 {
		stats.HitRate = float64(totalHits) / float64(totalHits+totalMisses)
	}
	stats.OldestEntry = oldestTime
	stats.NewestEntry = newestTime
	stats.MemoryUsage = memoryUsage
	if len(dc.cache) > 0 {
		stats.AverageAge = totalAge / time.Duration(len(dc.cache))
	}
	
	return stats
}

// evictLRU evicts the least recently used entry
func (dc *DNSCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	
	for key, entry := range dc.cache {
		if first || entry.LastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.LastAccess
			first = false
		}
	}
	
	if oldestKey != "" {
		delete(dc.cache, oldestKey)
	}
}

// evictExpired removes expired entries
func (dc *DNSCache) evictExpired() int {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()
	
	now := time.Now()
	expiredCount := 0
	
	for key, entry := range dc.cache {
		if now.After(entry.ExpiresAt) {
			delete(dc.cache, key)
			expiredCount++
		}
	}
	
	return expiredCount
}

// runCleanup runs the periodic cleanup process
func (dc *DNSCache) runCleanup() {
	for {
		select {
		case <-dc.cleanupTicker.C:
			expiredCount := dc.evictExpired()
			if expiredCount > 0 {
				// Could log cleanup info here if logger was available
				_ = expiredCount
			}
			
		case <-dc.stopCleanup:
			dc.cleanupTicker.Stop()
			return
		}
	}
}

// Stop stops the cache and cleanup processes
func (dc *DNSCache) Stop() {
	if dc.cleanupTicker != nil {
		close(dc.stopCleanup)
	}
}

// estimateEntrySize estimates the memory size of a cache entry
func (dc *DNSCache) estimateEntrySize(entry *CacheEntry) int64 {
	// Basic estimation - could be more sophisticated
	baseSize := int64(64) // Struct overhead
	keySize := int64(len(entry.Key))
	
	// Estimate value size based on type
	valueSize := int64(0)
	switch v := entry.Value.(type) {
	case string:
		valueSize = int64(len(v))
	case []byte:
		valueSize = int64(len(v))
	case *GeoTestResult:
		// Rough estimate for GeoTestResult
		valueSize = int64(1024) // Base struct size
		valueSize += int64(len(v.Domain))
		valueSize += int64(len(v.RegionResults) * 512) // Estimate per region
	default:
		valueSize = int64(256) // Default estimate
	}
	
	return baseSize + keySize + valueSize
}

// GetOrSet gets a value from cache, or sets and returns it if not found
func (dc *DNSCache) GetOrSet(key string, fetchFunc func() interface{}) interface{} {
	// Try to get from cache first
	if value := dc.Get(key); value != nil {
		return value
	}
	
	// Not in cache, fetch the value
	value := fetchFunc()
	if value != nil {
		dc.Set(key, value)
	}
	
	return value
}

// GetWithTTL gets a value from cache along with its remaining TTL
func (dc *DNSCache) GetWithTTL(key string) (interface{}, time.Duration) {
	if !dc.config.Enabled {
		return nil, 0
	}
	
	dc.mutex.RLock()
	entry, exists := dc.cache[key]
	dc.mutex.RUnlock()
	
	if !exists {
		return nil, 0
	}
	
	now := time.Now()
	if now.After(entry.ExpiresAt) {
		dc.mutex.Lock()
		delete(dc.cache, key)
		dc.mutex.Unlock()
		return nil, 0
	}
	
	// Update access statistics
	dc.mutex.Lock()
	entry.AccessCount++
	entry.LastAccess = now
	dc.mutex.Unlock()
	
	remainingTTL := entry.ExpiresAt.Sub(now)
	return entry.Value, remainingTTL
}

// SetWithTTL sets a value with a specific TTL
func (dc *DNSCache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	if !dc.config.Enabled {
		return
	}
	
	dc.mutex.Lock()
	defer dc.mutex.Unlock()
	
	// Check if we need to evict entries due to size limit
	if dc.config.MaxEntries > 0 && len(dc.cache) >= dc.config.MaxEntries {
		dc.evictLRU()
	}
	
	// Create new entry with custom TTL
	entry := &CacheEntry{
		Key:         key,
		Value:       value,
		ExpiresAt:   time.Now().Add(ttl),
		CreatedAt:   time.Now(),
		AccessCount: 0,
		LastAccess:  time.Now(),
	}
	
	dc.cache[key] = entry
}

// Refresh refreshes the TTL of an existing entry
func (dc *DNSCache) Refresh(key string) bool {
	if !dc.config.Enabled {
		return false
	}
	
	dc.mutex.Lock()
	defer dc.mutex.Unlock()
	
	entry, exists := dc.cache[key]
	if !exists {
		return false
	}
	
	// Extend the expiration time
	entry.ExpiresAt = time.Now().Add(dc.config.TTL)
	entry.LastAccess = time.Now()
	
	return true
}

// HasKey checks if a key exists in the cache (without updating access time)
func (dc *DNSCache) HasKey(key string) bool {
	if !dc.config.Enabled {
		return false
	}
	
	dc.mutex.RLock()
	entry, exists := dc.cache[key]
	dc.mutex.RUnlock()
	
	if !exists {
		return false
	}
	
	// Check if expired
	return time.Now().Before(entry.ExpiresAt)
}

// GetEntries returns all non-expired entries (for debugging/monitoring)
func (dc *DNSCache) GetEntries() map[string]*CacheEntry {
	if !dc.config.Enabled {
		return nil
	}
	
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()
	
	result := make(map[string]*CacheEntry)
	now := time.Now()
	
	for key, entry := range dc.cache {
		if now.Before(entry.ExpiresAt) {
			// Create a copy to avoid concurrent access issues
			entryCopy := &CacheEntry{
				Key:         entry.Key,
				Value:       entry.Value,
				ExpiresAt:   entry.ExpiresAt,
				CreatedAt:   entry.CreatedAt,
				AccessCount: entry.AccessCount,
				LastAccess:  entry.LastAccess,
			}
			result[key] = entryCopy
		}
	}
	
	return result
}