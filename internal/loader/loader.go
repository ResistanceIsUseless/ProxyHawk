package loader

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// LoadProxies loads and validates proxy addresses from a file
func LoadProxies(filename string) ([]string, []string, error) {
	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("proxy file '%s' not found", filename)
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open proxy file: %v", err)
	}
	defer file.Close()

	var proxies []string
	var warnings []string
	lineCount := 0
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		lineCount++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Extract proxy URL (first field if there are multiple)
		proxy := strings.Fields(line)[0]
		if proxy == "" {
			continue
		}

		// Remove trailing slashes
		proxy = strings.TrimRight(proxy, "/")

		// Add scheme if missing
		if !strings.Contains(proxy, "://") {
			proxy = "http://" + proxy
		}

		// Validate URL
		if _, err := url.Parse(proxy); err != nil {
			warnings = append(warnings, fmt.Sprintf("Line %d: Invalid proxy URL '%s': %v", lineCount, proxy, err))
			continue
		}

		proxies = append(proxies, proxy)
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		return nil, warnings, fmt.Errorf("error reading proxy file: %v", err)
	}

	// Check if file was empty or had no valid proxies
	if len(proxies) == 0 {
		if lineCount == 0 {
			return nil, warnings, fmt.Errorf("proxy file '%s' is empty", filename)
		} else {
			return nil, warnings, fmt.Errorf("no valid proxies found in '%s' (found %d lines, %d warnings)", filename, lineCount, len(warnings))
		}
	}

	return proxies, warnings, nil
}