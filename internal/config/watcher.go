package config

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatcherConfig holds configuration for the config file watcher
type WatcherConfig struct {
	// Debounce delay to avoid multiple rapid reloads
	DebounceDelay time.Duration
	// Callback function when config is successfully reloaded
	OnReload func(config *Config, result *ValidationResult)
	// Callback function when reload fails
	OnError func(err error)
	// Whether to validate config before reload
	ValidateBeforeReload bool
}

// DefaultWatcherConfig returns default watcher configuration
func DefaultWatcherConfig() WatcherConfig {
	return WatcherConfig{
		DebounceDelay:        500 * time.Millisecond,
		ValidateBeforeReload: true,
		OnReload: func(config *Config, result *ValidationResult) {
			// Default: do nothing
		},
		OnError: func(err error) {
			// Default: do nothing
		},
	}
}

// ConfigWatcher watches for configuration file changes and reloads automatically
type ConfigWatcher struct {
	configPath string
	config     WatcherConfig
	watcher    *fsnotify.Watcher

	// Current configuration
	currentConfig *Config
	configMutex   sync.RWMutex

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	// Debounce timer
	debounceTimer *time.Timer
	debounceMutex sync.Mutex
}

// NewConfigWatcher creates a new configuration file watcher
func NewConfigWatcher(configPath string, config WatcherConfig) (*ConfigWatcher, error) {
	// Ensure we have an absolute path
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Load initial configuration
	initialConfig, validationResult, err := ValidateAndLoad(absPath)
	if err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to load initial configuration: %w", err)
	}

	if !validationResult.Valid {
		watcher.Close()
		return nil, errors.New("initial configuration is invalid")
	}

	ctx, cancel := context.WithCancel(context.Background())

	cw := &ConfigWatcher{
		configPath:    absPath,
		config:        config,
		watcher:       watcher,
		currentConfig: initialConfig,
		ctx:           ctx,
		cancel:        cancel,
		done:          make(chan struct{}),
	}

	// Add the config file to the watcher
	// Watch the directory instead of the file directly to handle editor save patterns
	configDir := filepath.Dir(absPath)
	if err := watcher.Add(configDir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch config directory: %w", err)
	}

	// Start watching
	go cw.watch()

	return cw, nil
}

// GetConfig returns the current configuration (thread-safe)
func (cw *ConfigWatcher) GetConfig() *Config {
	cw.configMutex.RLock()
	defer cw.configMutex.RUnlock()
	return cw.currentConfig
}

// watch monitors for file changes
func (cw *ConfigWatcher) watch() {
	defer close(cw.done)

	for {
		select {
		case <-cw.ctx.Done():
			return

		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}

			// Check if the event is for our config file
			if filepath.Clean(event.Name) != filepath.Clean(cw.configPath) {
				continue
			}

			// Handle different event types
			switch {
			case event.Op&fsnotify.Write == fsnotify.Write:
				cw.handleConfigChange("write")
			case event.Op&fsnotify.Create == fsnotify.Create:
				// Some editors delete and recreate files
				cw.handleConfigChange("create")
			case event.Op&fsnotify.Rename == fsnotify.Rename:
				// Handle rename operations (some editors use rename for atomic saves)
				cw.handleConfigChange("rename")
			}

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			cw.config.OnError(fmt.Errorf("watcher error: %w", err))
		}
	}
}

// handleConfigChange handles configuration file changes with debouncing
func (cw *ConfigWatcher) handleConfigChange(operation string) {
	cw.debounceMutex.Lock()
	defer cw.debounceMutex.Unlock()

	// Cancel any existing timer
	if cw.debounceTimer != nil {
		cw.debounceTimer.Stop()
	}

	// Set up new timer
	cw.debounceTimer = time.AfterFunc(cw.config.DebounceDelay, func() {
		cw.reloadConfig(operation)
	})
}

// reloadConfig attempts to reload the configuration
func (cw *ConfigWatcher) reloadConfig(operation string) {
	// Load and validate new configuration
	newConfig, validationResult, err := ValidateAndLoad(cw.configPath)
	if err != nil {
		cw.config.OnError(fmt.Errorf("failed to reload config after %s: %w", operation, err))
		return
	}

	// Check if validation is required
	if cw.config.ValidateBeforeReload && !validationResult.Valid {
		var errMsg string
		for _, validationErr := range validationResult.Errors {
			errMsg += validationErr.Error() + "; "
		}
		cw.config.OnError(fmt.Errorf("config validation failed after %s: %s", operation, errMsg))
		return
	}

	// Update current configuration
	cw.configMutex.Lock()
	cw.currentConfig = newConfig
	cw.configMutex.Unlock()

	// Call reload callback
	cw.config.OnReload(newConfig, validationResult)
}

// Stop stops watching for configuration changes
func (cw *ConfigWatcher) Stop() error {
	cw.cancel()

	// Stop any pending debounce timer
	cw.debounceMutex.Lock()
	if cw.debounceTimer != nil {
		cw.debounceTimer.Stop()
	}
	cw.debounceMutex.Unlock()

	// Close the watcher
	err := cw.watcher.Close()

	// Wait for the watch goroutine to finish
	<-cw.done

	return err
}

// UpdateCallback updates the reload callback function
func (cw *ConfigWatcher) UpdateCallback(onReload func(config *Config, result *ValidationResult)) {
	cw.config.OnReload = onReload
}

// UpdateErrorCallback updates the error callback function
func (cw *ConfigWatcher) UpdateErrorCallback(onError func(err error)) {
	cw.config.OnError = onError
}
