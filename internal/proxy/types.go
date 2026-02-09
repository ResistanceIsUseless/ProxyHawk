package proxy

import (
	"sync"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/cloudcheck"
)

// ProxyType represents the type of proxy
type ProxyType string

const (
	ProxyTypeUnknown ProxyType = "unknown"
	ProxyTypeHTTP    ProxyType = "http"
	ProxyTypeHTTPS   ProxyType = "https"
	ProxyTypeHTTP2   ProxyType = "http2"
	ProxyTypeHTTP3   ProxyType = "http3"
	ProxyTypeSOCKS4  ProxyType = "socks4"
	ProxyTypeSOCKS5  ProxyType = "socks5"
)

// Config represents proxy checker configuration
type Config struct {
	// General settings
	Timeout            time.Duration
	ValidationURL      string
	ValidationPattern  string
	DisallowedKeywords []string
	MinResponseBytes   int
	DefaultHeaders     map[string]string
	UserAgent          string
	EnableCloudChecks  bool
	CloudProviders     []cloudcheck.CloudProvider
	UseRDNS            bool // Whether to use rDNS lookup for host headers

	// Rate limiting settings
	RateLimitEnabled  bool          // Whether rate limiting is enabled
	RateLimitDelay    time.Duration // Delay between requests to the same host
	RateLimitPerHost  bool          // Whether to apply rate limiting per host or globally
	RateLimitPerProxy bool          // Whether to apply rate limiting per individual proxy

	// Retry settings
	RetryEnabled    bool          // Whether retry mechanism is enabled
	MaxRetries      int           // Maximum number of retry attempts (default: 3)
	InitialDelay    time.Duration // Initial delay before first retry (default: 1s)
	MaxDelay        time.Duration // Maximum delay between retries (default: 30s)
	BackoffFactor   float64       // Exponential backoff multiplier (default: 2.0)
	RetryableErrors []string      // List of error patterns that should trigger retries

	// Authentication settings
	AuthEnabled     bool     // Whether proxy authentication is enabled
	DefaultUsername string   // Default username for proxies (if not in URL)
	DefaultPassword string   // Default password for proxies (if not in URL)
	AuthMethods     []string // Supported authentication methods (basic, digest)

	// Response validation settings
	RequireStatusCode   int
	RequireContentMatch string
	RequireHeaderFields []string

	// Advanced security checks
	AdvancedChecks AdvancedChecks

	// Interactsh settings (used only when advanced checks are enabled)
	InteractshURL   string // URL of the Interactsh server (optional)
	InteractshToken string // Token for the Interactsh server (optional)

	// Connection pool settings
	ConnectionPool interface{} // Will be set to *pool.ConnectionPool, but using interface{} to avoid circular import

	// HTTP/2 and HTTP/3 settings
	EnableHTTP2 bool // Whether to enable HTTP/2 protocol detection and support
	EnableHTTP3 bool // Whether to enable HTTP/3 protocol detection and support

	// Fingerprinting settings
	EnableFingerprint bool // Whether to enable proxy software fingerprinting
}

// CheckResult represents the result of a single check
type CheckResult struct {
	URL        string
	Success    bool
	Speed      time.Duration
	Error      string
	StatusCode int
	BodySize   int64
}

// AnonymityLevel represents the anonymity level of a proxy
type AnonymityLevel string

const (
	AnonymityNone        AnonymityLevel = "transparent" // Transparent proxy - real IP exposed
	AnonymityBasic       AnonymityLevel = "anonymous"   // Anonymous proxy - proxy headers present
	AnonymityElite       AnonymityLevel = "elite"       // High anonymous - no proxy headers
	AnonymityCompromised AnonymityLevel = "compromised" // Leaks detected (WebRTC, DNS, etc.)
	AnonymityUnknown     AnonymityLevel = "unknown"     // Anonymity check failed or not performed
)

// ProxyResult represents the comprehensive result of checking a proxy
type ProxyResult struct {
	Proxy                 string
	ProxyURL              string
	Working               bool
	Speed                 time.Duration
	Error                 error
	Type                  ProxyType
	ProxyType             ProxyType
	CheckResults          []CheckResult
	IsAnonymous           bool
	AnonymityLevel        AnonymityLevel // Detailed anonymity level
	RealIP                string
	ProxyIP               string
	DetectedIP            string         // IP address detected during anonymity check
	LeakingHeaders        []string       // Headers that leak information
	ProxyChainDetected    bool           // Whether proxy-behind-proxy was detected
	ProxyChainInfo        string         // Details about proxy chain
	CloudProvider         string
	InternalAccess        bool
	MetadataAccess        bool
	ResolvedHost          string
	AdvancedChecksPassed  bool
	AdvancedChecksDetails map[string]interface{}
	DebugInfo             string
	SecurityWarnings      []string // Security warnings (e.g., TLS verification disabled)

	// New fields for protocol support
	SupportsHTTP  bool
	SupportsHTTPS bool
	SupportsHTTP2 bool
	SupportsHTTP3 bool

	// Fingerprinting information
	Fingerprint *FingerprintResult `json:"fingerprint,omitempty"`

	// Vulnerability scan results
	NginxVulnerabilities    *NginxVulnResult    `json:"nginx_vulnerabilities,omitempty"`
	ApacheVulnerabilities   *ApacheVulnResult   `json:"apache_vulnerabilities,omitempty"`
	KongVulnerabilities     *KongVulnResult     `json:"kong_vulnerabilities,omitempty"`
	GenericVulnerabilities  *GenericVulnResult  `json:"generic_vulnerabilities,omitempty"`
	ExtendedVulnerabilities *ExtendedVulnResult `json:"extended_vulnerabilities,omitempty"`
}

// Checker represents the main proxy checker
type Checker struct {
	config          Config
	debug           bool
	rateLimiter     map[string]time.Time // Map of host to last request time
	rateLimiterLock sync.Mutex           // Mutex to protect the rate limiter map
}
