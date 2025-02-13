package config

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

// CloudProvider represents a cloud provider configuration
type CloudProvider struct {
	Name           string   `yaml:"name"`
	MetadataIPs    []string `yaml:"metadata_ips"`
	MetadataURLs   []string `yaml:"metadata_urls"`
	InternalRanges []string `yaml:"internal_ranges"`
	ASNs           []string `yaml:"asns"`
	OrgNames       []string `yaml:"org_names"`
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
