package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	
	"github.com/ResistanceIsUseless/ProxyHawk/pkg/server"
	"gopkg.in/yaml.v2"
)

// SimpleLogger implements the server.Logger interface
type SimpleLogger struct{}

func (l *SimpleLogger) Info(msg string, keysAndValues ...interface{}) {
	log.Printf("[INFO] %s %v", msg, keysAndValues)
}

func (l *SimpleLogger) Debug(msg string, keysAndValues ...interface{}) {
	log.Printf("[DEBUG] %s %v", msg, keysAndValues)
}

func (l *SimpleLogger) Warn(msg string, keysAndValues ...interface{}) {
	log.Printf("[WARN] %s %v", msg, keysAndValues)
}

func (l *SimpleLogger) Error(msg string, keysAndValues ...interface{}) {
	log.Printf("[ERROR] %s %v", msg, keysAndValues)
}

func main() {
	var (
		// Server mode flags
		mode = flag.String("mode", "dual", "Server mode: proxy, agent, dual")
		
		// Proxy server flags
		socksAddr = flag.String("socks", ":1080", "SOCKS5 proxy address")
		httpAddr  = flag.String("http", ":8080", "HTTP proxy address")
		
		// Geographic testing API
		apiAddr = flag.String("api", ":8888", "API/WebSocket address")
		
		// Configuration
		configFile = flag.String("config", "", "Configuration file (default: ~/.config/proxyhawk/server.yaml)")
		
		// Logging
		_ = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		
		// Metrics
		enableMetrics = flag.Bool("metrics", false, "Enable Prometheus metrics")
		metricsAddr   = flag.String("metrics-addr", ":9090", "Metrics server address")
		
		// Help
		showHelp = flag.Bool("help", false, "Show help message")
		showVersion = flag.Bool("version", false, "Show version information")
	)
	
	flag.Parse()
	
	if *showHelp {
		printHelp()
		os.Exit(0)
	}
	
	if *showVersion {
		printVersion()
		os.Exit(0)
	}
	
	// Validate mode
	serverMode := server.ServerMode(*mode)
	switch serverMode {
	case server.ModeProxy, server.ModeAgent, server.ModeDual:
		// Valid modes
	default:
		fmt.Fprintf(os.Stderr, "Invalid mode: %s. Must be one of: proxy, agent, dual\n", *mode)
		os.Exit(1)
	}
	
	// Create logger
	logger := &SimpleLogger{}
	logger.Info("Starting ProxyHawk Server", 
		"mode", *mode,
		"version", "1.0.0")
	
	// Load configuration
	config := createDefaultConfig(serverMode, *enableMetrics, *metricsAddr)
	
	// Determine config file path
	configPath := *configFile
	if configPath == "" {
		// Use default XDG config directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			logger.Warn("Could not determine home directory, using current directory")
			configPath = "server.yaml"
		} else {
			configPath = filepath.Join(homeDir, ".config", "proxyhawk", "server.yaml")
		}
	}
	
	// Load from YAML config file if it exists
	if fileExists(configPath) {
		if loadedConfig, err := loadConfigFromYAML(configPath, logger); err != nil {
			logger.Warn("Failed to load config file, using defaults", "file", configPath, "error", err)
		} else {
			config = mergeConfigs(config, loadedConfig)
			logger.Info("Loaded configuration from file", "file", configPath)
		}
	} else if *configFile != "" {
		// User specified a config file but it doesn't exist
		logger.Warn("Specified config file does not exist, using defaults", "file", configPath)
	} else {
		// No config file found, suggest creating one
		logger.Info("No config file found", "default_path", configPath, "suggestion", "Create config file or use -config flag")
	}
	
	// Initialize the unified server
	srv := server.NewProxyHawkServer(config, logger)
	
	// Set up graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		var err error
		switch serverMode {
		case server.ModeProxy:
			err = srv.StartProxyOnly(*socksAddr, *httpAddr)
		case server.ModeAgent:
			err = srv.StartAgentOnly(*apiAddr)
		case server.ModeDual:
			err = srv.StartDual(*socksAddr, *httpAddr, *apiAddr)
		}
		errChan <- err
	}()
	
	// Wait for shutdown signal or error
	select {
	case err := <-errChan:
		if err != nil {
			logger.Error("Server failed", "error", err)
			os.Exit(1)
		}
	case sig := <-signalChan:
		logger.Info("Received shutdown signal", "signal", sig)
		
		// Graceful shutdown
		if err := srv.Shutdown(); err != nil {
			logger.Error("Shutdown failed", "error", err)
			os.Exit(1)
		}
		
		logger.Info("Server shut down successfully")
	}
}

// createDefaultConfig creates a default configuration
func createDefaultConfig(mode server.ServerMode, metricsEnabled bool, metricsAddr string) *server.Config {
	return &server.Config{
		Mode:       mode,
		SOCKS5Addr: ":1080",
		HTTPAddr:   ":8080",
		APIAddr:    ":8888",
		
		// Sample regions configuration
		Regions: map[string]*server.RegionConfig{
			"us-west": {
				Name: "US West Coast",
				Proxies: []server.ProxyConfig{
					{
						URL:            "socks5://us-west-1.example.com:1080",
						Weight:         10,
						HealthCheckURL: "http://httpbin.org/ip",
					},
				},
			},
			"us-east": {
				Name: "US East Coast", 
				Proxies: []server.ProxyConfig{
					{
						URL:            "socks5://us-east-1.example.com:1080",
						Weight:         10,
						HealthCheckURL: "http://httpbin.org/ip",
					},
				},
			},
			"eu-west": {
				Name: "Western Europe",
				Proxies: []server.ProxyConfig{
					{
						URL:            "socks5://eu-west-1.example.com:1080",
						Weight:         10,
						HealthCheckURL: "http://httpbin.org/ip",
					},
				},
			},
		},
		
		SelectionStrategy: server.StrategySmart,
		
		RoundRobinDetection: server.RoundRobinConfig{
			Enabled:             true,
			MinSamples:          5,
			SampleInterval:      2 * time.Second,
			ConfidenceThreshold: 0.85,
		},
		
		HealthCheck: server.HealthCheckConfig{
			Enabled:          true,
			Interval:         1 * time.Minute,
			Timeout:          10 * time.Second,
			FailureThreshold: 3,
			SuccessThreshold: 2,
		},
		
		CacheConfig: server.CacheConfig{
			Enabled:    true,
			TTL:        5 * time.Minute,
			MaxEntries: 10000,
		},
		
		MetricsEnabled: metricsEnabled,
		MetricsAddr:    metricsAddr,
		LogLevel:       "info",
		LogFormat:      "text",
	}
}

// fileExists checks if a file exists
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// printHelp prints usage help
func printHelp() {
	fmt.Print(`ProxyHawk Dual-Mode Proxy Server

DESCRIPTION:
    ProxyHawk operates as both a traditional proxy server (SOCKS5/HTTP) and a 
    geographic testing service with WebSocket support for persistent connections.

USAGE:
    proxyhawk-server [flags]

FLAGS:
    -mode string
        Server mode: proxy, agent, dual (default "dual")
    
    -socks string
        SOCKS5 proxy address (default ":1080")
    
    -http string  
        HTTP proxy address (default ":8080")
    
    -api string
        API/WebSocket address (default ":8888")
    
    -config string
        Configuration file (default "~/.config/proxyhawk/server.yaml")
    
    -log-level string
        Log level: debug, info, warn, error (default "info")
    
    -metrics
        Enable Prometheus metrics
    
    -metrics-addr string
        Metrics server address (default ":9090")
    
    -help
        Show this help message
    
    -version
        Show version information

MODES:
    proxy    Traditional proxy only (SOCKS5 + HTTP)
    agent    Geographic testing only (WebSocket API)
    dual     Both proxy and agent (default)

EXAMPLES:
    # Start in dual mode (default)
    proxyhawk-server
    
    # Start only as proxy server
    proxyhawk-server -mode proxy -socks :1080 -http :8080
    
    # Start only as geographic agent
    proxyhawk-server -mode agent -api :8888
    
    # Start with custom addresses and metrics
    proxyhawk-server -socks :2080 -http :3080 -api :4080 -metrics

INTEGRATION:
    # Use with proxychains
    proxychains4 curl https://example.com
    
    # Use as HTTP proxy
    export http_proxy=http://localhost:8080
    curl https://example.com
    
    # WebSocket API endpoint
    ws://localhost:8888/ws
    
    # Health check endpoint  
    http://localhost:8888/api/health

For more information, see the documentation.
`)
}

// printVersion prints version information
func printVersion() {
	fmt.Print(`ProxyHawk Server v1.0.0

Built with Go
Copyright 2024 ProxyHawk Contributors

Features:
- Dual-mode operation (proxy + geographic agent)
- SOCKS5 and HTTP proxy support
- WebSocket API for real-time testing
- Smart proxy selection and health checking
- Round-robin DNS detection
- Geographic testing across regions
- Prometheus metrics support
- Proxychains compatibility

Homepage: https://github.com/ResistanceIsUseless/ProxyHawk
`)
}

// loadConfigFromYAML loads configuration from a YAML file
func loadConfigFromYAML(filename string, logger *SimpleLogger) (*server.Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	var yamlConfig YAMLConfig
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}
	
	// Convert YAML config to server.Config
	config := convertYAMLToConfig(&yamlConfig)
	return config, nil
}

// mergeConfigs merges a loaded config with the default config
func mergeConfigs(defaultConfig, loadedConfig *server.Config) *server.Config {
	// Start with default config as base
	merged := *defaultConfig
	
	// Override with loaded values if they are non-zero/non-empty
	if loadedConfig.SOCKS5Addr != "" {
		merged.SOCKS5Addr = loadedConfig.SOCKS5Addr
	}
	if loadedConfig.HTTPAddr != "" {
		merged.HTTPAddr = loadedConfig.HTTPAddr
	}
	if loadedConfig.APIAddr != "" {
		merged.APIAddr = loadedConfig.APIAddr
	}
	
	// Merge regions if provided
	if len(loadedConfig.Regions) > 0 {
		merged.Regions = loadedConfig.Regions
	}
	
	// Merge other configurations
	if loadedConfig.SelectionStrategy != "" {
		merged.SelectionStrategy = loadedConfig.SelectionStrategy
	}
	
	// Override boolean and numeric values
	if loadedConfig.RoundRobinDetection.Enabled {
		merged.RoundRobinDetection = loadedConfig.RoundRobinDetection
	}
	if loadedConfig.HealthCheck.Enabled {
		merged.HealthCheck = loadedConfig.HealthCheck
	}
	if loadedConfig.CacheConfig.Enabled {
		merged.CacheConfig = loadedConfig.CacheConfig
	}
	
	if loadedConfig.MetricsEnabled {
		merged.MetricsEnabled = loadedConfig.MetricsEnabled
		merged.MetricsAddr = loadedConfig.MetricsAddr
	}
	
	if loadedConfig.LogLevel != "" {
		merged.LogLevel = loadedConfig.LogLevel
	}
	if loadedConfig.LogFormat != "" {
		merged.LogFormat = loadedConfig.LogFormat
	}
	
	return &merged
}

// YAMLConfig represents the YAML configuration structure
type YAMLConfig struct {
	Mode         string                   `yaml:"mode"`
	SOCKS5Addr   string                   `yaml:"socks5_addr"`
	HTTPAddr     string                   `yaml:"http_addr"`
	APIAddr      string                   `yaml:"api_addr"`
	
	Regions  map[string]YAMLRegion    `yaml:"regions"`
	Strategy string                   `yaml:"selection_strategy"`
	
	RoundRobinDetection YAMLRoundRobinConfig `yaml:"round_robin_detection"`
	HealthCheck        YAMLHealthCheckConfig `yaml:"health_check"`
	Cache              YAMLCacheConfig       `yaml:"cache"`
	
	Metrics YAMLMetricsConfig `yaml:"metrics"`
	
	LogLevel  string `yaml:"log_level"`
	LogFormat string `yaml:"log_format"`
}

// YAMLRegion represents a region configuration in YAML
type YAMLRegion struct {
	Name    string        `yaml:"name"`
	Proxies []YAMLProxy   `yaml:"proxies"`
}

// YAMLProxy represents a proxy configuration in YAML
type YAMLProxy struct {
	URL            string `yaml:"url"`
	Weight         int    `yaml:"weight"`
	HealthCheckURL string `yaml:"health_check_url"`
}

// YAMLRoundRobinConfig represents round robin configuration in YAML
type YAMLRoundRobinConfig struct {
	Enabled             bool          `yaml:"enabled"`
	MinSamples          int           `yaml:"min_samples"`
	SampleInterval      time.Duration `yaml:"sample_interval"`
	ConfidenceThreshold float64       `yaml:"confidence_threshold"`
}

// YAMLHealthCheckConfig represents health check configuration in YAML
type YAMLHealthCheckConfig struct {
	Enabled          bool          `yaml:"enabled"`
	Interval         time.Duration `yaml:"interval"`
	Timeout          time.Duration `yaml:"timeout"`
	FailureThreshold int           `yaml:"failure_threshold"`
	SuccessThreshold int           `yaml:"success_threshold"`
}

// YAMLCacheConfig represents cache configuration in YAML
type YAMLCacheConfig struct {
	Enabled    bool          `yaml:"enabled"`
	TTL        time.Duration `yaml:"ttl"`
	MaxEntries int           `yaml:"max_entries"`
}

// YAMLMetricsConfig represents metrics configuration in YAML
type YAMLMetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
}

// convertYAMLToConfig converts YAML config to server.Config
func convertYAMLToConfig(yamlConfig *YAMLConfig) *server.Config {
	config := &server.Config{
		Mode:       server.ServerMode(yamlConfig.Mode),
		SOCKS5Addr: yamlConfig.SOCKS5Addr,
		HTTPAddr:   yamlConfig.HTTPAddr,
		APIAddr:    yamlConfig.APIAddr,
		LogLevel:   yamlConfig.LogLevel,
		LogFormat:  yamlConfig.LogFormat,
	}
	
	// Convert selection strategy
	switch yamlConfig.Strategy {
	case "random":
		config.SelectionStrategy = server.StrategyRandom
	case "round_robin":
		config.SelectionStrategy = server.StrategyRoundRobin
	case "smart":
		config.SelectionStrategy = server.StrategySmart
	case "weighted":
		config.SelectionStrategy = server.StrategyWeighted
	}
	
	// Convert regions
	config.Regions = make(map[string]*server.RegionConfig)
	for regionName, yamlRegion := range yamlConfig.Regions {
		region := &server.RegionConfig{
			Name:    yamlRegion.Name,
			Proxies: make([]server.ProxyConfig, 0, len(yamlRegion.Proxies)),
		}
		
		for _, yamlProxy := range yamlRegion.Proxies {
			proxy := server.ProxyConfig{
				URL:            yamlProxy.URL,
				Weight:         yamlProxy.Weight,
				HealthCheckURL: yamlProxy.HealthCheckURL,
			}
			region.Proxies = append(region.Proxies, proxy)
		}
		
		config.Regions[regionName] = region
	}
	
	// Convert round robin detection
	config.RoundRobinDetection = server.RoundRobinConfig{
		Enabled:             yamlConfig.RoundRobinDetection.Enabled,
		MinSamples:          yamlConfig.RoundRobinDetection.MinSamples,
		SampleInterval:      yamlConfig.RoundRobinDetection.SampleInterval,
		ConfidenceThreshold: yamlConfig.RoundRobinDetection.ConfidenceThreshold,
	}
	
	// Convert health check
	config.HealthCheck = server.HealthCheckConfig{
		Enabled:          yamlConfig.HealthCheck.Enabled,
		Interval:         yamlConfig.HealthCheck.Interval,
		Timeout:          yamlConfig.HealthCheck.Timeout,
		FailureThreshold: yamlConfig.HealthCheck.FailureThreshold,
		SuccessThreshold: yamlConfig.HealthCheck.SuccessThreshold,
	}
	
	// Convert cache config
	config.CacheConfig = server.CacheConfig{
		Enabled:    yamlConfig.Cache.Enabled,
		TTL:        yamlConfig.Cache.TTL,
		MaxEntries: yamlConfig.Cache.MaxEntries,
	}
	
	// Convert metrics
	config.MetricsEnabled = yamlConfig.Metrics.Enabled
	config.MetricsAddr = yamlConfig.Metrics.Addr
	
	return config
}