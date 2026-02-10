package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GetUserConfigPath returns the path to the user's config file
// following XDG Base Directory specification
func GetUserConfigPath() string {
	// Check XDG_CONFIG_HOME first
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "proxyhawk", "config.yaml")
	}

	// Fall back to ~/.config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// If we can't get home dir, use current directory
		return "config/client/default.yaml"
	}

	return filepath.Join(homeDir, ".config", "proxyhawk", "config.yaml")
}

// GetUserConfigDir returns the user's config directory
func GetUserConfigDir() string {
	return filepath.Dir(GetUserConfigPath())
}

// InitializeUserConfig creates the user config directory and file if they don't exist
// Returns the path to the config file and any error encountered
func InitializeUserConfig() (string, error) {
	configPath := GetUserConfigPath()
	configDir := GetUserConfigDir()

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		// Config already exists, return the path
		return configPath, nil
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	// Generate default config
	defaultCfg := GetDefaultConfig()

	// Marshal to YAML
	data, err := yaml.Marshal(defaultCfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal default config: %w", err)
	}

	// Add helpful header comment
	header := `# ProxyHawk Configuration
# Auto-generated default configuration
#
# This file was created automatically by ProxyHawk.
# Customize these settings for your proxy checking and vulnerability scanning needs.
#
# For detailed documentation, see:
#   https://github.com/ResistanceIsUseless/ProxyHawk
#   or config/client/default.yaml in the repository
#
# Configuration precedence:
#   1. Command-line flags (highest priority)
#   2. Environment variables (PROXYHAWK_*)
#   3. This config file
#   4. Built-in defaults (lowest priority)
#
# Last updated: ` + "auto-generated" + `

`

	// Write config file
	fullConfig := header + string(data)
	if err := os.WriteFile(configPath, []byte(fullConfig), 0644); err != nil {
		return "", fmt.Errorf("failed to write config file: %w", err)
	}

	return configPath, nil
}

// GetConfigPath determines the config file path to use
// Priority: 1. CLI flag, 2. User config, 3. Default
func GetConfigPath(cliPath string) (string, bool, error) {
	// If CLI flag is provided and not the default, use it
	if cliPath != "" && cliPath != "config/default.yaml" && cliPath != "config/client/default.yaml" {
		return cliPath, false, nil
	}

	// Try user config first
	userConfig := GetUserConfigPath()
	if _, err := os.Stat(userConfig); err == nil {
		return userConfig, false, nil
	}

	// User config doesn't exist - offer to create it
	// For now, return default and indicate initialization is available
	return "config/client/default.yaml", true, nil
}

// EnsureUserConfig checks if user config exists and creates it if needed
// Returns the config path and whether it was newly created
func EnsureUserConfig() (string, bool, error) {
	userConfig := GetUserConfigPath()

	// Check if it already exists
	if _, err := os.Stat(userConfig); err == nil {
		return userConfig, false, nil
	}

	// Create it
	configPath, err := InitializeUserConfig()
	if err != nil {
		return "", false, err
	}

	return configPath, true, nil
}
