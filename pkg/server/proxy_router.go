package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
	
	"github.com/armon/go-socks5"
	"github.com/elazarl/goproxy"
)

// ProxyRouter handles both SOCKS5 and HTTP proxy requests
type ProxyRouter struct {
	pools    *ProxyPoolManager
	strategy SelectionStrategy
	logger   Logger
	
	// Session management for sticky sessions
	sessions     map[string]*SessionInfo
	sessionMutex sync.RWMutex
	
	// Configuration
	config *RouterConfig
	
	// Proxy chaining
	proxyChain *ProxyChain
}

// RouterConfig contains proxy routing configuration  
type RouterConfig struct {
	DefaultRegion string
	RegionHeader  string // e.g., "X-Proxy-Region: us-west"
	IPRegionMap   map[string]string
	
	// Sticky sessions
	StickySessions  bool
	SessionTTL      time.Duration
	
	// Advanced features
	SmartRouting    bool
	CDNDetection    bool
	GeoIPLookup     bool
	
	// Proxy chaining features
	EnableChaining  bool
	TorIntegration  TorConfig
	MaxChainLength  int
	ChainTimeout    time.Duration
	ChainRetries    int
}

// TorConfig holds Tor integration settings
type TorConfig struct {
	Enabled        bool
	SOCKSAddr      string // Default: 127.0.0.1:9050
	ControlAddr    string // Default: 127.0.0.1:9051
	Password       string
	NewCircuitFreq time.Duration // How often to request new circuits
	ExitNodes      []string      // Preferred exit nodes by country
	ExcludeNodes   []string      // Nodes to avoid
}

// SessionInfo holds sticky session information
type SessionInfo struct {
	ProxyURL   string
	Region     string
	CreatedAt  time.Time
	LastUsed   time.Time
	RequestCount int
}

// NewProxyRouter creates a new proxy router
func NewProxyRouter(pools *ProxyPoolManager, strategy SelectionStrategy, logger Logger) *ProxyRouter {
	config := &RouterConfig{
		DefaultRegion:  "us-west",
		RegionHeader:   "X-Proxy-Region",
		StickySessions: false,
		SessionTTL:     5 * time.Minute,
		SmartRouting:   true,
		CDNDetection:   true,
		GeoIPLookup:    true,
		EnableChaining: false, // Disabled by default
		MaxChainLength: 3,     // Reasonable default
		ChainTimeout:   30 * time.Second,
		ChainRetries:   2,
	}
	
	router := &ProxyRouter{
		pools:    pools,
		strategy: strategy,
		logger:   logger,
		sessions: make(map[string]*SessionInfo),
		config:   config,
	}
	
	// Initialize proxy chain handler
	router.proxyChain = NewProxyChain(config, logger)
	if router.proxyChain == nil {
		logger.Error("Failed to initialize proxy chain handler - disabling chaining")
		config.EnableChaining = false
	}
	
	return router
}

// StartSOCKS5Server starts the SOCKS5 proxy server
func (r *ProxyRouter) StartSOCKS5Server(addr string) error {
	// Custom SOCKS5 configuration
	conf := &socks5.Config{
		Dial: r.dialWithRegion,
		Resolver: &customResolver{router: r},
		Rules: &permissiveRules{},
		Logger: log.New(os.Stderr, "[SOCKS5] ", log.LstdFlags),
	}
	
	server, err := socks5.New(conf)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 server: %w", err)
	}
	
	r.logger.Info("Starting SOCKS5 proxy", "addr", addr)
	return server.ListenAndServe("tcp", addr)
}

// StartHTTPProxy starts the HTTP proxy server
func (r *ProxyRouter) StartHTTPProxy(addr string) error {
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = false
	
	// Set global proxy function
	proxy.Tr.Proxy = func(req *http.Request) (*url.URL, error) {
		// Extract region from header or use smart selection
		region := r.selectRegion(req)
		
		// Get proxy for region
		proxyInfo := r.pools.GetProxy(region)
		if proxyInfo == nil {
			r.logger.Warn("No proxy available", "region", region)
			return nil, nil
		}
		
		// Parse proxy URL
		proxyURL, err := url.Parse(proxyInfo.URL)
		if err != nil {
			r.logger.Error("Invalid proxy URL", "url", proxyInfo.URL, "error", err)
			return nil, nil
		}
		
		// Add tracking headers
		req.Header.Set("X-ProxyHawk-Region", region)
		req.Header.Set("X-ProxyHawk-Proxy", proxyURL.Host)
		
		// Handle sticky sessions
		if r.config.StickySessions {
			r.updateSession(req.Host, proxyInfo.URL, region)
		}
		
		return proxyURL, nil
	}
	
	// Intercept HTTPS CONNECT requests
	proxy.OnRequest().HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		// Select region for HTTPS
		region := r.smartSelectRegion(host)
		
		// Get proxy for region
		proxyInfo := r.pools.GetProxy(region)
		if proxyInfo == nil {
			r.logger.Warn("No proxy available for HTTPS", "region", region, "host", host)
			return goproxy.OkConnect, host
		}
		
		// Handle sticky sessions
		if r.config.StickySessions {
			r.updateSession(host, proxyInfo.URL, region)
		}
		
		// Return action to use upstream proxy
		return goproxy.OkConnect, host
	})
	
	r.logger.Info("Starting HTTP proxy", "addr", addr)
	return http.ListenAndServe(addr, proxy)
}

// dialWithRegion creates a connection through a regional proxy
func (r *ProxyRouter) dialWithRegion(ctx context.Context, network, addr string) (net.Conn, error) {
	// Extract region from context or use smart selection
	region := r.extractRegionFromContext(ctx)
	if region == "" {
		region = r.smartSelectRegion(addr)
	}
	
	// Check for sticky session
	if r.config.StickySessions {
		if session := r.getSession(addr); session != nil {
			region = session.Region
		}
	}
	
	// Get proxy for selected region
	proxyInfo := r.pools.GetHealthyProxy(region)
	if proxyInfo == nil {
		return nil, fmt.Errorf("no healthy proxy for region %s", region)
	}
	
	r.logger.Debug("Dialing through proxy", 
		"target", addr,
		"region", region,
		"proxy", proxyInfo.URL)
	
	// Check if proxy chaining is enabled and chain is configured
	if r.config.EnableChaining && len(proxyInfo.Chain) > 0 {
		// Use proxy chaining
		return r.dialThroughChain(ctx, proxyInfo.Chain, addr)
	} else if proxyInfo.URL != "" {
		// Use single proxy
		proxyURL, err := url.Parse(proxyInfo.URL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		return r.dialThroughProxy(proxyURL, network, addr)
	} else {
		return nil, fmt.Errorf("no proxy configuration found for region %s", region)
	}
}

// dialThroughChain establishes connection through a proxy chain
func (r *ProxyRouter) dialThroughChain(ctx context.Context, proxyChain []string, targetAddr string) (net.Conn, error) {
	if r.proxyChain == nil {
		return nil, fmt.Errorf("proxy chain handler not initialized")
	}
	
	r.logger.Debug("Establishing connection through proxy chain",
		"chain_length", len(proxyChain),
		"target", targetAddr)
	
	chainedConn, err := r.proxyChain.EstablishChain(ctx, proxyChain, targetAddr)
	if err != nil {
		// Retry logic if enabled
		if r.config.ChainRetries > 0 {
			for retry := 1; retry <= r.config.ChainRetries; retry++ {
				r.logger.Debug("Retrying proxy chain", "attempt", retry)
				time.Sleep(time.Duration(retry) * time.Second) // Exponential backoff
				
				chainedConn, err = r.proxyChain.EstablishChain(ctx, proxyChain, targetAddr)
				if err == nil {
					break
				}
			}
		}
		
		if err != nil {
			return nil, fmt.Errorf("failed to establish proxy chain after %d retries: %w", r.config.ChainRetries, err)
		}
	}
	
	r.logger.Info("Proxy chain established",
		"chain_length", len(proxyChain),
		"latency", chainedConn.GetLatency(),
		"target", targetAddr)
	
	return chainedConn, nil
}

// dialThroughProxy establishes connection through a proxy
func (r *ProxyRouter) dialThroughProxy(proxyURL *url.URL, network, addr string) (net.Conn, error) {
	// Connect to proxy
	proxyConn, err := net.DialTimeout("tcp", proxyURL.Host, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}
	
	// Handle different proxy types
	switch proxyURL.Scheme {
	case "socks5":
		return r.setupSOCKS5Connection(proxyConn, addr)
	case "http", "https":
		return r.setupHTTPConnection(proxyConn, addr)
	default:
		proxyConn.Close()
		return nil, fmt.Errorf("unsupported proxy scheme: %s", proxyURL.Scheme)
	}
}

// setupSOCKS5Connection sets up a SOCKS5 connection
func (r *ProxyRouter) setupSOCKS5Connection(conn net.Conn, targetAddr string) (net.Conn, error) {
	// Simple SOCKS5 handshake (no auth)
	// Send greeting
	greeting := []byte{0x05, 0x01, 0x00} // Version 5, 1 method, no auth
	if _, err := conn.Write(greeting); err != nil {
		conn.Close()
		return nil, err
	}
	
	// Read response
	response := make([]byte, 2)
	if _, err := io.ReadFull(conn, response); err != nil {
		conn.Close()
		return nil, err
	}
	
	if response[0] != 0x05 || response[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 handshake failed")
	}
	
	// Parse target address
	host, port, err := net.SplitHostPort(targetAddr)
	if err != nil {
		conn.Close()
		return nil, err
	}
	
	// Build connect request
	req := buildSOCKS5ConnectRequest(host, port)
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, err
	}
	
	// Read connect response
	resp := make([]byte, 10) // Minimum response size
	if _, err := io.ReadFull(conn, resp[:4]); err != nil {
		conn.Close()
		return nil, err
	}
	
	if resp[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 connect failed with code %d", resp[1])
	}
	
	// Skip the rest of the response based on address type
	switch resp[3] {
	case 0x01: // IPv4
		io.ReadFull(conn, resp[4:10])
	case 0x03: // Domain
		var domainLen [1]byte
		io.ReadFull(conn, domainLen[:])
		io.CopyN(io.Discard, conn, int64(domainLen[0])+2)
	case 0x04: // IPv6
		io.CopyN(io.Discard, conn, 18)
	}
	
	return conn, nil
}

// buildSOCKS5ConnectRequest builds a SOCKS5 connect request
func buildSOCKS5ConnectRequest(host, port string) []byte {
	req := []byte{0x05, 0x01, 0x00} // Version, Connect command, Reserved
	
	// Try to parse as IP
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			// IPv4
			req = append(req, 0x01)
			req = append(req, ip4...)
		} else {
			// IPv6
			req = append(req, 0x04)
			req = append(req, ip...)
		}
	} else {
		// Domain name
		req = append(req, 0x03)
		req = append(req, byte(len(host)))
		req = append(req, []byte(host)...)
	}
	
	// Add port (big-endian)
	portNum := parsePort(port)
	req = append(req, byte(portNum>>8), byte(portNum))
	
	return req
}

// setupHTTPConnection sets up an HTTP CONNECT tunnel
func (r *ProxyRouter) setupHTTPConnection(conn net.Conn, targetAddr string) (net.Conn, error) {
	// Send CONNECT request
	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", targetAddr, targetAddr)
	if _, err := conn.Write([]byte(connectReq)); err != nil {
		conn.Close()
		return nil, err
	}
	
	// Read response
	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err != nil {
		conn.Close()
		return nil, err
	}
	
	// Check for 200 OK
	if !strings.Contains(string(response[:n]), "200") {
		conn.Close()
		return nil, fmt.Errorf("HTTP CONNECT failed: %s", string(response[:n]))
	}
	
	return conn, nil
}

// selectRegion selects the appropriate region for a request
func (r *ProxyRouter) selectRegion(req *http.Request) string {
	// Check for explicit region header
	if region := req.Header.Get(r.config.RegionHeader); region != "" {
		return region
	}
	
	// Check for sticky session
	if r.config.StickySessions {
		if session := r.getSession(req.Host); session != nil {
			return session.Region
		}
	}
	
	// Use smart selection
	if r.config.SmartRouting {
		return r.smartSelectRegion(req.Host)
	}
	
	return r.config.DefaultRegion
}

// smartSelectRegion intelligently selects region based on target
func (r *ProxyRouter) smartSelectRegion(addr string) string {
	// Parse target address
	host, _, _ := net.SplitHostPort(addr)
	if host == "" {
		host = addr
	}
	
	// Check if it's a CDN or known geographic service
	if r.config.CDNDetection && r.isCDN(host) {
		// For CDNs, rotate through regions to test all endpoints
		return r.pools.GetNextRegion()
	}
	
	// Check target's geographic location
	if r.config.GeoIPLookup {
		if targetRegion := r.geolocateTarget(host); targetRegion != "" {
			return targetRegion
		}
	}
	
	// Check IP region map
	if r.config.IPRegionMap != nil {
		if ip := net.ParseIP(host); ip != nil {
			for cidr, region := range r.config.IPRegionMap {
				if _, network, err := net.ParseCIDR(cidr); err == nil {
					if network.Contains(ip) {
						return region
					}
				}
			}
		}
	}
	
	// Default to configured region
	return r.config.DefaultRegion
}

// isCDN checks if the host is a known CDN
func (r *ProxyRouter) isCDN(host string) bool {
	cdnPatterns := []string{
		"cloudflare", "cloudfront", "akamai", "fastly",
		"cdn", "edge", "cache", "static",
		"amazonaws.com", "azureedge.net", "googleusercontent.com",
	}
	
	hostLower := strings.ToLower(host)
	for _, pattern := range cdnPatterns {
		if strings.Contains(hostLower, pattern) {
			return true
		}
	}
	
	return false
}

// geolocateTarget attempts to determine the geographic location of the target
func (r *ProxyRouter) geolocateTarget(host string) string {
	// Simple heuristic based on TLD and known patterns
	if strings.HasSuffix(host, ".eu") || strings.Contains(host, "europe") {
		return "eu-west"
	}
	if strings.HasSuffix(host, ".asia") || strings.Contains(host, "asia") {
		return "asia"
	}
	if strings.HasSuffix(host, ".au") || strings.Contains(host, "australia") {
		return "au"
	}
	
	// Could integrate with a GeoIP service here
	return ""
}

// extractRegionFromContext extracts region from context
func (r *ProxyRouter) extractRegionFromContext(ctx context.Context) string {
	if region, ok := ctx.Value("region").(string); ok {
		return region
	}
	return ""
}

// Session management methods

func (r *ProxyRouter) getSession(host string) *SessionInfo {
	r.sessionMutex.RLock()
	defer r.sessionMutex.RUnlock()
	
	session, exists := r.sessions[host]
	if !exists {
		return nil
	}
	
	// Check if session is expired
	if time.Since(session.LastUsed) > r.config.SessionTTL {
		return nil
	}
	
	return session
}

func (r *ProxyRouter) updateSession(host, proxyURL, region string) {
	r.sessionMutex.Lock()
	defer r.sessionMutex.Unlock()
	
	if session, exists := r.sessions[host]; exists {
		session.LastUsed = time.Now()
		session.RequestCount++
	} else {
		r.sessions[host] = &SessionInfo{
			ProxyURL:     proxyURL,
			Region:       region,
			CreatedAt:    time.Now(),
			LastUsed:     time.Now(),
			RequestCount: 1,
		}
	}
	
	// Clean up expired sessions periodically
	if len(r.sessions) > 1000 {
		r.cleanupSessions()
	}
}

func (r *ProxyRouter) cleanupSessions() {
	now := time.Now()
	for host, session := range r.sessions {
		if now.Sub(session.LastUsed) > r.config.SessionTTL {
			delete(r.sessions, host)
		}
	}
}

// Helper types for SOCKS5

type customResolver struct {
	router *ProxyRouter
}

func (r *customResolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	// Could implement custom DNS resolution here
	addr, err := net.ResolveIPAddr("ip", name)
	if err != nil {
		return ctx, nil, err
	}
	return ctx, addr.IP, nil
}

type permissiveRules struct{}

func (r *permissiveRules) Allow(ctx context.Context, req *socks5.Request) (context.Context, bool) {
	// Allow all connections
	return ctx, true
}

type socks5Logger struct {
	logger Logger
}

func (l *socks5Logger) Printf(format string, v ...interface{}) {
	l.logger.Debug(fmt.Sprintf(format, v...))
}

// Helper function to parse port
func parsePort(portStr string) uint16 {
	var port uint16
	fmt.Sscanf(portStr, "%d", &port)
	return port
}

// SetChainConfig updates the proxy chaining configuration
func (r *ProxyRouter) SetChainConfig(config *RouterConfig) {
	r.config = config
	if r.proxyChain != nil {
		r.proxyChain.config = config
	}
}

// EnableChaining enables or disables proxy chaining
func (r *ProxyRouter) EnableChaining(enabled bool) {
	r.config.EnableChaining = enabled
}

// RequestNewTorCircuit requests a new Tor circuit if Tor integration is enabled
func (r *ProxyRouter) RequestNewTorCircuit() error {
	if r.proxyChain == nil {
		return fmt.Errorf("proxy chain handler not initialized")
	}
	return r.proxyChain.RequestNewTorCircuit()
}

// GetChainStats returns statistics about proxy chains
func (r *ProxyRouter) GetChainStats() map[string]interface{} {
	stats := make(map[string]interface{})
	stats["chaining_enabled"] = r.config.EnableChaining
	stats["max_chain_length"] = r.config.MaxChainLength
	stats["chain_timeout"] = r.config.ChainTimeout.String()
	stats["chain_retries"] = r.config.ChainRetries
	stats["tor_enabled"] = r.config.TorIntegration.Enabled
	return stats
}