package proxy

import (
	"sync"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/cloudcheck"
)

// ProxyType represents the type of proxy
type ProxyType string

const (
	ProxyTypeUnknown ProxyType = "unknown"
	ProxyTypeHTTP    ProxyType = "http"
	ProxyTypeHTTPS   ProxyType = "https"
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
	RealIP                string
	ProxyIP               string
	CloudProvider         string
	InternalAccess        bool
	MetadataAccess        bool
	ResolvedHost          string
	AdvancedChecksPassed  bool
	AdvancedChecksDetails map[string]interface{}
	DebugInfo             string

	// New fields for protocol support
	SupportsHTTP  bool
	SupportsHTTPS bool
}

// Checker represents the main proxy checker
type Checker struct {
	config          Config
	debug           bool
	rateLimiter     map[string]time.Time // Map of host to last request time
	rateLimiterLock sync.Mutex           // Mutex to protect the rate limiter map
}
