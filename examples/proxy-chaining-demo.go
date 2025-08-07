package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/pkg/server"
)

// Example logger implementation
type exampleLogger struct{}

func (l *exampleLogger) Info(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[INFO] %s", msg)
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			fmt.Printf(" %v=%v", keysAndValues[i], keysAndValues[i+1])
		}
	}
	fmt.Println()
}

func (l *exampleLogger) Debug(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[DEBUG] %s", msg)
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			fmt.Printf(" %v=%v", keysAndValues[i], keysAndValues[i+1])
		}
	}
	fmt.Println()
}

func (l *exampleLogger) Warn(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[WARN] %s", msg)
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			fmt.Printf(" %v=%v", keysAndValues[i], keysAndValues[i+1])
		}
	}
	fmt.Println()
}

func (l *exampleLogger) Error(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[ERROR] %s", msg)
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			fmt.Printf(" %v=%v", keysAndValues[i], keysAndValues[i+1])
		}
	}
	fmt.Println()
}

func main() {
	logger := &exampleLogger{}

	// Create configuration with proxy chaining enabled
	config := &server.Config{
		Mode:       server.ModeDual,
		SOCKS5Addr: ":1080",
		HTTPAddr:   ":8080",
		APIAddr:    ":8888",
		SelectionStrategy: server.StrategySmart,
		
		// Configure regions with proxy chains
		Regions: map[string]*server.RegionConfig{
			"us-west": {
				Name: "us-west",
				Proxies: []server.ProxyConfig{
					{
						URL:            "http://proxy1.example.com:8080",
						Weight:         10,
						HealthCheckURL: "http://httpbin.org/ip",
					},
				},
			},
			"us-east": {
				Name: "us-east",
				Proxies: []server.ProxyConfig{
					{
						// Proxy chain example
						Chain: []string{
							"http://proxy2.example.com:8080",
							"socks5://proxy3.example.com:1080",
							"http://proxy4.example.com:3128",
						},
						Weight:         20,
						ChainTimeout:   30 * time.Second,
						RetryOnFailure: true,
						HealthCheckURL: "http://httpbin.org/ip",
					},
				},
			},
			"tor-chain": {
				Name: "tor-chain",
				Proxies: []server.ProxyConfig{
					{
						// Chain that will route through Tor when enabled
						Chain: []string{
							"http://proxy5.example.com:8080",
							"socks5://proxy6.example.com:1080",
						},
						Weight:         30,
						ChainTimeout:   45 * time.Second,
						RetryOnFailure: true,
						HealthCheckURL: "http://httpbin.org/ip",
					},
				},
			},
		},
		
		RoundRobinDetection: server.RoundRobinConfig{
			Enabled:             true,
			MinSamples:          3,
			SampleInterval:      2 * time.Second,
			ConfidenceThreshold: 0.8,
		},
		
		HealthCheck: server.HealthCheckConfig{
			Enabled:          true,
			Interval:         30 * time.Second,
			Timeout:          10 * time.Second,
			FailureThreshold: 3,
			SuccessThreshold: 2,
		},
		
		CacheConfig: server.CacheConfig{
			Enabled:    true,
			TTL:        5 * time.Minute,
			MaxEntries: 1000,
		},
		
		MetricsEnabled: true,
		MetricsAddr:    ":9090",
		LogLevel:       "info",
		LogFormat:      "json",
	}

	// Create the ProxyHawk server
	server := server.NewProxyHawkServer(config, logger)

	// Configure proxy chaining
	if router := server.GetProxyRouter(); router != nil {
		routerConfig := &server.RouterConfig{
			DefaultRegion:   "us-west",
			RegionHeader:    "X-Proxy-Region",
			StickySessions:  true,
			SessionTTL:      5 * time.Minute,
			SmartRouting:    true,
			CDNDetection:    true,
			GeoIPLookup:     true,
			EnableChaining:  true,
			MaxChainLength:  3,
			ChainTimeout:    30 * time.Second,
			ChainRetries:    2,
			TorIntegration: server.TorConfig{
				Enabled:        true,
				SOCKSAddr:      "127.0.0.1:9050",
				ControlAddr:    "127.0.0.1:9051",
				Password:       "",
				NewCircuitFreq: 10 * time.Minute,
				ExitNodes:      []string{"us", "de", "nl"},
				ExcludeNodes:   []string{"cn", "ru"},
			},
		}
		
		router.SetChainConfig(routerConfig)
		router.EnableChaining(true)
	}

	// Start the server in a goroutine
	go func() {
		logger.Info("Starting ProxyHawk server with proxy chaining")
		
		if err := server.StartDual(":1080", ":8080", ":8888"); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait a moment for server to start
	time.Sleep(2 * time.Second)

	// Example client usage
	demonstrateProxyChaining(logger)

	// Example WebSocket API usage
	demonstrateWebSocketAPI(logger)

	// Graceful shutdown
	logger.Info("Shutting down server")
	if err := server.Shutdown(); err != nil {
		logger.Error("Shutdown error", "error", err)
	}
}

func demonstrateProxyChaining(logger server.Logger) {
	// Example 1: Use ProxyHawk as a SOCKS5 proxy
	logger.Info("=== Example 1: Using ProxyHawk as SOCKS5 proxy ===")
	
	// This would route through the configured proxy chains
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*http.URL, error) {
				return http.ProxyURL(&http.URL{
					Scheme: "socks5",
					Host:   "localhost:1080",
				})(req)
			},
		},
	}

	// Make request with region header
	req, _ := http.NewRequest("GET", "http://httpbin.org/ip", nil)
	req.Header.Set("X-Proxy-Region", "tor-chain") // Use Tor chain region
	
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Request failed", "error", err)
		return
	}
	defer resp.Body.Close()
	
	logger.Info("Request successful", "status", resp.Status)

	// Example 2: Use ProxyHawk as HTTP proxy
	logger.Info("=== Example 2: Using ProxyHawk as HTTP proxy ===")
	
	client2 := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*http.URL, error) {
				return http.ProxyURL(&http.URL{
					Scheme: "http",
					Host:   "localhost:8080",
				})(req)
			},
		},
	}

	req2, _ := http.NewRequest("GET", "http://httpbin.org/headers", nil)
	req2.Header.Set("X-Proxy-Region", "us-east") // Use proxy chain region
	
	resp2, err := client2.Do(req2)
	if err != nil {
		logger.Error("Request failed", "error", err)
		return
	}
	defer resp2.Body.Close()
	
	logger.Info("Request successful", "status", resp2.Status)
}

func demonstrateWebSocketAPI(logger server.Logger) {
	logger.Info("=== Example 3: WebSocket API usage ===")
	
	// This would demonstrate:
	// 1. Connecting to WebSocket at ws://localhost:8888/ws
	// 2. Sending commands to request new Tor circuits
	// 3. Getting proxy chain statistics
	// 4. Performing geographic DNS testing through proxy chains
	
	logger.Info("WebSocket API available at ws://localhost:8888/ws")
	logger.Info("Example commands:")
	logger.Info("  New Tor circuit: {\"action\": \"new_tor_circuit\"}")
	logger.Info("  Get stats: {\"action\": \"get_stats\"}")
	logger.Info("  Test domain: {\"action\": \"test_domain\", \"domain\": \"example.com\", \"region\": \"tor-chain\"}")
	logger.Info("  Set region: {\"action\": \"set_region\", \"region\": \"us-east\"}")
}

// Additional helper function that would be available on the server
func (s *server.ProxyHawkServer) GetProxyRouter() *server.ProxyRouter {
	// This method would need to be added to the server struct
	// to provide access to the proxy router for configuration
	return nil // Placeholder
}