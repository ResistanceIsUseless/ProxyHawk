package tests

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

// Config represents the application configuration
type Config struct {
	CloudProviders []CloudProvider   `yaml:"cloud_providers"`
	DefaultHeaders map[string]string `yaml:"default_headers"`
	UserAgent      string            `yaml:"user_agent"`
	Validation     struct {
		RequireStatusCode   int      `yaml:"require_status_code"`
		RequireContentMatch string   `yaml:"require_content_match"`
		RequireHeaderFields []string `yaml:"require_header_fields"`
		DisallowedKeywords  []string `yaml:"disallowed_keywords"`
		MinResponseBytes    int      `yaml:"min_response_bytes"`
		AdvancedChecks      struct {
			TestProtocolSmuggling   bool     `yaml:"test_protocol_smuggling"`
			TestDNSRebinding        bool     `yaml:"test_dns_rebinding"`
			TestNonStandardPorts    []int    `yaml:"test_nonstandard_ports"`
			TestIPv6                bool     `yaml:"test_ipv6"`
			TestHTTPMethods         []string `yaml:"test_http_methods"`
			TestPathTraversal       bool     `yaml:"test_path_traversal"`
			TestCachePoisoning      bool     `yaml:"test_cache_poisoning"`
			TestHostHeaderInjection bool     `yaml:"test_host_header_injection"`
		} `yaml:"advanced_checks"`
	} `yaml:"validation"`
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
