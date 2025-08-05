package loader

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/errors"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/validation"
)

// LoadProxies loads and validates proxy addresses from a file using default validation
func LoadProxies(filename string) ([]string, []string, error) {
	return LoadProxiesWithValidator(filename, validation.NewProxyValidator())
}

// LoadProxiesWithValidator loads and validates proxy addresses with a custom validator
func LoadProxiesWithValidator(filename string, validator *validation.ProxyValidator) ([]string, []string, error) {
	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, nil, errors.NewFileError(errors.ErrorFileNotFound, "proxy file not found", filename, err)
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, errors.NewFileError(errors.ErrorFileReadFailed, "failed to open proxy file", filename, err)
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

		// Normalize the proxy URL
		normalizedProxy, err := validator.NormalizeProxyURL(proxy)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Line %d: %v", lineCount, err))
			continue
		}

		// Validate the normalized proxy
		if err := validator.ValidateProxyURL(normalizedProxy); err != nil {
			warnings = append(warnings, fmt.Sprintf("Line %d: %v", lineCount, err))
			continue
		}

		proxies = append(proxies, normalizedProxy)
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		return nil, warnings, errors.NewFileError(errors.ErrorFileReadFailed, "error reading proxy file", filename, err)
	}

	// Check if file was empty or had no valid proxies
	if len(proxies) == 0 {
		if lineCount == 0 {
			return nil, warnings, errors.NewFileError(errors.ErrorFileEmpty, "proxy file is empty", filename, nil)
		} else {
			return nil, warnings, errors.NewFileError(errors.ErrorFileInvalidFormat, "no valid proxies found in file", filename, nil).
				WithDetail("lines_read", lineCount).
				WithDetail("warnings", len(warnings))
		}
	}

	return proxies, warnings, nil
}
