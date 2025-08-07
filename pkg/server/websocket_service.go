package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
	
	"github.com/gorilla/websocket"
)

const (
	// WebSocket configuration
	maxMessageSize = 1024 * 1024 // 1MB
	pongWait       = 60 * time.Second
	pingPeriod     = 54 * time.Second
	writeWait      = 10 * time.Second
)

// WebSocketService handles persistent connections for geographic testing
type WebSocketService struct {
	// Active connections
	clients    map[*WSClient]bool
	clientsMux sync.RWMutex
	
	// Message channels
	broadcast  chan Message
	register   chan *WSClient
	unregister chan *WSClient
	
	// Dependencies
	geoTester *GeographicTester
	dnsCache  *DNSCache
	logger    Logger
	
	// HTTP server
	server   *http.Server
	upgrader websocket.Upgrader
	
	// Subscription management
	subscriptions map[string]map[*WSClient]bool // domain -> clients
	subsMux       sync.RWMutex
}

// WSClient represents a WebSocket client
type WSClient struct {
	ID         string
	conn       *websocket.Conn
	send       chan Message
	
	// Client configuration
	regions    []string
	testMode   TestMode
	config     *ClientConfig
	
	// Connection info
	remoteAddr string
	userAgent  string
	connectedAt time.Time
	lastPong   time.Time
	
	// Message handlers
	handlers  map[string]chan Message
	handlerMu sync.RWMutex
	
	// Reference to service
	service *WebSocketService
}

// Message represents a WebSocket message
type Message struct {
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"`
	Domain    string          `json:"domain,omitempty"`
	Action    string          `json:"action,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Error     string          `json:"error,omitempty"`
}

// TestMode defines the testing mode
type TestMode string

const (
	TestModeBasic      TestMode = "basic"
	TestModeDetailed   TestMode = "detailed"
	TestModeComprehensive TestMode = "comprehensive"
)

// ClientConfig holds client-specific configuration
type ClientConfig struct {
	MaxConcurrentRequests int
	RequestTimeout        time.Duration
	EnableRealTimeUpdates bool
	BatchSize            int
}

// GeoTestRequest represents a geographic test request
type GeoTestRequest struct {
	Domain  string   `json:"domain"`
	Regions []string `json:"regions,omitempty"`
	Mode    TestMode `json:"mode,omitempty"`
}

// BatchTestRequest represents a batch test request
type BatchTestRequest struct {
	Domains []string `json:"domains"`
	Regions []string `json:"regions,omitempty"`
	Mode    TestMode `json:"mode,omitempty"`
}

// NewWebSocketService creates a new WebSocket service
func NewWebSocketService(geoTester *GeographicTester, dnsCache *DNSCache, logger Logger) *WebSocketService {
	return &WebSocketService{
		clients:       make(map[*WSClient]bool),
		broadcast:     make(chan Message, 256),
		register:      make(chan *WSClient, 64),
		unregister:    make(chan *WSClient, 64),
		geoTester:     geoTester,
		dnsCache:      dnsCache,
		logger:        logger,
		subscriptions: make(map[string]map[*WSClient]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Configure origin checking for security
				return true // For development - should be more restrictive in production
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

// Start starts the WebSocket service
func (s *WebSocketService) Start(addr string) error {
	// Create HTTP server
	mux := http.NewServeMux()
	
	// WebSocket endpoint
	mux.HandleFunc("/ws", s.handleWebSocket)
	
	// REST API endpoints
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/regions", s.handleRegions)
	mux.HandleFunc("/api/stats", s.handleStats)
	
	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	
	// Start message hub
	go s.run()
	
	s.logger.Info("Starting WebSocket service", "addr", addr)
	return s.server.ListenAndServe()
}

// run manages the WebSocket hub
func (s *WebSocketService) run() {
	for {
		select {
		case client := <-s.register:
			s.clientsMux.Lock()
			s.clients[client] = true
			s.clientsMux.Unlock()
			
			s.logger.Info("Client connected", 
				"id", client.ID,
				"addr", client.remoteAddr,
				"total_clients", len(s.clients))
			
			// Send welcome message
			welcome := Message{
				Type:      "welcome",
				Data:      marshalJSON(map[string]interface{}{
					"server": "ProxyHawk Geographic Agent",
					"version": "1.0.0",
					"client_id": client.ID,
				}),
				Timestamp: time.Now(),
			}
			
			select {
			case client.send <- welcome:
			default:
				close(client.send)
				delete(s.clients, client)
			}
			
		case client := <-s.unregister:
			s.clientsMux.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.send)
				
				// Remove from all subscriptions
				s.removeClientSubscriptions(client)
				
				s.logger.Info("Client disconnected",
					"id", client.ID,
					"addr", client.remoteAddr,
					"duration", time.Since(client.connectedAt),
					"total_clients", len(s.clients))
			}
			s.clientsMux.Unlock()
			
		case message := <-s.broadcast:
			s.clientsMux.RLock()
			for client := range s.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(s.clients, client)
				}
			}
			s.clientsMux.RUnlock()
		}
	}
}

// handleWebSocket handles WebSocket connection upgrades
func (s *WebSocketService) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("WebSocket upgrade failed", "error", err)
		return
	}
	
	// Create new client
	client := &WSClient{
		ID:          generateClientID(),
		conn:        conn,
		send:        make(chan Message, 256),
		regions:     []string{"us-west", "us-east", "eu-west"},
		testMode:    TestModeBasic,
		config:      &ClientConfig{
			MaxConcurrentRequests: 10,
			RequestTimeout:        30 * time.Second,
			EnableRealTimeUpdates: true,
			BatchSize:            50,
		},
		remoteAddr:  r.RemoteAddr,
		userAgent:   r.UserAgent(),
		connectedAt: time.Now(),
		lastPong:    time.Now(),
		handlers:    make(map[string]chan Message),
		service:     s,
	}
	
	s.register <- client
	
	// Start client goroutines
	go client.writePump()
	go client.readPump()
}

// handleHealth handles health check requests
func (s *WebSocketService) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.clientsMux.RLock()
	clientCount := len(s.clients)
	s.clientsMux.RUnlock()
	
	health := map[string]interface{}{
		"status": "healthy",
		"clients": clientCount,
		"uptime": time.Since(time.Now()).String(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleRegions handles regions API request
func (s *WebSocketService) handleRegions(w http.ResponseWriter, r *http.Request) {
	regions := []string{"us-west", "us-east", "eu-west", "asia", "au"}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{
		"regions": regions,
	})
}

// handleStats handles statistics API request
func (s *WebSocketService) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.GetStats()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// Client methods

// readPump handles incoming messages from clients
func (c *WSClient) readPump() {
	defer func() {
		c.service.unregister <- c
		c.conn.Close()
	}()
	
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.lastPong = time.Now()
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	
	for {
		var msg Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.service.logger.Error("WebSocket error", "client", c.ID, "error", err)
			}
			break
		}
		
		// Process message
		c.service.processMessage(c, msg)
	}
}

// writePump handles outgoing messages to clients
func (c *WSClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			
			if err := c.conn.WriteJSON(message); err != nil {
				c.service.logger.Error("Write error", "client", c.ID, "error", err)
				return
			}
			
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Message processing methods

// processMessage handles different message types
func (s *WebSocketService) processMessage(client *WSClient, msg Message) {
	switch msg.Type {
	case "test":
		s.handleGeoTest(client, msg)
	case "batch_test":
		s.handleBatchTest(client, msg)
	case "subscribe":
		s.handleSubscribe(client, msg)
	case "unsubscribe":
		s.handleUnsubscribe(client, msg)
	case "get_regions":
		s.handleGetRegions(client, msg)
	case "set_config":
		s.handleSetConfig(client, msg)
	case "ping":
		s.handlePing(client, msg)
	default:
		s.sendError(client, msg.ID, fmt.Sprintf("Unknown message type: %s", msg.Type))
	}
}

// handleGeoTest performs geographic testing for a domain
func (s *WebSocketService) handleGeoTest(client *WSClient, msg Message) {
	var request GeoTestRequest
	if err := json.Unmarshal(msg.Data, &request); err != nil {
		s.sendError(client, msg.ID, "Invalid request format")
		return
	}
	
	// Use client's regions if not specified
	if len(request.Regions) == 0 {
		request.Regions = client.regions
	}
	
	// Check cache first
	cacheKey := fmt.Sprintf("%s:%v", request.Domain, request.Regions)
	if s.dnsCache != nil {
		if cached := s.dnsCache.Get(cacheKey); cached != nil {
			s.sendCachedResult(client, msg.ID, cached)
			return
		}
	}
	
	// Perform geographic testing asynchronously
	go func() {
		result := s.geoTester.TestDomain(request.Domain, request.Regions)
		
		// Cache result
		if s.dnsCache != nil {
			s.dnsCache.Set(cacheKey, result)
		}
		
		// Send result to client
		response := Message{
			Type:      "test_result",
			ID:        msg.ID,
			Domain:    request.Domain,
			Data:      marshalJSON(result),
			Timestamp: time.Now(),
		}
		
		select {
		case client.send <- response:
		default:
			s.logger.Warn("Failed to send test result", "client", client.ID)
		}
		
		// Notify subscribed clients if geographic differences found
		if result.HasGeographicDifferences {
			s.notifySubscribers(request.Domain, result)
		}
	}()
}

// handleBatchTest performs batch testing
func (s *WebSocketService) handleBatchTest(client *WSClient, msg Message) {
	var request BatchTestRequest
	if err := json.Unmarshal(msg.Data, &request); err != nil {
		s.sendError(client, msg.ID, "Invalid batch request format")
		return
	}
	
	// Use client's regions if not specified
	if len(request.Regions) == 0 {
		request.Regions = client.regions
	}
	
	// Process batch asynchronously
	go func() {
		results := make([]*GeoTestResult, 0, len(request.Domains))
		
		// Process domains in batches
		batchSize := client.config.BatchSize
		for i := 0; i < len(request.Domains); i += batchSize {
			end := i + batchSize
			if end > len(request.Domains) {
				end = len(request.Domains)
			}
			
			batch := request.Domains[i:end]
			batchResults := s.geoTester.TestDomainsBatch(batch, request.Regions)
			results = append(results, batchResults...)
			
			// Send partial results
			partialResponse := Message{
				Type:   "batch_partial",
				ID:     msg.ID,
				Data:   marshalJSON(map[string]interface{}{
					"progress": float64(end) / float64(len(request.Domains)),
					"results":  batchResults,
				}),
				Timestamp: time.Now(),
			}
			
			select {
			case client.send <- partialResponse:
			default:
				s.logger.Warn("Failed to send batch partial result", "client", client.ID)
				return
			}
		}
		
		// Send final results
		response := Message{
			Type:      "batch_result",
			ID:        msg.ID,
			Data:      marshalJSON(results),
			Timestamp: time.Now(),
		}
		
		select {
		case client.send <- response:
		default:
			s.logger.Warn("Failed to send batch result", "client", client.ID)
		}
	}()
}

// handleSubscribe handles domain subscriptions
func (s *WebSocketService) handleSubscribe(client *WSClient, msg Message) {
	domain := msg.Domain
	if domain == "" {
		s.sendError(client, msg.ID, "Domain required for subscription")
		return
	}
	
	s.subsMux.Lock()
	if s.subscriptions[domain] == nil {
		s.subscriptions[domain] = make(map[*WSClient]bool)
	}
	s.subscriptions[domain][client] = true
	s.subsMux.Unlock()
	
	// Send confirmation
	response := Message{
		Type:   "subscribed",
		ID:     msg.ID,
		Domain: domain,
		Data:   marshalJSON(map[string]string{"status": "subscribed"}),
		Timestamp: time.Now(),
	}
	
	select {
	case client.send <- response:
	default:
		s.logger.Warn("Failed to send subscription confirmation", "client", client.ID)
	}
	
	s.logger.Info("Client subscribed", "client", client.ID, "domain", domain)
}

// handleUnsubscribe handles unsubscribing from domains
func (s *WebSocketService) handleUnsubscribe(client *WSClient, msg Message) {
	domain := msg.Domain
	
	s.subsMux.Lock()
	if clients, exists := s.subscriptions[domain]; exists {
		delete(clients, client)
		if len(clients) == 0 {
			delete(s.subscriptions, domain)
		}
	}
	s.subsMux.Unlock()
	
	// Send confirmation
	response := Message{
		Type:   "unsubscribed",
		ID:     msg.ID,
		Domain: domain,
		Data:   marshalJSON(map[string]string{"status": "unsubscribed"}),
		Timestamp: time.Now(),
	}
	
	select {
	case client.send <- response:
	default:
		s.logger.Warn("Failed to send unsubscription confirmation", "client", client.ID)
	}
}

// handleGetRegions returns available regions
func (s *WebSocketService) handleGetRegions(client *WSClient, msg Message) {
	regions := []string{"us-west", "us-east", "eu-west", "asia", "au"}
	
	response := Message{
		Type:      "regions",
		ID:        msg.ID,
		Data:      marshalJSON(regions),
		Timestamp: time.Now(),
	}
	
	select {
	case client.send <- response:
	default:
		s.logger.Warn("Failed to send regions", "client", client.ID)
	}
}

// handleSetConfig updates client configuration
func (s *WebSocketService) handleSetConfig(client *WSClient, msg Message) {
	var config map[string]interface{}
	if err := json.Unmarshal(msg.Data, &config); err != nil {
		s.sendError(client, msg.ID, "Invalid config format")
		return
	}
	
	// Update client configuration
	if regions, ok := config["regions"].([]interface{}); ok {
		client.regions = make([]string, len(regions))
		for i, r := range regions {
			if region, ok := r.(string); ok {
				client.regions[i] = region
			}
		}
	}
	
	if mode, ok := config["test_mode"].(string); ok {
		client.testMode = TestMode(mode)
	}
	
	// Send confirmation
	response := Message{
		Type:      "config_updated",
		ID:        msg.ID,
		Data:      marshalJSON(map[string]string{"status": "updated"}),
		Timestamp: time.Now(),
	}
	
	select {
	case client.send <- response:
	default:
		s.logger.Warn("Failed to send config confirmation", "client", client.ID)
	}
}

// handlePing responds to ping messages
func (s *WebSocketService) handlePing(client *WSClient, msg Message) {
	response := Message{
		Type:      "pong",
		ID:        msg.ID,
		Timestamp: time.Now(),
	}
	
	select {
	case client.send <- response:
	default:
		s.logger.Warn("Failed to send pong", "client", client.ID)
	}
}

// Helper methods

// sendError sends an error message to a client
func (s *WebSocketService) sendError(client *WSClient, msgID, errorMsg string) {
	response := Message{
		Type:      "error",
		ID:        msgID,
		Error:     errorMsg,
		Timestamp: time.Now(),
	}
	
	select {
	case client.send <- response:
	default:
		s.logger.Warn("Failed to send error", "client", client.ID, "error", errorMsg)
	}
}

// sendCachedResult sends a cached test result
func (s *WebSocketService) sendCachedResult(client *WSClient, msgID string, result interface{}) {
	response := Message{
		Type:      "test_result",
		ID:        msgID,
		Data:      marshalJSON(map[string]interface{}{
			"cached": true,
			"result": result,
		}),
		Timestamp: time.Now(),
	}
	
	select {
	case client.send <- response:
	default:
		s.logger.Warn("Failed to send cached result", "client", client.ID)
	}
}

// notifySubscribers notifies subscribed clients about domain changes
func (s *WebSocketService) notifySubscribers(domain string, result interface{}) {
	s.subsMux.RLock()
	clients := s.subscriptions[domain]
	s.subsMux.RUnlock()
	
	if len(clients) == 0 {
		return
	}
	
	notification := Message{
		Type:   "domain_update",
		Domain: domain,
		Data:   marshalJSON(result),
		Timestamp: time.Now(),
	}
	
	for client := range clients {
		select {
		case client.send <- notification:
		default:
			s.logger.Warn("Failed to send domain update", "client", client.ID, "domain", domain)
		}
	}
}

// removeClientSubscriptions removes all subscriptions for a client
func (s *WebSocketService) removeClientSubscriptions(client *WSClient) {
	s.subsMux.Lock()
	defer s.subsMux.Unlock()
	
	for domain, clients := range s.subscriptions {
		delete(clients, client)
		if len(clients) == 0 {
			delete(s.subscriptions, domain)
		}
	}
}

// GetStats returns WebSocket service statistics
func (s *WebSocketService) GetStats() map[string]interface{} {
	s.clientsMux.RLock()
	clientCount := len(s.clients)
	s.clientsMux.RUnlock()
	
	s.subsMux.RLock()
	subscriptionCount := len(s.subscriptions)
	totalSubs := 0
	for _, clients := range s.subscriptions {
		totalSubs += len(clients)
	}
	s.subsMux.RUnlock()
	
	return map[string]interface{}{
		"connected_clients":  clientCount,
		"unique_subscriptions": subscriptionCount,
		"total_subscriptions": totalSubs,
	}
}

// Utility functions

// generateClientID generates a unique client ID
func generateClientID() string {
	return fmt.Sprintf("client_%d", time.Now().UnixNano())
}

// marshalJSON marshals data to JSON
func marshalJSON(data interface{}) json.RawMessage {
	if data == nil {
		return json.RawMessage("null")
	}
	
	bytes, err := json.Marshal(data)
	if err != nil {
		return json.RawMessage(`{"error": "marshal failed"}`)
	}
	
	return json.RawMessage(bytes)
}