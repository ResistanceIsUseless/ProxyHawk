package proxy

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Package-level variables
var config Config

// loadProxies loads proxies from a file and returns them in a standardized format
func loadProxies(filename string) ([]string, []string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	var proxies []string
	var warnings []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		proxy := strings.TrimSpace(scanner.Text())
		if proxy == "" {
			continue
		}

		// Remove any trailing slashes
		proxy = strings.TrimRight(proxy, "/")

		// Check if the proxy already has a scheme
		hasScheme := strings.Contains(proxy, "://")
		if !hasScheme {
			// If no scheme, default to http://
			proxy = "http://" + proxy
		}

		// Parse the URL to validate and clean it
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Warning: invalid proxy URL '%s': %v", proxy, err))
			continue
		}

		// Clean up the host:port
		host := proxyURL.Hostname() // Get host without port
		port := proxyURL.Port()     // Get port if specified

		// Handle different schemes and ports
		switch proxyURL.Scheme {
		case "http":
			if port == "" || port == "80" {
				proxies = append(proxies, fmt.Sprintf("http://%s", host))
			} else {
				proxies = append(proxies, fmt.Sprintf("http://%s:%s", host, port))
			}
		case "https":
			if port == "" || port == "443" {
				proxies = append(proxies, fmt.Sprintf("https://%s", host))
			} else {
				proxies = append(proxies, fmt.Sprintf("https://%s:%s", host, port))
			}
		case "socks4", "socks5":
			if port == "" {
				port = "1080" // Default SOCKS port
			}
			proxies = append(proxies, fmt.Sprintf("%s://%s:%s", proxyURL.Scheme, host, port))
		default:
			warnings = append(warnings, fmt.Sprintf("Warning: defaulting to HTTP for unknown scheme '%s' in proxy '%s'", proxyURL.Scheme, proxy))
			if port == "" {
				proxies = append(proxies, fmt.Sprintf("http://%s", host))
			} else {
				proxies = append(proxies, fmt.Sprintf("http://%s:%s", host, port))
			}
		}
	}

	if len(proxies) == 0 {
		return nil, warnings, fmt.Errorf("no valid proxies found in file")
	}

	return proxies, warnings, scanner.Err()
}

// loadConfig loads the application configuration from a file
func loadConfig(filename string) error {
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

// checkProxyWithInteractsh checks a proxy with optional Interactsh validation
func checkProxyWithInteractsh(proxy string, targetURL string, useInteractsh bool, useIPInfo bool, useCloud bool, debug bool, timeout time.Duration, results chan<- ProxyResult) {
	start := time.Now()
	debugInfo := ""

	if debug {
		debugInfo += fmt.Sprintf("\nStarting proxy check for: %s\n", proxy)
		debugInfo += fmt.Sprintf("Target URL: %s\n", targetURL)
		debugInfo += fmt.Sprintf("Timeout: %v\n", timeout)
	}

	proxyURL, err := url.Parse(proxy)
	if err != nil {
		if debug {
			debugInfo += fmt.Sprintf("Error parsing proxy URL: %v\n", err)
		}
		results <- ProxyResult{
			Proxy:     proxy,
			Working:   false,
			Error:     fmt.Errorf("invalid proxy URL: %v", err),
			DebugInfo: debugInfo,
		}
		return
	}

	// Create a test client with the proxy
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	// Make a test request
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		results <- ProxyResult{
			Proxy:     proxy,
			Working:   false,
			Error:     err,
			DebugInfo: debugInfo,
		}
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		results <- ProxyResult{
			Proxy:     proxy,
			Working:   false,
			Error:     err,
			DebugInfo: debugInfo,
		}
		return
	}
	defer resp.Body.Close()

	// Check if the response is valid
	if resp.StatusCode != http.StatusOK {
		results <- ProxyResult{
			Proxy:     proxy,
			Working:   false,
			Error:     fmt.Errorf("invalid status code: %d", resp.StatusCode),
			DebugInfo: debugInfo,
		}
		return
	}

	results <- ProxyResult{
		Proxy:     proxy,
		Working:   true,
		Speed:     time.Since(start),
		DebugInfo: debugInfo,
		CheckResults: []CheckResult{
			{
				URL:        targetURL,
				Success:    true,
				Speed:      time.Since(start),
				StatusCode: resp.StatusCode,
			},
		},
	}
}
