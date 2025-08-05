package pool

import (
	"net/http"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxIdleConns != 100 {
		t.Errorf("Expected MaxIdleConns to be 100, got %d", config.MaxIdleConns)
	}

	if config.MaxIdleConnsPerHost != 10 {
		t.Errorf("Expected MaxIdleConnsPerHost to be 10, got %d", config.MaxIdleConnsPerHost)
	}

	if config.MaxConnsPerHost != 50 {
		t.Errorf("Expected MaxConnsPerHost to be 50, got %d", config.MaxConnsPerHost)
	}

	if config.IdleConnTimeout != 90*time.Second {
		t.Errorf("Expected IdleConnTimeout to be 90s, got %v", config.IdleConnTimeout)
	}

	if config.KeepAliveTimeout != 30*time.Second {
		t.Errorf("Expected KeepAliveTimeout to be 30s, got %v", config.KeepAliveTimeout)
	}

	if config.TLSHandshakeTimeout != 10*time.Second {
		t.Errorf("Expected TLSHandshakeTimeout to be 10s, got %v", config.TLSHandshakeTimeout)
	}

	if config.DisableKeepAlives {
		t.Error("Expected DisableKeepAlives to be false")
	}

	if config.DisableCompression {
		t.Error("Expected DisableCompression to be false")
	}

	if config.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be false")
	}
}

func TestNewConnectionPool(t *testing.T) {
	config := DefaultConfig()
	pool := NewConnectionPool(config)

	if pool == nil {
		t.Fatal("NewConnectionPool returned nil")
	}

	if pool.maxIdleConns != config.MaxIdleConns {
		t.Errorf("Expected maxIdleConns to be %d, got %d", config.MaxIdleConns, pool.maxIdleConns)
	}

	if pool.clients == nil {
		t.Error("Expected clients map to be initialized")
	}

	if len(pool.clients) != 0 {
		t.Errorf("Expected clients map to be empty initially, got %d clients", len(pool.clients))
	}
}

func TestGetDirectClient(t *testing.T) {
	config := DefaultConfig()
	pool := NewConnectionPool(config)

	timeout := 30 * time.Second
	client := pool.GetDirectClient(timeout)

	if client == nil {
		t.Fatal("GetDirectClient returned nil")
	}

	if client.Timeout != timeout {
		t.Errorf("Expected client timeout to be %v, got %v", timeout, client.Timeout)
	}

	// Check transport configuration
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected client to have http.Transport")
	}

	if transport.MaxIdleConns != config.MaxIdleConns {
		t.Errorf("Expected MaxIdleConns to be %d, got %d", config.MaxIdleConns, transport.MaxIdleConns)
	}

	if transport.MaxIdleConnsPerHost != config.MaxIdleConnsPerHost {
		t.Errorf("Expected MaxIdleConnsPerHost to be %d, got %d", config.MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
	}
}

func TestGetClient(t *testing.T) {
	config := DefaultConfig()
	pool := NewConnectionPool(config)

	proxyURL := "http://proxy.example.com:8080"
	timeout := 30 * time.Second

	client, err := pool.GetClient(proxyURL, timeout)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}

	if client == nil {
		t.Fatal("GetClient returned nil client")
	}

	if client.Timeout != timeout {
		t.Errorf("Expected client timeout to be %v, got %v", timeout, client.Timeout)
	}

	// Check that transport has proxy configured
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected client to have http.Transport")
	}

	if transport.Proxy == nil {
		t.Fatal("Expected transport to have proxy function")
	}

	// Test proxy function
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	proxyFunc := transport.Proxy
	resultURL, err := proxyFunc(req)
	if err != nil {
		t.Fatalf("Proxy function failed: %v", err)
	}

	if resultURL.String() != proxyURL {
		t.Errorf("Expected proxy URL to be %s, got %s", proxyURL, resultURL.String())
	}
}

func TestGetClientCaching(t *testing.T) {
	config := DefaultConfig()
	pool := NewConnectionPool(config)

	proxyURL := "http://proxy.example.com:8080"
	timeout := 30 * time.Second

	// Get client first time
	client1, err := pool.GetClient(proxyURL, timeout)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}

	// Get client second time with same parameters
	client2, err := pool.GetClient(proxyURL, timeout)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}

	// Should return the same client instance
	if client1 != client2 {
		t.Error("Expected GetClient to return cached client instance")
	}

	// Check client count
	if pool.GetClientCount() != 1 {
		t.Errorf("Expected 1 cached client, got %d", pool.GetClientCount())
	}

	// Get client with different timeout (should create new client)
	client3, err := pool.GetClient(proxyURL, 60*time.Second)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}

	if client1 == client3 {
		t.Error("Expected GetClient to return different client for different timeout")
	}

	// Check client count
	if pool.GetClientCount() != 2 {
		t.Errorf("Expected 2 cached clients, got %d", pool.GetClientCount())
	}
}

func TestGetClientInvalidProxy(t *testing.T) {
	config := DefaultConfig()
	pool := NewConnectionPool(config)

	invalidProxyURL := "://invalid-url"
	timeout := 30 * time.Second

	client, err := pool.GetClient(invalidProxyURL, timeout)
	if err == nil {
		t.Error("Expected GetClient to fail with invalid proxy URL")
	}

	if client != nil {
		t.Error("Expected GetClient to return nil client on error")
	}
}

func TestGetStats(t *testing.T) {
	config := DefaultConfig()
	pool := NewConnectionPool(config)

	stats := pool.GetStats()

	if stats.CachedClients != 0 {
		t.Errorf("Expected 0 cached clients initially, got %d", stats.CachedClients)
	}

	if stats.MaxIdleConns != config.MaxIdleConns {
		t.Errorf("Expected MaxIdleConns to be %d, got %d", config.MaxIdleConns, stats.MaxIdleConns)
	}

	if stats.MaxIdleConnsPerHost != config.MaxIdleConnsPerHost {
		t.Errorf("Expected MaxIdleConnsPerHost to be %d, got %d", config.MaxIdleConnsPerHost, stats.MaxIdleConnsPerHost)
	}

	// Add a client and check stats again
	pool.GetClient("http://proxy.example.com:8080", 30*time.Second)
	stats = pool.GetStats()

	if stats.CachedClients != 1 {
		t.Errorf("Expected 1 cached client after GetClient, got %d", stats.CachedClients)
	}
}

func TestCloseIdleConnections(t *testing.T) {
	config := DefaultConfig()
	pool := NewConnectionPool(config)

	// Create some clients
	pool.GetClient("http://proxy1.example.com:8080", 30*time.Second)
	pool.GetClient("http://proxy2.example.com:8080", 30*time.Second)

	// This should not panic or error
	pool.CloseIdleConnections()

	// Clients should still be cached
	if pool.GetClientCount() != 2 {
		t.Errorf("Expected 2 cached clients after CloseIdleConnections, got %d", pool.GetClientCount())
	}
}

func TestReset(t *testing.T) {
	config := DefaultConfig()
	pool := NewConnectionPool(config)

	// Create some clients
	pool.GetClient("http://proxy1.example.com:8080", 30*time.Second)
	pool.GetClient("http://proxy2.example.com:8080", 30*time.Second)

	if pool.GetClientCount() != 2 {
		t.Errorf("Expected 2 cached clients before Reset, got %d", pool.GetClientCount())
	}

	// Reset should clear all cached clients
	pool.Reset()

	if pool.GetClientCount() != 0 {
		t.Errorf("Expected 0 cached clients after Reset, got %d", pool.GetClientCount())
	}
}

func TestUpdateConfig(t *testing.T) {
	config := DefaultConfig()
	pool := NewConnectionPool(config)

	// Create a client with original config
	client1, _ := pool.GetClient("http://proxy.example.com:8080", 30*time.Second)

	// Update config
	newConfig := config
	newConfig.MaxIdleConns = 200
	newConfig.MaxIdleConnsPerHost = 20
	pool.UpdateConfig(newConfig)

	// Existing client should not be affected
	client2, _ := pool.GetClient("http://proxy.example.com:8080", 30*time.Second)
	if client1 != client2 {
		t.Error("Expected to get same cached client after UpdateConfig")
	}

	// New client with different proxy should use new config
	client3, _ := pool.GetClient("http://proxy2.example.com:8080", 30*time.Second)
	transport := client3.Transport.(*http.Transport)

	if transport.MaxIdleConns != newConfig.MaxIdleConns {
		t.Errorf("Expected new client to have MaxIdleConns %d, got %d", newConfig.MaxIdleConns, transport.MaxIdleConns)
	}

	if transport.MaxIdleConnsPerHost != newConfig.MaxIdleConnsPerHost {
		t.Errorf("Expected new client to have MaxIdleConnsPerHost %d, got %d", newConfig.MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
	}
}

func TestConcurrentAccess(t *testing.T) {
	config := DefaultConfig()
	pool := NewConnectionPool(config)

	// Test concurrent access to the pool
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			proxyURL := "http://proxy.example.com:8080"
			timeout := 30 * time.Second

			client, err := pool.GetClient(proxyURL, timeout)
			if err != nil {
				t.Errorf("Goroutine %d: GetClient failed: %v", id, err)
				return
			}

			if client == nil {
				t.Errorf("Goroutine %d: GetClient returned nil", id)
				return
			}

			// Get stats
			stats := pool.GetStats()
			if stats.CachedClients < 0 {
				t.Errorf("Goroutine %d: Invalid cached clients count: %d", id, stats.CachedClients)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have exactly 1 cached client since all used same proxy URL and timeout
	if pool.GetClientCount() != 1 {
		t.Errorf("Expected 1 cached client after concurrent access, got %d", pool.GetClientCount())
	}
}

func TestTransportConfiguration(t *testing.T) {
	config := Config{
		MaxIdleConns:          50,
		MaxIdleConnsPerHost:   5,
		MaxConnsPerHost:       25,
		IdleConnTimeout:       60 * time.Second,
		KeepAliveTimeout:      15 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 500 * time.Millisecond,
		DisableKeepAlives:     true,
		DisableCompression:    true,
		InsecureSkipVerify:    true,
	}

	pool := NewConnectionPool(config)
	client, err := pool.GetClient("http://proxy.example.com:8080", 30*time.Second)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected client to have http.Transport")
	}

	if transport.MaxIdleConns != config.MaxIdleConns {
		t.Errorf("Expected MaxIdleConns to be %d, got %d", config.MaxIdleConns, transport.MaxIdleConns)
	}

	if transport.MaxIdleConnsPerHost != config.MaxIdleConnsPerHost {
		t.Errorf("Expected MaxIdleConnsPerHost to be %d, got %d", config.MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
	}

	if transport.MaxConnsPerHost != config.MaxConnsPerHost {
		t.Errorf("Expected MaxConnsPerHost to be %d, got %d", config.MaxConnsPerHost, transport.MaxConnsPerHost)
	}

	if transport.IdleConnTimeout != config.IdleConnTimeout {
		t.Errorf("Expected IdleConnTimeout to be %v, got %v", config.IdleConnTimeout, transport.IdleConnTimeout)
	}

	if transport.TLSHandshakeTimeout != config.TLSHandshakeTimeout {
		t.Errorf("Expected TLSHandshakeTimeout to be %v, got %v", config.TLSHandshakeTimeout, transport.TLSHandshakeTimeout)
	}

	if transport.ExpectContinueTimeout != config.ExpectContinueTimeout {
		t.Errorf("Expected ExpectContinueTimeout to be %v, got %v", config.ExpectContinueTimeout, transport.ExpectContinueTimeout)
	}

	if transport.DisableKeepAlives != config.DisableKeepAlives {
		t.Errorf("Expected DisableKeepAlives to be %v, got %v", config.DisableKeepAlives, transport.DisableKeepAlives)
	}

	if transport.DisableCompression != config.DisableCompression {
		t.Errorf("Expected DisableCompression to be %v, got %v", config.DisableCompression, transport.DisableCompression)
	}

	if transport.TLSClientConfig.InsecureSkipVerify != config.InsecureSkipVerify {
		t.Errorf("Expected InsecureSkipVerify to be %v, got %v", config.InsecureSkipVerify, transport.TLSClientConfig.InsecureSkipVerify)
	}

	if !transport.ForceAttemptHTTP2 {
		t.Error("Expected ForceAttemptHTTP2 to be true")
	}
}

// Benchmark tests
func BenchmarkGetClient(b *testing.B) {
	config := DefaultConfig()
	pool := NewConnectionPool(config)
	proxyURL := "http://proxy.example.com:8080"
	timeout := 30 * time.Second

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pool.GetClient(proxyURL, timeout)
		if err != nil {
			b.Fatalf("GetClient failed: %v", err)
		}
	}
}

func BenchmarkGetClientConcurrent(b *testing.B) {
	config := DefaultConfig()
	pool := NewConnectionPool(config)
	proxyURL := "http://proxy.example.com:8080"
	timeout := 30 * time.Second

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := pool.GetClient(proxyURL, timeout)
			if err != nil {
				b.Fatalf("GetClient failed: %v", err)
			}
		}
	})
}

func BenchmarkGetDirectClient(b *testing.B) {
	config := DefaultConfig()
	pool := NewConnectionPool(config)
	timeout := 30 * time.Second

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client := pool.GetDirectClient(timeout)
		if client == nil {
			b.Fatal("GetDirectClient returned nil")
		}
	}
}