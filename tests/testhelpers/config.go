package testhelpers

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Config represents the test configuration
type Config struct {
	ProxyTypes []string `yaml:"proxy_types"`
	Timeouts   struct {
		Connect int `yaml:"connect"`
		Read    int `yaml:"read"`
		Write   int `yaml:"write"`
	} `yaml:"timeouts"`
	ValidationURL      string            `yaml:"validation_url"`
	ValidationPattern  string            `yaml:"validation_pattern"`
	DisallowedKeywords []string          `yaml:"disallowed_keywords"`
	MinResponseBytes   int               `yaml:"min_response_bytes"`
	DefaultHeaders     map[string]string `yaml:"default_headers"`
}

// LoadConfig loads the configuration from a YAML file
func LoadConfig(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
