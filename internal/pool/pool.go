package pool

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ConnectionPool manages HTTP client connections with connection pooling
type ConnectionPool struct {
	// Connection pool settings
	maxIdleConns        int
	maxIdleConnsPerHost int
	maxConnsPerHost     int
	idleConnTimeout     time.Duration
	keepAliveTimeout    time.Duration
	tlsHandshakeTimeout time.Duration
	expectContinueTimeout time.Duration

	// Client cache for different proxy configurations
	clients map[string]*http.Client
	mutex   sync.RWMutex

	// Transport settings
	disableKeepAlives     bool
	disableCompression    bool
	insecureSkipVerify    bool
}

// Config represents connection pool configuration
type Config struct {
	MaxIdleConns          int           `yaml:"max_idle_conns"`
	MaxIdleConnsPerHost   int           `yaml:"max_idle_conns_per_host"`
	MaxConnsPerHost       int           `yaml:"max_conns_per_host"`
	IdleConnTimeout       time.Duration `yaml:"idle_conn_timeout"`
	KeepAliveTimeout      time.Duration `yaml:"keep_alive_timeout"`
	TLSHandshakeTimeout   time.Duration `yaml:"tls_handshake_timeout"`
	ExpectContinueTimeout time.Duration `yaml:"expect_continue_timeout"`
	DisableKeepAlives     bool          `yaml:"disable_keep_alives"`
	DisableCompression    bool          `yaml:"disable_compression"`
	InsecureSkipVerify    bool          `yaml:"insecure_skip_verify"`
}

// DefaultConfig returns a connection pool configuration with sensible defaults
func DefaultConfig() Config {
	return Config{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       50,
		IdleConnTimeout:       90 * time.Second,
		KeepAliveTimeout:      30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
		DisableCompression:    false,
		InsecureSkipVerify:    false,
	}
}

// NewConnectionPool creates a new connection pool with the given configuration
func NewConnectionPool(config Config) *ConnectionPool {
	return &ConnectionPool{
		maxIdleConns:          config.MaxIdleConns,
		maxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		maxConnsPerHost:       config.MaxConnsPerHost,
		idleConnTimeout:       config.IdleConnTimeout,
		keepAliveTimeout:      config.KeepAliveTimeout,
		tlsHandshakeTimeout:   config.TLSHandshakeTimeout,
		expectContinueTimeout: config.ExpectContinueTimeout,
		disableKeepAlives:     config.DisableKeepAlives,
		disableCompression:    config.DisableCompression,
		insecureSkipVerify:    config.InsecureSkipVerify,
		clients:               make(map[string]*http.Client),
		mutex:                 sync.RWMutex{},
	}
}

// GetClient returns an HTTP client configured for the given proxy URL
// It reuses existing clients when possible to leverage connection pooling
func (p *ConnectionPool) GetClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	// Create a cache key that includes proxy URL and timeout
	cacheKey := p.getCacheKey(proxyURL, timeout)

	// Check if we already have a client for this configuration
	p.mutex.RLock()
	if client, exists := p.clients[cacheKey]; exists {
		p.mutex.RUnlock()
		return client, nil
	}
	p.mutex.RUnlock()

	// Create a new client
	client, err := p.createClient(proxyURL, timeout)
	if err != nil {
		return nil, err
	}

	// Store the client in the cache
	p.mutex.Lock()
	p.clients[cacheKey] = client
	p.mutex.Unlock()

	return client, nil
}

// GetDirectClient returns an HTTP client for direct connections (no proxy)
func (p *ConnectionPool) GetDirectClient(timeout time.Duration) *http.Client {
	return p.createDirectClient(timeout)
}

// createClient creates a new HTTP client with the specified proxy configuration
func (p *ConnectionPool) createClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	// Parse proxy URL
	parsedProxy, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	// Create custom dialer with keep-alive settings
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: p.keepAliveTimeout,
	}

	// Create transport with connection pooling settings
	transport := &http.Transport{
		Proxy:                 http.ProxyURL(parsedProxy),
		DialContext:           dialer.DialContext,
		MaxIdleConns:          p.maxIdleConns,
		MaxIdleConnsPerHost:   p.maxIdleConnsPerHost,
		MaxConnsPerHost:       p.maxConnsPerHost,
		IdleConnTimeout:       p.idleConnTimeout,
		TLSHandshakeTimeout:   p.tlsHandshakeTimeout,
		ExpectContinueTimeout: p.expectContinueTimeout,
		DisableKeepAlives:     p.disableKeepAlives,
		DisableCompression:    p.disableCompression,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: p.insecureSkipVerify,
		},
		// Enable HTTP/2 support
		ForceAttemptHTTP2: true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}, nil
}

// createDirectClient creates an HTTP client for direct connections
func (p *ConnectionPool) createDirectClient(timeout time.Duration) *http.Client {
	// Create custom dialer with keep-alive settings
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: p.keepAliveTimeout,
	}

	// Create transport with connection pooling settings
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          p.maxIdleConns,
		MaxIdleConnsPerHost:   p.maxIdleConnsPerHost,
		MaxConnsPerHost:       p.maxConnsPerHost,
		IdleConnTimeout:       p.idleConnTimeout,
		TLSHandshakeTimeout:   p.tlsHandshakeTimeout,
		ExpectContinueTimeout: p.expectContinueTimeout,
		DisableKeepAlives:     p.disableKeepAlives,
		DisableCompression:    p.disableCompression,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: p.insecureSkipVerify,
		},
		// Enable HTTP/2 support
		ForceAttemptHTTP2: true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

// getCacheKey generates a cache key for the client based on proxy URL and timeout
func (p *ConnectionPool) getCacheKey(proxyURL string, timeout time.Duration) string {
	return proxyURL + ":" + timeout.String()
}

// CloseIdleConnections closes idle connections for all cached clients
func (p *ConnectionPool) CloseIdleConnections() {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	for _, client := range p.clients {
		if transport, ok := client.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
}

// GetStats returns statistics about the connection pool
func (p *ConnectionPool) GetStats() PoolStats {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	stats := PoolStats{
		CachedClients:       len(p.clients),
		MaxIdleConns:        p.maxIdleConns,
		MaxIdleConnsPerHost: p.maxIdleConnsPerHost,
		MaxConnsPerHost:     p.maxConnsPerHost,
		IdleConnTimeout:     p.idleConnTimeout,
		KeepAliveTimeout:    p.keepAliveTimeout,
	}

	return stats
}

// PoolStats contains statistics about the connection pool
type PoolStats struct {
	CachedClients       int           `json:"cached_clients"`
	MaxIdleConns        int           `json:"max_idle_conns"`
	MaxIdleConnsPerHost int           `json:"max_idle_conns_per_host"`
	MaxConnsPerHost     int           `json:"max_conns_per_host"`
	IdleConnTimeout     time.Duration `json:"idle_conn_timeout"`
	KeepAliveTimeout    time.Duration `json:"keep_alive_timeout"`
}

// Reset clears all cached clients and forces recreation
func (p *ConnectionPool) Reset() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Close idle connections before clearing cache
	for _, client := range p.clients {
		if transport, ok := client.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}

	// Clear the cache
	p.clients = make(map[string]*http.Client)
}

// UpdateConfig updates the connection pool configuration
// Note: This only affects newly created clients
func (p *ConnectionPool) UpdateConfig(config Config) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.maxIdleConns = config.MaxIdleConns
	p.maxIdleConnsPerHost = config.MaxIdleConnsPerHost
	p.maxConnsPerHost = config.MaxConnsPerHost
	p.idleConnTimeout = config.IdleConnTimeout
	p.keepAliveTimeout = config.KeepAliveTimeout
	p.tlsHandshakeTimeout = config.TLSHandshakeTimeout
	p.expectContinueTimeout = config.ExpectContinueTimeout
	p.disableKeepAlives = config.DisableKeepAlives
	p.disableCompression = config.DisableCompression
	p.insecureSkipVerify = config.InsecureSkipVerify
}

// GetClientCount returns the number of cached HTTP clients
func (p *ConnectionPool) GetClientCount() int {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return len(p.clients)
}