package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	
	"github.com/ResistanceIsUseless/ProxyHawk/pkg/server"
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
		configFile = flag.String("config", "config.yaml", "Configuration file")
		
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
	
	// TODO: Load from YAML config file if it exists
	if *configFile != "config.yaml" || fileExists(*configFile) {
		logger.Info("Config file support will be added", "file", *configFile)
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
        Configuration file (default "config.yaml")
    
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