package proxy

import (
	"net/http"
	"net/url"
	"time"
)

// ProxyResultOutput represents the output format for a single proxy check result
type ProxyResultOutput struct {
	Proxy          string        `json:"proxy"`
	Working        bool          `json:"working"`
	Speed          time.Duration `json:"speed_ns"`
	Error          string        `json:"error,omitempty"`
	InteractshTest bool          `json:"interactsh_test"`
	RealIP         string        `json:"real_ip,omitempty"`
	ProxyIP        string        `json:"proxy_ip,omitempty"`
	IsAnonymous    bool          `json:"is_anonymous"`
	CloudProvider  string        `json:"cloud_provider,omitempty"`
	InternalAccess bool          `json:"internal_access"`
	MetadataAccess bool          `json:"metadata_access"`
	Timestamp      time.Time     `json:"timestamp"`
}

// SummaryOutput represents the summary of all proxy check results
type SummaryOutput struct {
	TotalProxies        int                 `json:"total_proxies"`
	WorkingProxies      int                 `json:"working_proxies"`
	InteractshProxies   int                 `json:"interactsh_proxies"`
	AnonymousProxies    int                 `json:"anonymous_proxies"`
	CloudProxies        int                 `json:"cloud_proxies"`
	InternalAccessCount int                 `json:"internal_access_count"`
	SuccessRate         float64             `json:"success_rate"`
	Results             []ProxyResultOutput `json:"results"`
}

// ProxyResult represents the result of a proxy check
type ProxyResult struct {
	Proxy          string
	Working        bool
	Speed          time.Duration
	Error          error
	InteractshTest bool
	DebugInfo      string
	RealIP         string
	ProxyIP        string
	IsAnonymous    bool
	CloudProvider  *CloudProvider
	InternalAccess bool
	MetadataAccess bool
	AdvancedChecks *AdvancedCheckResult
	TestResults    map[string]bool
	CheckResults   []CheckResult
}

// CheckResult represents the result of a single proxy check
type CheckResult struct {
	URL        string
	Success    bool
	Speed      time.Duration
	Error      string
	StatusCode int
	BodySize   int64
}

// CloudProvider represents a cloud provider configuration
type CloudProvider struct {
	Name           string   `yaml:"name"`
	MetadataIPs    []string `yaml:"metadata_ips"`
	MetadataURLs   []string `yaml:"metadata_urls"`
	InternalRanges []string `yaml:"internal_ranges"`
	ASNs           []string `yaml:"asns"`
	OrgNames       []string `yaml:"org_names"`
}

// AdvancedCheckResult represents the results of advanced proxy checks
type AdvancedCheckResult struct {
	ProtocolSmuggling   bool              `yaml:"protocol_smuggling"`
	DNSRebinding        bool              `yaml:"dns_rebinding"`
	NonStandardPorts    map[int]bool      `yaml:"nonstandard_ports"`
	IPv6Supported       bool              `yaml:"ipv6_supported"`
	MethodSupport       map[string]bool   `yaml:"method_support"`
	PathTraversal       bool              `yaml:"path_traversal"`
	CachePoisoning      bool              `yaml:"cache_poisoning"`
	HostHeaderInjection bool              `yaml:"host_header_injection"`
	VulnDetails         map[string]string `yaml:"vuln_details"`
}

// Config represents the application configuration
type Config struct {
	CloudProviders []CloudProvider   `yaml:"cloud_providers"`
	DefaultHeaders map[string]string `yaml:"default_headers"`
	UserAgent      string            `yaml:"user_agent"`
	TestURLs       TestURLConfig     `yaml:"test_urls"`
	Validation     struct {
		RequireStatusCode   int      `yaml:"require_status_code"`
		RequireContentMatch string   `yaml:"require_content_match"`
		RequireHeaderFields []string `yaml:"require_header_fields"`
		DisallowedKeywords  []string `yaml:"disallowed_keywords"`
		MinResponseBytes    int      `yaml:"min_response_bytes"`
		AdvancedChecks      struct {
			TestProtocolSmuggling   bool     `yaml:"test_protocol_smuggling"`
			TestDNSRebinding        bool     `yaml:"test_dns_rebinding"`
			TestIPv6                bool     `yaml:"test_ipv6"`
			TestHTTPMethods         []string `yaml:"test_http_methods"`
			TestPathTraversal       bool     `yaml:"test_path_traversal"`
			TestCachePoisoning      bool     `yaml:"test_cache_poisoning"`
			TestHostHeaderInjection bool     `yaml:"test_host_header_injection"`
		} `yaml:"advanced_checks"`
	} `yaml:"validation"`
}

// TestURLConfig represents the test URL configuration
type TestURLConfig struct {
	DefaultURL           string    `yaml:"default_url"`
	RequiredSuccessCount int       `yaml:"required_success_count"`
	URLs                 []TestURL `yaml:"urls"`
}

// TestURL represents a test URL configuration
type TestURL struct {
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

// Functions that need to be implemented in the main package
var (
	loadConfig                         func(string) error
	createProxyClient                  func(*url.URL, time.Duration) (*http.Client, error)
	checkProtocolSmuggling             func(*http.Client, bool) (bool, string)
	checkDNSRebinding                  func(*http.Client, bool) (bool, string)
	checkCachePoisoning                func(*http.Client, bool) (bool, string)
	checkHostHeaderInjection           func(*http.Client, bool) (bool, string)
	writeTextOutput                    func(string, []ProxyResultOutput, SummaryOutput) error
	writeWorkingProxiesOutput          func(string, []ProxyResultOutput) error
	writeWorkingAnonymousProxiesOutput func(string, []ProxyResultOutput) error
)

// Global variables that need to be set from the main package
var config Config
