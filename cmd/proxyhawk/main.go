package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/config"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/discovery"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/errors"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/help"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/loader"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/logging"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/metrics"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/output"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/pool"
	progresspkg "github.com/ResistanceIsUseless/ProxyHawk/internal/progress"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/proxy"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/ui"
)

// AppState represents the application state
type AppState struct {
	view        *ui.View
	checker     *proxy.Checker
	proxies     []string
	results     []*proxy.ProxyResult
	concurrency int
	verbose     bool
	debug       bool
	logger      *logging.Logger
	mutex       sync.Mutex   // Mutex to protect shared state
	updateChan  chan tea.Msg // Channel for sending updates to the UI

	// Graceful shutdown support
	ctx          context.Context
	cancel       context.CancelFunc
	shutdownChan chan os.Signal

	// Output options
	outputFile    string
	jsonFile      string
	workingFile   string
	anonymousFile string
	noUI          bool

	// Progress indicator for non-TUI mode
	progressIndicator progresspkg.ProgressIndicator

	// Metrics collection
	metricsCollector *metrics.Collector

	// Config watcher for hot-reloading
	configWatcher *config.ConfigWatcher
}

// Define custom message types
type tickMsg struct{}
type progressUpdateMsg struct{}

func main() {
	// Parse command line flags
	proxyList := flag.String("l", "", "File containing list of proxies")
	configFile := flag.String("config", "config/default.yaml", "Path to config file")
	verbose := flag.Bool("v", false, "Enable verbose output")
	debug := flag.Bool("d", false, "Enable debug mode")
	concurrency := flag.Int("c", 0, "Number of concurrent checks (overrides config)")
	useRDNS := flag.Bool("r", false, "Use rDNS lookup for host headers")
	timeout := flag.Int("t", 0, "Timeout in seconds (overrides config)")
	hotReload := flag.Bool("hot-reload", false, "Enable configuration hot-reloading")

	// Rate limiting flags
	rateLimitEnabled := flag.Bool("rate-limit", false, "Enable rate limiting")
	rateLimitDelay := flag.Duration("rate-delay", 1*time.Second, "Delay between requests (e.g. 500ms, 1s, 2s)")
	rateLimitPerHost := flag.Bool("rate-per-host", true, "Apply rate limiting per host instead of globally")
	rateLimitPerProxy := flag.Bool("rate-per-proxy", false, "Apply rate limiting per individual proxy (takes precedence over per-host)")

	// Output flags
	outputFile := flag.String("o", "", "Output results to text file")
	jsonFile := flag.String("j", "", "Output results to JSON file")
	workingFile := flag.String("wp", "", "Output working proxies to file")
	anonymousFile := flag.String("wpa", "", "Output working anonymous proxies to file")
	noUI := flag.Bool("no-ui", false, "Disable terminal UI (for automation/scripting)")

	// Progress indicator flags
	progressType := flag.String("progress", "bar", "Progress indicator type for non-TUI mode (none, basic, bar, spinner, dots, percent)")
	progressWidth := flag.Int("progress-width", 50, "Width of progress bar")
	progressNoColor := flag.Bool("progress-no-color", false, "Disable colored progress output")

	// Metrics flags
	enableMetrics := flag.Bool("metrics", false, "Enable Prometheus metrics endpoint")
	metricsAddr := flag.String("metrics-addr", ":9090", "Address to serve metrics on")
	metricsPath := flag.String("metrics-path", "/metrics", "Path for metrics endpoint")

	// Protocol flags
	enableHTTP2 := flag.Bool("http2", false, "Enable HTTP/2 protocol detection and support")
	enableHTTP3 := flag.Bool("http3", false, "Enable HTTP/3 protocol detection and support")

	// Discovery flags
	discoverMode := flag.Bool("discover", false, "Enable discovery mode to find proxy candidates")
	discoverSource := flag.String("discover-source", "all", "Discovery source to use (shodan, censys, freelists, webscraper, all)")
	discoverQuery := flag.String("discover-query", "", "Custom discovery query (uses preset if empty)")
	discoverLimit := flag.Int("discover-limit", 100, "Maximum number of candidates to discover")
	discoverValidate := flag.Bool("discover-validate", false, "Validate discovered candidates immediately")
	discoverCountries := flag.String("discover-countries", "", "Comma-separated list of country codes to target")
	discoverMinConfidence := flag.Float64("discover-min-confidence", 0.0, "Minimum confidence score for candidates")

	// Help and version flags
	showHelp := flag.Bool("help", false, "Show help message")
	showHelpShort := flag.Bool("h", false, "Show help message (short)")
	showVersion := flag.Bool("version", false, "Show version information")
	showQuickStart := flag.Bool("quickstart", false, "Show quick start guide")

	// Custom usage function
	flag.Usage = func() {
		noColor := help.DetectNoColor()
		help.PrintHelp(os.Stderr, noColor)
	}

	flag.Parse()

	// Handle help and version flags before anything else
	noColor := help.DetectNoColor()

	if *showHelp || *showHelpShort {
		help.PrintHelp(os.Stdout, noColor)
		os.Exit(0)
	}

	if *showVersion {
		help.PrintVersion(os.Stdout, noColor)
		os.Exit(0)
	}

	if *showQuickStart {
		help.PrintQuickStart(os.Stdout, noColor)
		os.Exit(0)
	}

	// Validate required flags - proxy list is required unless in discovery mode
	if *proxyList == "" && !*discoverMode {
		help.PrintUsageError(os.Stderr, fmt.Errorf("proxy list file is required (or use -discover mode)"), noColor)
		os.Exit(1)
	}

	// Initialize logger based on debug/verbose flags
	logLevel := logging.LevelInfo
	if *debug {
		logLevel = logging.LevelDebug
	}
	logger := logging.NewLogger(logging.Config{
		Level:  logLevel,
		Format: "text",
	})

	// Load and validate configuration
	cfg, validationResult, err := config.ValidateAndLoad(*configFile)
	if err != nil {
		// Enhanced error logging with error categorization
		category := errors.GetErrorCategory(err)
		logger.Error("Failed to load configuration",
			"error", err,
			"file", *configFile,
			"category", category,
			"critical", errors.IsCritical(err))
		os.Exit(1)
	}

	// Log validation warnings if any
	if len(validationResult.Warnings) > 0 {
		for _, warning := range validationResult.Warnings {
			logger.Warn("Configuration validation warning", "warning", warning)
		}
	}

	// Check for validation errors
	if !validationResult.Valid {
		logger.Error("Configuration validation failed", "errors", len(validationResult.Errors))
		for _, validationErr := range validationResult.Errors {
			logger.Error("Configuration error", "error", validationErr.Error())
		}
		os.Exit(1)
	}

	logger.ConfigLoaded(*configFile)

	// Set up config hot-reloading if enabled
	var configWatcher *config.ConfigWatcher
	if *hotReload {
		watcherConfig := config.WatcherConfig{
			DebounceDelay:        1 * time.Second,
			ValidateBeforeReload: true,
			OnReload: func(newConfig *config.Config, result *config.ValidationResult) {
				logger.Info("Configuration reloaded successfully", "file", *configFile)

				// Log any warnings
				for _, warning := range result.Warnings {
					logger.Warn("Configuration warning after reload", "warning", warning)
				}

				// Note: We don't update the running configuration here because
				// that would require stopping and restarting workers, which is complex.
				// For now, hot-reload will take effect on the next run.
				logger.Info("Configuration changes will take effect on next proxy check run")
			},
			OnError: func(err error) {
				logger.Error("Configuration reload failed", "error", err)
			},
		}

		var err error
		configWatcher, err = config.NewConfigWatcher(*configFile, watcherConfig)
		if err != nil {
			logger.Warn("Failed to enable configuration hot-reloading", "error", err)
			// Continue without hot-reload
		} else {
			logger.Info("Configuration hot-reloading enabled", "file", *configFile)
		}
	}

	// Override config with command line flags if specified
	if *concurrency > 0 {
		cfg.Concurrency = *concurrency
	}
	if *timeout > 0 {
		cfg.Timeout = *timeout
	}

	// Override metrics config with CLI flags
	if *enableMetrics {
		cfg.Metrics.Enabled = true
		cfg.Metrics.ListenAddr = *metricsAddr
		cfg.Metrics.Path = *metricsPath
	}

	// Override protocol settings with CLI flags
	if *enableHTTP2 {
		cfg.EnableHTTP2 = true
	}
	if *enableHTTP3 {
		cfg.EnableHTTP3 = true
	}

	// Override discovery settings with CLI flags
	if *discoverCountries != "" {
		cfg.Discovery.Countries = strings.Split(*discoverCountries, ",")
	}
	if *discoverMinConfidence > 0 {
		cfg.Discovery.MinConfidence = *discoverMinConfidence
	}

	// Handle discovery mode
	if *discoverMode {
		runDiscoveryMode(cfg, logger, *discoverSource, *discoverQuery, *discoverLimit, *discoverValidate, *outputFile, *jsonFile)
		return
	}

	// Load proxies
	proxies, warnings, err := loader.LoadProxies(*proxyList)
	if err != nil {
		// Enhanced error logging with error categorization
		category := errors.GetErrorCategory(err)
		logger.Error("Failed to load proxies",
			"error", err,
			"file", *proxyList,
			"category", category,
			"retryable", errors.IsRetryable(err))
		os.Exit(1)
	}

	// Check if we have any proxies to work with
	if len(proxies) == 0 {
		logger.Error("No valid proxies found to check", "file", *proxyList)
		os.Exit(1)
	}

	logger.ProxiesLoaded(len(proxies), *proxyList)

	// Log any warnings
	for _, warning := range warnings {
		logger.Warn("Proxy loading warning", "warning", warning)
	}

	// Initialize metrics collector
	var metricsCollector *metrics.Collector
	if cfg.Metrics.Enabled {
		metricsCollector = metrics.NewCollector()
		if err := metricsCollector.StartServer(cfg.Metrics.ListenAddr); err != nil {
			logger.Warn("Failed to start metrics server", "error", err, "addr", cfg.Metrics.ListenAddr)
		} else {
			logger.Info("Metrics server started", "addr", cfg.Metrics.ListenAddr, "path", cfg.Metrics.Path)
		}
	}

	// Create connection pool
	poolConfig := pool.Config{
		MaxIdleConns:          cfg.ConnectionPool.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.ConnectionPool.MaxIdleConnsPerHost,
		MaxConnsPerHost:       cfg.ConnectionPool.MaxConnsPerHost,
		IdleConnTimeout:       cfg.ConnectionPool.IdleConnTimeout,
		KeepAliveTimeout:      cfg.ConnectionPool.KeepAliveTimeout,
		TLSHandshakeTimeout:   cfg.ConnectionPool.TLSHandshakeTimeout,
		ExpectContinueTimeout: cfg.ConnectionPool.ExpectContinueTimeout,
		DisableKeepAlives:     cfg.ConnectionPool.DisableKeepAlives,
		DisableCompression:    cfg.ConnectionPool.DisableCompression,
		InsecureSkipVerify:    cfg.InsecureSkipVerify,
	}
	connectionPool := pool.NewConnectionPool(poolConfig)
	logger.Info("Connection pool initialized",
		"max_idle_conns", poolConfig.MaxIdleConns,
		"max_idle_conns_per_host", poolConfig.MaxIdleConnsPerHost,
		"max_conns_per_host", poolConfig.MaxConnsPerHost)

	// Create proxy checker
	checker := proxy.NewChecker(proxy.Config{
		Timeout:             time.Duration(cfg.Timeout) * time.Second,
		ValidationURL:       cfg.TestURLs.DefaultURL,
		DisallowedKeywords:  cfg.Validation.DisallowedKeywords,
		MinResponseBytes:    cfg.Validation.MinResponseBytes,
		DefaultHeaders:      cfg.DefaultHeaders,
		UserAgent:           cfg.UserAgent,
		EnableCloudChecks:   cfg.EnableCloudChecks,
		CloudProviders:      cfg.CloudProviders,
		RequireStatusCode:   cfg.RequireStatusCode,
		RequireContentMatch: cfg.RequireContentMatch,
		RequireHeaderFields: cfg.RequireHeaderFields,
		AdvancedChecks:      cfg.AdvancedChecks,
		UseRDNS:             *useRDNS,
		InteractshURL:       cfg.InteractshURL,
		InteractshToken:     cfg.InteractshToken,

		// Rate limiting settings
		RateLimitEnabled:  *rateLimitEnabled,
		RateLimitDelay:    *rateLimitDelay,
		RateLimitPerHost:  *rateLimitPerHost,
		RateLimitPerProxy: *rateLimitPerProxy,

		// Retry settings
		RetryEnabled:    cfg.RetryEnabled,
		MaxRetries:      cfg.MaxRetries,
		InitialDelay:    cfg.InitialRetryDelay,
		MaxDelay:        cfg.MaxRetryDelay,
		BackoffFactor:   cfg.BackoffFactor,
		RetryableErrors: cfg.RetryableErrors,

		// Authentication settings
		AuthEnabled:     cfg.AuthEnabled,
		DefaultUsername: cfg.DefaultUsername,
		DefaultPassword: cfg.DefaultPassword,
		AuthMethods:     cfg.AuthMethods,

		// Connection pool
		ConnectionPool: connectionPool,

		// HTTP/2 and HTTP/3 settings
		EnableHTTP2: cfg.EnableHTTP2,
		EnableHTTP3: cfg.EnableHTTP3,
	}, *debug || cfg.AdvancedChecks.TestProtocolSmuggling || cfg.AdvancedChecks.TestDNSRebinding)

	// Initialize UI
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	view := &ui.View{
		Progress: p,
		Total:    len(proxies),
		DisplayMode: ui.ViewDisplayMode{
			IsVerbose: *verbose,                                                                                  // Only use verbose flag
			IsDebug:   *debug || cfg.AdvancedChecks.TestProtocolSmuggling || cfg.AdvancedChecks.TestDNSRebinding, // Use debug mode if flag or advanced checks are enabled
		},
		ActiveChecks: make(map[string]*ui.CheckStatus),
	}

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	// Create progress indicator for non-TUI mode
	var progressIndicator progresspkg.ProgressIndicator
	if *noUI {
		progressConfig := progresspkg.Config{
			Type:      progresspkg.ProgressType(*progressType),
			Width:     *progressWidth,
			NoColor:   *progressNoColor,
			ShowETA:   true,
			ShowStats: true,
		}
		progressIndicator = progresspkg.NewProgressIndicator(progressConfig)
	}

	// Create application state
	state := &AppState{
		view:              view,
		checker:           checker,
		proxies:           proxies,
		concurrency:       cfg.Concurrency,
		verbose:           *verbose, // Only use verbose flag
		debug:             *debug || cfg.AdvancedChecks.TestProtocolSmuggling || cfg.AdvancedChecks.TestDNSRebinding,
		logger:            logger,
		updateChan:        make(chan tea.Msg, 100), // Buffer for update messages
		ctx:               ctx,
		cancel:            cancel,
		shutdownChan:      shutdownChan,
		outputFile:        *outputFile,
		jsonFile:          *jsonFile,
		workingFile:       *workingFile,
		anonymousFile:     *anonymousFile,
		noUI:              *noUI,
		progressIndicator: progressIndicator,
		metricsCollector:  metricsCollector,
		configWatcher:     configWatcher,
	}

	// Start shutdown handler goroutine
	go func() {
		<-shutdownChan
		logger.ShutdownReceived()
		cancel() // Cancel the context to signal all goroutines to stop

		// Give goroutines time to clean up
		time.Sleep(2 * time.Second)

		// Process any remaining results
		processResults(state)

		// Stop config watcher
		if state.configWatcher != nil {
			if err := state.configWatcher.Stop(); err != nil {
				logger.Warn("Error stopping config watcher", "error", err)
			} else {
				logger.Info("Config watcher stopped")
			}
		}

		// Stop metrics server
		if state.metricsCollector != nil {
			if err := state.metricsCollector.StopServer(); err != nil {
				logger.Warn("Error stopping metrics server", "error", err)
			} else {
				logger.Info("Metrics server stopped")
			}
		}

		// Clean up connection pool
		connectionPool.CloseIdleConnections()
		logger.Info("Connection pool cleaned up")

		logger.ShutdownComplete()
		os.Exit(0)
	}()

	if state.noUI {
		// Run without UI
		logger.ProxyCheckStart(len(state.proxies), state.concurrency)
		state.startCheckingNoUI()
	} else {
		// Start the UI
		program := tea.NewProgram(state)

		// Start a goroutine to forward messages from updateChan to the program
		go func() {
			for msg := range state.updateChan {
				program.Send(msg)
			}
		}()

		if _, err := program.Run(); err != nil {
			logger.Error("Failed to run TUI program", "error", err)
			os.Exit(1)
		}
	}

	// Process results
	processResults(state)
}

func processResults(state *AppState) {
	// Generate summary
	summary := output.GenerateSummary(state.results)
	outputResults := output.ConvertToOutputFormat(state.results)

	// Log summary statistics
	state.logger.SummaryStats(summary.TotalProxies, summary.WorkingProxies, summary.AnonymousProxies, summary.SuccessRate)

	// Write output files if specified
	if state.outputFile != "" {
		if err := output.WriteTextOutput(state.outputFile, outputResults, summary); err != nil {
			state.logger.Error("Failed to write text output", "error", err, "file", state.outputFile)
		} else {
			state.logger.ResultsSaved(state.outputFile, "text")
		}
	}

	if state.jsonFile != "" {
		if err := output.WriteJSONOutput(state.jsonFile, summary); err != nil {
			state.logger.Error("Failed to write JSON output", "error", err, "file", state.jsonFile)
		} else {
			state.logger.ResultsSaved(state.jsonFile, "json")
		}
	}

	if state.workingFile != "" {
		if err := output.WriteWorkingProxiesOutput(state.workingFile, outputResults); err != nil {
			state.logger.Error("Failed to write working proxies", "error", err, "file", state.workingFile)
		} else {
			state.logger.ResultsSaved(state.workingFile, "working_proxies")
		}
	}

	if state.anonymousFile != "" {
		if err := output.WriteAnonymousProxiesOutput(state.anonymousFile, outputResults); err != nil {
			state.logger.Error("Failed to write anonymous proxies", "error", err, "file", state.anonymousFile)
		} else {
			state.logger.ResultsSaved(state.anonymousFile, "anonymous_proxies")
		}
	}
}

// Tea model implementation
func (s *AppState) Init() tea.Cmd {
	// Start proxy checking
	go s.startChecking()
	// Start a ticker to update the UI regularly
	return tea.Tick(time.Millisecond*100, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (s *AppState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q", "ctrl+c":
			return s, tea.Quit
		}
	case tickMsg, progressUpdateMsg:
		// Update progress bar
		s.mutex.Lock()
		progress := float64(s.view.Current) / float64(s.view.Total)
		progressCmd := s.view.Progress.SetPercent(progress)

		// Update other metrics
		s.view.Metrics.ActiveJobs = 0
		for _, status := range s.view.ActiveChecks {
			if status.IsActive && time.Since(status.LastUpdate) < 5*time.Second {
				s.view.Metrics.ActiveJobs++
			}
		}

		// Calculate success rate
		workingProxies := 0
		for _, result := range s.results {
			if result.Working {
				workingProxies++
			}
		}
		if s.view.Current > 0 {
			s.view.Metrics.SuccessRate = float64(workingProxies) / float64(s.view.Current) * 100
		}

		// Calculate average speed
		var totalSpeed time.Duration
		var speedCount int
		for _, result := range s.results {
			if result.Speed > 0 {
				totalSpeed += result.Speed
				speedCount++
			}
		}
		if speedCount > 0 {
			s.view.Metrics.AvgSpeed = totalSpeed / time.Duration(speedCount)
		}

		// Update metrics if enabled
		if s.metricsCollector != nil {
			s.metricsCollector.SetActiveChecks(s.view.Metrics.ActiveJobs)
			s.metricsCollector.SetQueueSize(s.view.Metrics.QueueSize)
			s.metricsCollector.SetWorkersActive(s.concurrency) // This could be made more dynamic
		}

		s.mutex.Unlock()

		// Continue the ticker for regular updates
		return s, tea.Batch(
			progressCmd,
			tea.Tick(time.Millisecond*100, func(time.Time) tea.Msg {
				return tickMsg{}
			}),
		)
	}

	// Update spinner every time the UI updates
	s.view.SpinnerIdx++

	return s, nil
}

func (s *AppState) View() string {
	if s.view.DisplayMode.IsDebug {
		return s.view.RenderDebug()
	}
	if s.view.DisplayMode.IsVerbose {
		return s.view.RenderVerbose()
	}
	return s.view.RenderDefault()
}

func (s *AppState) startChecking() {
	var wg sync.WaitGroup
	proxyChan := make(chan string)

	// Set initial queue size
	s.mutex.Lock()
	s.view.Metrics.QueueSize = len(s.proxies)
	s.mutex.Unlock()

	// Send initial update
	s.updateChan <- progressUpdateMsg{}

	if s.debug {
		s.mutex.Lock()
		s.view.DebugInfo += fmt.Sprintf("[DEBUG] Starting proxy checks with concurrency: %d\n", s.concurrency)
		s.view.DebugInfo += fmt.Sprintf("[DEBUG] Total proxies to check: %d\n", len(s.proxies))
		s.mutex.Unlock()

		// Send update
		s.updateChan <- progressUpdateMsg{}
	}

	// Start workers
	for i := 0; i < s.concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			// Add panic recovery to prevent worker crashes from affecting the whole application
			defer func() {
				if r := recover(); r != nil {
					s.mutex.Lock()
					s.view.DebugInfo += fmt.Sprintf("[ERROR] Worker %d panicked: %v\n", workerID, r)
					s.mutex.Unlock()

					// Send update
					s.updateChan <- progressUpdateMsg{}
				}
				wg.Done()
			}()

			if s.debug {
				s.mutex.Lock()
				s.view.DebugInfo += fmt.Sprintf("[DEBUG] Worker %d started\n", workerID)
				s.mutex.Unlock()

				// Send update
				s.updateChan <- progressUpdateMsg{}
			}

			for proxy := range proxyChan {
				// Check for cancellation before processing
				select {
				case <-s.ctx.Done():
					if s.debug {
						s.mutex.Lock()
						s.view.DebugInfo += fmt.Sprintf("[DEBUG] Worker %d cancelled\n", workerID)
						s.mutex.Unlock()
						s.updateChan <- progressUpdateMsg{}
					}
					return
				default:
					// Continue processing
				}

				// Update active job status when starting a check
				s.mutex.Lock()
				status := &ui.CheckStatus{
					Proxy:      proxy,
					IsActive:   true,
					LastUpdate: time.Now(),
				}
				s.view.ActiveChecks[proxy] = status

				// Update queue size when starting a check
				s.view.Metrics.QueueSize = len(s.proxies) - s.view.Current - s.view.Metrics.ActiveJobs
				if s.view.Metrics.QueueSize < 0 {
					s.view.Metrics.QueueSize = 0
				}
				s.mutex.Unlock()

				// Send update
				s.updateChan <- progressUpdateMsg{}

				if s.debug {
					s.mutex.Lock()
					s.view.DebugInfo += fmt.Sprintf("[DEBUG] Worker %d checking: %s\n", workerID, proxy)
					s.mutex.Unlock()

					// Send update
					s.updateChan <- progressUpdateMsg{}
				}

				result := s.checker.Check(proxy)

				// Record metrics if enabled
				if s.metricsCollector != nil {
					s.metricsCollector.RecordProxyCheck(result.Working, string(result.Type), result.Speed)
					if result.IsAnonymous {
						s.metricsCollector.RecordAnonymousProxy()
					}
					if result.CloudProvider != "" {
						s.metricsCollector.RecordCloudProvider(result.CloudProvider)
					}
					if result.Error != nil {
						s.metricsCollector.RecordError("proxy_check_failed")
					}
				}

				// Update queue size after each check is no longer needed here as it will be updated in processResult
				// or when marking a job as inactive

				// Send update
				s.updateChan <- progressUpdateMsg{}

				if !result.Working {
					if s.debug {
						s.mutex.Lock()
						// Create a more concise error message
						errorMsg := "Proxy not working"
						if result.Error != nil {
							errorMsg = result.Error.Error()
							// Truncate long error messages
							if len(errorMsg) > 100 {
								errorMsg = errorMsg[:97] + "..."
							}
						}
						s.view.DebugInfo += fmt.Sprintf("[DEBUG] Worker %d failed: %s - %s\n",
							workerID,
							proxy,
							errorMsg)
						s.mutex.Unlock()

						// Send update
						s.updateChan <- progressUpdateMsg{}
					}

					// Mark job as inactive on error
					s.mutex.Lock()
					if status, ok := s.view.ActiveChecks[proxy]; ok {
						status.IsActive = false
						status.LastUpdate = time.Now()
					}

					// Update queue size when a job is marked as inactive
					s.view.Metrics.QueueSize = len(s.proxies) - s.view.Current - s.view.Metrics.ActiveJobs
					if s.view.Metrics.QueueSize < 0 {
						s.view.Metrics.QueueSize = 0
					}
					s.mutex.Unlock()

					// Send update
					s.updateChan <- progressUpdateMsg{}

					continue
				}

				if s.debug {
					s.mutex.Lock()
					s.view.DebugInfo += fmt.Sprintf("[DEBUG] Worker %d success: %s (%s)\n",
						workerID,
						proxy,
						result.Type)
					s.mutex.Unlock()

					// Send update
					s.updateChan <- progressUpdateMsg{}
				}

				s.processResult(result)

				// Send update
				s.updateChan <- progressUpdateMsg{}
			}

			if s.debug {
				s.mutex.Lock()
				s.view.DebugInfo += fmt.Sprintf("[DEBUG] Worker %d finished\n", workerID)
				s.mutex.Unlock()

				// Send update
				s.updateChan <- progressUpdateMsg{}
			}
		}(i)
	}

	// Feed proxies to workers
	if s.debug {
		s.mutex.Lock()
		s.view.DebugInfo += fmt.Sprintf("[DEBUG] Starting to feed proxies to workers\n")
		s.mutex.Unlock()

		// Send update
		s.updateChan <- progressUpdateMsg{}
	}

	for _, proxy := range s.proxies {
		// Check for cancellation before sending each proxy
		select {
		case <-s.ctx.Done():
			if s.debug {
				s.mutex.Lock()
				s.view.DebugInfo += fmt.Sprintf("[DEBUG] Proxy feeding cancelled\n")
				s.mutex.Unlock()
				s.updateChan <- progressUpdateMsg{}
			}
			close(proxyChan)
			return
		default:
			if s.debug {
				s.mutex.Lock()
				s.view.DebugInfo += fmt.Sprintf("[DEBUG] Sending proxy to channel: %s\n", proxy)
				s.mutex.Unlock()
			}
			proxyChan <- proxy
		}
	}

	if s.debug {
		s.mutex.Lock()
		s.view.DebugInfo += fmt.Sprintf("[DEBUG] All proxies sent to channel, closing\n")
		s.mutex.Unlock()

		// Send update
		s.updateChan <- progressUpdateMsg{}
	}
	close(proxyChan)

	if s.debug {
		s.mutex.Lock()
		s.view.DebugInfo += fmt.Sprintf("[DEBUG] Waiting for workers to finish\n")
		s.mutex.Unlock()

		// Send update
		s.updateChan <- progressUpdateMsg{}
	}

	// Add a timeout mechanism to prevent deadlocks
	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	// Set a reasonable timeout (adjust as needed)
	timeout := 30 * time.Second
	if s.concurrency > 10 {
		// Add more time for higher concurrency
		timeout = time.Duration(s.concurrency*3) * time.Second
	}

	select {
	case <-waitCh:
		// All workers finished normally
		if s.debug {
			s.mutex.Lock()
			s.view.DebugInfo += fmt.Sprintf("[DEBUG] All workers finished successfully\n")
			s.mutex.Unlock()

			// Send update
			s.updateChan <- progressUpdateMsg{}
		}
	case <-time.After(timeout):
		// Timeout occurred, some workers might be stuck
		if s.debug {
			s.mutex.Lock()
			s.view.DebugInfo += fmt.Sprintf("[DEBUG] WARNING: Timed out waiting for some workers after %v\n", timeout)
			s.view.DebugInfo += fmt.Sprintf("[DEBUG] Proceeding with available results\n")
			s.mutex.Unlock()

			// Send update
			s.updateChan <- progressUpdateMsg{}
		}
	}

	// Close the update channel
	close(s.updateChan)
}

func (s *AppState) processResult(result *proxy.ProxyResult) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.results = append(s.results, result)
	s.view.Current++

	// Convert check results
	uiCheckResults := make([]ui.CheckResult, len(result.CheckResults))
	for i, cr := range result.CheckResults {
		uiCheckResults[i] = ui.CheckResult{
			URL:        cr.URL,
			Success:    cr.Success,
			Speed:      cr.Speed,
			Error:      cr.Error,
			StatusCode: cr.StatusCode,
			BodySize:   cr.BodySize,
		}
	}

	// Update UI state
	status := &ui.CheckStatus{
		Proxy:          result.ProxyURL,
		TotalChecks:    len(result.CheckResults),
		DoneChecks:     len(result.CheckResults),
		LastUpdate:     time.Now(),
		Speed:          result.Speed,
		ProxyType:      string(result.Type),
		IsActive:       false, // Mark as inactive since check is complete
		CloudProvider:  result.CloudProvider,
		InternalAccess: result.InternalAccess,
		MetadataAccess: result.MetadataAccess,
		SupportsHTTP:   result.SupportsHTTP,
		SupportsHTTPS:  result.SupportsHTTPS,
		CheckResults:   uiCheckResults,
		DebugInfo:      result.DebugInfo,
	}

	s.view.ActiveChecks[result.ProxyURL] = status

	// Update queue size - calculate remaining proxies to check
	s.view.Metrics.QueueSize = len(s.proxies) - s.view.Current - s.view.Metrics.ActiveJobs
	if s.view.Metrics.QueueSize < 0 {
		s.view.Metrics.QueueSize = 0
	}

	// Add debug info to the main debug output if in debug mode
	if s.debug && result.DebugInfo != "" {
		s.view.DebugInfo += result.DebugInfo
	}
}

// startCheckingNoUI runs proxy checking without UI (for automation)
func (s *AppState) startCheckingNoUI() {
	var wg sync.WaitGroup
	proxyChan := make(chan string)

	s.logger.Info("Starting proxy tests", "total", len(s.proxies), "concurrency", s.concurrency)

	// Start progress indicator if available
	if s.progressIndicator != nil {
		s.progressIndicator.Start(len(s.proxies))
	}

	// Start workers
	for i := 0; i < s.concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for proxy := range proxyChan {
				// Check for cancellation before processing
				select {
				case <-s.ctx.Done():
					if s.verbose {
						s.logger.WithWorker(workerID).Debug("Worker cancelled")
					}
					return
				default:
					// Continue processing
				}

				if s.verbose {
					s.logger.WithWorker(workerID).WithProxy(proxy).Debug("Testing proxy")
				}

				result := s.checker.Check(proxy)

				// Record metrics if enabled
				if s.metricsCollector != nil {
					s.metricsCollector.RecordProxyCheck(result.Working, string(result.Type), result.Speed)
					if result.IsAnonymous {
						s.metricsCollector.RecordAnonymousProxy()
					}
					if result.CloudProvider != "" {
						s.metricsCollector.RecordCloudProvider(result.CloudProvider)
					}
					if result.Error != nil {
						s.metricsCollector.RecordError("proxy_check_failed")
					}
				}

				s.mutex.Lock()
				s.results = append(s.results, result)
				current := len(s.results)
				s.mutex.Unlock()

				// Update progress indicator
				if s.progressIndicator != nil {
					var message string
					if result.Working {
						if result.IsAnonymous {
							message = "working anonymous proxy"
						} else {
							message = "working proxy"
						}
					} else {
						message = "failed proxy check"
					}
					s.progressIndicator.Update(current, message)
				}

				if result.Working {
					s.logger.WithContext("progress", fmt.Sprintf("%d/%d", current, len(s.proxies))).ProxySuccess(proxy, result.Speed.Seconds(), result.IsAnonymous, result.CloudProvider)
				} else {
					if s.verbose {
						s.logger.WithContext("progress", fmt.Sprintf("%d/%d", current, len(s.proxies))).ProxyFailure(proxy, result.Error)
					}
				}
			}
		}(i)
	}

	// Feed proxies to workers
	for _, proxy := range s.proxies {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Shutdown requested, stopping proxy feeding")
			close(proxyChan)
			return
		case proxyChan <- proxy:
			// Proxy sent successfully, continue
		}
	}
	close(proxyChan)

	// Wait for all workers to finish
	wg.Wait()

	// Finish progress indicator
	if s.progressIndicator != nil {
		s.progressIndicator.Finish("Proxy checking completed")
	}

	s.logger.ProxyCheckComplete()
}

// runDiscoveryMode handles the proxy discovery workflow
func runDiscoveryMode(cfg *config.Config, logger *logging.Logger, source, query string, limit int, validate bool, outputFile, jsonFile string) {
	logger.Info("Starting proxy discovery mode",
		"source", source,
		"query", query,
		"limit", limit,
		"validate", validate)

	// Create discovery manager
	manager := discovery.NewManager(cfg.Discovery, logger)

	// Check available sources
	availableSources := manager.GetAvailableSources()
	if len(availableSources) == 0 {
		logger.Error("No discovery sources configured",
			"required", "API keys in config file or environment variables")
		fmt.Fprintf(os.Stderr, "Error: No discovery sources configured.\n")
		fmt.Fprintf(os.Stderr, "Available sources:\n")
		fmt.Fprintf(os.Stderr, "  - Shodan: Add shodan_api_key to config or set SHODAN_API_KEY env var\n")
		fmt.Fprintf(os.Stderr, "  - Censys: Add censys_api_key and censys_secret to config\n")
		fmt.Fprintf(os.Stderr, "  - Free Lists: Always available (no API key required)\n")
		fmt.Fprintf(os.Stderr, "  - Web Scraper: Always available (no API key required)\n")
		os.Exit(1)
	}

	logger.Info("Available discovery sources", "sources", availableSources)

	// Use default query if none provided
	if query == "" {
		presets := manager.GetPresetQueries()
		if sourceQueries, exists := presets[source]; exists && len(sourceQueries) > 0 {
			query = sourceQueries[0] // Use first preset query
			logger.Info("Using preset query", "query", query)
		} else {
			query = "proxy server" // Fallback
			logger.Info("Using fallback query", "query", query)
		}
	}

	// Execute discovery
	var result *discovery.DiscoveryResult
	var err error

	if source == "all" {
		result, err = manager.SearchAll(query, limit)
	} else {
		result, err = manager.SearchSource(source, query, limit)
	}

	if err != nil {
		logger.Error("Discovery failed", "error", err)
		fmt.Fprintf(os.Stderr, "Discovery failed: %v\n", err)
		os.Exit(1)
	}

	// Display discovery results
	logger.Info("Discovery completed",
		"candidates_found", len(result.Candidates),
		"duration", result.Duration,
		"source", result.Source)

	fmt.Printf("\n🔍 Discovery Results\n")
	fmt.Printf("====================\n")
	fmt.Printf("Query: %s\n", result.Query)
	fmt.Printf("Source: %s\n", result.Source)
	fmt.Printf("Duration: %v\n", result.Duration)
	fmt.Printf("Candidates found: %d\n", len(result.Candidates))
	
	if len(result.Errors) > 0 {
		fmt.Printf("Errors: %d\n", len(result.Errors))
		for _, errMsg := range result.Errors {
			fmt.Printf("  - %s\n", errMsg)
		}
	}
	fmt.Printf("\n")

	if len(result.Candidates) == 0 {
		fmt.Printf("No proxy candidates found. Try adjusting your search query or filters.\n")
		return
	}

	// Show top candidates
	fmt.Printf("🎯 Top Candidates (by confidence):\n")
	fmt.Printf("===================================\n")
	showLimit := 10
	if len(result.Candidates) < showLimit {
		showLimit = len(result.Candidates)
	}

	for i, candidate := range result.Candidates[:showLimit] {
		fmt.Printf("%d. %s:%d (%s) - Confidence: %.2f\n",
			i+1, candidate.IP, candidate.Port, candidate.Protocol, candidate.Confidence)
		if candidate.Country != "" {
			fmt.Printf("   Location: %s", candidate.Country)
			if candidate.City != "" {
				fmt.Printf(", %s", candidate.City)
			}
			fmt.Printf("\n")
		}
		if candidate.ProxyType != "" && candidate.ProxyType != "unknown" {
			fmt.Printf("   Type: %s\n", candidate.ProxyType)
		}
		if candidate.ServerHeader != "" {
			fmt.Printf("   Server: %s\n", candidate.ServerHeader)
		}
		fmt.Printf("\n")
	}

	// Validate candidates if requested
	if validate {
		fmt.Printf("🧪 Validating discovered candidates...\n")
		fmt.Printf("=====================================\n")
		
		// Convert candidates to proxy URLs for validation
		proxyURLs := make([]string, len(result.Candidates))
		for i, candidate := range result.Candidates {
			proxyURLs[i] = fmt.Sprintf("%s://%s:%d", candidate.Protocol, candidate.IP, candidate.Port)
		}

		// Create proxy checker
		poolConfig := pool.Config{
			MaxIdleConns:          cfg.ConnectionPool.MaxIdleConns,
			MaxIdleConnsPerHost:   cfg.ConnectionPool.MaxIdleConnsPerHost,
			MaxConnsPerHost:       cfg.ConnectionPool.MaxConnsPerHost,
			IdleConnTimeout:       cfg.ConnectionPool.IdleConnTimeout,
			KeepAliveTimeout:      cfg.ConnectionPool.KeepAliveTimeout,
			TLSHandshakeTimeout:   cfg.ConnectionPool.TLSHandshakeTimeout,
			ExpectContinueTimeout: cfg.ConnectionPool.ExpectContinueTimeout,
			DisableKeepAlives:     cfg.ConnectionPool.DisableKeepAlives,
			DisableCompression:    cfg.ConnectionPool.DisableCompression,
			InsecureSkipVerify:    cfg.InsecureSkipVerify,
		}
		connectionPool := pool.NewConnectionPool(poolConfig)

		checker := proxy.NewChecker(proxy.Config{
			Timeout:             time.Duration(cfg.Timeout) * time.Second,
			ValidationURL:       cfg.TestURLs.DefaultURL,
			DisallowedKeywords:  cfg.Validation.DisallowedKeywords,
			MinResponseBytes:    cfg.Validation.MinResponseBytes,
			DefaultHeaders:      cfg.DefaultHeaders,
			UserAgent:           cfg.UserAgent,
			ConnectionPool:      connectionPool,
		}, false) // Don't use debug mode for validation

		// Validate proxies concurrently
		workingCandidates := make([]*discovery.ProxyCandidate, 0)
		
		// Simple validation - test a few candidates
		testLimit := 20
		if len(result.Candidates) < testLimit {
			testLimit = len(result.Candidates)
		}

		fmt.Printf("Testing top %d candidates...\n\n", testLimit)
		
		for i, candidate := range result.Candidates[:testLimit] {
			proxyURL := fmt.Sprintf("%s://%s:%d", candidate.Protocol, candidate.IP, candidate.Port)
			
			fmt.Printf("Testing %d/%d: %s... ", i+1, testLimit, proxyURL)
			
			proxyResult := checker.Check(proxyURL)
			if proxyResult.Working {
				fmt.Printf("✅ Working (%.2fs)\n", proxyResult.Speed.Seconds())
				workingCandidates = append(workingCandidates, &candidate)
			} else {
				fmt.Printf("❌ Failed\n")
			}
		}

		fmt.Printf("\n✅ Validation Summary:\n")
		fmt.Printf("Tested: %d\n", testLimit)
		fmt.Printf("Working: %d\n", len(workingCandidates))
		fmt.Printf("Success Rate: %.1f%%\n", float64(len(workingCandidates))/float64(testLimit)*100)
	}

	// Save results to files if requested
	if outputFile != "" {
		err := saveCandidatesToText(result.Candidates, outputFile)
		if err != nil {
			logger.Error("Failed to save text output", "error", err, "file", outputFile)
		} else {
			logger.Info("Candidates saved to text file", "file", outputFile, "count", len(result.Candidates))
			fmt.Printf("📝 Candidates saved to: %s\n", outputFile)
		}
	}

	if jsonFile != "" {
		err := saveCandidatesToJSON(result, jsonFile)
		if err != nil {
			logger.Error("Failed to save JSON output", "error", err, "file", jsonFile)
		} else {
			logger.Info("Discovery result saved to JSON file", "file", jsonFile)
			fmt.Printf("💾 Full results saved to: %s\n", jsonFile)
		}
	}

	fmt.Printf("\n🎉 Discovery completed successfully!\n")
}

// saveCandidatesToText saves discovery candidates to a text file
func saveCandidatesToText(candidates []discovery.ProxyCandidate, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	fmt.Fprintf(file, "# ProxyHawk Discovery Results\n")
	fmt.Fprintf(file, "# Generated: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "# Total candidates: %d\n\n", len(candidates))

	for _, candidate := range candidates {
		// Write in format: protocol://ip:port
		fmt.Fprintf(file, "%s://%s:%d\n", candidate.Protocol, candidate.IP, candidate.Port)
	}

	return nil
}

// saveCandidatesToJSON saves the full discovery result to a JSON file
func saveCandidatesToJSON(result *discovery.DiscoveryResult, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}
