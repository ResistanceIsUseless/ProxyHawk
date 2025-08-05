package discovery

import (
	"time"
)

// ProxyCandidate represents a potential proxy discovered from various sources
type ProxyCandidate struct {
	// Basic connection info
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // http, https, socks4, socks5

	// Discovery metadata
	Source      string    `json:"source"`       // shodan, censys, etc.
	Confidence  float64   `json:"confidence"`   // 0-1 score based on discovery signals
	LastSeen    time.Time `json:"last_seen"`    // When this was discovered
	FirstSeen   time.Time `json:"first_seen"`   // When first discovered
	DiscoveryID string    `json:"discovery_id"` // Unique ID from discovery source

	// Location and network info
	Country     string `json:"country,omitempty"`
	City        string `json:"city,omitempty"`
	ASN         string `json:"asn,omitempty"`
	ASNOrg      string `json:"asn_org,omitempty"`
	ISP         string `json:"isp,omitempty"`
	Hostname    string `json:"hostname,omitempty"`

	// Technical details
	ServerHeader string            `json:"server_header,omitempty"`
	ResponseSize int64             `json:"response_size,omitempty"`
	TLSEnabled   bool              `json:"tls_enabled"`
	OpenPorts    []int             `json:"open_ports,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`

	// Proxy-specific indicators
	ProxyHeaders []string `json:"proxy_headers,omitempty"` // Via, X-Forwarded-For, etc.
	ProxyType    string   `json:"proxy_type,omitempty"`    // squid, nginx, apache, etc.
	AuthRequired bool     `json:"auth_required,omitempty"`

	// Quality metrics
	ResponseTime time.Duration `json:"response_time,omitempty"`
	Uptime       float64       `json:"uptime,omitempty"` // 0-1, if available
	
	// Security context
	IsMalicious   bool     `json:"is_malicious,omitempty"`
	ThreatTags    []string `json:"threat_tags,omitempty"`
	ReputationScore float64 `json:"reputation_score,omitempty"` // 0-1, higher is better
}

// DiscoveryConfig holds configuration for proxy discovery
type DiscoveryConfig struct {
	// API credentials
	ShodanAPIKey string `yaml:"shodan_api_key"`
	CensysAPIKey string `yaml:"censys_api_key"`
	CensysSecret string `yaml:"censys_secret"`

	// Search parameters
	MaxResults      int      `yaml:"max_results"`
	Countries       []string `yaml:"countries"`
	MinConfidence   float64  `yaml:"min_confidence"`
	Timeout         int      `yaml:"timeout"`
	RateLimit       int      `yaml:"rate_limit"` // requests per minute
	
	// Filtering
	ExcludeResidential bool     `yaml:"exclude_residential"`
	ExcludeCDN         bool     `yaml:"exclude_cdn"`
	ExcludeMalicious   bool     `yaml:"exclude_malicious"`
	RequiredPorts      []int    `yaml:"required_ports"`
	ExcludedASNs       []string `yaml:"excluded_asns"`

	// Output options
	OutputFormat string `yaml:"output_format"` // json, csv, txt
	Deduplicate  bool   `yaml:"deduplicate"`
}

// DiscoveryResult contains the results of a discovery operation
type DiscoveryResult struct {
	Query       string            `json:"query"`
	Source      string            `json:"source"`
	Timestamp   time.Time         `json:"timestamp"`
	Duration    time.Duration     `json:"duration"`
	Total       int               `json:"total"`
	Filtered    int               `json:"filtered"`
	Candidates  []ProxyCandidate  `json:"candidates"`
	Errors      []string          `json:"errors,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Discoverer interface for different discovery sources
type Discoverer interface {
	// Search for proxy candidates
	Search(query string, limit int) (*DiscoveryResult, error)
	
	// Get detailed information about a specific target
	GetDetails(ip string) (*ProxyCandidate, error)
	
	// Get the name of this discoverer
	Name() string
	
	// Check if the discoverer is properly configured
	IsConfigured() bool
	
	// Get rate limit information
	GetRateLimit() (remaining int, resetTime time.Time, err error)
}

// FilterOptions defines how to filter discovered candidates
type FilterOptions struct {
	MinConfidence    float64
	MaxAge           time.Duration
	Countries        []string
	ExcludeCountries []string
	Protocols        []string
	MinPort          int
	MaxPort          int
	RequireAuth      *bool // nil = don't care, true = require, false = exclude
	ExcludeMalicious bool
	MaxResults       int
}

// ScoringWeights defines how to score proxy candidates
type ScoringWeights struct {
	SourceReliability float64 // Weight for discovery source reliability
	LocationDesirability float64 // Weight for geographic location
	TechnicalIndicators float64 // Weight for technical proxy indicators
	Freshness          float64 // Weight for how recently discovered
	NetworkQuality     float64 // Weight for ASN/ISP quality
}

// DefaultScoringWeights returns sensible default scoring weights
func DefaultScoringWeights() ScoringWeights {
	return ScoringWeights{
		SourceReliability:   0.3,
		LocationDesirability: 0.2,
		TechnicalIndicators: 0.3,
		Freshness:          0.1,
		NetworkQuality:     0.1,
	}
}