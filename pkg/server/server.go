package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ServerMode defines the operating mode of ProxyHawk
type ServerMode string

const (
	ModeProxy    ServerMode = "proxy"    // Traditional proxy only
	ModeAgent    ServerMode = "agent"    // Geographic testing only  
	ModeDual     ServerMode = "dual"     // Both proxy and agent
)

// ProxyHawkServer is the main dual-mode server
type ProxyHawkServer struct {
	config        *Config
	proxyRouter   *ProxyRouter
	wsService     *WebSocketService
	poolManager   *ProxyPoolManager
	geoTester     *GeographicTester
	dnsCache      *DNSCache
	metricsServer *http.Server
	
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	logger Logger
}

// Config holds server configuration
type Config struct {
	// Server mode
	Mode ServerMode
	
	// Proxy server settings
	SOCKS5Addr string
	HTTPAddr   string
	
	// WebSocket/API settings
	APIAddr string
	
	// Regional proxy configuration
	Regions map[string]*RegionConfig
	
	// Selection strategy
	SelectionStrategy SelectionStrategy
	
	// Round-robin detection settings
	RoundRobinDetection RoundRobinConfig
	
	// Health check settings
	HealthCheck HealthCheckConfig
	
	// Cache settings
	CacheConfig CacheConfig
	
	// Metrics settings
	MetricsEnabled bool
	MetricsAddr    string
	
	// Logging
	LogLevel string
	LogFormat string
}

// RegionConfig defines a regional proxy pool
type RegionConfig struct {
	Name    string
	Proxies []ProxyConfig
}

// ProxyConfig defines a single proxy or proxy chain
type ProxyConfig struct {
	URL            string   // Single proxy URL
	Chain          []string // Proxy chain URLs (if URL is empty, use chain)
	Weight         int
	HealthCheckURL string
	
	// Chain-specific settings
	ChainTimeout   time.Duration // Timeout for chain establishment
	RetryOnFailure bool          // Retry with different chain on failure
}

// SelectionStrategy defines how proxies are selected
type SelectionStrategy string

const (
	StrategyRoundRobin SelectionStrategy = "round-robin"
	StrategyRandom     SelectionStrategy = "random"
	StrategyWeighted   SelectionStrategy = "weighted"
	StrategySmart      SelectionStrategy = "smart"
	StrategySticky     SelectionStrategy = "sticky"
)

// RoundRobinConfig holds round-robin detection settings
type RoundRobinConfig struct {
	Enabled             bool
	MinSamples          int
	SampleInterval      time.Duration
	ConfidenceThreshold float64
}

// HealthCheckConfig holds health check settings
type HealthCheckConfig struct {
	Enabled           bool
	Interval          time.Duration
	Timeout           time.Duration
	FailureThreshold  int
	SuccessThreshold  int
}

// CacheConfig holds cache settings
type CacheConfig struct {
	Enabled    bool
	TTL        time.Duration
	MaxEntries int
}

// Logger interface for logging
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// NewProxyHawkServer creates a new dual-mode server
func NewProxyHawkServer(config *Config, logger Logger) *ProxyHawkServer {
	ctx, cancel := context.WithCancel(context.Background())
	
	server := &ProxyHawkServer{
		config: config,
		ctx:    ctx,
		cancel: cancel,
		logger: logger,
	}
	
	// Initialize components based on mode
	server.initializeComponents()
	
	return server
}

// initializeComponents initializes server components based on mode
func (s *ProxyHawkServer) initializeComponents() {
	// Initialize DNS cache (used by all modes)
	if s.config.CacheConfig.Enabled {
		s.dnsCache = NewDNSCache(s.config.CacheConfig)
		s.logger.Info("DNS cache initialized", 
			"ttl", s.config.CacheConfig.TTL,
			"max_entries", s.config.CacheConfig.MaxEntries)
	}
	
	// Initialize proxy pool manager (used by proxy and dual modes)
	if s.config.Mode == ModeProxy || s.config.Mode == ModeDual {
		s.poolManager = NewProxyPoolManager(s.config.Regions, s.config.SelectionStrategy)
		s.logger.Info("Proxy pool manager initialized",
			"regions", len(s.config.Regions),
			"strategy", s.config.SelectionStrategy)
		
		// Start health checking if enabled
		if s.config.HealthCheck.Enabled {
			s.poolManager.StartHealthChecking(s.config.HealthCheck)
			s.logger.Info("Health checking started",
				"interval", s.config.HealthCheck.Interval)
		}
	}
	
	// Initialize geographic tester (used by agent and dual modes)
	if s.config.Mode == ModeAgent || s.config.Mode == ModeDual {
		s.geoTester = NewGeographicTester(s.poolManager, s.dnsCache, s.config.RoundRobinDetection)
		s.logger.Info("Geographic tester initialized",
			"round_robin_detection", s.config.RoundRobinDetection.Enabled)
	}
	
	// Initialize proxy router (used by proxy and dual modes)
	if s.config.Mode == ModeProxy || s.config.Mode == ModeDual {
		s.proxyRouter = NewProxyRouter(s.poolManager, s.config.SelectionStrategy, s.logger)
		s.logger.Info("Proxy router initialized")
	}
	
	// Initialize WebSocket service (used by agent and dual modes)
	if s.config.Mode == ModeAgent || s.config.Mode == ModeDual {
		s.wsService = NewWebSocketService(s.geoTester, s.dnsCache, s.logger)
		s.logger.Info("WebSocket service initialized")
	}
}

// StartProxyOnly starts only the proxy servers
func (s *ProxyHawkServer) StartProxyOnly(socksAddr, httpAddr string) error {
	s.logger.Info("Starting proxy-only mode",
		"socks5", socksAddr,
		"http", httpAddr)
	
	// Start SOCKS5 server
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.proxyRouter.StartSOCKS5Server(socksAddr); err != nil {
			s.logger.Error("SOCKS5 server failed", "error", err)
		}
	}()
	
	// Start HTTP proxy server
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.proxyRouter.StartHTTPProxy(httpAddr); err != nil {
			s.logger.Error("HTTP proxy server failed", "error", err)
		}
	}()
	
	// Start metrics server if enabled
	if s.config.MetricsEnabled {
		s.startMetricsServer()
	}
	
	// Wait for shutdown
	<-s.ctx.Done()
	s.logger.Info("Shutting down proxy servers")
	
	return nil
}

// StartAgentOnly starts only the WebSocket/API server
func (s *ProxyHawkServer) StartAgentOnly(apiAddr string) error {
	s.logger.Info("Starting agent-only mode", "api", apiAddr)
	
	// Start WebSocket service
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.wsService.Start(apiAddr); err != nil {
			s.logger.Error("WebSocket service failed", "error", err)
		}
	}()
	
	// Start metrics server if enabled
	if s.config.MetricsEnabled {
		s.startMetricsServer()
	}
	
	// Wait for shutdown
	<-s.ctx.Done()
	s.logger.Info("Shutting down agent server")
	
	return nil
}

// StartDual starts both proxy and agent servers
func (s *ProxyHawkServer) StartDual(socksAddr, httpAddr, apiAddr string) error {
	s.logger.Info("Starting dual mode",
		"socks5", socksAddr,
		"http", httpAddr,
		"api", apiAddr)
	
	// Start SOCKS5 server
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.proxyRouter.StartSOCKS5Server(socksAddr); err != nil {
			s.logger.Error("SOCKS5 server failed", "error", err)
		}
	}()
	
	// Start HTTP proxy server
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.proxyRouter.StartHTTPProxy(httpAddr); err != nil {
			s.logger.Error("HTTP proxy server failed", "error", err)
		}
	}()
	
	// Start WebSocket service
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.wsService.Start(apiAddr); err != nil {
			s.logger.Error("WebSocket service failed", "error", err)
		}
	}()
	
	// Start metrics server if enabled
	if s.config.MetricsEnabled {
		s.startMetricsServer()
	}
	
	// Wait for shutdown
	<-s.ctx.Done()
	s.logger.Info("Shutting down dual-mode servers")
	
	return nil
}

// startMetricsServer starts the Prometheus metrics server
func (s *ProxyHawkServer) startMetricsServer() {
	s.metricsServer = &http.Server{
		Addr:    s.config.MetricsAddr,
		Handler: nil, // Will be set by metrics package
	}
	
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Info("Starting metrics server", "addr", s.config.MetricsAddr)
		if err := s.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Metrics server failed", "error", err)
		}
	}()
}

// Shutdown gracefully shuts down the server
func (s *ProxyHawkServer) Shutdown() error {
	s.logger.Info("Initiating graceful shutdown")
	
	// Cancel context to signal shutdown
	s.cancel()
	
	// Stop health checking if running
	if s.poolManager != nil {
		s.poolManager.StopHealthChecking()
	}
	
	// Shutdown metrics server
	if s.metricsServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.metricsServer.Shutdown(ctx); err != nil {
			s.logger.Warn("Error shutting down metrics server", "error", err)
		}
	}
	
	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		s.logger.Info("Graceful shutdown completed")
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("shutdown timeout after 30 seconds")
	}
}

// GetStats returns server statistics
func (s *ProxyHawkServer) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})
	
	// Add pool manager stats
	if s.poolManager != nil {
		stats["pool"] = s.poolManager.GetStats()
	}
	
	// Add cache stats
	if s.dnsCache != nil {
		stats["cache"] = s.dnsCache.GetStats()
	}
	
	// Add WebSocket service stats
	if s.wsService != nil {
		stats["websocket"] = s.wsService.GetStats()
	}
	
	return stats
}