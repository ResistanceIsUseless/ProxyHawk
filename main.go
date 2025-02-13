package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	interactsh "github.com/projectdiscovery/interactsh/pkg/client"
	"github.com/projectdiscovery/interactsh/pkg/server"
	"golang.org/x/net/proxy"
	"gopkg.in/yaml.v3"
)

// Styles
var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("87")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("87")).
			Padding(0, 1)

	progressStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("39")).
			Padding(0, 1).
			Margin(0, 0, 1, 0)

	statusBlockStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("99")).
				Padding(0, 1).
				Margin(0, 0, 1, 0)

	metricBlockStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("207")).
				Padding(0, 1).
				Margin(0, 0, 1, 0)

	metricLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Bold(true)

	metricValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	anonymousStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("105")).
			Bold(true)

	cloudStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true)
)

// Model represents the application state
type Model struct {
	progress    progress.Model
	total       int
	current     int
	results     []ProxyResultOutput
	quitting    bool
	err         error
	activeJobs  int
	queueSize   int
	successRate float64
	avgSpeed    time.Duration
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.String() == "q" {
			m.quitting = true
			return m, tea.Quit
		}
	case progressMsg:
		m.current = msg.current
		if msg.result.Working {
			m.results = append(m.results, msg.result)
		}
		m.activeJobs = msg.activeJobs
		m.queueSize = msg.queueSize
		m.successRate = msg.successRate
		m.avgSpeed = msg.avgSpeed

		if m.current >= m.total {
			m.quitting = true
			return m, tea.Quit
		}
		cmd := m.progress.SetPercent(float64(m.current) / float64(m.total))
		return m, cmd
	}

	progressModel, cmd := m.progress.Update(msg)
	m.progress = progressModel.(progress.Model)
	return m, cmd
}

func (m Model) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\nPress q to quit", m.err))
	}

	if m.quitting {
		return successStyle.Render("Quitting...")
	}

	str := strings.Builder{}

	// Title
	str.WriteString(headerStyle.Render("ProxyCheck Progress") + "\n\n")

	// Progress section
	progressBlock := strings.Builder{}
	progressBlock.WriteString(fmt.Sprintf("%s %s/%s\n",
		metricLabelStyle.Render("Progress:"),
		metricValueStyle.Render(fmt.Sprintf("%d", m.current)),
		metricValueStyle.Render(fmt.Sprintf("%d", m.total))))
	progressBlock.WriteString(m.progress.View())
	str.WriteString(progressStyle.Render(progressBlock.String()) + "\n")

	// Status metrics
	statusBlock := strings.Builder{}
	statusBlock.WriteString(fmt.Sprintf("%s %s\n",
		metricLabelStyle.Render("Active Workers:"),
		metricValueStyle.Render(fmt.Sprintf("%d", m.activeJobs))))
	statusBlock.WriteString(fmt.Sprintf("%s %s",
		metricLabelStyle.Render("Queue Size:"),
		metricValueStyle.Render(fmt.Sprintf("%d", m.queueSize))))
	str.WriteString(statusBlockStyle.Render(statusBlock.String()))

	// Performance metrics
	if m.current > 0 {
		perfBlock := strings.Builder{}
		successRateColor := "87" // Default color (good)
		if m.successRate < 50 {
			successRateColor = "203" // Red for low success rate
		} else if m.successRate < 80 {
			successRateColor = "214" // Yellow for medium success rate
		}

		perfBlock.WriteString(fmt.Sprintf("%s %s\n",
			metricLabelStyle.Render("Success Rate:"),
			lipgloss.NewStyle().
				Foreground(lipgloss.Color(successRateColor)).
				Bold(true).
				Render(fmt.Sprintf("%.1f%%", m.successRate))))

		if m.avgSpeed > 0 {
			speedColor := "87" // Default color (good)
			if m.avgSpeed > 2*time.Second {
				speedColor = "203" // Red for slow speed
			} else if m.avgSpeed > time.Second {
				speedColor = "214" // Yellow for medium speed
			}

			perfBlock.WriteString(fmt.Sprintf("%s %s",
				metricLabelStyle.Render("Average Speed:"),
				lipgloss.NewStyle().
					Foreground(lipgloss.Color(speedColor)).
					Render(m.avgSpeed.Round(time.Millisecond).String())))
		}
		str.WriteString(metricBlockStyle.Render(perfBlock.String()))
	}

	// Last few results
	if len(m.results) > 0 {
		resultBlock := strings.Builder{}
		resultBlock.WriteString(metricLabelStyle.Render("Recent Results:") + "\n")

		// Show last 5 results
		start := len(m.results)
		if start > 5 {
			start = len(m.results) - 5
		}

		for _, result := range m.results[start:] {
			status := successStyle.Render("✓")
			proxyStyle := successStyle
			if !result.Working {
				status = errorStyle.Render("✗")
				proxyStyle = errorStyle
			} else if result.IsAnonymous {
				status = anonymousStyle.Render("🔒")
			} else if result.CloudProvider != "" {
				status = cloudStyle.Render("☁")
			}

			line := fmt.Sprintf("%s %s", status, proxyStyle.Render(result.Proxy))
			if result.Working {
				line += fmt.Sprintf(" - %s", infoStyle.Render(result.Speed.String()))
			}
			if result.Error != "" {
				line += " " + errorStyle.Render(result.Error)
			}
			resultBlock.WriteString(line + "\n")
		}
		str.WriteString(metricBlockStyle.Render(resultBlock.String()) + "\n")
	}

	// Controls
	str.WriteString("\n" + infoStyle.Render("Press q to quit"))

	return str.String()
}

type progressMsg struct {
	current     int
	result      ProxyResultOutput
	activeJobs  int
	queueSize   int
	successRate float64
	avgSpeed    time.Duration
}

func checkProxiesWithProgress(proxies []string, singleURL string, useInteractsh, useIPInfo, useCloud, debug bool, timeout time.Duration, concurrency int) tea.Model {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	model := Model{
		progress: p,
		total:    len(proxies),
	}

	go func() {
		// Create buffered channels
		results := make(chan ProxyResult, len(proxies)) // Buffer all possible results
		jobs := make(chan int, len(proxies))            // Buffer all jobs
		activeJobs := make(chan struct{}, concurrency)  // Buffer active jobs

		// Fill jobs channel
		for i := range proxies {
			jobs <- i
		}
		close(jobs)

		// Track metrics
		var (
			mu           sync.Mutex
			successCount int64
			totalTime    time.Duration
		)

		// Create wait group for workers
		var wg sync.WaitGroup
		wg.Add(concurrency)

		// Start workers
		for i := 0; i < concurrency; i++ {
			go func() {
				defer wg.Done()
				for proxyIndex := range jobs {
					// Track active job
					activeJobs <- struct{}{}

					// Start timing
					start := time.Now()

					// Check proxy URL validity
					if _, err := url.Parse(proxies[proxyIndex]); err != nil {
						results <- ProxyResult{
							Proxy:   proxies[proxyIndex],
							Working: false,
							Error:   fmt.Errorf("invalid proxy URL: %v", err),
						}
					} else {
						// Perform the proxy check
						checkProxyWithInteractsh(proxies[proxyIndex], singleURL, useInteractsh, useIPInfo, useCloud, debug, timeout, results)
					}

					// Update metrics safely
					result := <-results // Get the result that was just sent
					mu.Lock()
					if result.Working {
						successCount++
						totalTime += time.Since(start)
					}
					mu.Unlock()

					<-activeJobs // Remove from active jobs

					// Send progress update
					program.Send(progressMsg{
						current: proxyIndex + 1,
						result: ProxyResultOutput{
							Proxy:          result.Proxy,
							Working:        result.Working,
							Speed:          result.Speed,
							Error:          result.Error.Error(),
							InteractshTest: result.InteractshTest,
							RealIP:         result.RealIP,
							ProxyIP:        result.ProxyIP,
							IsAnonymous:    result.IsAnonymous,
							CloudProvider:  result.CloudProvider.Name,
							InternalAccess: result.InternalAccess,
							MetadataAccess: result.MetadataAccess,
							Timestamp:      time.Now(),
						},
						activeJobs:  len(activeJobs),
						queueSize:   len(jobs),
						successRate: float64(successCount) / float64(proxyIndex+1) * 100,
						avgSpeed:    totalTime / time.Duration(successCount),
					})
				}
			}()
		}

		// Wait for all workers to finish
		go func() {
			wg.Wait()
			close(results)
			close(activeJobs)
		}()
	}()

	return model
}

var program *tea.Program

type CloudProvider struct {
	Name           string   `yaml:"name"`
	MetadataIPs    []string `yaml:"metadata_ips"`
	MetadataURLs   []string `yaml:"metadata_urls"`
	InternalRanges []string `yaml:"internal_ranges"`
	ASNs           []string `yaml:"asns"`
	OrgNames       []string `yaml:"org_names"`
}

type TestURL struct {
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

type TestURLConfig struct {
	DefaultURL           string    `yaml:"default_url"`
	RequiredSuccessCount int       `yaml:"required_success_count"`
	URLs                 []TestURL `yaml:"urls"`
}

type Config struct {
	CloudProviders []CloudProvider   `yaml:"cloud_providers"`
	DefaultHeaders map[string]string `yaml:"default_headers"`
	UserAgent      string            `yaml:"user_agent"`
	TestURLs       TestURLConfig     `yaml:"test_urls"`
	Validation     struct {
		RequireStatusCode   int      `yaml:"require_status_code"`
		RequireContentMatch string   `yaml:"require_content_match"`
		RequireHeaderFields []string `yaml:"require_header_fields"`
		DisallowedKeywords  []string `yaml:"disallowed_keywords"`
		MinResponseBytes    int      `yaml:"min_response_bytes"`
		AdvancedChecks      struct {
			TestProtocolSmuggling   bool     `yaml:"test_protocol_smuggling"`
			TestDNSRebinding        bool     `yaml:"test_dns_rebinding"`
			TestNonStandardPorts    []int    `yaml:"test_nonstandard_ports"`
			TestIPv6                bool     `yaml:"test_ipv6"`
			TestHTTPMethods         []string `yaml:"test_http_methods"`
			TestPathTraversal       bool     `yaml:"test_path_traversal"`
			TestCachePoisoning      bool     `yaml:"test_cache_poisoning"`
			TestHostHeaderInjection bool     `yaml:"test_host_header_injection"`
		} `yaml:"advanced_checks"`
	} `yaml:"validation"`
}

var config Config

func loadConfig(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return fmt.Errorf("error parsing config file: %v", err)
	}

	return nil
}

type IPInfoResponse struct {
	IP      string `json:"ip"`
	Org     string `json:"org"`
	ASN     string `json:"asn"`
	Company struct {
		Name string `json:"name"`
	} `json:"company"`
}

type ProxyResult struct {
	Proxy          string
	Working        bool
	Speed          time.Duration
	Error          error
	InteractshTest bool
	DebugInfo      string
	RealIP         string
	ProxyIP        string
	IsAnonymous    bool
	CloudProvider  *CloudProvider
	InternalAccess bool
	MetadataAccess bool
	AdvancedChecks *AdvancedCheckResult
	TestResults    map[string]bool
}

// ProxyResultOutput is used for JSON output
type ProxyResultOutput struct {
	Proxy          string               `json:"proxy"`
	Working        bool                 `json:"working"`
	Speed          time.Duration        `json:"speed_ns"`
	Error          string               `json:"error,omitempty"`
	InteractshTest bool                 `json:"interactsh_test"`
	RealIP         string               `json:"real_ip,omitempty"`
	ProxyIP        string               `json:"proxy_ip,omitempty"`
	IsAnonymous    bool                 `json:"is_anonymous"`
	CloudProvider  string               `json:"cloud_provider,omitempty"`
	InternalAccess bool                 `json:"internal_access"`
	MetadataAccess bool                 `json:"metadata_access"`
	Timestamp      time.Time            `json:"timestamp"`
	AdvancedChecks *AdvancedCheckResult `json:"advanced_checks,omitempty"`
}

type SummaryOutput struct {
	TotalProxies        int                 `json:"total_proxies"`
	WorkingProxies      int                 `json:"working_proxies"`
	InteractshProxies   int                 `json:"interactsh_proxies"`
	AnonymousProxies    int                 `json:"anonymous_proxies"`
	CloudProxies        int                 `json:"cloud_proxies"`
	InternalAccessCount int                 `json:"internal_access_count"`
	SuccessRate         float64             `json:"success_rate"`
	Results             []ProxyResultOutput `json:"results"`
}

// Add new function for WHOIS lookup
func getWhoisInfo(ip string) (string, error) {
	conn, err := net.Dial("tcp", "whois.iana.org:43")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	fmt.Fprintf(conn, "%s\n", ip)
	result, err := io.ReadAll(conn)
	if err != nil {
		return "", err
	}

	// Check if we need to query a specific WHOIS server
	whoisServer := ""
	for _, line := range strings.Split(string(result), "\n") {
		if strings.HasPrefix(line, "whois:") {
			whoisServer = strings.TrimSpace(strings.TrimPrefix(line, "whois:"))
			break
		}
	}

	if whoisServer != "" {
		conn, err = net.Dial("tcp", whoisServer+":43")
		if err != nil {
			return "", err
		}
		defer conn.Close()

		fmt.Fprintf(conn, "%s\n", ip)
		result, err = io.ReadAll(conn)
		if err != nil {
			return "", err
		}
	}

	return string(result), nil
}

// Add function to detect cloud provider from WHOIS data
func detectCloudProviderFromWhois(whoisData string) *CloudProvider {
	whoisUpper := strings.ToUpper(whoisData)
	for _, provider := range config.CloudProviders {
		for _, orgName := range provider.OrgNames {
			if strings.Contains(whoisUpper, strings.ToUpper(orgName)) {
				return &provider
			}
		}
	}
	return nil
}

// Add function to get random IPs from internal ranges
func getRandomInternalIPs(provider *CloudProvider, count int) []string {
	var ips []string
	for _, cidr := range provider.InternalRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}

		// Get first and last IP of range
		firstIP := network.IP
		lastIP := getLastIP(network)

		// Generate random IPs in this range
		for i := 0; i < count && len(ips) < count; i++ {
			randomIP := generateRandomIP(firstIP, lastIP)
			ips = append(ips, randomIP.String())
		}
	}
	return ips
}

// Add function to generate a random IP between two IPs
func generateRandomIP(first, last net.IP) net.IP {
	// Convert IPs to integers for random generation
	firstInt := ipToInt(first)
	lastInt := ipToInt(last)

	// Generate random number between first and last
	random := firstInt + rand.Int63n(lastInt-firstInt+1)

	// Convert back to IP
	return intToIP(random)
}

func ipToInt(ip net.IP) int64 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return int64(ip[0])<<24 | int64(ip[1])<<16 | int64(ip[2])<<8 | int64(ip[3])
}

func intToIP(i int64) net.IP {
	ip := make(net.IP, 4)
	ip[0] = byte(i >> 24)
	ip[1] = byte(i >> 16)
	ip[2] = byte(i >> 8)
	ip[3] = byte(i)
	return ip
}

// Add back getLastIP function
func getLastIP(network *net.IPNet) net.IP {
	lastIP := make(net.IP, len(network.IP))
	copy(lastIP, network.IP)
	for i := len(lastIP) - 1; i >= 0; i-- {
		lastIP[i] |= ^network.Mask[i]
	}
	return lastIP
}

// Modify checkInternalAccess function to separate concerns
func checkInternalAccess(client *http.Client, provider *CloudProvider, debug bool) (bool, bool, string) {
	debugInfo := ""
	if provider == nil {
		return false, false, debugInfo
	}

	// Step 1: Check internal IP ranges
	internalAccess := false
	randomIPs := getRandomInternalIPs(provider, 5)

	for _, ip := range randomIPs {
		url := fmt.Sprintf("http://%s/", ip)
		if debug {
			debugInfo += fmt.Sprintf("\nTrying internal IP: %s\n", url)
		}
		req, _ := http.NewRequest("GET", url, nil)

		// Add headers
		if config.UserAgent != "" {
			req.Header.Set("User-Agent", config.UserAgent)
		}
		for key, value := range config.DefaultHeaders {
			req.Header.Set(key, value)
		}

		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if debug {
				debugInfo += fmt.Sprintf("Internal access successful to %s!\n", ip)
			}
			internalAccess = true
			break
		}
	}

	// Step 2: Check metadata endpoints
	metadataAccess := false

	// First try standard metadata IPs
	for _, metadataIP := range provider.MetadataIPs {
		// Try both HTTP and HTTPS
		for _, scheme := range []string{"http", "https"} {
			metadataURL := fmt.Sprintf("%s://%s/", scheme, metadataIP)
			if debug {
				debugInfo += fmt.Sprintf("\nTrying metadata IP: %s\n", metadataURL)
			}

			req, _ := http.NewRequest("GET", metadataURL, nil)

			// Add headers
			if config.UserAgent != "" {
				req.Header.Set("User-Agent", config.UserAgent)
			}
			for key, value := range config.DefaultHeaders {
				req.Header.Set(key, value)
			}

			// Add common metadata headers
			req.Header.Set("Metadata", "true")
			req.Header.Set("Metadata-Flavor", "Google")
			req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "21600")

			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					if debug {
						body, _ := io.ReadAll(resp.Body)
						debugInfo += fmt.Sprintf("Metadata access successful via IP! Response:\n%s\n", string(body))
					}
					metadataAccess = true
					break
				}
			}
		}
		if metadataAccess {
			break
		}
	}

	return internalAccess, metadataAccess, debugInfo
}

func getIPInfo(client *http.Client) (IPInfoResponse, error) {
	req, err := http.NewRequest("GET", "https://ipinfo.io/json", nil)
	if err != nil {
		return IPInfoResponse{}, err
	}

	// Add headers
	if config.UserAgent != "" {
		req.Header.Set("User-Agent", config.UserAgent)
	}
	for key, value := range config.DefaultHeaders {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return IPInfoResponse{}, err
	}
	defer resp.Body.Close()

	var ipInfo IPInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&ipInfo); err != nil {
		return IPInfoResponse{}, err
	}
	return ipInfo, nil
}

func createProxyClient(proxyURL *url.URL, timeout time.Duration) (*http.Client, error) {
	transport := &http.Transport{
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
	}

	switch proxyURL.Scheme {
	case "http", "https":
		transport.Proxy = http.ProxyURL(proxyURL)
	case "socks4", "socks5":
		// Create a dialer for SOCKS
		baseDialer := &net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
		}

		// Handle both SOCKS4 and SOCKS5
		var auth *proxy.Auth
		if proxyURL.User != nil {
			password, _ := proxyURL.User.Password()
			auth = &proxy.Auth{
				User:     proxyURL.User.Username(),
				Password: password,
			}
		}

		// Try SOCKS5 first, then fallback to SOCKS4 if needed
		dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, auth, baseDialer)
		if err != nil {
			return nil, fmt.Errorf("failed to create SOCKS dialer: %v", err)
		}

		// Use the SOCKS dialer for all connections
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", proxyURL.Scheme)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}, nil
}

// Add new function to validate proxy response
func validateProxyResponse(resp *http.Response, body []byte, debug bool) (bool, string) {
	debugInfo := ""

	// 1. Check status code
	if config.Validation.RequireStatusCode != 0 && resp.StatusCode != config.Validation.RequireStatusCode {
		if debug {
			debugInfo += fmt.Sprintf("Status code %d does not match required %d\n",
				resp.StatusCode, config.Validation.RequireStatusCode)
		}
		return false, debugInfo
	}

	// 2. Check response size
	if config.Validation.MinResponseBytes > 0 && len(body) < config.Validation.MinResponseBytes {
		if debug {
			debugInfo += fmt.Sprintf("Response size %d bytes is less than required %d bytes\n",
				len(body), config.Validation.MinResponseBytes)
		}
		return false, debugInfo
	}

	// 3. Check required headers
	for _, header := range config.Validation.RequireHeaderFields {
		if resp.Header.Get(header) == "" {
			if debug {
				debugInfo += fmt.Sprintf("Required header '%s' is missing\n", header)
			}
			return false, debugInfo
		}
	}

	// 4. Check content match if specified
	if config.Validation.RequireContentMatch != "" {
		if !strings.Contains(string(body), config.Validation.RequireContentMatch) {
			if debug {
				debugInfo += fmt.Sprintf("Response does not contain required content '%s'\n",
					config.Validation.RequireContentMatch)
			}
			return false, debugInfo
		}
	}

	// 5. Check for disallowed keywords
	for _, keyword := range config.Validation.DisallowedKeywords {
		if strings.Contains(string(body), keyword) {
			if debug {
				debugInfo += fmt.Sprintf("Response contains disallowed keyword '%s'\n", keyword)
			}
			return false, debugInfo
		}
	}

	if debug {
		debugInfo += "Response validation successful\n"
	}
	return true, debugInfo
}

// Modify checkProxyWithInteractsh to use config URLs
func checkProxyWithInteractsh(proxy string, customURL string, useInteractsh bool, useIPInfo bool, useCloud bool, debug bool, timeout time.Duration, results chan<- ProxyResult) {
	start := time.Now()
	proxyURL, err := url.Parse(proxy)
	if err != nil {
		results <- ProxyResult{Proxy: proxy, Working: false, Error: fmt.Errorf("invalid proxy URL: %v", err)}
		return
	}

	// Initialize test results map
	testResults := make(map[string]bool)

	// Create client with appropriate proxy support
	client, err := createProxyClient(proxyURL, timeout)
	if err != nil {
		results <- ProxyResult{
			Proxy:         proxy,
			Working:       false,
			Error:         fmt.Errorf("failed to create proxy client: %v", err),
			CloudProvider: nil,
			TestResults:   testResults,
		}
		return
	}

	// Get URLs to test from config
	urlsToTest := config.TestURLs.URLs
	if customURL != "" {
		// Add custom URL if provided
		urlsToTest = append(urlsToTest, TestURL{
			URL:         customURL,
			Description: "Custom test URL",
			Required:    true,
		})
	}

	debugInfo := ""
	successfulTests := 0
	requiredTestsPassed := true
	var firstError error

	// Test each URL
	for _, testURL := range urlsToTest {
		if debug {
			debugInfo += fmt.Sprintf("\nTesting URL: %s (%s)\n", testURL.URL, testURL.Description)
		}

		req, err := http.NewRequest("GET", testURL.URL, nil)
		if err != nil {
			testResults[testURL.URL] = false
			if firstError == nil {
				firstError = fmt.Errorf("failed to create request for %s: %v", testURL.URL, err)
			}
			if testURL.Required {
				requiredTestsPassed = false
			}
			continue
		}

		// Add headers
		if config.UserAgent != "" {
			req.Header.Set("User-Agent", config.UserAgent)
		}
		for key, value := range config.DefaultHeaders {
			req.Header.Set(key, value)
		}

		if debug {
			reqDump, err := httputil.DumpRequestOut(req, true)
			if err == nil {
				debugInfo += fmt.Sprintf("Request:\n%s\n", string(reqDump))
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			testResults[testURL.URL] = false
			if firstError == nil {
				firstError = err
			}
			if debug {
				debugInfo += fmt.Sprintf("Error testing %s: %v\n", testURL.URL, err)
			}
			if testURL.Required {
				requiredTestsPassed = false
			}
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			testResults[testURL.URL] = false
			if firstError == nil {
				firstError = err
			}
			if testURL.Required {
				requiredTestsPassed = false
			}
			continue
		}

		// Validate response
		valid, validationDebug := validateProxyResponse(resp, body, debug)
		debugInfo += validationDebug
		testResults[testURL.URL] = valid
		if valid {
			successfulTests++
		} else if testURL.Required {
			requiredTestsPassed = false
		}
	}

	// Consider proxy working if:
	// 1. All required tests passed
	// 2. Total successful tests meets or exceeds the required count
	working := requiredTestsPassed && successfulTests >= config.TestURLs.RequiredSuccessCount

	// Continue with the rest of the checks (IPInfo, Cloud, etc.) only if basic checks passed
	if working {
		// Check proxy host IP for cloud provider detection using WHOIS
		var cloudProvider *CloudProvider
		if useCloud {
			proxyHost := proxyURL.Hostname()
			ips, err := net.LookupIP(proxyHost)
			if err == nil && len(ips) > 0 {
				proxyHostIP := ips[0].String()
				if debug {
					fmt.Printf("\nProxy host IP: %s\n", proxyHostIP)
				}

				// Do WHOIS lookup
				whoisData, err := getWhoisInfo(proxyHostIP)
				if err != nil && debug {
					fmt.Printf("Warning: Failed to get WHOIS data: %v\n", err)
				} else {
					cloudProvider = detectCloudProviderFromWhois(whoisData)
					if cloudProvider != nil && debug {
						fmt.Printf("Detected cloud provider from WHOIS: %s\n", cloudProvider.Name)
					}
				}
			}
		}

		// First get our real IP
		realIP := ""
		if useIPInfo {
			directClient := &http.Client{Timeout: timeout}
			ipInfo, err := getIPInfo(directClient)
			if err != nil && debug {
				fmt.Printf("Warning: Failed to get real IP: %v\n", err)
			} else {
				realIP = ipInfo.IP
			}
		}

		// Create client with appropriate proxy support
		client, err := createProxyClient(proxyURL, timeout)
		if err != nil {
			results <- ProxyResult{
				Proxy:         proxy,
				Working:       false,
				Error:         fmt.Errorf("failed to create proxy client: %v", err),
				CloudProvider: cloudProvider,
				TestResults:   testResults,
			}
			return
		}

		// First check with regular HTTP request
		testURLStr := config.TestURLs.DefaultURL
		if customURL != "" {
			testURLStr = customURL
		}
		req, err := http.NewRequest("GET", testURLStr, nil)
		if err != nil {
			results <- ProxyResult{
				Proxy:       proxy,
				Working:     false,
				Error:       err,
				DebugInfo:   debugInfo,
				RealIP:      realIP,
				TestResults: testResults,
			}
			return
		}

		// Add User-Agent if configured
		if config.UserAgent != "" {
			req.Header.Set("User-Agent", config.UserAgent)
		} else {
			// Set a default modern User-Agent if none specified
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		}

		// Add default headers from config
		for key, value := range config.DefaultHeaders {
			req.Header.Set(key, value)
		}

		debugInfo := ""
		if debug {
			reqDump, err := httputil.DumpRequestOut(req, true)
			if err == nil {
				debugInfo += fmt.Sprintf("Request:\n%s\n", string(reqDump))
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			results <- ProxyResult{
				Proxy:       proxy,
				Working:     false,
				Error:       err,
				DebugInfo:   debugInfo,
				RealIP:      realIP,
				TestResults: testResults,
			}
			return
		}
		defer resp.Body.Close()

		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			results <- ProxyResult{
				Proxy:       proxy,
				Working:     false,
				Error:       fmt.Errorf("failed to read response: %v", err),
				DebugInfo:   debugInfo,
				RealIP:      realIP,
				TestResults: testResults,
			}
			return
		}

		if debug {
			respDump, err := httputil.DumpResponse(resp, false) // Don't include body in dump
			if err == nil {
				debugInfo += fmt.Sprintf("\nResponse Headers:\n%s\n", string(respDump))
			}
			debugInfo += fmt.Sprintf("\nResponse Body Length: %d bytes\n", len(body))
		}

		elapsed := time.Since(start)
		basicCheck, validationDebug := validateProxyResponse(resp, body, debug)
		debugInfo += validationDebug

		// Check proxy IP and cloud provider if enabled
		proxyIP := ""
		isAnonymous := false
		var internalAccess, metadataAccess bool
		var advancedChecks *AdvancedCheckResult
		if basicCheck {
			if useIPInfo || useCloud {
				ipInfo, err := getIPInfo(client)
				if err != nil {
					if debug {
						debugInfo += fmt.Sprintf("\nFailed to get proxy IP info: %v\n", err)
					}
				} else {
					proxyIP = ipInfo.IP
					isAnonymous = proxyIP != realIP && proxyIP != ""

					if useCloud && cloudProvider != nil {
						if debug {
							debugInfo += fmt.Sprintf("\nUsing detected cloud provider: %s\n", cloudProvider.Name)
						}
						internalAccess, metadataAccess, cloudDebugInfo := checkInternalAccess(client, cloudProvider, debug)
						debugInfo += cloudDebugInfo
						if debug {
							debugInfo += fmt.Sprintf("Internal network access: %v\n", internalAccess)
							debugInfo += fmt.Sprintf("Metadata access: %v\n", metadataAccess)
						}
					}
				}
			}

			// Perform advanced security checks if the proxy is working
			advChecks, advDebugInfo := performAdvancedChecks(client, debug)
			debugInfo += advDebugInfo
			advancedChecks = advChecks
		}

		// If Interactsh validation is not requested or basic check failed, return early
		if !useInteractsh || !basicCheck {
			results <- ProxyResult{
				Proxy:          proxy,
				Working:        basicCheck,
				Speed:          elapsed,
				Error:          nil,
				DebugInfo:      debugInfo,
				RealIP:         realIP,
				ProxyIP:        proxyIP,
				IsAnonymous:    isAnonymous,
				CloudProvider:  cloudProvider,
				InternalAccess: internalAccess,
				MetadataAccess: metadataAccess,
				AdvancedChecks: advancedChecks,
				TestResults:    testResults,
			}
			return
		}

		// Initialize Interactsh client
		interactshClient, err := interactsh.New(&interactsh.Options{
			ServerURL: "oast.pro",
		})
		if err != nil {
			results <- ProxyResult{
				Proxy:          proxy,
				Working:        basicCheck,
				Speed:          elapsed,
				Error:          fmt.Errorf("failed to initialize Interactsh client: %v", err),
				DebugInfo:      debugInfo + fmt.Sprintf("\nInteractsh initialization error: %v", err),
				RealIP:         realIP,
				ProxyIP:        proxyIP,
				IsAnonymous:    isAnonymous,
				CloudProvider:  cloudProvider,
				InternalAccess: internalAccess,
				MetadataAccess: metadataAccess,
				AdvancedChecks: advancedChecks,
				TestResults:    testResults,
			}
			return
		}
		defer interactshClient.Close()

		// Generate unique URL for testing
		interactURL := interactshClient.URL()
		if debug {
			debugInfo += fmt.Sprintf("\nGenerated Interactsh URL: %s\n", interactURL)
		}

		// Make request through proxy to Interactsh URL
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		req, err = http.NewRequestWithContext(ctx, "GET", "http://"+interactURL, nil)
		if err != nil {
			results <- ProxyResult{
				Proxy:          proxy,
				Working:        basicCheck,
				Speed:          elapsed,
				Error:          fmt.Errorf("failed to create Interactsh request: %v", err),
				DebugInfo:      debugInfo + fmt.Sprintf("\nRequest creation error: %v", err),
				RealIP:         realIP,
				ProxyIP:        proxyIP,
				IsAnonymous:    isAnonymous,
				CloudProvider:  cloudProvider,
				InternalAccess: internalAccess,
				MetadataAccess: metadataAccess,
				AdvancedChecks: advancedChecks,
				TestResults:    testResults,
			}
			return
		}

		// Add headers for Interactsh request
		if config.UserAgent != "" {
			req.Header.Set("User-Agent", config.UserAgent)
		}
		for key, value := range config.DefaultHeaders {
			req.Header.Set(key, value)
		}

		if debug {
			reqDump, err := httputil.DumpRequestOut(req, true)
			if err == nil {
				debugInfo += fmt.Sprintf("\nInteractsh Request:\n%s\n", string(reqDump))
			}
		}

		proxyResp, err := client.Do(req)
		var netErr net.Error
		var urlErr *url.Error
		switch {
		case err == nil:
			// Request succeeded, continue processing
			if debug {
				debugInfo += "\nInteractsh request sent successfully\n"
			}
		case errors.As(err, &netErr) && netErr.Timeout():
			results <- ProxyResult{
				Proxy:          proxy,
				Working:        basicCheck,
				Speed:          elapsed,
				Error:          fmt.Errorf("Interactsh request timed out: %v", err),
				DebugInfo:      debugInfo + fmt.Sprintf("\nTimeout error: %v\nNetwork error details: %+v", err, netErr),
				RealIP:         realIP,
				ProxyIP:        proxyIP,
				IsAnonymous:    isAnonymous,
				CloudProvider:  cloudProvider,
				InternalAccess: internalAccess,
				MetadataAccess: metadataAccess,
				AdvancedChecks: advancedChecks,
				TestResults:    testResults,
			}
			return
		case errors.As(err, &urlErr):
			results <- ProxyResult{
				Proxy:          proxy,
				Working:        basicCheck,
				Speed:          elapsed,
				Error:          fmt.Errorf("Interactsh URL error: %v (op: %s)", urlErr.Err, urlErr.Op),
				DebugInfo:      debugInfo + fmt.Sprintf("\nURL error: %+v\nOperation: %s\nURL: %s", urlErr.Err, urlErr.Op, urlErr.URL),
				RealIP:         realIP,
				ProxyIP:        proxyIP,
				IsAnonymous:    isAnonymous,
				CloudProvider:  cloudProvider,
				InternalAccess: internalAccess,
				MetadataAccess: metadataAccess,
				AdvancedChecks: advancedChecks,
				TestResults:    testResults,
			}
			return
		default:
			results <- ProxyResult{
				Proxy:          proxy,
				Working:        basicCheck,
				Speed:          elapsed,
				Error:          fmt.Errorf("Interactsh request failed: %v", err),
				DebugInfo:      debugInfo + fmt.Sprintf("\nRequest error: %v\nError type: %T", err, err),
				RealIP:         realIP,
				ProxyIP:        proxyIP,
				IsAnonymous:    isAnonymous,
				CloudProvider:  cloudProvider,
				InternalAccess: internalAccess,
				MetadataAccess: metadataAccess,
				AdvancedChecks: advancedChecks,
			}
			return
		}
		defer proxyResp.Body.Close()

		if debug {
			respDump, err := httputil.DumpResponse(proxyResp, true)
			if err == nil {
				debugInfo += fmt.Sprintf("\nInteractsh Response:\n%s\n", string(respDump))
			}
			debugInfo += fmt.Sprintf("\nResponse Status: %s\n", proxyResp.Status)
		}

		// Wait for interaction with a timeout
		interaction := false
		interactionChan := make(chan bool, 1)
		pollCtx, pollCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pollCancel()

		var pollingError error
		// Start polling in a goroutine
		go func() {
			defer func() {
				if r := recover(); r != nil {
					pollingError = fmt.Errorf("polling panic recovered: %v", r)
					if debug {
						debugInfo += fmt.Sprintf("\nPolling panic: %v\n", r)
					}
				}
			}()

			interactshClient.StartPolling(time.Duration(500*time.Millisecond), func(i *server.Interaction) {
				interaction = true
				if debug {
					debugInfo += fmt.Sprintf("\nInteractsh Interaction:\nProtocol: %s\nID: %s\nFull Request:\n%s\n",
						i.Protocol, i.FullId, i.RawRequest)
				}
				interactionChan <- true
			})
		}()

		// Wait for either an interaction or timeout
		select {
		case <-interactionChan:
			if debug {
				debugInfo += "Received Interactsh interaction\n"
			}
		case <-pollCtx.Done():
			if debug {
				debugInfo += fmt.Sprintf("Interactsh polling timed out after 5 seconds\nContext error: %v\n", pollCtx.Err())
			}
		}

		// Check for polling errors
		if pollingError != nil {
			if debug {
				debugInfo += fmt.Sprintf("\nPolling error occurred: %v\n", pollingError)
			}
		}

		results <- ProxyResult{
			Proxy:          proxy,
			Working:        basicCheck,
			Speed:          elapsed,
			Error:          pollingError,
			InteractshTest: interaction,
			DebugInfo:      debugInfo,
			RealIP:         realIP,
			ProxyIP:        proxyIP,
			IsAnonymous:    isAnonymous,
			CloudProvider:  cloudProvider,
			InternalAccess: internalAccess,
			MetadataAccess: metadataAccess,
			AdvancedChecks: advancedChecks,
		}
	}
}

// Add new function for writing working proxies output
func writeWorkingProxiesOutput(filename string, results []ProxyResultOutput) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)

	// Write only working proxies
	for _, result := range results {
		if result.Working {
			fmt.Fprintf(w, "%s - Speed: %v\n", result.Proxy, result.Speed)
		}
	}

	return w.Flush()
}

// Add new function for writing working anonymous proxies output
func writeWorkingAnonymousProxiesOutput(filename string, results []ProxyResultOutput) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)

	// Write only working anonymous proxies
	for _, result := range results {
		if result.Working && result.IsAnonymous {
			fmt.Fprintf(w, "%s - Speed: %v\n", result.Proxy, result.Speed)
		}
	}

	return w.Flush()
}

// Add loadProxies function
func loadProxies(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var proxies []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		proxy := strings.TrimSpace(scanner.Text())
		if proxy != "" {
			// Check if the proxy already has a scheme
			hasScheme := strings.Contains(proxy, "://")
			if !hasScheme {
				// If no scheme, default to http://
				proxy = "http://" + proxy
			} else {
				// Validate supported schemes
				scheme := strings.ToLower(strings.Split(proxy, "://")[0])
				switch scheme {
				case "http", "https", "socks4", "socks5":
					// Valid scheme, keep as is
				default:
					// Invalid scheme
					return nil, fmt.Errorf("unsupported proxy scheme: %s", scheme)
				}
			}
			proxies = append(proxies, proxy)
		}
	}
	return proxies, scanner.Err()
}

// Add writeTextOutput function
func writeTextOutput(filename string, results []ProxyResultOutput, summary SummaryOutput) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)

	// Write results
	for _, result := range results {
		status := successStyle.Render("✓")
		if !result.Working {
			status = errorStyle.Render("✗")
		} else if result.IsAnonymous {
			status = anonymousStyle.Render("🔒")
		} else if result.CloudProvider != "" {
			status = cloudStyle.Render("☁")
		}

		line := fmt.Sprintf("%s %s - Speed: %v", status, result.Proxy, result.Speed)
		if result.Error != "" {
			line += fmt.Sprintf(" - Error: %s", result.Error)
		}
		if result.InteractshTest {
			line += " " + successStyle.Render("(Interactsh: ✓)")
		}
		if result.ProxyIP != "" {
			line += fmt.Sprintf(" [Real IP: %s -> Proxy IP: %s]", result.RealIP, result.ProxyIP)
		}
		if result.CloudProvider != "" {
			line += fmt.Sprintf(" [Cloud: %s]", result.CloudProvider)
		}
		fmt.Fprintln(w, line)
	}

	// Write summary
	fmt.Fprintln(w, "\nSummary:")
	fmt.Fprintf(w, "Total proxies checked: %d\n", summary.TotalProxies)
	fmt.Fprintf(w, "Working proxies (HTTP): %d\n", summary.WorkingProxies)
	fmt.Fprintf(w, "Working proxies (Interactsh): %d\n", summary.InteractshProxies)
	fmt.Fprintf(w, "Anonymous proxies: %d\n", summary.AnonymousProxies)
	fmt.Fprintf(w, "Cloud provider proxies: %d\n", summary.InternalAccessCount)
	fmt.Fprintf(w, "Success rate: %.2f%%\n", summary.SuccessRate)

	return w.Flush()
}

// Add AdvancedCheckResult type definition
type AdvancedCheckResult struct {
	ProtocolSmuggling   bool              `json:"protocol_smuggling"`
	DNSRebinding        bool              `json:"dns_rebinding"`
	NonStandardPorts    map[int]bool      `json:"nonstandard_ports"`
	IPv6Supported       bool              `json:"ipv6_supported"`
	MethodSupport       map[string]bool   `json:"method_support"`
	PathTraversal       bool              `json:"path_traversal"`
	CachePoisoning      bool              `json:"cache_poisoning"`
	HostHeaderInjection bool              `json:"host_header_injection"`
	VulnDetails         map[string]string `json:"vuln_details"`
}

// Add function declarations for advanced checks
var (
	checkProtocolSmuggling   func(client *http.Client, debug bool) (bool, string)
	checkDNSRebinding        func(client *http.Client, debug bool) (bool, string)
	checkCachePoisoning      func(client *http.Client, debug bool) (bool, string)
	checkHostHeaderInjection func(client *http.Client, debug bool) (bool, string)
)

// Add performAdvancedChecks function
func performAdvancedChecks(client *http.Client, debug bool) (*AdvancedCheckResult, string) {
	debugInfo := ""
	result := &AdvancedCheckResult{
		VulnDetails: make(map[string]string),
	}

	if config.Validation.AdvancedChecks.TestProtocolSmuggling {
		result.ProtocolSmuggling, debugInfo = checkProtocolSmuggling(client, debug)
	}

	if config.Validation.AdvancedChecks.TestDNSRebinding {
		result.DNSRebinding, debugInfo = checkDNSRebinding(client, debug)
	}

	if config.Validation.AdvancedChecks.TestCachePoisoning {
		result.CachePoisoning, debugInfo = checkCachePoisoning(client, debug)
	}

	if config.Validation.AdvancedChecks.TestHostHeaderInjection {
		result.HostHeaderInjection, debugInfo = checkHostHeaderInjection(client, debug)
	}

	return result, debugInfo
}

func main() {
	var (
		// Input options
		proxyList   = flag.String("l", "", "File containing proxy list (one per line)")
		singleProxy = flag.String("proxy", "", "Test a single proxy (e.g., http://proxy:port, socks5://proxy:port)")

		// Test options
		testURL     = flag.String("u", "https://www.google.com", "URL to test proxy against")
		timeout     = flag.Duration("t", 10*time.Second, "Timeout for each proxy check")
		concurrency = flag.Int("c", 10, "Number of concurrent proxy checks")

		// Validation options
		useInteractsh = flag.Bool("i", false, "Use Interactsh for additional validation")
		useIPInfo     = flag.Bool("p", false, "Use IPInfo to check proxy anonymity")
		useCloud      = flag.Bool("cloud", false, "Enable cloud provider detection and internal network testing")

		// Header options
		configFile = flag.String("config", "config.yaml", "Path to configuration file")
		userAgent  = flag.String("ua", "", "Custom User-Agent header (overrides config file)")
		headers    = flag.String("H", "", "Additional headers in format 'Key1:Value1,Key2:Value2'")

		// Output options
		debug      = flag.Bool("d", false, "Enable debug output")
		outputFile = flag.String("o", "", "Output file for results in text format")
		jsonFile   = flag.String("j", "", "Output file for results in JSON format")
		wpFile     = flag.String("wp", "", "Output file for working proxies only")
		wpaFile    = flag.String("wpa", "", "Output file for working anonymous proxies only")
	)

	flag.Parse()

	// Always load config file for headers
	if err := loadConfig(*configFile); err != nil {
		fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: Error loading configuration: %v. Using default headers.", err)))
		// Initialize default headers if config loading fails
		config.DefaultHeaders = map[string]string{
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
			"Accept-Language": "en-US,en;q=0.9",
			"Accept-Encoding": "gzip, deflate",
			"Connection":      "keep-alive",
			"Cache-Control":   "no-cache",
			"Pragma":          "no-cache",
		}
		config.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	}

	if *useCloud {
		fmt.Println(infoStyle.Render(fmt.Sprintf("Loaded %d cloud provider configurations", len(config.CloudProviders))))
	}

	// Override User-Agent from command line if specified
	if *userAgent != "" {
		config.UserAgent = *userAgent
	}

	// Parse and add additional headers from command line
	if *headers != "" {
		if config.DefaultHeaders == nil {
			config.DefaultHeaders = make(map[string]string)
		}
		headerPairs := strings.Split(*headers, ",")
		for _, pair := range headerPairs {
			parts := strings.SplitN(pair, ":", 2)
			if len(parts) == 2 {
				config.DefaultHeaders[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	if *testURL == "" {
		*testURL = "https://www.google.com" // Default URL if none specified
	}

	// Validate test URL
	parsedURL, urlErr := url.Parse(*testURL)
	if urlErr != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Invalid test URL: %v. Using https://www.google.com", urlErr)))
		*testURL = "https://www.google.com"
	}

	var proxies []string
	var err error

	// Handle single proxy test
	if *singleProxy != "" {
		// Validate the single proxy URL
		if _, err := url.Parse(*singleProxy); err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Invalid proxy URL: %v", err)))
			os.Exit(1)
		}
		proxies = []string{*singleProxy}
	} else if *proxyList != "" {
		// Load proxies from file
		proxies, err = loadProxies(*proxyList)
		if err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Error loading proxies: %v", err)))
			os.Exit(1)
		}
	} else {
		fmt.Println(errorStyle.Render("Please provide either a proxy list file (-l) or a single proxy (--proxy)"))
		flag.Usage()
		os.Exit(1)
	}

	if len(proxies) == 0 {
		fmt.Println(errorStyle.Render("No proxies to check"))
		os.Exit(1)
	}

	// Print header
	header := []string{
		fmt.Sprintf("Starting to check %d %s...", len(proxies),
			map[bool]string{true: "proxy", false: "proxies"}[len(proxies) == 1]),
	}
	if *useInteractsh {
		header = append(header, "Interactsh validation enabled")
	}
	if *useIPInfo {
		header = append(header, "IPInfo validation enabled")
	}
	if *useCloud {
		header = append(header, "Cloud provider detection enabled")
	}
	if *debug {
		header = append(header, "Debug output enabled")
	}

	fmt.Println(headerStyle.Render(strings.Join(header, "\n")))

	// Initialize and run the TUI
	model := checkProxiesWithProgress(proxies, *testURL, *useInteractsh, *useIPInfo, *useCloud, *debug, *timeout, *concurrency)
	program = tea.NewProgram(model)
	finalModel, err := program.Run()
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Error running program: %v", err)))
		os.Exit(1)
	}

	// Process final results
	m := finalModel.(Model)
	workingCount := 0
	for _, result := range m.results {
		if result.Working {
			workingCount++
		}
	}

	summary := SummaryOutput{
		TotalProxies:   m.total,
		Results:        m.results,
		WorkingProxies: workingCount,
		SuccessRate:    float64(workingCount) / float64(m.total) * 100,
	}

	// Count various types
	for _, result := range m.results {
		if result.InteractshTest {
			summary.InteractshProxies++
		}
		if result.IsAnonymous {
			summary.AnonymousProxies++
		}
		if result.CloudProvider != "" {
			summary.InternalAccessCount++
		}
	}

	// Write output files if specified
	if *outputFile != "" {
		if err := writeTextOutput(*outputFile, m.results, summary); err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Error writing text output: %v", err)))
		} else {
			fmt.Println(infoStyle.Render(fmt.Sprintf("Results written to %s", *outputFile)))
		}
	}

	if *jsonFile != "" {
		jsonData, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Error creating JSON output: %v", err)))
		} else if err := os.WriteFile(*jsonFile, jsonData, 0644); err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Error writing JSON file: %v", err)))
		} else {
			fmt.Println(infoStyle.Render(fmt.Sprintf("Results written to %s", *jsonFile)))
		}
	}

	if *wpFile != "" {
		if err := writeWorkingProxiesOutput(*wpFile, m.results); err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Error writing working proxies output: %v", err)))
		} else {
			fmt.Println(infoStyle.Render(fmt.Sprintf("Working proxies written to %s", *wpFile)))
		}
	}

	if *wpaFile != "" {
		if err := writeWorkingAnonymousProxiesOutput(*wpaFile, m.results); err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Error writing working anonymous proxies output: %v", err)))
		} else {
			fmt.Println(infoStyle.Render(fmt.Sprintf("Working anonymous proxies written to %s", *wpaFile)))
		}
	}

	// Print final summary with style
	fmt.Println("\n" + headerStyle.Render("Summary"))
	fmt.Printf("%s: %d\n", infoStyle.Render("Total proxies checked"), summary.TotalProxies)
	fmt.Printf("%s: %d\n", successStyle.Render("Working proxies (HTTP)"), summary.WorkingProxies)
	if *useInteractsh {
		fmt.Printf("%s: %d\n", successStyle.Render("Working proxies (Interactsh)"), summary.InteractshProxies)
	}
	if *useIPInfo {
		fmt.Printf("%s: %d\n", anonymousStyle.Render("Anonymous proxies"), summary.AnonymousProxies)
	}
	if *useCloud {
		fmt.Printf("%s: %d\n", cloudStyle.Render("Cloud provider proxies"), summary.InternalAccessCount)
	}
	fmt.Printf("%s: %.2f%%\n", infoStyle.Render("Success rate"), summary.SuccessRate)
}
