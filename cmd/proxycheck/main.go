package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ResistanceIsUseless/ProxyCheck/cloudcheck"
	"github.com/ResistanceIsUseless/ProxyCheck/internal/proxy"
	"github.com/ResistanceIsUseless/ProxyCheck/internal/ui"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	// General settings
	Timeout              int               `yaml:"timeout"`
	InsecureSkipVerify   bool              `yaml:"insecure_skip_verify"`
	UserAgent            string            `yaml:"user_agent"`
	DefaultHeaders       map[string]string `yaml:"default_headers"`
	EnableCloudChecks    bool              `yaml:"enable_cloud_checks"`
	EnableAnonymityCheck bool              `yaml:"enable_anonymity_check"`
	Concurrency          int               `yaml:"concurrency"`

	// Test URLs configuration
	TestURLs TestURLConfig `yaml:"test_urls"`

	// Validation settings
	Validation ValidationConfig `yaml:"validation"`

	// Cloud provider settings
	CloudProviders []cloudcheck.CloudProvider `yaml:"cloud_providers"`

	// Advanced security checks
	AdvancedChecks struct {
		TestProtocolSmuggling   bool     `yaml:"test_protocol_smuggling"`
		TestDNSRebinding        bool     `yaml:"test_dns_rebinding"`
		TestIPv6                bool     `yaml:"test_ipv6"`
		TestHTTPMethods         []string `yaml:"test_http_methods"`
		TestPathTraversal       bool     `yaml:"test_path_traversal"`
		TestCachePoisoning      bool     `yaml:"test_cache_poisoning"`
		TestHostHeaderInjection bool     `yaml:"test_host_header_injection"`
	} `yaml:"advanced_checks"`

	// Response validation settings
	RequireStatusCode   int      `yaml:"require_status_code"`
	RequireContentMatch string   `yaml:"require_content_match"`
	RequireHeaderFields []string `yaml:"require_header_fields"`
}

type TestURLConfig struct {
	DefaultURL           string    `yaml:"default_url"`
	RequiredSuccessCount int       `yaml:"required_success_count"`
	URLs                 []TestURL `yaml:"urls"`
}

type TestURL struct {
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

type ValidationConfig struct {
	MinResponseBytes   int      `yaml:"min_response_bytes"`
	DisallowedKeywords []string `yaml:"disallowed_keywords"`
}

// AppState represents the application state
type AppState struct {
	view        *ui.View
	checker     *proxy.Checker
	proxies     []string
	results     []*proxy.ProxyResult
	concurrency int
	verbose     bool
	debug       bool
}

func main() {
	// Parse command line flags
	proxyList := flag.String("l", "", "File containing list of proxies")
	configFile := flag.String("config", "config.yaml", "Path to config file")
	verbose := flag.Bool("v", false, "Enable verbose output")
	debug := flag.Bool("d", false, "Enable debug mode")
	concurrency := flag.Int("c", 0, "Number of concurrent checks (overrides config)")
	flag.Parse()

	// Load configuration
	config, err := loadConfig(*configFile)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Override config with command line flags if specified
	if *concurrency > 0 {
		config.Concurrency = *concurrency
	}

	// Load proxies
	proxies, warnings, err := loadProxies(*proxyList)
	if err != nil {
		fmt.Printf("Error loading proxies: %v\n", err)
		os.Exit(1)
	}

	// Print any warnings
	for _, warning := range warnings {
		fmt.Printf("Warning: %s\n", warning)
	}

	// Create proxy checker
	checker := proxy.NewChecker(proxy.Config{
		Timeout:             time.Duration(config.Timeout) * time.Second,
		ValidationURL:       config.TestURLs.DefaultURL,
		DisallowedKeywords:  config.Validation.DisallowedKeywords,
		MinResponseBytes:    config.Validation.MinResponseBytes,
		DefaultHeaders:      config.DefaultHeaders,
		UserAgent:           config.UserAgent,
		EnableCloudChecks:   config.EnableCloudChecks,
		CloudProviders:      config.CloudProviders,
		RequireStatusCode:   config.RequireStatusCode,
		RequireContentMatch: config.RequireContentMatch,
		RequireHeaderFields: config.RequireHeaderFields,
		AdvancedChecks:      config.AdvancedChecks,
	}, config.AdvancedChecks.TestProtocolSmuggling || config.AdvancedChecks.TestDNSRebinding)

	// Initialize UI
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	view := &ui.View{
		Progress:     p,
		Total:        len(proxies),
		IsVerbose:    *verbose || config.Validation.MinResponseBytes > 0,                                              // Use verbose mode if flag or detailed validation is enabled
		IsDebug:      *debug || config.AdvancedChecks.TestProtocolSmuggling || config.AdvancedChecks.TestDNSRebinding, // Use debug mode if flag or advanced checks are enabled
		ActiveChecks: make(map[string]*ui.CheckStatus),
	}

	// Create application state
	state := &AppState{
		view:        view,
		checker:     checker,
		proxies:     proxies,
		concurrency: config.Concurrency,
		verbose:     *verbose || config.Validation.MinResponseBytes > 0,
		debug:       *debug || config.AdvancedChecks.TestProtocolSmuggling || config.AdvancedChecks.TestDNSRebinding,
	}

	// Start the UI
	program := tea.NewProgram(state)
	if err := program.Start(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	// Process results
	processResults(state)
}

func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	// Set default concurrency if not specified
	if config.Concurrency <= 0 {
		config.Concurrency = 10
	}

	return &config, nil
}

func getDefaultConfig() *Config {
	return &Config{
		Timeout:              10,
		InsecureSkipVerify:   false,
		EnableCloudChecks:    false,
		EnableAnonymityCheck: false,
		DefaultHeaders: map[string]string{
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
			"Accept-Language": "en-US,en;q=0.9",
			"Accept-Encoding": "gzip, deflate",
			"Connection":      "keep-alive",
			"Cache-Control":   "no-cache",
			"Pragma":          "no-cache",
		},
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Validation: ValidationConfig{
			DisallowedKeywords: []string{
				"Access Denied",
				"Proxy Error",
				"Bad Gateway",
				"Gateway Timeout",
				"Service Unavailable",
			},
			MinResponseBytes: 100,
		},
	}
}

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

		// Remove trailing slashes
		proxy = strings.TrimRight(proxy, "/")

		// Add scheme if missing
		if !strings.Contains(proxy, "://") {
			proxy = "http://" + proxy
		}

		// Validate URL
		if _, err := url.Parse(proxy); err != nil {
			warnings = append(warnings, fmt.Sprintf("Invalid proxy URL '%s': %v", proxy, err))
			continue
		}

		proxies = append(proxies, proxy)
	}

	if len(proxies) == 0 {
		return nil, warnings, fmt.Errorf("no valid proxies found in file")
	}

	return proxies, warnings, scanner.Err()
}

func processResults(state *AppState) {
	// Process and display final results
	var working, anonymous int
	for _, result := range state.results {
		if result.Working {
			working++
			if result.IsAnonymous {
				anonymous++
			}
		}
	}

	fmt.Printf("\nResults Summary:\n")
	fmt.Printf("Total proxies: %d\n", len(state.proxies))
	fmt.Printf("Working proxies: %d\n", working)
	fmt.Printf("Anonymous proxies: %d\n", anonymous)
}

// Tea model implementation
func (s *AppState) Init() tea.Cmd {
	// Start proxy checking
	go s.startChecking()
	return nil
}

func (s *AppState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return s, tea.Quit
		}
	}
	return s, nil
}

func (s *AppState) View() string {
	if s.view.IsDebug {
		return s.view.RenderDebug()
	}
	if s.view.IsVerbose {
		return s.view.RenderVerbose()
	}
	return s.view.RenderDefault()
}

func (s *AppState) startChecking() {
	var wg sync.WaitGroup
	proxyChan := make(chan string)

	// Start workers
	for i := 0; i < s.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for proxy := range proxyChan {
				result, err := s.checker.Check(proxy)
				if err != nil {
					continue
				}
				s.processResult(result)
			}
		}()
	}

	// Feed proxies to workers
	for _, proxy := range s.proxies {
		proxyChan <- proxy
	}
	close(proxyChan)

	wg.Wait()
}

func (s *AppState) processResult(result *proxy.ProxyResult) {
	s.results = append(s.results, result)
	s.view.Current++

	// Update UI state
	status := &ui.CheckStatus{
		Proxy:          result.ProxyURL,
		TotalChecks:    len(result.CheckResults),
		DoneChecks:     len(result.CheckResults),
		LastUpdate:     time.Now(),
		Speed:          result.Speed,
		ProxyType:      string(result.Type),
		IsActive:       false,
		CloudProvider:  result.CloudProvider,
		InternalAccess: result.InternalAccess,
		MetadataAccess: result.MetadataAccess,
	}
	s.view.ActiveChecks[result.ProxyURL] = status
}
