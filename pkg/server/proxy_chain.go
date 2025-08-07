package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ProxyChain handles chaining multiple proxies together
type ProxyChain struct {
	config  *RouterConfig
	logger  Logger
	torCtrl *TorController
}

// ChainedConnection represents a connection through a proxy chain
type ChainedConnection struct {
	finalConn net.Conn
	chain     []string
	latency   time.Duration
}

// TorController manages Tor integration
type TorController struct {
	controlAddr string
	password    string
	conn        net.Conn
	logger      Logger
}

// NewProxyChain creates a new proxy chain handler
func NewProxyChain(config *RouterConfig, logger Logger) *ProxyChain {
	// Validate required parameters
	if config == nil {
		if logger != nil {
			logger.Error("NewProxyChain called with nil config")
		}
		return nil
	}
	if logger == nil {
		// Use a no-op logger if none provided
		logger = &noOpLogger{}
	}
	
	pc := &ProxyChain{
		config: config,
		logger: logger,
	}
	
	// Initialize Tor controller if enabled
	if config.TorIntegration.Enabled {
		pc.torCtrl = &TorController{
			controlAddr: config.TorIntegration.ControlAddr,
			password:    config.TorIntegration.Password,
			logger:      logger,
		}
		
		// Set defaults
		if pc.torCtrl.controlAddr == "" {
			pc.torCtrl.controlAddr = "127.0.0.1:9051"
		}
	}
	
	return pc
}

// EstablishChain creates a connection through a chain of proxies
func (pc *ProxyChain) EstablishChain(ctx context.Context, proxyChain []string, targetAddr string) (*ChainedConnection, error) {
	if len(proxyChain) == 0 {
		return nil, fmt.Errorf("empty proxy chain")
	}
	
	if len(proxyChain) > pc.config.MaxChainLength {
		return nil, fmt.Errorf("chain too long: %d > %d", len(proxyChain), pc.config.MaxChainLength)
	}
	
	start := time.Now()
	
	pc.logger.Debug("Establishing proxy chain", 
		"chain", proxyChain,
		"target", targetAddr)
	
	// Check if we should route through Tor first
	if pc.config.TorIntegration.Enabled && pc.shouldUseTor(targetAddr) {
		proxyChain = pc.prependTorProxy(proxyChain)
	}
	
	var currentConn net.Conn
	var err error
	
	// Establish connection through each proxy in the chain
	for i, proxyURL := range proxyChain {
		if i == 0 {
			// First proxy - direct connection
			currentConn, err = pc.connectToProxy(ctx, proxyURL)
		} else {
			// Subsequent proxies - tunnel through previous connection
			currentConn, err = pc.tunnelThroughProxy(ctx, currentConn, proxyURL)
		}
		
		if err != nil {
			if currentConn != nil {
				currentConn.Close()
			}
			return nil, fmt.Errorf("failed at proxy %d (%s): %w", i+1, proxyURL, err)
		}
		
		pc.logger.Debug("Successfully connected through proxy",
			"proxy", proxyURL,
			"step", fmt.Sprintf("%d/%d", i+1, len(proxyChain)))
	}
	
	// Finally connect to target through the chain
	finalConn, err := pc.connectToTarget(ctx, currentConn, proxyChain[len(proxyChain)-1], targetAddr)
	if err != nil {
		currentConn.Close()
		return nil, fmt.Errorf("failed to connect to target %s: %w", targetAddr, err)
	}
	
	latency := time.Since(start)
	
	pc.logger.Info("Proxy chain established successfully",
		"chain_length", len(proxyChain),
		"latency", latency,
		"target", targetAddr)
	
	return &ChainedConnection{
		finalConn: finalConn,
		chain:     proxyChain,
		latency:   latency,
	}, nil
}

// connectToProxy establishes initial connection to the first proxy
func (pc *ProxyChain) connectToProxy(ctx context.Context, proxyURL string) (net.Conn, error) {
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}
	
	// Set timeout
	timeout := pc.config.ChainTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", parsed.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}
	
	return conn, nil
}

// tunnelThroughProxy establishes a tunnel through an existing proxy connection
func (pc *ProxyChain) tunnelThroughProxy(ctx context.Context, existingConn net.Conn, nextProxyURL string) (net.Conn, error) {
	parsed, err := url.Parse(nextProxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid next proxy URL: %w", err)
	}
	
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return pc.tunnelThroughHTTPProxy(existingConn, parsed.Host)
	case "socks5":
		return pc.tunnelThroughSOCKS5Proxy(existingConn, parsed.Host)
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", parsed.Scheme)
	}
}

// tunnelThroughHTTPProxy creates an HTTP CONNECT tunnel
func (pc *ProxyChain) tunnelThroughHTTPProxy(conn net.Conn, nextProxyHost string) (net.Conn, error) {
	// Send HTTP CONNECT request
	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Connection: keep-alive\r\n\r\n", 
		nextProxyHost, nextProxyHost)
	
	if _, err := conn.Write([]byte(connectReq)); err != nil {
		return nil, fmt.Errorf("failed to send CONNECT: %w", err)
	}
	
	// Read response
	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err != nil {
		return nil, fmt.Errorf("failed to read CONNECT response: %w", err)
	}
	
	responseStr := string(response[:n])
	if !strings.Contains(responseStr, "200") {
		return nil, fmt.Errorf("HTTP CONNECT failed: %s", responseStr)
	}
	
	pc.logger.Debug("HTTP tunnel established", "next_proxy", nextProxyHost)
	return conn, nil
}

// tunnelThroughSOCKS5Proxy creates a SOCKS5 tunnel
func (pc *ProxyChain) tunnelThroughSOCKS5Proxy(conn net.Conn, nextProxyHost string) (net.Conn, error) {
	// SOCKS5 handshake
	if err := pc.socks5Handshake(conn); err != nil {
		return nil, fmt.Errorf("SOCKS5 handshake failed: %w", err)
	}
	
	// SOCKS5 connect request
	if err := pc.socks5Connect(conn, nextProxyHost); err != nil {
		return nil, fmt.Errorf("SOCKS5 connect failed: %w", err)
	}
	
	pc.logger.Debug("SOCKS5 tunnel established", "next_proxy", nextProxyHost)
	return conn, nil
}

// connectToTarget makes final connection to the target through the established chain
func (pc *ProxyChain) connectToTarget(ctx context.Context, chainConn net.Conn, lastProxyURL, targetAddr string) (net.Conn, error) {
	parsed, err := url.Parse(lastProxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid last proxy URL: %w", err)
	}
	
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return pc.connectTargetThroughHTTP(chainConn, targetAddr)
	case "socks5":
		return pc.connectTargetThroughSOCKS5(chainConn, targetAddr)
	default:
		return nil, fmt.Errorf("unsupported final proxy scheme: %s", parsed.Scheme)
	}
}

// connectTargetThroughHTTP connects to target through HTTP proxy
func (pc *ProxyChain) connectTargetThroughHTTP(conn net.Conn, targetAddr string) (net.Conn, error) {
	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Connection: keep-alive\r\n\r\n", 
		targetAddr, targetAddr)
	
	if _, err := conn.Write([]byte(connectReq)); err != nil {
		return nil, fmt.Errorf("failed to send target CONNECT: %w", err)
	}
	
	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err != nil {
		return nil, fmt.Errorf("failed to read target CONNECT response: %w", err)
	}
	
	responseStr := string(response[:n])
	if !strings.Contains(responseStr, "200") {
		return nil, fmt.Errorf("target HTTP CONNECT failed: %s", responseStr)
	}
	
	return conn, nil
}

// connectTargetThroughSOCKS5 connects to target through SOCKS5 proxy
func (pc *ProxyChain) connectTargetThroughSOCKS5(conn net.Conn, targetAddr string) (net.Conn, error) {
	err := pc.socks5Connect(conn, targetAddr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// SOCKS5 helper methods

func (pc *ProxyChain) socks5Handshake(conn net.Conn) error {
	// Send greeting (no authentication)
	greeting := []byte{0x05, 0x01, 0x00}
	if _, err := conn.Write(greeting); err != nil {
		return err
	}
	
	// Read response
	response := make([]byte, 2)
	if _, err := io.ReadFull(conn, response); err != nil {
		return err
	}
	
	if response[0] != 0x05 || response[1] != 0x00 {
		return fmt.Errorf("SOCKS5 handshake rejected: %v", response)
	}
	
	return nil
}

func (pc *ProxyChain) socks5Connect(conn net.Conn, targetAddr string) error {
	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return fmt.Errorf("invalid target address: %w", err)
	}
	
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid port: %w", err)
	}
	
	// Build SOCKS5 connect request
	req := []byte{0x05, 0x01, 0x00} // Version, Connect, Reserved
	
	// Add address
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
	
	// Add port
	req = append(req, byte(port>>8), byte(port))
	
	if _, err := conn.Write(req); err != nil {
		return err
	}
	
	// Read response
	resp := make([]byte, 4)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return err
	}
	
	if resp[1] != 0x00 {
		return fmt.Errorf("SOCKS5 connect failed with code %d", resp[1])
	}
	
	// Skip the rest of the response based on address type
	switch resp[3] {
	case 0x01: // IPv4
		io.CopyN(io.Discard, conn, 6) // 4 bytes IP + 2 bytes port
	case 0x03: // Domain
		var domainLen [1]byte
		io.ReadFull(conn, domainLen[:])
		io.CopyN(io.Discard, conn, int64(domainLen[0])+2)
	case 0x04: // IPv6
		io.CopyN(io.Discard, conn, 18) // 16 bytes IP + 2 bytes port
	}
	
	return nil
}

// Tor integration methods

func (pc *ProxyChain) shouldUseTor(targetAddr string) bool {
	if !pc.config.TorIntegration.Enabled {
		return false
	}
	
	// Add logic here to determine when to use Tor
	// For now, use Tor for .onion domains or when explicitly configured
	return strings.HasSuffix(targetAddr, ".onion")
}

func (pc *ProxyChain) prependTorProxy(chain []string) []string {
	torProxy := pc.config.TorIntegration.SOCKSAddr
	if torProxy == "" {
		torProxy = "127.0.0.1:9050"
	}
	
	// Add socks5 scheme if not present
	if !strings.Contains(torProxy, "://") {
		torProxy = "socks5://" + torProxy
	}
	
	return append([]string{torProxy}, chain...)
}

// RequestNewTorCircuit requests a new Tor circuit
func (pc *ProxyChain) RequestNewTorCircuit() error {
	if pc.torCtrl == nil {
		return fmt.Errorf("Tor controller not initialized")
	}
	
	return pc.torCtrl.NewCircuit()
}

// Tor controller methods

func (tc *TorController) connect() error {
	if tc.conn != nil {
		return nil // Already connected
	}
	
	conn, err := net.Dial("tcp", tc.controlAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to Tor control: %w", err)
	}
	
	tc.conn = conn
	
	// Authenticate if password is set
	if tc.password != "" {
		if err := tc.authenticate(); err != nil {
			conn.Close()
			tc.conn = nil
			return fmt.Errorf("Tor authentication failed: %w", err)
		}
	}
	
	return nil
}

func (tc *TorController) authenticate() error {
	cmd := fmt.Sprintf("AUTHENTICATE \"%s\"\r\n", tc.password)
	if _, err := tc.conn.Write([]byte(cmd)); err != nil {
		return err
	}
	
	response := make([]byte, 1024)
	n, err := tc.conn.Read(response)
	if err != nil {
		return err
	}
	
	if !strings.Contains(string(response[:n]), "250 OK") {
		return fmt.Errorf("authentication rejected: %s", string(response[:n]))
	}
	
	return nil
}

func (tc *TorController) NewCircuit() error {
	if err := tc.connect(); err != nil {
		return err
	}
	
	cmd := "SIGNAL NEWNYM\r\n"
	if _, err := tc.conn.Write([]byte(cmd)); err != nil {
		return err
	}
	
	response := make([]byte, 1024)
	n, err := tc.conn.Read(response)
	if err != nil {
		return err
	}
	
	if !strings.Contains(string(response[:n]), "250 OK") {
		return fmt.Errorf("new circuit request failed: %s", string(response[:n]))
	}
	
	tc.logger.Info("Requested new Tor circuit")
	return nil
}

// ChainedConnection methods

func (cc *ChainedConnection) Read(b []byte) (n int, err error) {
	return cc.finalConn.Read(b)
}

func (cc *ChainedConnection) Write(b []byte) (n int, err error) {
	return cc.finalConn.Write(b)
}

func (cc *ChainedConnection) Close() error {
	return cc.finalConn.Close()
}

func (cc *ChainedConnection) LocalAddr() net.Addr {
	return cc.finalConn.LocalAddr()
}

func (cc *ChainedConnection) RemoteAddr() net.Addr {
	return cc.finalConn.RemoteAddr()
}

func (cc *ChainedConnection) SetDeadline(t time.Time) error {
	return cc.finalConn.SetDeadline(t)
}

func (cc *ChainedConnection) SetReadDeadline(t time.Time) error {
	return cc.finalConn.SetReadDeadline(t)
}

func (cc *ChainedConnection) SetWriteDeadline(t time.Time) error {
	return cc.finalConn.SetWriteDeadline(t)
}

// GetChain returns the proxy chain used for this connection
func (cc *ChainedConnection) GetChain() []string {
	return cc.chain
}

// GetLatency returns the time taken to establish the chain
func (cc *ChainedConnection) GetLatency() time.Duration {
	return cc.latency
}

// noOpLogger is a no-operation logger for fallback cases
type noOpLogger struct{}

func (l *noOpLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *noOpLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (l *noOpLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *noOpLogger) Error(msg string, keysAndValues ...interface{}) {}