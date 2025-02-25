package proxy

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/projectdiscovery/interactsh/pkg/client"
	"github.com/projectdiscovery/interactsh/pkg/server"
)

// defaultInteractshClient implements the InteractshClient interface
type defaultInteractshClient struct {
	serverURL string
	token     string
	client    *http.Client
}

// InteractshClient represents our interface for Interactsh functionality
type InteractshClient interface {
	GenerateURL() (string, error)
	Poll(duration time.Duration) []Interaction
	Close()
}

// Interaction represents an interaction with the Interactsh server
type Interaction struct {
	Protocol string
	FullID   string
	RawData  string
	RemoteIP string
}

// InteractshTester handles Interactsh-based security testing
type InteractshTester struct {
	client       *client.Client
	interactions map[string][]server.Interaction
	mu           sync.RWMutex
	stopPolling  chan struct{}
}

// NewInteractshClient creates a new Interactsh client
func NewInteractshClient(serverURL, token string) (InteractshClient, error) {
	if serverURL == "" {
		serverURL = "https://interact.sh"
	}

	return &defaultInteractshClient{
		serverURL: serverURL,
		token:     token,
		client:    &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// NewInteractshTester creates a new Interactsh tester
func NewInteractshTester() (*InteractshTester, error) {
	c, err := client.New(client.DefaultOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create interactsh client: %v", err)
	}

	tester := &InteractshTester{
		client:       c,
		interactions: make(map[string][]server.Interaction),
		stopPolling:  make(chan struct{}),
	}

	// Start polling for interactions
	go tester.startPolling()

	return tester, nil
}

// GenerateURL generates a unique Interactsh URL
func (c *defaultInteractshClient) GenerateURL() (string, error) {
	// For now, generate a simple unique subdomain
	// In a real implementation, this would register with the Interactsh server
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%d.%s", timestamp, c.serverURL), nil
}

// Poll checks for interactions within the specified duration
func (c *defaultInteractshClient) Poll(duration time.Duration) []Interaction {
	// In a real implementation, this would poll the Interactsh server
	// For now, just return an empty slice
	return []Interaction{}
}

// Close closes the client connection
func (c *defaultInteractshClient) Close() {
	// Cleanup any resources if needed
}

// Close stops the Interactsh tester and releases resources
func (t *InteractshTester) Close() {
	close(t.stopPolling)
	t.client.StopPolling()
	t.client.Close()
}

// startPolling starts polling for Interactsh interactions
func (t *InteractshTester) startPolling() {
	t.client.StartPolling(time.Second, func(interaction *server.Interaction) {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.interactions[interaction.FullId] = append(t.interactions[interaction.FullId], *interaction)
	})
}

// GenerateURL generates a new Interactsh URL
func (t *InteractshTester) GenerateURL() string {
	return t.client.URL()
}

// CheckInteractions checks for interactions with the given correlation ID
func (t *InteractshTester) CheckInteractions(correlationID string, timeout time.Duration) []server.Interaction {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		t.mu.RLock()
		interactions := t.interactions[correlationID]
		t.mu.RUnlock()
		if len(interactions) > 0 {
			return interactions
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// PerformInteractshTest performs a test using Interactsh
func (t *InteractshTester) PerformInteractshTest(client *http.Client, testFunc func(url string) (*http.Request, error)) (*CheckResult, error) {
	url := t.GenerateURL()
	result := &CheckResult{
		URL:     fmt.Sprintf("http://%s", url),
		Success: false,
	}

	req, err := testFunc(url)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	defer resp.Body.Close()

	result.Speed = time.Since(start)
	result.StatusCode = resp.StatusCode

	// Check for interactions
	interactions := t.CheckInteractions(url, 2*time.Second)
	result.Success = len(interactions) > 0

	return result, nil
}
