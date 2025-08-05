package discovery

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/config"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/errors"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/logging"
)

// Manager coordinates proxy discovery across multiple sources
type Manager struct {
	config      config.DiscoveryConfig
	discoverers map[string]Discoverer
	logger      *logging.Logger
	filters     FilterOptions
	scoring     ScoringWeights
}

// NewManager creates a new discovery manager
func NewManager(discoveryConfig config.DiscoveryConfig, logger *logging.Logger) *Manager {
	m := &Manager{
		config:      discoveryConfig,
		discoverers: make(map[string]Discoverer),
		logger:      logger,
		filters:     DefaultFilterOptions(),
		scoring:     DefaultScoringWeights(),
	}

	// Initialize discoverers based on configuration
	if discoveryConfig.ShodanAPIKey != "" {
		m.discoverers["shodan"] = NewShodanDiscoverer(discoveryConfig.ShodanAPIKey)
	}

	return m
}

// DefaultFilterOptions returns sensible default filter options
func DefaultFilterOptions() FilterOptions {
	return FilterOptions{
		MinConfidence:    0.3,
		MaxAge:          24 * time.Hour,
		Countries:       []string{}, // Empty means all countries
		ExcludeCountries: []string{"CN", "RU", "KP", "IR"}, // Common exclusions
		Protocols:       []string{"http", "https", "socks4", "socks5"},
		MinPort:         1,
		MaxPort:         65535,
		RequireAuth:     nil, // Don't care about auth
		ExcludeMalicious: true,
		MaxResults:      1000,
	}
}

// SetFilters updates the filter options
func (m *Manager) SetFilters(filters FilterOptions) {
	m.filters = filters
}

// SetScoring updates the scoring weights
func (m *Manager) SetScoring(scoring ScoringWeights) {
	m.scoring = scoring
}

// GetAvailableSources returns the names of configured discoverers
func (m *Manager) GetAvailableSources() []string {
	var sources []string
	for name, discoverer := range m.discoverers {
		if discoverer.IsConfigured() {
			sources = append(sources, name)
		}
	}
	sort.Strings(sources)
	return sources
}

// SearchAll searches for proxy candidates across all configured sources
func (m *Manager) SearchAll(query string, maxResults int) (*DiscoveryResult, error) {
	if len(m.discoverers) == 0 {
		return nil, errors.NewConfigError(errors.ErrorConfigNotFound, "no discovery sources configured", nil)
	}

	sources := m.GetAvailableSources()
	if len(sources) == 0 {
		return nil, errors.NewConfigError(errors.ErrorConfigNotFound, "no discovery sources available", nil)
	}

	m.logger.Info("Starting proxy discovery across all sources",
		"query", query,
		"max_results", maxResults,
		"sources", sources)

	start := time.Now()
	allCandidates := make([]ProxyCandidate, 0, maxResults)
	allErrors := make([]string, 0)
	metadata := make(map[string]interface{})

	// Search each source
	for _, sourceName := range sources {
		discoverer := m.discoverers[sourceName]
		
		m.logger.Info("Searching source", "source", sourceName)
		
		result, err := discoverer.Search(query, maxResults)
		if err != nil {
			errMsg := fmt.Sprintf("%s: %v", sourceName, err)
			allErrors = append(allErrors, errMsg)
			m.logger.Warn("Source search failed", "source", sourceName, "error", err)
			continue
		}

		m.logger.Info("Source search completed",
			"source", sourceName,
			"candidates", len(result.Candidates),
			"total", result.Total,
			"duration", result.Duration)

		allCandidates = append(allCandidates, result.Candidates...)
		metadata[sourceName+"_results"] = len(result.Candidates)
		metadata[sourceName+"_total"] = result.Total
		metadata[sourceName+"_duration"] = result.Duration.String()
	}

	// Deduplicate candidates
	if m.config.Deduplicate {
		allCandidates = m.deduplicateCandidates(allCandidates)
		m.logger.Info("Deduplicated candidates", "count", len(allCandidates))
	}

	// Filter candidates
	filtered := m.filterCandidates(allCandidates)
	m.logger.Info("Filtered candidates", "before", len(allCandidates), "after", len(filtered))

	// Score and sort candidates
	scored := m.scoreCandidates(filtered)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Confidence > scored[j].Confidence
	})

	// Limit results
	if len(scored) > maxResults {
		scored = scored[:maxResults]
	}

	duration := time.Since(start)
	m.logger.Info("Discovery completed",
		"total_found", len(allCandidates),
		"after_filtering", len(filtered),
		"final_results", len(scored),
		"duration", duration,
		"sources_used", len(sources))

	return &DiscoveryResult{
		Query:      query,
		Source:     "all",
		Timestamp:  start,
		Duration:   duration,
		Total:      len(allCandidates),
		Filtered:   len(scored),
		Candidates: scored,
		Errors:     allErrors,
		Metadata:   metadata,
	}, nil
}

// SearchSource searches a specific discovery source
func (m *Manager) SearchSource(sourceName, query string, maxResults int) (*DiscoveryResult, error) {
	discoverer, exists := m.discoverers[sourceName]
	if !exists {
		return nil, fmt.Errorf("discovery source '%s' not available", sourceName)
	}

	if !discoverer.IsConfigured() {
		return nil, fmt.Errorf("discovery source '%s' not configured", sourceName)
	}

	m.logger.Info("Searching specific source",
		"source", sourceName,
		"query", query,
		"max_results", maxResults)

	result, err := discoverer.Search(query, maxResults)
	if err != nil {
		return nil, fmt.Errorf("search failed for source %s: %w", sourceName, err)
	}

	// Apply filtering and scoring
	filtered := m.filterCandidates(result.Candidates)
	scored := m.scoreCandidates(filtered)

	// Sort by confidence
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Confidence > scored[j].Confidence
	})

	// Update result
	result.Candidates = scored
	result.Filtered = len(scored)

	m.logger.Info("Source search completed",
		"source", sourceName,
		"original", len(result.Candidates),
		"filtered", len(scored),
		"duration", result.Duration)

	return result, nil
}

// GetHostDetails gets detailed information about a specific host
func (m *Manager) GetHostDetails(sourceName, ip string) (*ProxyCandidate, error) {
	discoverer, exists := m.discoverers[sourceName]
	if !exists {
		return nil, fmt.Errorf("discovery source '%s' not available", sourceName)
	}

	if !discoverer.IsConfigured() {
		return nil, fmt.Errorf("discovery source '%s' not configured", sourceName)
	}

	m.logger.Info("Getting host details", "source", sourceName, "ip", ip)

	candidate, err := discoverer.GetDetails(ip)
	if err != nil {
		return nil, fmt.Errorf("failed to get details for %s from %s: %w", ip, sourceName, err)
	}

	// Apply scoring
	scored := m.scoreCandidates([]ProxyCandidate{*candidate})
	if len(scored) > 0 {
		return &scored[0], nil
	}

	return candidate, nil
}

// GetPresetQueries returns preset search queries for different proxy types
func (m *Manager) GetPresetQueries() map[string][]string {
	queries := make(map[string][]string)

	// Add Shodan queries if available
	if _, exists := m.discoverers["shodan"]; exists {
		queries["shodan"] = ShodanProxyQueries
	}

	// Add general queries
	queries["general"] = []string{
		"proxy server",
		"squid proxy",
		"nginx proxy",
		"apache proxy",
		"socks proxy",
		"http proxy",
		"anonymous proxy",
	}

	return queries
}

// deduplicateCandidates removes duplicate candidates based on IP and port
func (m *Manager) deduplicateCandidates(candidates []ProxyCandidate) []ProxyCandidate {
	seen := make(map[string]*ProxyCandidate)
	
	for i := range candidates {
		candidate := &candidates[i]
		key := fmt.Sprintf("%s:%d", candidate.IP, candidate.Port)
		
		existing, exists := seen[key]
		if !exists {
			seen[key] = candidate
			continue
		}
		
		// Keep the candidate with higher confidence
		if candidate.Confidence > existing.Confidence {
			seen[key] = candidate
		} else if candidate.Confidence == existing.Confidence {
			// If confidence is equal, prefer more recent discovery
			if candidate.LastSeen.After(existing.LastSeen) {
				seen[key] = candidate
			}
		}
	}
	
	// Convert back to slice
	result := make([]ProxyCandidate, 0, len(seen))
	for _, candidate := range seen {
		result = append(result, *candidate)
	}
	
	return result
}

// filterCandidates applies filtering rules to candidates
func (m *Manager) filterCandidates(candidates []ProxyCandidate) []ProxyCandidate {
	filtered := make([]ProxyCandidate, 0, len(candidates))
	
	for _, candidate := range candidates {
		if m.shouldKeepCandidate(candidate) {
			filtered = append(filtered, candidate)
		}
	}
	
	return filtered
}

// shouldKeepCandidate determines if a candidate passes filtering rules
func (m *Manager) shouldKeepCandidate(candidate ProxyCandidate) bool {
	// Check confidence threshold
	if candidate.Confidence < m.filters.MinConfidence {
		return false
	}
	
	// Check age
	if m.filters.MaxAge > 0 && time.Since(candidate.LastSeen) > m.filters.MaxAge {
		return false
	}
	
	// Check country inclusion
	if len(m.filters.Countries) > 0 {
		found := false
		for _, country := range m.filters.Countries {
			if strings.EqualFold(candidate.Country, country) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check country exclusion
	for _, excludeCountry := range m.filters.ExcludeCountries {
		if strings.EqualFold(candidate.Country, excludeCountry) {
			return false
		}
	}
	
	// Check protocol
	if len(m.filters.Protocols) > 0 {
		found := false
		for _, protocol := range m.filters.Protocols {
			if strings.EqualFold(candidate.Protocol, protocol) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check port range
	if candidate.Port < m.filters.MinPort || candidate.Port > m.filters.MaxPort {
		return false
	}
	
	// Check auth requirement
	if m.filters.RequireAuth != nil {
		if *m.filters.RequireAuth != candidate.AuthRequired {
			return false
		}
	}
	
	// Check malicious exclusion
	if m.filters.ExcludeMalicious && candidate.IsMalicious {
		return false
	}
	
	return true
}

// scoreCandidates applies scoring to candidates based on various factors
func (m *Manager) scoreCandidates(candidates []ProxyCandidate) []ProxyCandidate {
	scored := make([]ProxyCandidate, len(candidates))
	copy(scored, candidates)
	
	for i := range scored {
		scored[i].Confidence = m.calculateScore(&scored[i])
	}
	
	return scored
}

// calculateScore calculates a comprehensive score for a proxy candidate
func (m *Manager) calculateScore(candidate *ProxyCandidate) float64 {
	_ = candidate.Confidence // Original confidence (unused in final calculation)
	
	// Source reliability scoring
	sourceScore := 0.0
	switch candidate.Source {
	case "shodan":
		sourceScore = 0.9 // Shodan is highly reliable
	case "censys":
		sourceScore = 0.8
	case "free-lists":
		sourceScore = 0.3 // Free lists are less reliable
	default:
		sourceScore = 0.5
	}
	
	// Location desirability scoring
	locationScore := 0.5 // Default neutral
	desirableCountries := []string{"US", "GB", "DE", "NL", "FR", "CA", "AU", "SE", "NO", "DK"}
	for _, country := range desirableCountries {
		if strings.EqualFold(candidate.Country, country) {
			locationScore = 0.8
			break
		}
	}
	
	// Technical indicators scoring
	techScore := 0.0
	if len(candidate.ProxyHeaders) > 0 {
		techScore += 0.3
	}
	if candidate.ProxyType != "unknown" && candidate.ProxyType != "" {
		techScore += 0.2
	}
	if candidate.ServerHeader != "" {
		techScore += 0.1
	}
	if candidate.TLSEnabled {
		techScore += 0.1
	}
	
	// Freshness scoring
	freshnessScore := 1.0
	age := time.Since(candidate.LastSeen)
	if age > 24*time.Hour {
		freshnessScore = 0.8
	}
	if age > 7*24*time.Hour {
		freshnessScore = 0.6
	}
	if age > 30*24*time.Hour {
		freshnessScore = 0.3
	}
	
	// Network quality scoring (simplified)
	networkScore := 0.5
	if candidate.ASN != "" {
		networkScore += 0.2
	}
	if candidate.ISP != "" {
		networkScore += 0.1
	}
	
	// Apply weights and combine scores
	finalScore := (sourceScore * m.scoring.SourceReliability) +
		(locationScore * m.scoring.LocationDesirability) +
		(techScore * m.scoring.TechnicalIndicators) +
		(freshnessScore * m.scoring.Freshness) +
		(networkScore * m.scoring.NetworkQuality)
	
	// Ensure score is between 0 and 1
	if finalScore > 1.0 {
		finalScore = 1.0
	}
	if finalScore < 0.0 {
		finalScore = 0.0
	}
	
	return finalScore
}