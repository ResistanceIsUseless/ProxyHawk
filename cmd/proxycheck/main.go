package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"flag"

	"github.com/ResistanceIsUseless/ProxyCheck/cloudcheck"
	proxylib "github.com/ResistanceIsUseless/ProxyCheck/proxy"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

	// Add spinner frames
	spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

// Add spinner helper
func getSpinnerFrame(idx int) string {
	return spinnerFrames[idx%len(spinnerFrames)]
}

// Model represents the application state
type Model struct {
	progress      progress.Model
	total         int
	current       int
	results       []ProxyResultOutput
	quitting      bool
	err           error
	activeJobs    int
	queueSize     int
	successRate   float64
	avgSpeed      time.Duration
	debugInfo     string
	warnings      []string
	spinnerIdx    int
	activeChecks  map[string]*CheckStatus
	lastUpdate    time.Time
	cleanupTicker time.Time // Add cleanup ticker
	useInteractsh bool
	useCloud      bool
	verbose       bool
	debug         bool // Add debug mode flag
}

// Add this struct to track check status
type CheckStatus struct {
	Proxy        string
	TotalChecks  int
	DoneChecks   int
	LastUpdate   time.Time
	CheckResults []CheckResult
	Speed        time.Duration
	IsActive     bool
	ProxyType    string
	Position     int // Add position field
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tick(), m.progress.Init())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.String() == "q" {
			m.quitting = true
			return m, tea.Quit
		}

	case tickMsg:
		m.spinnerIdx++
		now := time.Now()

		// Cleanup stalled checks every second
		if now.Sub(m.cleanupTicker) >= time.Second {
			m.cleanupTicker = now
			for proxy, status := range m.activeChecks {
				// If a check hasn't updated in 10 seconds or is complete, remove it
				if now.Sub(status.LastUpdate) > 10*time.Second ||
					(status.DoneChecks >= status.TotalChecks && !status.IsActive) {
					delete(m.activeChecks, proxy)
				}
			}
		}

		// Update active jobs count to match actual active checks
		activeCount := 0
		for _, status := range m.activeChecks {
			if status.IsActive && time.Since(status.LastUpdate) < 5*time.Second {
				activeCount++
			}
		}
		if activeCount != m.activeJobs {
			m.activeJobs = activeCount
		}

		return m, tick()

	case progressMsg:
		if msg.current > m.current {
			m.current = msg.current
			m.activeJobs = msg.activeJobs
			m.queueSize = msg.queueSize
			m.successRate = msg.successRate
			m.avgSpeed = msg.avgSpeed
			m.debugInfo = msg.debugInfo
			m.lastUpdate = time.Now()

			// Update active checks
			proxy := msg.result.Proxy
			if len(msg.result.CheckResults) > 0 {
				status, exists := m.activeChecks[proxy]
				if !exists {
					status = &CheckStatus{
						Proxy:        proxy,
						TotalChecks:  len(config.TestURLs.URLs), // Use actual number from config
						CheckResults: make([]CheckResult, 0),
						IsActive:     true,
					}
					m.activeChecks[proxy] = status
				}
				status.DoneChecks = len(msg.result.CheckResults)
				status.CheckResults = msg.result.CheckResults
				status.Speed = msg.result.Speed
				status.LastUpdate = time.Now()

				// Only mark as inactive if all checks are complete
				if status.DoneChecks >= status.TotalChecks {
					status.IsActive = false
				}
			}

			if len(m.results) == 0 || m.results[len(m.results)-1].Proxy != msg.result.Proxy {
				m.results = append(m.results, msg.result)
			}
			return m, tea.Batch(
				m.progress.SetPercent(float64(m.current)/float64(m.total)),
				tick(),
			)
		}
		return m, tick()

	case quitMsg:
		m.quitting = true
		return m, tea.Quit
	}

	return m, tick()
}

func (m Model) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\nPress q to quit", m.err))
	}

	if m.quitting {
		return successStyle.Render("Quitting...")
	}

	if m.debug {
		return m.debugView()
	}

	if m.verbose {
		return m.verboseView()
	}

	return m.defaultView()
}

func (m Model) defaultView() string {
	str := strings.Builder{}

	// Title
	str.WriteString(headerStyle.Render("ProxyCheck Progress") + "\n\n")

	// Basic progress information
	progressBlock := strings.Builder{}
	progressBlock.WriteString(fmt.Sprintf("%s %s/%s\n",
		metricLabelStyle.Render("Progress:"),
		metricValueStyle.Render(fmt.Sprintf("%d", m.current)),
		metricValueStyle.Render(fmt.Sprintf("%d", m.total))))
	progressBlock.WriteString(m.progress.View())
	str.WriteString(progressStyle.Render(progressBlock.String()) + "\n")

	// Current Checks Section - Show all active proxies
	str.WriteString("\nCurrent Checks:\n")

	// Create a sorted slice of positions and their corresponding checks
	type proxyStatus struct {
		proxy  string
		status *CheckStatus
	}
	var activeList []proxyStatus
	positions := make(map[int]proxyStatus)

	// First, collect all active checks and their positions
	for proxy, status := range m.activeChecks {
		if status.IsActive && time.Since(status.LastUpdate) < 5*time.Second {
			positions[status.Position] = proxyStatus{proxy, status}
		}
	}

	// Create a sorted list based on positions
	maxPosition := -1
	for pos := range positions {
		if pos > maxPosition {
			maxPosition = pos
		}
	}

	// Fill the list in order of positions
	for i := 0; i <= maxPosition; i++ {
		if ps, exists := positions[i]; exists {
			activeList = append(activeList, ps)
		}
	}

	// Display active checks in their fixed positions
	for _, ps := range activeList {
		proxy, status := ps.proxy, ps.status
		hostPort := proxy
		if strings.Contains(hostPort, "://") {
			hostPort = strings.Split(hostPort, "://")[1]
		}

		spinner := getSpinnerFrame(m.spinnerIdx)
		successCount := 0
		for _, check := range status.CheckResults {
			if check.Success {
				successCount++
			}
		}

		// Create status indicator based on check results
		var statusIndicator string
		if len(status.CheckResults) == 0 {
			statusIndicator = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Render("CHECKING")
		} else if successCount == 0 && len(status.CheckResults) == len(config.TestURLs.URLs) {
			statusIndicator = errorStyle.Render("FAILED")
		} else if successCount == len(config.TestURLs.URLs) {
			statusIndicator = successStyle.Render("SUCCESS")
		} else if len(status.CheckResults) == len(config.TestURLs.URLs) {
			statusIndicator = warningStyle.Render("PARTIAL")
		} else {
			statusIndicator = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Render("CHECKING")
		}

		// Format the check count with appropriate color
		checkCountStyle := errorStyle
		if successCount > 0 {
			if successCount == len(config.TestURLs.URLs) {
				checkCountStyle = successStyle
			} else {
				checkCountStyle = warningStyle
			}
		}
		checkCount := checkCountStyle.Render(fmt.Sprintf("%d/%d", successCount, len(config.TestURLs.URLs)))

		statusLine := fmt.Sprintf("%s %s [%s] %s (%s checks)",
			lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(spinner),
			lipgloss.NewStyle().Bold(true).Render(hostPort),
			successStyle.Render(status.ProxyType),
			statusIndicator,
			checkCount)
		str.WriteString(statusLine + "\n")
	}

	// If we have fewer active checks than concurrency, show pending slots
	if len(activeList) < m.activeJobs {
		remainingSlots := m.activeJobs - len(activeList)
		waitingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			PaddingLeft(2)
		for i := 0; i < remainingSlots; i++ {
			str.WriteString(waitingStyle.Render("Waiting for next proxy...") + "\n")
		}
	}

	// Controls
	str.WriteString("\n" + infoStyle.Render("Press q to quit"))

	return str.String()
}

func (m Model) verboseView() string {
	str := strings.Builder{}

	// Title
	str.WriteString(headerStyle.Render("ProxyCheck Progress (Verbose Mode)") + "\n\n")

	// Basic progress information
	progressBlock := strings.Builder{}
	progressBlock.WriteString(fmt.Sprintf("%s %s/%s\n",
		metricLabelStyle.Render("Progress:"),
		metricValueStyle.Render(fmt.Sprintf("%d", m.current)),
		metricValueStyle.Render(fmt.Sprintf("%d", m.total))))
	progressBlock.WriteString(m.progress.View())
	str.WriteString(progressStyle.Render(progressBlock.String()) + "\n")

	// Metrics Section
	metricsBlock := strings.Builder{}
	metricsBlock.WriteString(fmt.Sprintf("%s %d\n",
		metricLabelStyle.Render("Active Jobs:"),
		m.activeJobs))
	metricsBlock.WriteString(fmt.Sprintf("%s %d\n",
		metricLabelStyle.Render("Queue Size:"),
		m.queueSize))
	metricsBlock.WriteString(fmt.Sprintf("%s %.2f%%\n",
		metricLabelStyle.Render("Success Rate:"),
		m.successRate))
	metricsBlock.WriteString(fmt.Sprintf("%s %v\n",
		metricLabelStyle.Render("Average Speed:"),
		m.avgSpeed.Round(time.Millisecond)))
	str.WriteString(metricBlockStyle.Render(metricsBlock.String()) + "\n")

	// Current Checks Section with detailed information
	str.WriteString("\nCurrent Checks:\n")

	type proxyStatus struct {
		proxy  string
		status *CheckStatus
	}
	var activeList []proxyStatus
	positions := make(map[int]proxyStatus)

	// First, collect all active checks and their positions
	for proxy, status := range m.activeChecks {
		if status.IsActive && time.Since(status.LastUpdate) < 5*time.Second {
			positions[status.Position] = proxyStatus{proxy, status}
		}
	}

	// Create a sorted list based on positions
	maxPosition := -1
	for pos := range positions {
		if pos > maxPosition {
			maxPosition = pos
		}
	}

	// Fill the list in order of positions
	for i := 0; i <= maxPosition; i++ {
		if ps, exists := positions[i]; exists {
			activeList = append(activeList, ps)
		}
	}

	// Display active checks in their fixed positions with detailed information
	for _, ps := range activeList {
		proxy, status := ps.proxy, ps.status
		hostPort := proxy
		if strings.Contains(hostPort, "://") {
			hostPort = strings.Split(hostPort, "://")[1]
		}

		spinner := getSpinnerFrame(m.spinnerIdx)
		successCount := 0
		for _, check := range status.CheckResults {
			if check.Success {
				successCount++
			}
		}

		// Main status line with more details
		str.WriteString(fmt.Sprintf("%s %s\n",
			lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(spinner),
			lipgloss.NewStyle().Bold(true).Render(hostPort)))

		str.WriteString(fmt.Sprintf("  Type: %s\n", successStyle.Render(status.ProxyType)))
		str.WriteString(fmt.Sprintf("  Status: %s\n", getStatusIndicator(status)))
		str.WriteString(fmt.Sprintf("  Progress: %s checks\n", getCheckCount(status)))
		if status.Speed > 0 {
			str.WriteString(fmt.Sprintf("  Speed: %v\n", status.Speed.Round(time.Millisecond)))
		}

		// Add detailed check results
		if len(status.CheckResults) > 0 {
			str.WriteString("  Check Results:\n")
			for _, check := range status.CheckResults {
				checkStatus := successStyle.Render("✓")
				if !check.Success {
					checkStatus = errorStyle.Render("✗")
				}

				// Format URL to show only the hostname
				displayURL := check.URL
				if u, err := url.Parse(check.URL); err == nil {
					displayURL = u.Host
				}

				// Build detailed check information
				details := []string{
					fmt.Sprintf("Status: %d", check.StatusCode),
					fmt.Sprintf("Speed: %v", check.Speed.Round(time.Millisecond)),
				}
				if check.BodySize > 0 {
					details = append(details, fmt.Sprintf("Size: %d bytes", check.BodySize))
				}
				if check.Error != "" {
					details = append(details, fmt.Sprintf("Error: %s", check.Error))
				}

				str.WriteString(fmt.Sprintf("    %s %s - %s\n",
					checkStatus,
					displayURL,
					strings.Join(details, ", ")))
			}
		}
		str.WriteString("\n")
	}

	// If we have fewer active checks than concurrency, show pending slots
	if len(activeList) < m.activeJobs {
		remainingSlots := m.activeJobs - len(activeList)
		waitingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			PaddingLeft(2)
		for i := 0; i < remainingSlots; i++ {
			str.WriteString(waitingStyle.Render("Waiting for next proxy...") + "\n")
		}
	}

	// Successful Proxies Section
	str.WriteString("\nSuccessful Proxies:\n")
	successBlock := strings.Builder{}
	successCount := 0
	for _, result := range m.results {
		if result.Working {
			successCount++
			hostPort := result.Proxy
			proxyType := "http"
			if strings.HasPrefix(strings.ToLower(hostPort), "socks4://") {
				proxyType = "socks4"
			} else if strings.HasPrefix(strings.ToLower(hostPort), "socks5://") {
				proxyType = "socks5"
			} else if strings.HasPrefix(strings.ToLower(hostPort), "https://") {
				proxyType = "https"
			}

			if strings.Contains(hostPort, "://") {
				hostPort = strings.Split(hostPort, "://")[1]
			}

			// Calculate success rate for this proxy
			successfulChecks := 0
			totalChecks := len(result.CheckResults)
			for _, check := range result.CheckResults {
				if check.Success {
					successfulChecks++
				}
			}

			// Format the details
			details := []string{
				fmt.Sprintf("type=%s", proxyType),
				fmt.Sprintf("checks=%d/%d", successfulChecks, totalChecks),
				fmt.Sprintf("speed=%v", result.Speed.Round(time.Millisecond)),
			}
			if result.IsAnonymous {
				details = append(details, "anonymous=true")
			}
			if result.CloudProvider != "" {
				details = append(details, fmt.Sprintf("cloud=%s", result.CloudProvider))
			}

			successBlock.WriteString(fmt.Sprintf("%s %s [%s]\n",
				successStyle.Render("✓"),
				hostPort,
				strings.Join(details, ", ")))
		}
	}
	if successCount > 0 {
		str.WriteString(successStyle.Render(fmt.Sprintf("Found %d working proxies:\n", successCount)))
		str.WriteString(successBlock.String())
	} else {
		str.WriteString(infoStyle.Render("No working proxies found yet\n"))
	}

	// Debug Information Section
	if m.debugInfo != "" {
		str.WriteString("\nDebug Information:\n")
		str.WriteString(debugBlockStyle.Render(m.debugInfo) + "\n")
	}

	// Controls
	str.WriteString("\n" + infoStyle.Render("Press q to quit"))

	return str.String()
}

func (m Model) debugView() string {
	str := strings.Builder{}

	// Title
	str.WriteString(headerStyle.Render("ProxyCheck Progress (Debug Mode)") + "\n\n")

	// Basic progress information
	progressBlock := strings.Builder{}
	progressBlock.WriteString(fmt.Sprintf("%s %s/%s\n",
		metricLabelStyle.Render("Progress:"),
		metricValueStyle.Render(fmt.Sprintf("%d", m.current)),
		metricValueStyle.Render(fmt.Sprintf("%d", m.total))))
	progressBlock.WriteString(m.progress.View())
	str.WriteString(progressStyle.Render(progressBlock.String()) + "\n")

	// Current Checks Section with detailed debug information
	str.WriteString("\nCurrent Checks:\n")

	type proxyStatus struct {
		proxy  string
		status *CheckStatus
	}
	var activeList []proxyStatus
	positions := make(map[int]proxyStatus)

	// First, collect all active checks and their positions
	for proxy, status := range m.activeChecks {
		if status.IsActive && time.Since(status.LastUpdate) < 5*time.Second {
			positions[status.Position] = proxyStatus{proxy, status}
		}
	}

	// Create a sorted list based on positions
	maxPosition := -1
	for pos := range positions {
		if pos > maxPosition {
			maxPosition = pos
		}
	}

	// Fill the list in order of positions
	for i := 0; i <= maxPosition; i++ {
		if ps, exists := positions[i]; exists {
			activeList = append(activeList, ps)
		}
	}

	// Display active checks in their fixed positions with debug information
	for _, ps := range activeList {
		proxy, status := ps.proxy, ps.status
		hostPort := proxy
		if strings.Contains(hostPort, "://") {
			hostPort = strings.Split(hostPort, "://")[1]
		}

		spinner := getSpinnerFrame(m.spinnerIdx)
		successCount := 0
		for _, check := range status.CheckResults {
			if check.Success {
				successCount++
			}
		}

		// Main status line
		statusLine := fmt.Sprintf("%s %s [%s] (%d/%d checks)\n",
			lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(spinner),
			lipgloss.NewStyle().Bold(true).Render(hostPort),
			successStyle.Render(status.ProxyType),
			successCount,
			status.TotalChecks)
		str.WriteString(statusLine)

		// Add detailed check results with debug information
		for _, check := range status.CheckResults {
			checkStatus := successStyle.Render("✓")
			if !check.Success {
				checkStatus = errorStyle.Render("✗")
			}

			// Debug information for each check
			str.WriteString(fmt.Sprintf("  %s %s\n", checkStatus, check.URL))
			str.WriteString(fmt.Sprintf("    Status Code: %d\n", check.StatusCode))
			str.WriteString(fmt.Sprintf("    Response Size: %d bytes\n", check.BodySize))
			str.WriteString(fmt.Sprintf("    Response Time: %v\n", check.Speed.Round(time.Millisecond)))

			if check.Error != "" {
				str.WriteString(fmt.Sprintf("    Error: %s\n", check.Error))
			}
		}

		// Add request/response debug info if available
		if status.LastUpdate.IsZero() {
			str.WriteString("    No requests made yet\n")
		} else {
			str.WriteString(fmt.Sprintf("    Last Update: %v ago\n",
				time.Since(status.LastUpdate).Round(time.Millisecond)))
		}

		str.WriteString("\n")
	}

	// If we have fewer active checks than concurrency, show pending slots
	if len(activeList) < m.activeJobs {
		remainingSlots := m.activeJobs - len(activeList)
		waitingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			PaddingLeft(2)
		for i := 0; i < remainingSlots; i++ {
			str.WriteString(waitingStyle.Render("Waiting for next proxy...") + "\n")
		}
	}

	// Debug Information Section
	if m.debugInfo != "" {
		str.WriteString("\nDebug Information:\n")
		str.WriteString(debugBlockStyle.Render(m.debugInfo) + "\n")
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

func checkProxiesWithProgress(proxies []string, singleURL string, useInteractsh, useIPInfo, useCloud, verbose, debug bool, timeout time.Duration, concurrency int) tea.Model {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	model := Model{
		progress:      p,
		total:         len(proxies),
		results:       make([]ProxyResultOutput, 0, len(proxies)),
		activeChecks:  make(map[string]*CheckStatus),
		useInteractsh: useInteractsh,
		useCloud:      useCloud,
		verbose:       verbose,
		activeJobs:    concurrency,
		debug:         debug,
	}

	// Store any warnings from proxy loading
	for _, proxy := range proxies {
		if strings.HasPrefix(proxy, "socks") && !strings.HasPrefix(proxy, "socks4") && !strings.HasPrefix(proxy, "socks5") {
			model.warnings = append(model.warnings, fmt.Sprintf("Warning: unsupported SOCKS version for proxy '%s' (only SOCKS4 and SOCKS5 are supported)", proxy))
		}
	}

	go func() {
		// Create buffered channels
		proxyQueue := make(chan string, concurrency)
		updateQueue := make(chan progressMsg, concurrency)
		var wg sync.WaitGroup

		// Track metrics and position
		var (
			mu           sync.Mutex
			successCount int64
			totalTime    time.Duration
			completed    int32
			nextPosition int
		)

		// Start update aggregator
		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			var lastUpdate progressMsg
			for {
				select {
				case update := <-updateQueue:
					lastUpdate = update
				case <-ticker.C:
					if lastUpdate.current > 0 || len(model.activeChecks) > 0 {
						program.Send(lastUpdate)
					}
				}
			}
		}()

		// Start workers
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for proxy := range proxyQueue {
					start := time.Now()

					// Determine proxy type
					proxyType := "http"
					if strings.HasPrefix(strings.ToLower(proxy), "socks4://") {
						proxyType = "socks4"
					} else if strings.HasPrefix(strings.ToLower(proxy), "socks5://") {
						proxyType = "socks5"
					} else if strings.HasPrefix(strings.ToLower(proxy), "https://") {
						proxyType = "https"
					}

					// Initialize check status for this proxy
					mu.Lock()
					position := nextPosition
					nextPosition++
					status := &CheckStatus{
						Proxy:        proxy,
						TotalChecks:  len(config.TestURLs.URLs),
						CheckResults: make([]CheckResult, 0),
						LastUpdate:   time.Now(),
						IsActive:     true,
						ProxyType:    proxyType,
						Position:     position,
					}
					model.activeChecks[proxy] = status
					mu.Unlock()

					// Send initial status update
					updateQueue <- progressMsg{
						current:    int(atomic.LoadInt32(&completed)),
						result:     ProxyResultOutput{Proxy: proxy},
						activeJobs: model.activeJobs,
						queueSize:  len(proxyQueue),
					}

					// Perform all checks for this proxy
					result := ProxyResult{
						URL:          proxy,
						Working:      false,
						CheckResults: make([]CheckResult, 0),
					}

					// Create proxy client
					proxyURL, err := url.Parse(proxy)
					if err != nil {
						if debug {
							result.DebugInfo = fmt.Sprintf("Error parsing proxy URL: %v\n", err)
						}
						result.Error = err.Error()
					} else {
						client, err := proxylib.CreateClient(proxyURL, time.Duration(config.Timeout)*time.Second)
						if err != nil {
							if debug {
								result.DebugInfo = fmt.Sprintf("Error creating proxy client: %v\n", err)
							}
							result.Error = err.Error()
						} else {
							// Test all configured URLs
							successfulChecks := 0

							for _, testURL := range config.TestURLs.URLs {
								checkStart := time.Now()
								if debug {
									result.DebugInfo += fmt.Sprintf("\nTesting URL: %s\n", testURL.URL)
								}

								req, err := http.NewRequest("GET", testURL.URL, nil)
								if err != nil {
									if debug {
										result.DebugInfo += fmt.Sprintf("Failed to create request: %v\n", err)
									}
									result.CheckResults = append(result.CheckResults, CheckResult{
										URL:     testURL.URL,
										Success: false,
										Speed:   time.Since(checkStart),
										Error:   fmt.Sprintf("Failed to create request: %v", err),
									})
									continue
								}

								// Add configured headers
								req.Header.Set("User-Agent", config.UserAgent)
								for key, value := range config.DefaultHeaders {
									req.Header.Set(key, value)
								}

								resp, err := client.Do(req)
								if err != nil {
									if debug {
										result.DebugInfo += fmt.Sprintf("Request failed: %v\n", err)
									}
									result.CheckResults = append(result.CheckResults, CheckResult{
										URL:     testURL.URL,
										Success: false,
										Speed:   time.Since(checkStart),
										Error:   fmt.Sprintf("Request failed: %v", err),
									})
									continue
								}

								body, err := io.ReadAll(resp.Body)
								resp.Body.Close()

								if err != nil {
									if debug {
										result.DebugInfo += fmt.Sprintf("Failed to read response body: %v\n", err)
									}
									result.CheckResults = append(result.CheckResults, CheckResult{
										URL:        testURL.URL,
										Success:    false,
										Speed:      time.Since(checkStart),
										Error:      fmt.Sprintf("Failed to read response: %v", err),
										StatusCode: resp.StatusCode,
									})
									continue
								}

								// Validate response
								valid, validationDebug := proxylib.ValidateResponse(resp, body, &proxylib.Config{
									MinResponseBytes:   config.Validation.MinResponseBytes,
									DisallowedKeywords: config.Validation.DisallowedKeywords,
								}, debug)

								if debug {
									result.DebugInfo += validationDebug
								}

								checkResult := CheckResult{
									URL:        testURL.URL,
									Success:    valid,
									Speed:      time.Since(checkStart),
									StatusCode: resp.StatusCode,
									BodySize:   int64(len(body)),
								}

								if !valid {
									checkResult.Error = "Response validation failed"
								} else {
									successfulChecks++
								}

								result.CheckResults = append(result.CheckResults, checkResult)
							}

							// Only mark as working if we met the minimum required successful checks
							if successfulChecks >= config.TestURLs.RequiredSuccessCount {
								result.Working = true
							}

							result.Speed = time.Since(start)
							if debug {
								result.DebugInfo += fmt.Sprintf("\nTotal Time: %v\n", result.Speed)
								result.DebugInfo += fmt.Sprintf("Successful Checks: %d/%d\n", successfulChecks, len(config.TestURLs.URLs))
							}
						}
					}

					mu.Lock()
					if result.Working {
						successCount++
						totalTime += result.Speed
					}

					// Update check status with results
					if status, exists := model.activeChecks[proxy]; exists {
						status.CheckResults = result.CheckResults
						status.Speed = result.Speed
						status.LastUpdate = time.Now()
						status.DoneChecks = len(result.CheckResults)
						if status.DoneChecks >= status.TotalChecks {
							status.IsActive = false
						}
					}

					// Store final result
					model.results = append(model.results, ProxyResultOutput{
						Proxy:          result.URL,
						Working:        result.Working,
						Speed:          result.Speed,
						Error:          result.Error,
						CloudProvider:  result.CloudProvider,
						InternalAccess: result.InternalAccess,
						MetadataAccess: result.MetadataAccess,
						Timestamp:      time.Now(),
						CheckResults:   result.CheckResults,
					})
					mu.Unlock()

					// Update progress
					current := atomic.AddInt32(&completed, 1)

					// Send final progress update for this proxy
					updateQueue <- progressMsg{
						current:     int(current),
						result:      model.results[len(model.results)-1],
						activeJobs:  model.activeJobs,
						queueSize:   len(proxyQueue),
						successRate: float64(successCount) / float64(current) * 100,
						avgSpeed:    totalTime / time.Duration(max(successCount, 1)),
						debugInfo:   result.DebugInfo,
					}
				}
			}()
		}

		// Feed proxies to workers
		for _, proxy := range proxies {
			proxyQueue <- proxy
		}
		close(proxyQueue)

		// Wait for all workers to finish
		wg.Wait()

		// Update active jobs to 0 when complete
		model.activeJobs = 0

		// Send quit message
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
	Timeout              int                        `yaml:"timeout"`
	InsecureSkipVerify   bool                       `yaml:"insecure_skip_verify"`
	UserAgent            string                     `yaml:"user_agent"`
	DefaultHeaders       map[string]string          `yaml:"default_headers"`
	CloudProviders       []cloudcheck.CloudProvider `yaml:"cloud_providers"`
	EnableCloudChecks    bool                       `yaml:"enable_cloud_checks"`
	EnableAnonymityCheck bool                       `yaml:"enable_anonymity_check"`
	TestURLs             TestURLConfig              `yaml:"test_urls"`
	Validation           struct {
		RequireStatusCode   int      `yaml:"require_status_code"`
		RequireContentMatch string   `yaml:"require_content_match"`
		RequireHeaderFields []string `yaml:"require_header_fields"`
		DisallowedKeywords  []string `yaml:"disallowed_keywords"`
		MinResponseBytes    int      `yaml:"min_response_bytes"`
		AdvancedChecks      struct {
			TestProtocolSmuggling   bool     `yaml:"test_protocol_smuggling"`
			TestDNSRebinding        bool     `yaml:"test_dns_rebinding"`
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
				DefaultURL:           "", // Will be loaded from config file
				RequiredSuccessCount: 1,
				URLs:                 []TestURL{}, // Will be loaded from config file
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
	Country string `json:"country"`
	ISP     string `json:"isp"`
}

type ProxyResult struct {
	URL            string               `json:"url"`
	IP             string               `json:"ip"`
	Country        string               `json:"country"`
	ISP            string               `json:"isp"`
	Working        bool                 `json:"working"`
	Error          string               `json:"error,omitempty"`
	CloudProvider  string               `json:"cloud_provider,omitempty"`
	InternalAccess bool                 `json:"internal_access"`
	MetadataAccess bool                 `json:"metadata_access"`
	DebugInfo      string               `json:"debug_info,omitempty"`
	Speed          time.Duration        `json:"speed_ns"`
	CheckResults   []CheckResult        `json:"check_results,omitempty"`
	IsAnonymous    bool                 `json:"is_anonymous"`
	RealIP         string               `json:"real_ip,omitempty"`
	ProxyIP        string               `json:"proxy_ip,omitempty"`
	AdvancedChecks *AdvancedCheckResult `json:"advanced_checks,omitempty"`
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

func getIPInfo(client *http.Client) (IPInfoResponse, error) {
	var info IPInfoResponse

	resp, err := client.Get("https://ipinfo.io/json")
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	err = json.NewDecoder(resp.Body).Decode(&info)
	if err != nil {
		return info, err
	}

	return info, nil
}

// Add additional validation functions
func checkProxyAnonymity(client *http.Client, debug bool) (bool, string, string, error) {
	// Get real IP first
	realIP, err := getIPInfo(http.DefaultClient)
	if err != nil {
		return false, "", "", err
	}

	// Get IP through proxy
	proxyIP, err := getIPInfo(client)
	if err != nil {
		return false, "", "", err
	}

	return realIP.IP != proxyIP.IP, realIP.IP, proxyIP.IP, nil
}

func checkCloudProvider(client *http.Client, proxy string, debug bool) (string, bool, bool, error) {
	for _, provider := range config.CloudProviders {
		// Check metadata access
		for _, metadataURL := range provider.MetadataURLs {
			req, err := http.NewRequest("GET", metadataURL, nil)
			if err != nil {
				continue
			}
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return provider.Name, true, true, nil
				}
			}
		}

		// Check internal ranges
		for _, ipRange := range provider.InternalRanges {
			// Try to access internal IP
			internalURL := fmt.Sprintf("http://%s/", ipRange)
			req, err := http.NewRequest("GET", internalURL, nil)
			if err != nil {
				continue
			}
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return provider.Name, true, false, nil
				}
			}
		}
	}

	return "", false, false, nil
}

func performAdvancedChecks(client *http.Client, proxy string, debug bool) *AdvancedCheckResult {
	result := &AdvancedCheckResult{
		MethodSupport:    make(map[string]bool),
		NonStandardPorts: make(map[int]bool),
		VulnDetails:      make(map[string]string),
	}

	// Test HTTP methods
	if config.Validation.AdvancedChecks.TestHTTPMethods != nil {
		for _, method := range config.Validation.AdvancedChecks.TestHTTPMethods {
			req, err := http.NewRequest(method, config.TestURLs.DefaultURL, nil)
			if err != nil {
				continue
			}
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				result.MethodSupport[method] = resp.StatusCode < 400
			}
		}
	}

	// Test path traversal if enabled
	if config.Validation.AdvancedChecks.TestPathTraversal {
		traversalPaths := []string{
			"../../../etc/passwd",
			"..%2f..%2f..%2fetc%2fpasswd",
			"%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd",
		}

		for _, path := range traversalPaths {
			testURL := config.TestURLs.DefaultURL + path
			req, err := http.NewRequest("GET", testURL, nil)
			if err != nil {
				continue
			}
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					result.PathTraversal = true
					result.VulnDetails["path_traversal"] = "Possible path traversal vulnerability detected"
					break
				}
			}
		}
	}

	// Test IPv6 support if enabled
	if config.Validation.AdvancedChecks.TestIPv6 {
		// Try to resolve proxy host to IPv6
		proxyURL, err := url.Parse(proxy)
		if err == nil {
			host := proxyURL.Hostname()
			ips, err := net.LookupIP(host)
			if err == nil {
				for _, ip := range ips {
					if ip.To4() == nil {
						result.IPv6Supported = true
						break
					}
				}
			}
		}
	}

	return result
}

// Update the checkProxyWithInteractsh function to include new checks
func checkProxyWithInteractsh(proxyURL string, debug bool) ProxyResult {
	result := ProxyResult{
		URL:          proxyURL,
		Working:      false,
		CheckResults: make([]CheckResult, 0),
	}

	start := time.Now()

	// Create proxy client
	proxyURLParsed, err := url.Parse(proxyURL)
	if err != nil {
		result.Error = err.Error()
		if debug {
			result.DebugInfo = fmt.Sprintf("Error parsing proxy URL: %v\n", err)
		}
		return result
	}

	client, err := proxylib.CreateClient(proxyURLParsed, time.Duration(config.Timeout)*time.Second)
	if err != nil {
		result.Error = err.Error()
		if debug {
			result.DebugInfo = fmt.Sprintf("Error creating proxy client: %v\n", err)
		}
		return result
	}

	// Test all configured URLs
	successfulChecks := 0

	// If no URLs are configured but we have a default URL, use it
	if len(config.TestURLs.URLs) == 0 && config.TestURLs.DefaultURL != "" {
		config.TestURLs.URLs = []TestURL{
			{
				URL:         config.TestURLs.DefaultURL,
				Description: "Default test URL",
				Required:    true,
			},
		}
	} else if len(config.TestURLs.URLs) == 0 {
		result.Error = "No test URLs configured"
		if debug {
			result.DebugInfo = "Error: No test URLs configured\n"
		}
		return result
	}

	// Test each configured URL
	for _, testURL := range config.TestURLs.URLs {
		checkStart := time.Now()
		if debug {
			result.DebugInfo += fmt.Sprintf("\nTesting URL: %s\n", testURL.URL)
		}

		req, err := http.NewRequest("GET", testURL.URL, nil)
		if err != nil {
			if debug {
				result.DebugInfo += fmt.Sprintf("Failed to create request: %v\n", err)
			}
			result.CheckResults = append(result.CheckResults, CheckResult{
				URL:     testURL.URL,
				Success: false,
				Speed:   time.Since(checkStart),
				Error:   fmt.Sprintf("Failed to create request: %v", err),
			})
			continue
		}

		// Add configured headers
		req.Header.Set("User-Agent", config.UserAgent)
		for key, value := range config.DefaultHeaders {
			req.Header.Set(key, value)
		}

		if debug {
			result.DebugInfo += "Request Headers:\n"
			for key, values := range req.Header {
				result.DebugInfo += fmt.Sprintf("  %s: %s\n", key, values)
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			if debug {
				result.DebugInfo += fmt.Sprintf("Request failed: %v\n", err)
			}
			result.CheckResults = append(result.CheckResults, CheckResult{
				URL:     testURL.URL,
				Success: false,
				Speed:   time.Since(checkStart),
				Error:   fmt.Sprintf("Request failed: %v", err),
			})
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			if debug {
				result.DebugInfo += fmt.Sprintf("Failed to read response body: %v\n", err)
			}
			result.CheckResults = append(result.CheckResults, CheckResult{
				URL:        testURL.URL,
				Success:    false,
				Speed:      time.Since(checkStart),
				Error:      fmt.Sprintf("Failed to read response: %v", err),
				StatusCode: resp.StatusCode,
			})
			continue
		}

		if debug {
			result.DebugInfo += fmt.Sprintf("Response Status: %s\n", resp.Status)
			result.DebugInfo += "Response Headers:\n"
			for key, values := range resp.Header {
				result.DebugInfo += fmt.Sprintf("  %s: %s\n", key, values)
			}
			result.DebugInfo += fmt.Sprintf("Response Body Size: %d bytes\n", len(body))
			if len(body) > 200 {
				result.DebugInfo += fmt.Sprintf("Response Body Preview: %s...\n", body[:200])
			} else {
				result.DebugInfo += fmt.Sprintf("Response Body: %s\n", body)
			}
		}

		// Validate response
		valid, validationDebug := proxylib.ValidateResponse(resp, body, &proxylib.Config{
			MinResponseBytes:   config.Validation.MinResponseBytes,
			DisallowedKeywords: config.Validation.DisallowedKeywords,
		}, debug)

		if debug {
			result.DebugInfo += fmt.Sprintf("\nValidation Results for %s:\n%s\n", testURL.URL, validationDebug)
		}

		checkResult := CheckResult{
			URL:        testURL.URL,
			Success:    valid,
			Speed:      time.Since(checkStart),
			StatusCode: resp.StatusCode,
			BodySize:   int64(len(body)),
		}

		if !valid {
			checkResult.Error = "Response validation failed"
		} else {
			successfulChecks++
		}

		result.CheckResults = append(result.CheckResults, checkResult)
	}

	// Only mark as working if we met the minimum required successful checks
	// This happens after all checks are completed
	if successfulChecks >= config.TestURLs.RequiredSuccessCount {
		result.Working = true
	}

	result.Speed = time.Since(start)
	if debug {
		result.DebugInfo += fmt.Sprintf("\nTotal Time: %v\n", result.Speed)
		result.DebugInfo += fmt.Sprintf("Successful Checks: %d/%d\n", successfulChecks, len(config.TestURLs.URLs))
	}

	// Add anonymity check
	isAnonymous := false
	var realIP, proxyIP string
	if config.EnableAnonymityCheck {
		anonymous, rIP, pIP, err := checkProxyAnonymity(client, debug)
		if err == nil {
			isAnonymous = anonymous
			realIP = rIP
			proxyIP = pIP
		}
	}

	// Add cloud provider check
	var cloudProvider string
	var internalAccess, metadataAccess bool
	if config.EnableCloudChecks {
		provider, internal, metadata, err := checkCloudProvider(client, proxyURL, debug)
		if err == nil {
			cloudProvider = provider
			internalAccess = internal
			metadataAccess = metadata
		}
	}

	// Add advanced security checks
	var advancedChecks *AdvancedCheckResult
	if config.Validation.AdvancedChecks.TestProtocolSmuggling ||
		config.Validation.AdvancedChecks.TestDNSRebinding ||
		config.Validation.AdvancedChecks.TestIPv6 ||
		len(config.Validation.AdvancedChecks.TestHTTPMethods) > 0 ||
		config.Validation.AdvancedChecks.TestPathTraversal ||
		config.Validation.AdvancedChecks.TestCachePoisoning ||
		config.Validation.AdvancedChecks.TestHostHeaderInjection {
		advancedChecks = performAdvancedChecks(client, proxyURL, debug)
	}

	// Update result with new check information
	result.IsAnonymous = isAnonymous
	result.RealIP = realIP
	result.ProxyIP = proxyIP
	result.CloudProvider = cloudProvider
	result.InternalAccess = internalAccess
	result.MetadataAccess = metadataAccess
	result.AdvancedChecks = advancedChecks

	return result
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
		// Write main proxy line with status
		status := "✓"
		if !result.Working {
			status = "✗"
		}

		// Calculate check summary
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
			checkStatus := "✓"
			if !check.Success {
				checkStatus = "✗"
			}
			details := fmt.Sprintf("Status: %d, Speed: %v", check.StatusCode, check.Speed.Round(time.Millisecond))
			if check.Error != "" {
				details = fmt.Sprintf("Error: %s", check.Error)
			}
			fmt.Fprintf(w, "  %s %s - %s\n", checkStatus, check.URL, details)
		}

		// Add additional details if available
		if result.RealIP != "" {
			fmt.Fprintf(w, "  Real IP: %s\n", result.RealIP)
		}
		if result.ProxyIP != "" {
			fmt.Fprintf(w, "  Proxy IP: %s\n", result.ProxyIP)
		}
		if result.CloudProvider != "" {
			fmt.Fprintf(w, "  Cloud Provider: %s\n", result.CloudProvider)
		}
		if result.Error != "" {
			fmt.Fprintf(w, "  Error: %s\n", result.Error)
		}
		fmt.Fprintln(w) // Add blank line between proxies
	}

	// Write summary with more details
	fmt.Fprintln(w, "\nSummary:")
	fmt.Fprintf(w, "Total proxies checked: %d\n", summary.TotalProxies)
	fmt.Fprintf(w, "Working proxies: %d\n", summary.WorkingProxies)
	fmt.Fprintf(w, "Failed proxies: %d\n", summary.TotalProxies-summary.WorkingProxies)
	if summary.InteractshProxies > 0 {
		fmt.Fprintf(w, "Interactsh proxies: %d\n", summary.InteractshProxies)
	}
	if summary.AnonymousProxies > 0 {
		fmt.Fprintf(w, "Anonymous proxies: %d\n", summary.AnonymousProxies)
	}
	if summary.CloudProxies > 0 {
		fmt.Fprintf(w, "Cloud provider proxies: %d\n", summary.CloudProxies)
	}
	if summary.InternalAccessCount > 0 {
		fmt.Fprintf(w, "Internal access capable: %d\n", summary.InternalAccessCount)
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

// Helper function to get status indicator
func getStatusIndicator(status *CheckStatus) string {
	successCount := 0
	for _, check := range status.CheckResults {
		if check.Success {
			successCount++
		}
	}

	if len(status.CheckResults) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render("CHECKING")
	} else if successCount == 0 && len(status.CheckResults) == len(config.TestURLs.URLs) {
		return errorStyle.Render("FAILED")
	} else if successCount == len(config.TestURLs.URLs) {
		return successStyle.Render("SUCCESS")
	} else if len(status.CheckResults) == len(config.TestURLs.URLs) {
		return warningStyle.Render("PARTIAL")
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render("CHECKING")
}

// Helper function to get check count with appropriate styling
func getCheckCount(status *CheckStatus) string {
	successCount := 0
	for _, check := range status.CheckResults {
		if check.Success {
			successCount++
		}
	}

	style := errorStyle
	if successCount > 0 {
		if successCount == len(config.TestURLs.URLs) {
			style = successStyle
		} else {
			style = warningStyle
		}
	}
	return style.Render(fmt.Sprintf("%d/%d", successCount, len(config.TestURLs.URLs)))
}

// Add the main function at the end of the file
func main() {
	// Parse command line flags
	proxyList := flag.String("l", "", "File containing list of proxies")
	verbose := flag.Bool("v", false, "Enable verbose output with detailed progress")
	debug := flag.Bool("d", false, "Enable debug mode with request/response data")
	concurrency := flag.Int("c", 10, "Number of concurrent checks")
	timeout := flag.Int("t", 10, "Timeout in seconds")
	outputFile := flag.String("o", "", "Output file for results (text format)")
	workingProxiesFile := flag.String("wp", "", "Output file for working proxies")
	jsonOutputFile := flag.String("j", "", "Output file for results (JSON format)")
	configFile := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	// Load configuration
	if err := loadConfig(*configFile); err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
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

	// Create and start the bubble tea program
	p := tea.NewProgram(checkProxiesWithProgress(
		proxies,
		"",
		false,
		false,
		false,
		*verbose,
		*debug, // Pass debug flag
		time.Duration(*timeout)*time.Second,
		*concurrency,
	))
	program = p

	// Run the program
	model, err := p.Run()
	if err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	// Get results from model
	m := model.(Model)
	if m.err != nil {
		fmt.Printf("Error during execution: %v\n", m.err)
		os.Exit(1)
	}

	// Calculate summary
	var summary SummaryOutput
	summary.TotalProxies = len(proxies)
	for _, result := range m.results {
		if result.Working {
			summary.WorkingProxies++
		}
		if result.InteractshTest {
			summary.InteractshProxies++
		}
		if result.IsAnonymous {
			summary.AnonymousProxies++
		}
		if result.CloudProvider != "" {
			summary.CloudProxies++
		}
		if result.InternalAccess {
			summary.InternalAccessCount++
		}
	}
	if summary.TotalProxies > 0 {
		summary.SuccessRate = float64(summary.WorkingProxies) / float64(summary.TotalProxies) * 100
	}
	summary.Results = m.results

	// Write output files if specified
	if *outputFile != "" {
		if err := writeTextOutput(*outputFile, m.results, summary); err != nil {
			fmt.Printf("Error writing text output: %v\n", err)
		}
	}

	if *workingProxiesFile != "" {
		if err := writeWorkingProxiesOutput(*workingProxiesFile, m.results); err != nil {
			fmt.Printf("Error writing working proxies: %v\n", err)
		}
	}

	if *jsonOutputFile != "" {
		jsonData, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			fmt.Printf("Error creating JSON output: %v\n", err)
		} else {
			if err := os.WriteFile(*jsonOutputFile, jsonData, 0644); err != nil {
				fmt.Printf("Error writing JSON output: %v\n", err)
			}
		}
	}
}
