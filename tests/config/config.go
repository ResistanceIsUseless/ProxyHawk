package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// TestConfig represents the test configuration
type TestConfig struct {
	TestURLs struct {
		DefaultURL           string `yaml:"default_url"`
		RequiredSuccessCount int    `yaml:"required_success_count"`
		URLs                 []struct {
			URL         string `yaml:"url"`
			Description string `yaml:"description"`
			Required    bool   `yaml:"required"`
		} `yaml:"urls"`
	} `yaml:"test_urls"`
	Validation struct {
		MinResponseBytes   int      `yaml:"min_response_bytes"`
		DisallowedKeywords []string `yaml:"disallowed_keywords"`
		AdvancedChecks     struct {
			TestProtocolSmuggling   bool     `yaml:"test_protocol_smuggling"`
			TestDNSRebinding        bool     `yaml:"test_dns_rebinding"`
			TestIPv6                bool     `yaml:"test_ipv6"`
			TestHTTPMethods         []string `yaml:"test_http_methods"`
			TestPathTraversal       bool     `yaml:"test_path_traversal"`
			TestCachePoisoning      bool     `yaml:"test_cache_poisoning"`
			TestHostHeaderInjection bool     `yaml:"test_host_header_injection"`
		} `yaml:"advanced_checks"`
	} `yaml:"validation"`
	CloudProviders []struct {
		Name         string   `yaml:"name"`
		MetadataURLs []string `yaml:"metadata_urls"`
		MetadataIPs  []string `yaml:"metadata_ips"`
	} `yaml:"cloud_providers"`
}

// TestConfigInstance is the global test configuration instance
var TestConfigInstance TestConfig

// LoadTestConfig loads the test configuration from a file
func LoadTestConfig(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, &TestConfigInstance)
	if err != nil {
		return err
	}

	return nil
}
