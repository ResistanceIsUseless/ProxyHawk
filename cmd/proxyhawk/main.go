package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/config"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/loader"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/logging"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/output"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/proxy"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/ui"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)


// AppState represents the application state
type AppState struct {
	view          *ui.View
	checker       *proxy.Checker
	proxies       []string
	results       []*proxy.ProxyResult
	concurrency   int
	verbose       bool
	debug         bool
	logger        *logging.Logger
	mutex         sync.Mutex   // Mutex to protect shared state
	updateChan    chan tea.Msg // Channel for sending updates to the UI
	
	// Graceful shutdown support
	ctx           context.Context
	cancel        context.CancelFunc
	shutdownChan  chan os.Signal
	
	// Output options
	outputFile    string
	jsonFile      string
	workingFile   string
	anonymousFile string
	noUI          bool
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

	// Rate limiting flags
	rateLimitEnabled := flag.Bool("rate-limit", false, "Enable rate limiting")
	rateLimitDelay := flag.Duration("rate-delay", 1*time.Second, "Delay between requests (e.g. 500ms, 1s, 2s)")
	rateLimitPerHost := flag.Bool("rate-per-host", true, "Apply rate limiting per host instead of globally")

	// Output flags
	outputFile := flag.String("o", "", "Output results to text file")
	jsonFile := flag.String("j", "", "Output results to JSON file")
	workingFile := flag.String("wp", "", "Output working proxies to file")
	anonymousFile := flag.String("wpa", "", "Output working anonymous proxies to file")
	noUI := flag.Bool("no-ui", false, "Disable terminal UI (for automation/scripting)")

	flag.Parse()

	// Initialize logger based on debug/verbose flags
	logLevel := logging.LevelInfo
	if *debug {
		logLevel = logging.LevelDebug
	}
	logger := logging.NewLogger(logging.Config{
		Level:  logLevel,
		Format: "text",
	})

	// Load configuration
	config, err := config.LoadConfig(*configFile)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err, "file", *configFile)
		os.Exit(1)
	}
	logger.ConfigLoaded(*configFile)

	// Override config with command line flags if specified
	if *concurrency > 0 {
		config.Concurrency = *concurrency
	}
	if *timeout > 0 {
		config.Timeout = *timeout
	}

	// Load proxies
	proxies, warnings, err := loader.LoadProxies(*proxyList)
	if err != nil {
		logger.Error("Failed to load proxies", "error", err, "file", *proxyList)
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
		UseRDNS:             *useRDNS,
		InteractshURL:       config.InteractshURL,
		InteractshToken:     config.InteractshToken,

		// Rate limiting settings
		RateLimitEnabled: *rateLimitEnabled,
		RateLimitDelay:   *rateLimitDelay,
		RateLimitPerHost: *rateLimitPerHost,
	}, *debug || config.AdvancedChecks.TestProtocolSmuggling || config.AdvancedChecks.TestDNSRebinding)

	// Initialize UI
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	view := &ui.View{
		Progress:     p,
		Total:        len(proxies),
		DisplayMode: ui.ViewDisplayMode{
			IsVerbose: *verbose,                                                                                        // Only use verbose flag
			IsDebug:   *debug || config.AdvancedChecks.TestProtocolSmuggling || config.AdvancedChecks.TestDNSRebinding, // Use debug mode if flag or advanced checks are enabled
		},
		ActiveChecks: make(map[string]*ui.CheckStatus),
	}

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	// Create application state
	state := &AppState{
		view:          view,
		checker:       checker,
		proxies:       proxies,
		concurrency:   config.Concurrency,
		verbose:       *verbose, // Only use verbose flag
		debug:         *debug || config.AdvancedChecks.TestProtocolSmuggling || config.AdvancedChecks.TestDNSRebinding,
		logger:        logger,
		updateChan:    make(chan tea.Msg, 100), // Buffer for update messages
		ctx:           ctx,
		cancel:        cancel,
		shutdownChan:  shutdownChan,
		outputFile:    *outputFile,
		jsonFile:      *jsonFile,
		workingFile:   *workingFile,
		anonymousFile: *anonymousFile,
		noUI:          *noUI,
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
				
				s.mutex.Lock()
				s.results = append(s.results, result)
				current := len(s.results)
				s.mutex.Unlock()

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
	
	s.logger.ProxyCheckComplete()
}
