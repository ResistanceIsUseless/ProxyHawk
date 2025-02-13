package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
			Padding(0, 1).
			Align(lipgloss.Center).
			Width(50)

	progressStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("39")).
			Padding(0, 1).
			Width(50)

	statusBlockStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("99")).
				Padding(0, 1).
				Width(50)

	metricBlockStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("207")).
				Padding(0, 1).
				Width(50)

	debugBlockStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("208")).
			Padding(0, 1).
			Width(50)

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
	debugInfo   string
	warnings    []string
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
		if msg.current > m.current {
			m.current = msg.current
			m.activeJobs = msg.activeJobs
			m.queueSize = msg.queueSize
			m.successRate = msg.successRate
			m.avgSpeed = msg.avgSpeed
			m.debugInfo = msg.debugInfo
			// Always append the result
			if len(m.results) == 0 || m.results[len(m.results)-1].Proxy != msg.result.Proxy {
				m.results = append(m.results, msg.result)
			}
			return m, m.progress.SetPercent(float64(m.current) / float64(m.total))
		}
		return m, nil

	case quitMsg:
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
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

	// Warnings (if any)
	if len(m.warnings) > 0 {
		str.WriteString(warningStyle.Render("Proxy loading warnings:") + "\n")
		for _, warning := range m.warnings {
			str.WriteString(warningStyle.Render(warning) + "\n")
		}
		str.WriteString("\n")
	}

	// Debug info (if available)
	if m.debugInfo != "" {
		str.WriteString(debugBlockStyle.Render(fmt.Sprintf("Debug Info:\n%s", m.debugInfo)) + "\n\n")
	}

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
	str.WriteString(statusBlockStyle.Render(statusBlock.String()) + "\n")

	// Performance metrics
	if m.current > 0 {
		perfBlock := strings.Builder{}
		successRateColor := "87" // Default color (good)
		if m.successRate < 50 {
			successRateColor = "203" // Red for low success rate
		} else if m.successRate < 80 {
			successRateColor = "214" // Yellow for medium success rate
		}

		perfBlock.WriteString(fmt.Sprintf("%s %s",
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

			perfBlock.WriteString(fmt.Sprintf("\n%s %s",
				metricLabelStyle.Render("Average Speed:"),
				lipgloss.NewStyle().
					Foreground(lipgloss.Color(speedColor)).
					Render(m.avgSpeed.Round(time.Millisecond).String())))
		}
		str.WriteString(metricBlockStyle.Render(perfBlock.String()) + "\n")
	}

	// Recent Results
	if len(m.results) > 0 {
		str.WriteString("\nLatest Results:\n")
		start := len(m.results) - 5
		if start < 0 {
			start = 0
		}
		for _, result := range m.results[start:] {
			status := successStyle.Render("✓")
			if !result.Working {
				status = errorStyle.Render("✗")
			}

			// Create a summary of all checks
			checkSummary := ""
			if len(result.CheckResults) > 0 {
				successChecks := 0
				totalTime := time.Duration(0)
				for _, check := range result.CheckResults {
					if check.Success {
						successChecks++
					}
					totalTime += check.Speed
				}
				avgSpeed := totalTime / time.Duration(len(result.CheckResults))
				checkSummary = fmt.Sprintf("[%d/%d checks passed, avg speed: %v]",
					successChecks,
					len(result.CheckResults),
					avgSpeed.Round(time.Millisecond))
			}

			str.WriteString(fmt.Sprintf("%s %s %s\n", status, result.Proxy, checkSummary))

			// Show individual check results if debug is enabled
			if m.debugInfo != "" {
				for _, check := range result.CheckResults {
					checkStatus := successStyle.Render("✓")
					if !check.Success {
						checkStatus = errorStyle.Render("✗")
					}
					details := fmt.Sprintf("Status: %d, Speed: %v", check.StatusCode, check.Speed.Round(time.Millisecond))
					if check.Error != "" {
						details = fmt.Sprintf("Error: %s", check.Error)
					}
					str.WriteString(fmt.Sprintf("  %s %s - %s\n", checkStatus, check.URL, details))
				}
			}
		}
	}

	// Controls
	str.WriteString("\n" + infoStyle.Render("Press q to quit"))

	return str.String()
}

// Helper function to get the appropriate style for a result's status
func getStatusStyle(result ProxyResultOutput) lipgloss.Style {
	if !result.Working {
		return errorStyle
	} else if result.IsAnonymous {
		return anonymousStyle
	} else if result.CloudProvider != "" {
		return cloudStyle
	}
	return successStyle
}

type progressMsg struct {
	current     int
	result      ProxyResultOutput
	activeJobs  int
	queueSize   int
	successRate float64
	avgSpeed    time.Duration
	debugInfo   string
}

type quitMsg struct{}

func checkProxiesWithProgress(proxies []string, singleURL string, useInteractsh, useIPInfo, useCloud, debug bool, timeout time.Duration, concurrency int) tea.Model {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	model := Model{
		progress: p,
		total:    len(proxies),
		results:  make([]ProxyResultOutput, 0, len(proxies)),
	}

	// Store any warnings from proxy loading
	for _, proxy := range proxies {
		if strings.HasPrefix(proxy, "socks") {
			model.warnings = append(model.warnings, fmt.Sprintf("Warning: skipping unsupported scheme 'socks' for proxy '%s'", proxy))
		}
	}

	go func() {
		// Create buffered channels
		results := make(chan ProxyResult, len(proxies)) // Buffer all possible results
		jobs := make(chan string, concurrency)          // Buffer only concurrent jobs
		var activeWorkers int32                         // Use atomic counter instead of channel

		// Track metrics
		var (
			mu           sync.Mutex
			successCount int64
			totalTime    time.Duration
			completed    int32
		)

		// Start workers
		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for proxy := range jobs {
					atomic.AddInt32(&activeWorkers, 1) // Increment active workers
					start := time.Now()

					// Check proxy
					checkProxyWithInteractsh(proxy, singleURL, useInteractsh, useIPInfo, useCloud, debug, timeout, results)

					// Get the result
					result := <-results

					// Update metrics safely
					mu.Lock()
					if result.Working {
						successCount++
						totalTime += time.Since(start)
					}

					// Prepare error message
					var errorMsg string
					if result.Error != nil {
						errorMsg = result.Error.Error()
					}

					// Prepare cloud provider name
					var cloudProviderName string
					if result.CloudProvider != nil {
						cloudProviderName = result.CloudProvider.Name
					}

					// Store result
					model.results = append(model.results, ProxyResultOutput{
						Proxy:          result.Proxy,
						Working:        result.Working,
						Speed:          result.Speed,
						Error:          errorMsg,
						InteractshTest: result.InteractshTest,
						RealIP:         result.RealIP,
						ProxyIP:        result.ProxyIP,
						IsAnonymous:    result.IsAnonymous,
						CloudProvider:  cloudProviderName,
						InternalAccess: result.InternalAccess,
						MetadataAccess: result.MetadataAccess,
						Timestamp:      time.Now(),
						CheckResults:   result.CheckResults,
					})
					mu.Unlock()

					atomic.AddInt32(&activeWorkers, -1) // Decrement active workers

					// Update progress atomically
					current := atomic.AddInt32(&completed, 1)

					// Send progress update
					program.Send(progressMsg{
						current:     int(current),
						result:      model.results[len(model.results)-1],
						activeJobs:  int(atomic.LoadInt32(&activeWorkers)),
						queueSize:   len(jobs),
						successRate: float64(successCount) / float64(current) * 100,
						avgSpeed:    totalTime / time.Duration(max(successCount, 1)),
						debugInfo:   result.DebugInfo,
					})

					// Add small delay to prevent UI flicker
					time.Sleep(100 * time.Millisecond)
				}
			}()
		}

		// Feed jobs
		for _, proxy := range proxies {
			jobs <- proxy
		}
		close(jobs)

		// Wait for all workers to finish
		wg.Wait()
		close(results)

		// Send quit message only after all workers are done
		program.Send(quitMsg{})
	}()

	return model
}

// Helper function to get max of two int64s
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
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
		// Set default configuration
		config = Config{
			DefaultHeaders: map[string]string{
				"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
				"Accept-Language": "en-US,en;q=0.9",
				"Accept-Encoding": "gzip, deflate",
				"Connection":      "keep-alive",
				"Cache-Control":   "no-cache",
				"Pragma":          "no-cache",
			},
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			TestURLs: TestURLConfig{
				DefaultURL:           "https://www.google.com",
				RequiredSuccessCount: 1,
				URLs: []TestURL{
					{
						URL:         "https://www.google.com",
						Description: "Default test using Google",
						Required:    true,
					},
				},
			},
			Validation: struct {
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
			}{
				RequireStatusCode:   0, // Don't require specific status code
				RequireContentMatch: "",
				RequireHeaderFields: []string{},
				DisallowedKeywords: []string{
					"Access Denied",
					"Proxy Error",
					"Bad Gateway",
					"Gateway Timeout",
					"Service Unavailable",
				},
				MinResponseBytes: 100,
				AdvancedChecks: struct {
					TestProtocolSmuggling   bool     `yaml:"test_protocol_smuggling"`
					TestDNSRebinding        bool     `yaml:"test_dns_rebinding"`
					TestNonStandardPorts    []int    `yaml:"test_nonstandard_ports"`
					TestIPv6                bool     `yaml:"test_ipv6"`
					TestHTTPMethods         []string `yaml:"test_http_methods"`
					TestPathTraversal       bool     `yaml:"test_path_traversal"`
					TestCachePoisoning      bool     `yaml:"test_cache_poisoning"`
					TestHostHeaderInjection bool     `yaml:"test_host_header_injection"`
				}{
					TestHTTPMethods: []string{"GET"},
				},
			},
		}
		return nil // Return nil since we've set default values
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
	CheckResults   []CheckResult
}

type CheckResult struct {
	URL        string
	Success    bool
	Speed      time.Duration
	Error      string
	StatusCode int
	BodySize   int64
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
	CheckResults   []CheckResult        `json:"check_results,omitempty"`
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
		TLSHandshakeTimeout:   timeout / 2,
		ResponseHeaderTimeout: timeout / 2,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     true,  // Disable keep-alives to prevent hanging connections
		ForceAttemptHTTP2:     false, // Disable HTTP/2 for better compatibility
	}

	// Create a dialer with timeout
	baseDialer := &net.Dialer{
		Timeout:   timeout / 2, // Use half the timeout for connection establishment
		KeepAlive: -1,          // Disable keep-alive
	}

	switch proxyURL.Scheme {
	case "http", "https":
		transport.Proxy = http.ProxyURL(proxyURL)
		transport.DialContext = baseDialer.DialContext
	case "socks4", "socks5":
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
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}, nil
}

// Add new function to validate proxy response
func validateProxyResponse(resp *http.Response, body []byte, debug bool) (bool, string) {
	debugInfo := ""

	if debug {
		debugInfo += fmt.Sprintf("\nValidating Response:\n")
		debugInfo += fmt.Sprintf("Status Code: %d\n", resp.StatusCode)
		debugInfo += fmt.Sprintf("Response Size: %d bytes\n", len(body))
		debugInfo += fmt.Sprintf("Headers: %v\n", resp.Header)
		debugInfo += fmt.Sprintf("Body Preview: %s\n", string(body[:min(len(body), 200)]))
	}

	// 1. Check status code - Accept 200-299 range
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if debug {
			debugInfo += fmt.Sprintf("Status code %d is not in 2xx range\n", resp.StatusCode)
		}
		return false, debugInfo
	}

	// 2. Check response size - Reduced minimum size requirement
	if len(body) < 100 { // Reduced from 512 to 100 bytes
		if debug {
			debugInfo += fmt.Sprintf("Response size %d bytes is less than required 100 bytes\n",
				len(body))
		}
		return false, debugInfo
	}

	// 3. Check for disallowed keywords
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

// Helper function to get minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Modify checkProxyWithInteractsh to use config URLs
func checkProxyWithInteractsh(proxy string, customURL string, useInteractsh bool, useIPInfo bool, useCloud bool, debug bool, timeout time.Duration, results chan<- ProxyResult) {
	start := time.Now()
	debugInfo := ""
	checkResults := make([]CheckResult, 0)

	if debug {
		debugInfo += fmt.Sprintf("\nStarting proxy check for: %s\n", proxy)
		debugInfo += fmt.Sprintf("Target URL: %s\n", customURL)
		debugInfo += fmt.Sprintf("Timeout: %v\n", timeout)
	}

	proxyURL, err := url.Parse(proxy)
	if err != nil {
		if debug {
			debugInfo += fmt.Sprintf("Error parsing proxy URL: %v\n", err)
		}
		results <- ProxyResult{
			Proxy:     proxy,
			Working:   false,
			Error:     fmt.Errorf("invalid proxy URL: %v", err),
			DebugInfo: debugInfo,
		}
		return
	}

	if debug {
		debugInfo += fmt.Sprintf("Parsed proxy URL - Scheme: %s, Host: %s\n", proxyURL.Scheme, proxyURL.Host)
		debugInfo += "Creating proxy client...\n"
	}

	client, err := createProxyClient(proxyURL, timeout)
	if err != nil {
		if debug {
			debugInfo += fmt.Sprintf("Error creating proxy client: %v\n", err)
		}
		results <- ProxyResult{
			Proxy:     proxy,
			Working:   false,
			Error:     fmt.Errorf("failed to create proxy client: %v", err),
			DebugInfo: debugInfo,
		}
		return
	}

	// Define test URLs
	testURLs := []string{
		customURL,
		"http://example.com",
		"http://httpbin.org/ip",
		"http://httpbin.org/get",
		"http://httpbin.org/headers",
	}

	successCount := 0
	totalChecks := len(testURLs)

	for _, testURL := range testURLs {
		checkStart := time.Now()
		if debug {
			debugInfo += fmt.Sprintf("\nTesting URL: %s\n", testURL)
		}

		req, err := http.NewRequest("GET", testURL, nil)
		if err != nil {
			checkResults = append(checkResults, CheckResult{
				URL:     testURL,
				Success: false,
				Error:   err.Error(),
			})
			continue
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
			if debug {
				debugInfo += fmt.Sprintf("Request failed: %v\n", err)
			}
			checkResults = append(checkResults, CheckResult{
				URL:     testURL,
				Success: false,
				Speed:   time.Since(checkStart),
				Error:   err.Error(),
			})
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			checkResults = append(checkResults, CheckResult{
				URL:        testURL,
				Success:    false,
				Speed:      time.Since(checkStart),
				Error:      err.Error(),
				StatusCode: resp.StatusCode,
			})
			continue
		}

		valid, validationDebug := validateProxyResponse(resp, body, debug)
		debugInfo += validationDebug

		checkResults = append(checkResults, CheckResult{
			URL:        testURL,
			Success:    valid,
			Speed:      time.Since(checkStart),
			StatusCode: resp.StatusCode,
			BodySize:   int64(len(body)),
		})

		if valid {
			successCount++
		}
	}

	// Consider the proxy working if it succeeds with at least 50% of the URLs
	working := float64(successCount)/float64(totalChecks) >= 0.5

	results <- ProxyResult{
		Proxy:        proxy,
		Working:      working,
		Speed:        time.Since(start),
		DebugInfo:    debugInfo,
		CheckResults: checkResults,
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
func loadProxies(filename string) ([]string, []string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	var proxies []string
	var warnings []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		proxy := strings.TrimSpace(scanner.Text())
		if proxy == "" {
			continue
		}

		// Remove any trailing slashes
		proxy = strings.TrimRight(proxy, "/")

		// Check if the proxy already has a scheme
		hasScheme := strings.Contains(proxy, "://")
		if !hasScheme {
			// If no scheme, default to http://
			proxy = "http://" + proxy
		}

		// Parse the URL to validate and clean it
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Warning: invalid proxy URL '%s': %v", proxy, err))
			continue
		}

		// Clean up the host:port
		host := proxyURL.Hostname() // Get host without port
		port := proxyURL.Port()     // Get port if specified

		// Handle different schemes and ports
		switch proxyURL.Scheme {
		case "http":
			if port == "" || port == "80" {
				proxies = append(proxies, fmt.Sprintf("http://%s", host))
			} else {
				proxies = append(proxies, fmt.Sprintf("http://%s:%s", host, port))
			}
		case "https":
			if port == "" || port == "443" {
				proxies = append(proxies, fmt.Sprintf("https://%s", host))
			} else {
				proxies = append(proxies, fmt.Sprintf("https://%s:%s", host, port))
			}
		case "socks4", "socks5":
			if port == "" {
				port = "1080" // Default SOCKS port
			}
			proxies = append(proxies, fmt.Sprintf("%s://%s:%s", proxyURL.Scheme, host, port))
		default:
			warnings = append(warnings, fmt.Sprintf("Warning: defaulting to HTTP for unknown scheme '%s' in proxy '%s'", proxyURL.Scheme, proxy))
			if port == "" {
				proxies = append(proxies, fmt.Sprintf("http://%s", host))
			} else {
				proxies = append(proxies, fmt.Sprintf("http://%s:%s", host, port))
			}
		}
	}

	if len(proxies) == 0 {
		return nil, warnings, fmt.Errorf("no valid proxies found in file")
	}

	return proxies, warnings, scanner.Err()
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

		// Write main proxy line with summary
		successCount := 0
		totalChecks := len(result.CheckResults)
		totalTime := time.Duration(0)
		for _, check := range result.CheckResults {
			if check.Success {
				successCount++
			}
			totalTime += check.Speed
		}
		avgSpeed := time.Duration(0)
		if totalChecks > 0 {
			avgSpeed = totalTime / time.Duration(totalChecks)
		}

		fmt.Fprintf(w, "%s %s [%d/%d checks passed, avg speed: %v]\n",
			status, result.Proxy, successCount, totalChecks, avgSpeed.Round(time.Millisecond))

		// Write individual check results
		for _, check := range result.CheckResults {
			checkStatus := successStyle.Render("✓")
			if !check.Success {
				checkStatus = errorStyle.Render("✗")
			}
			details := fmt.Sprintf("Status: %d, Speed: %v", check.StatusCode, check.Speed.Round(time.Millisecond))
			if check.Error != "" {
				details = fmt.Sprintf("Error: %s", check.Error)
			}
			fmt.Fprintf(w, "  %s %s - %s\n", checkStatus, check.URL, details)
		}
		fmt.Fprintln(w) // Add blank line between proxies
	}

	// Write summary
	fmt.Fprintln(w, "\nSummary:")
	fmt.Fprintf(w, "Total proxies checked: %d\n", summary.TotalProxies)
	fmt.Fprintf(w, "Working proxies (HTTP): %d\n", summary.WorkingProxies)
	if summary.InteractshProxies > 0 {
		fmt.Fprintf(w, "Working proxies (Interactsh): %d\n", summary.InteractshProxies)
	}
	if summary.AnonymousProxies > 0 {
		fmt.Fprintf(w, "Anonymous proxies: %d\n", summary.AnonymousProxies)
	}
	if summary.CloudProxies > 0 {
		fmt.Fprintf(w, "Cloud provider proxies: %d\n", summary.CloudProxies)
	}
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
	var warnings []string
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
		proxies, warnings, err = loadProxies(*proxyList)
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

	// Print warnings if any
	if len(warnings) > 0 {
		fmt.Println(warningStyle.Render("\nProxy loading warnings:"))
		for _, warning := range warnings {
			fmt.Println(warningStyle.Render(warning))
		}
		fmt.Println()
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
