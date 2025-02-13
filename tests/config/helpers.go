package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Package-level variables
var config Config

// LoadConfig loads the application configuration from a file
func LoadConfig(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		// Set default configuration
		config = Config{
			DefaultHeaders: map[string]string{
				"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
				"Accept-Language": "en-US,en;q=0.9",
				"Accept-Encoding": "gzip, deflate",
				"Connection":      "keep-alive",
				"Cache-Control":   "no-cache",
				"Pragma":          "no-cache",
			},
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			TestURLs: TestURLConfig{
				DefaultURL:           "https://www.google.com",
				RequiredSuccessCount: 1,
				URLs: []TestURL{
					{
						URL:         "https://www.google.com",
						Description: "Default test using Google",
						Required:    true,
					},
				},
			},
		}
		return nil // Return nil since we've set default values
	}

	return yaml.Unmarshal(data, &config)
}

// GetConfig returns the current configuration
func GetConfig() Config {
	return config
}
