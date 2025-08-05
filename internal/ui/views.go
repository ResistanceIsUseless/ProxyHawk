package ui

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
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

	debugHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("208")).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("208")).
				Padding(0, 1).
				Align(lipgloss.Center).
				Width(50)

	debugTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Bold(false)

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

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true)

	spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

// View represents the UI state and data needed for rendering
type View struct {
	Progress     progress.Model
	Total        int
	Current      int
	ActiveJobs   int
	QueueSize    int
	SuccessRate  float64
	AvgSpeed     time.Duration
	DebugInfo    string
	SpinnerIdx   int
	ActiveChecks map[string]*CheckStatus
	IsVerbose    bool
	IsDebug      bool
}

// CheckStatus represents the current status of a proxy check
type CheckStatus struct {
	Proxy          string
	TotalChecks    int
	DoneChecks     int
	LastUpdate     time.Time
	CheckResults   []CheckResult
	Speed          time.Duration
	IsActive       bool
	ProxyType      string
	Position       int
	CloudProvider  string
	InternalAccess bool
	MetadataAccess bool
	SupportsHTTP   bool
	SupportsHTTPS  bool
	DebugInfo      string
}

// CheckResult represents the result of a single check
type CheckResult struct {
	URL        string
	Success    bool
	Speed      time.Duration
	Error      string
	StatusCode int
	BodySize   int64
}

// RenderDefault renders the default view
func (v *View) RenderDefault() string {
	str := strings.Builder{}

	// Title
	str.WriteString(headerStyle.Render("ProxyHawk Progress") + "\n\n")

	// Progress information
	progressBlock := strings.Builder{}
	progressBlock.WriteString(fmt.Sprintf("%s %s/%s\n",
		metricLabelStyle.Render("Progress:"),
		metricValueStyle.Render(fmt.Sprintf("%d", v.Current)),
		metricValueStyle.Render(fmt.Sprintf("%d", v.Total))))
	progressBlock.WriteString(v.Progress.View())
	str.WriteString(progressStyle.Render(progressBlock.String()) + "\n")

	// Current Checks Section
	str.WriteString("\nCurrent Checks:\n")
	str.WriteString(v.renderActiveChecks())

	// Controls
	str.WriteString("\n" + infoStyle.Render("Press q to quit"))

	return str.String()
}

// RenderVerbose renders the verbose view
func (v *View) RenderVerbose() string {
	str := strings.Builder{}

	// Title
	str.WriteString(headerStyle.Render("ProxyHawk Progress (Verbose Mode)") + "\n\n")

	// Progress and metrics
	str.WriteString(v.renderProgressAndMetrics())

	// Current checks with detailed information
	str.WriteString("\nCurrent Checks:\n")
	str.WriteString(v.renderActiveChecksVerbose())

	// Controls
	str.WriteString("\n" + infoStyle.Render("Press q to quit"))

	return str.String()
}

// RenderDebug renders the debug view
func (v *View) RenderDebug() string {
	// Build the primary view
	str := strings.Builder{}

	// Title and main metrics
	str.WriteString(headerStyle.Render("ProxyHawk Debug Mode") + "\n\n")

	// Top section with key metrics
	metricsBlock := strings.Builder{}

	// Progress information
	metricsBlock.WriteString(fmt.Sprintf("%s %s/%s (%.1f%%)\n",
		metricLabelStyle.Render("Progress:"),
		metricValueStyle.Render(fmt.Sprintf("%d", v.Current)),
		metricValueStyle.Render(fmt.Sprintf("%d", v.Total)),
		float64(v.Current)/float64(math.Max(1.0, float64(v.Total)))*100))

	// Performance metrics
	metricsBlock.WriteString(fmt.Sprintf("%s %d\n",
		metricLabelStyle.Render("Concurrency:"),
		v.ActiveJobs))

	// Calculate estimated total requests
	totalChecks := 0
	for _, status := range v.ActiveChecks {
		totalChecks += status.TotalChecks
	}
	estimatedRequests := v.Total * (totalChecks / int(math.Max(1.0, float64(len(v.ActiveChecks)))))
	if estimatedRequests == 0 && v.Total > 0 {
		estimatedRequests = v.Total // Default if we can't calculate yet
	}

	metricsBlock.WriteString(fmt.Sprintf("%s %s\n",
		metricLabelStyle.Render("Est. Requests:"),
		metricValueStyle.Render(fmt.Sprintf("%d", estimatedRequests))))

	metricsBlock.WriteString(fmt.Sprintf("%s %v\n",
		metricLabelStyle.Render("Avg Speed:"),
		metricValueStyle.Render(v.AvgSpeed.Round(time.Millisecond).String())))

	str.WriteString(metricBlockStyle.Render(metricsBlock.String()) + "\n\n")

	// Progress bar
	progressBlock := strings.Builder{}
	progressBlock.WriteString(v.Progress.View())
	str.WriteString(progressStyle.Render(progressBlock.String()) + "\n\n")

	// Results summary
	summaryBlock := strings.Builder{}

	// Calculate summary statistics
	workingProxies := 0
	for _, status := range v.ActiveChecks {
		successCount := 0
		for _, check := range status.CheckResults {
			if check.Success {
				successCount++
			}
		}
		if successCount > 0 {
			workingProxies++
		}
	}

	// Count proxies by type
	proxyTypes := make(map[string]int)
	httpSupport := 0
	httpsSupport := 0

	for _, status := range v.ActiveChecks {
		if status.ProxyType != "" {
			proxyTypes[status.ProxyType]++
		}
		if status.SupportsHTTP {
			httpSupport++
		}
		if status.SupportsHTTPS {
			httpsSupport++
		}
	}

	summaryBlock.WriteString(fmt.Sprintf("%s %s\n",
		metricLabelStyle.Render("Working Proxies:"),
		successStyle.Render(fmt.Sprintf("%d/%d (%.1f%%)",
			workingProxies,
			v.Current,
			float64(workingProxies)/float64(math.Max(1.0, float64(v.Current)))*100))))

	// Protocol support
	summaryBlock.WriteString(fmt.Sprintf("%s %s, %s %s\n",
		metricLabelStyle.Render("HTTP Support:"),
		successStyle.Render(fmt.Sprintf("%d", httpSupport)),
		metricLabelStyle.Render("HTTPS Support:"),
		successStyle.Render(fmt.Sprintf("%d", httpsSupport))))

	// Proxy types breakdown
	summaryBlock.WriteString(metricLabelStyle.Render("Proxy Types:") + " ")
	if len(proxyTypes) == 0 {
		summaryBlock.WriteString(infoStyle.Render("None detected yet"))
	} else {
		typeStrings := make([]string, 0, len(proxyTypes))
		for pType, count := range proxyTypes {
			typeStrings = append(typeStrings, fmt.Sprintf("%s: %d", pType, count))
		}
		summaryBlock.WriteString(successStyle.Render(strings.Join(typeStrings, ", ")))
	}

	str.WriteString(statusBlockStyle.Render(summaryBlock.String()) + "\n\n")

	// Active checks section
	str.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("87")).
		Render("ACTIVE CHECKS") + "\n")

	str.WriteString(v.renderActiveChecksSimple())

	// Recent log events
	if v.DebugInfo != "" {
		str.WriteString("\n" + lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("208")).
			Render("RECENT LOG EVENTS") + "\n")

		// Show only the most recent 20 lines
		lines := strings.Split(v.DebugInfo, "\n")
		start := 0
		if len(lines) > 20 {
			start = len(lines) - 20
		}

		for _, line := range lines[start:] {
			if line == "" {
				continue
			}

			// Color-code based on content
			if strings.Contains(line, "Success") || strings.Contains(line, "success") {
				str.WriteString(successStyle.Render(line) + "\n")
			} else if strings.Contains(line, "error") || strings.Contains(line, "failed") ||
				strings.Contains(line, "Error") || strings.Contains(line, "Failed") {
				str.WriteString(errorStyle.Render(line) + "\n")
			} else if strings.Contains(line, "Working") || strings.Contains(line, "Supports") {
				str.WriteString(warningStyle.Render(line) + "\n")
			} else {
				str.WriteString(debugTextStyle.Render(line) + "\n")
			}
		}
	}

	// Controls at the bottom
	str.WriteString("\n" + infoStyle.Render("Press q to quit"))

	return str.String()
}

// renderActiveChecksSimple renders a simplified list of active checks
func (v *View) renderActiveChecksSimple() string {
	str := strings.Builder{}
	activeList := v.getActiveChecksList()

	if len(activeList) == 0 {
		str.WriteString(infoStyle.Render("No active checks at the moment\n"))
		return str.String()
	}

	// Display in a simpler format
	for _, ps := range activeList {
		proxy, status := ps.proxy, ps.status

		// Determine status based on results
		statusChar := "⌛"
		statusStyle := infoStyle

		if status.DoneChecks > 0 {
			successCount := 0
			for _, check := range status.CheckResults {
				if check.Success {
					successCount++
				}
			}

			if successCount == status.TotalChecks && status.TotalChecks > 0 {
				statusChar = "✓"
				statusStyle = successStyle
			} else if successCount > 0 {
				statusChar = "⚠"
				statusStyle = warningStyle
			} else {
				statusChar = "✗"
				statusStyle = errorStyle
			}
		}

		// Format the proxy URL
		displayProxy := proxy
		if len(displayProxy) > 30 {
			displayProxy = displayProxy[:27] + "..."
		}

		// Build the status line
		statusLine := fmt.Sprintf("%s %s",
			statusStyle.Render(statusChar),
			lipgloss.NewStyle().Bold(true).Render(displayProxy))

		// Add proxy type
		if status.ProxyType != "" {
			statusLine += fmt.Sprintf(" [%s]", status.ProxyType)
		}

		// Add protocol support indicators
		protocols := []string{}
		if status.SupportsHTTP {
			protocols = append(protocols, successStyle.Render("HTTP"))
		}
		if status.SupportsHTTPS {
			protocols = append(protocols, successStyle.Render("HTTPS"))
		}

		if len(protocols) > 0 {
			statusLine += fmt.Sprintf(" %s", strings.Join(protocols, "+"))
		}

		// Add speed if available
		if status.Speed > 0 {
			statusLine += fmt.Sprintf(" (%s)", status.Speed.Round(time.Millisecond))
		}

		str.WriteString(statusLine + "\n")
	}

	return str.String()
}

// Helper functions for rendering components
func (v *View) renderProgressAndMetrics() string {
	str := strings.Builder{}

	// Progress block
	progressBlock := strings.Builder{}
	progressBlock.WriteString(fmt.Sprintf("%s %s/%s\n",
		metricLabelStyle.Render("Progress:"),
		metricValueStyle.Render(fmt.Sprintf("%d", v.Current)),
		metricValueStyle.Render(fmt.Sprintf("%d", v.Total))))
	progressBlock.WriteString(v.Progress.View())
	str.WriteString(progressStyle.Render(progressBlock.String()) + "\n")

	// Metrics block
	metricsBlock := strings.Builder{}
	metricsBlock.WriteString(fmt.Sprintf("%s %d\n",
		metricLabelStyle.Render("Active Jobs:"),
		v.ActiveJobs))
	metricsBlock.WriteString(fmt.Sprintf("%s %d\n",
		metricLabelStyle.Render("Queue Size:"),
		v.QueueSize))
	metricsBlock.WriteString(fmt.Sprintf("%s %.2f%%\n",
		metricLabelStyle.Render("Success Rate:"),
		v.SuccessRate))
	metricsBlock.WriteString(fmt.Sprintf("%s %v\n",
		metricLabelStyle.Render("Average Speed:"),
		v.AvgSpeed.Round(time.Millisecond)))
	str.WriteString(metricBlockStyle.Render(metricsBlock.String()) + "\n")

	return str.String()
}

func (v *View) renderActiveChecks() string {
	str := strings.Builder{}
	activeList := v.getActiveChecksList()

	// Add a header for active checks with count
	activeCount := len(activeList)
	completedCount := v.Current
	totalCount := v.Total

	str.WriteString(fmt.Sprintf("Active Checks: %s | Completed: %s | Total: %s\n",
		metricValueStyle.Render(fmt.Sprintf("%d", activeCount)),
		metricValueStyle.Render(fmt.Sprintf("%d", completedCount)),
		metricValueStyle.Render(fmt.Sprintf("%d", totalCount))))

	if activeCount == 0 && completedCount < totalCount {
		str.WriteString(infoStyle.Render("Waiting for checks to start...\n"))
	} else if activeCount == 0 && completedCount == totalCount {
		str.WriteString(successStyle.Render("All checks completed!\n"))
	} else {
		str.WriteString("\n")
	}

	for _, ps := range activeList {
		proxy, status := ps.proxy, ps.status
		spinner := spinnerFrames[v.SpinnerIdx%len(spinnerFrames)]

		// Determine status color based on check results
		proxyStatusStyle := infoStyle
		if status.DoneChecks > 0 {
			successCount := 0
			for _, check := range status.CheckResults {
				if check.Success {
					successCount++
				}
			}

			if successCount == status.TotalChecks && status.TotalChecks > 0 {
				proxyStatusStyle = successStyle
			} else if successCount > 0 {
				proxyStatusStyle = warningStyle
			} else {
				proxyStatusStyle = errorStyle
			}
		}

		// Format proxy URL to be more readable
		displayProxy := proxy
		if len(displayProxy) > 40 {
			// Truncate long proxy URLs
			displayProxy = displayProxy[:37] + "..."
		}

		// Main status line
		str.WriteString(fmt.Sprintf("%s %s",
			lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(spinner),
			lipgloss.NewStyle().Bold(true).Render(displayProxy)))

		// Add proxy type if available
		if status.ProxyType != "" {
			str.WriteString(fmt.Sprintf(" [%s]",
				successStyle.Render(status.ProxyType)))
		}

		// Ensure we have valid check counts
		doneChecks := status.DoneChecks
		totalChecks := status.TotalChecks

		// If TotalChecks is 0 but we have results, use the length of results
		if totalChecks == 0 && len(status.CheckResults) > 0 {
			totalChecks = len(status.CheckResults)
		}

		// If TotalChecks is still 0, default to at least 1
		if totalChecks == 0 {
			totalChecks = 1
		}

		// Add check progress
		str.WriteString(fmt.Sprintf(" %s\n",
			proxyStatusStyle.Render(fmt.Sprintf("(%d/%d checks)", doneChecks, totalChecks))))

		// Add protocol support indicators if available
		if status.SupportsHTTP || status.SupportsHTTPS {
			str.WriteString("  Supports: ")

			if status.SupportsHTTP {
				str.WriteString(successStyle.Render("HTTP"))
			}

			if status.SupportsHTTP && status.SupportsHTTPS {
				str.WriteString(" + ")
			}

			if status.SupportsHTTPS {
				str.WriteString(successStyle.Render("HTTPS"))
			}

			str.WriteString("\n")
		}
	}

	return str.String()
}

func (v *View) renderActiveChecksVerbose() string {
	str := strings.Builder{}
	activeList := v.getActiveChecksList()

	// Add a header for active checks with count
	activeCount := len(activeList)
	completedCount := v.Current
	totalCount := v.Total

	str.WriteString(fmt.Sprintf("Active Checks: %s | Completed: %s | Total: %s\n",
		metricValueStyle.Render(fmt.Sprintf("%d", activeCount)),
		metricValueStyle.Render(fmt.Sprintf("%d", completedCount)),
		metricValueStyle.Render(fmt.Sprintf("%d", totalCount))))

	if activeCount == 0 && completedCount < totalCount {
		str.WriteString(infoStyle.Render("Waiting for checks to start...\n"))
	} else if activeCount == 0 && completedCount == totalCount {
		str.WriteString(successStyle.Render("All checks completed!\n"))
	} else {
		str.WriteString("\n")
	}

	for _, ps := range activeList {
		proxy, status := ps.proxy, ps.status
		spinner := spinnerFrames[v.SpinnerIdx%len(spinnerFrames)]

		// Determine status color based on check results
		proxyStatusStyle := infoStyle
		if status.DoneChecks > 0 {
			successCount := 0
			for _, check := range status.CheckResults {
				if check.Success {
					successCount++
				}
			}

			if successCount == status.TotalChecks && status.TotalChecks > 0 {
				proxyStatusStyle = successStyle
			} else if successCount > 0 {
				proxyStatusStyle = warningStyle
			} else {
				proxyStatusStyle = errorStyle
			}
		}

		// Format proxy URL to be more readable
		displayProxy := proxy
		if len(displayProxy) > 40 {
			// Truncate long proxy URLs
			displayProxy = displayProxy[:37] + "..."
		}

		// Main status line
		str.WriteString(fmt.Sprintf("%s %s",
			lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(spinner),
			lipgloss.NewStyle().Bold(true).Render(displayProxy)))

		// Add proxy type if available
		if status.ProxyType != "" {
			str.WriteString(fmt.Sprintf(" [%s]",
				successStyle.Render(status.ProxyType)))
		}

		// Ensure we have valid check counts
		doneChecks := status.DoneChecks
		totalChecks := status.TotalChecks

		// If TotalChecks is 0 but we have results, use the length of results
		if totalChecks == 0 && len(status.CheckResults) > 0 {
			totalChecks = len(status.CheckResults)
		}

		// If TotalChecks is still 0, default to at least 1
		if totalChecks == 0 {
			totalChecks = 1
		}

		// Add check progress
		str.WriteString(fmt.Sprintf(" %s\n",
			proxyStatusStyle.Render(fmt.Sprintf("(%d/%d checks)", doneChecks, totalChecks))))

		// Show check results
		if len(status.CheckResults) > 0 {
			str.WriteString("  Results:\n")
			for i, check := range status.CheckResults {
				resultStyle := errorStyle
				if check.Success {
					resultStyle = successStyle
				}

				str.WriteString(fmt.Sprintf("    %d. %s: %s",
					i+1,
					check.URL,
					resultStyle.Render(fmt.Sprintf("%d", check.StatusCode))))

				if check.Speed > 0 {
					str.WriteString(fmt.Sprintf(" (%s)",
						metricValueStyle.Render(check.Speed.Round(time.Millisecond).String())))
				}
				str.WriteString("\n")

				if check.Error != "" {
					str.WriteString(fmt.Sprintf("       Error: %s\n", errorStyle.Render(check.Error)))
				}
			}
		}

		// Display protocol support information
		str.WriteString("  Protocol Support:\n")

		httpStatus := "No"
		httpStyle := errorStyle
		if status.SupportsHTTP {
			httpStatus = "Yes"
			httpStyle = successStyle
		}

		httpsStatus := "No"
		httpsStyle := errorStyle
		if status.SupportsHTTPS {
			httpsStatus = "Yes"
			httpsStyle = successStyle
		}

		str.WriteString(fmt.Sprintf("    HTTP: %s\n", httpStyle.Render(httpStatus)))
		str.WriteString(fmt.Sprintf("    HTTPS: %s\n", httpsStyle.Render(httpsStatus)))
		str.WriteString("\n")
	}

	return str.String()
}

func (v *View) renderActiveChecksDebug() string {
	str := strings.Builder{}
	activeList := v.getActiveChecksList()

	// Add a header for active checks with count
	activeCount := len(activeList)
	completedCount := v.Current
	totalCount := v.Total

	str.WriteString(fmt.Sprintf("Active Checks: %s | Completed: %s | Total: %s\n",
		metricValueStyle.Render(fmt.Sprintf("%d", activeCount)),
		metricValueStyle.Render(fmt.Sprintf("%d", completedCount)),
		metricValueStyle.Render(fmt.Sprintf("%d", totalCount))))

	if activeCount == 0 && completedCount < totalCount {
		str.WriteString(infoStyle.Render("Waiting for checks to start...\n"))
	} else if activeCount == 0 && completedCount == totalCount {
		str.WriteString(successStyle.Render("All checks completed!\n"))
	} else {
		str.WriteString("\n")
	}

	for _, ps := range activeList {
		proxy, status := ps.proxy, ps.status
		spinner := spinnerFrames[v.SpinnerIdx%len(spinnerFrames)]

		// Determine status color based on check results
		proxyStatusStyle := errorStyle
		if status.DoneChecks > 0 {
			successCount := 0
			for _, check := range status.CheckResults {
				if check.Success {
					successCount++
				}
			}

			if successCount == status.TotalChecks && status.TotalChecks > 0 {
				proxyStatusStyle = successStyle
			} else if successCount > 0 {
				proxyStatusStyle = warningStyle
			}
		}

		// Format proxy URL to be more readable
		displayProxy := proxy
		if len(displayProxy) > 40 {
			// Truncate long proxy URLs
			displayProxy = displayProxy[:37] + "..."
		}

		// Main status line with the updated format:
		// [⠋] 127.0.0.1:443 [Checks: 42]
		str.WriteString(fmt.Sprintf("[%s] %s [Checks: %s]\n",
			lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(spinner),
			lipgloss.NewStyle().Bold(true).Render(displayProxy),
			proxyStatusStyle.Render(fmt.Sprintf("%d", status.DoneChecks))))

		// Show check results in a compact format with * prefix
		if len(status.CheckResults) > 0 {
			for _, check := range status.CheckResults {
				resultStyle := errorStyle
				if check.Success {
					resultStyle = successStyle
				}

				// Extract method (GET, POST, CONNECT, etc.) from URL if possible
				method := "REQUEST"
				urlParts := strings.SplitN(check.URL, " ", 2)
				if len(urlParts) > 1 {
					method = urlParts[0]
				} else if strings.Contains(strings.ToLower(check.URL), "http") {
					method = "HTTP"
				} else if strings.Contains(strings.ToLower(check.URL), "https") {
					method = "HTTPS"
				} else if strings.Contains(strings.ToLower(check.URL), "socks4") {
					method = "SOCKS4"
				} else if strings.Contains(strings.ToLower(check.URL), "socks5") {
					method = "SOCKS5"
				}

				// Format: * METHOD - STATUS (TIME)
				resultLine := fmt.Sprintf("* %s - %s",
					method,
					resultStyle.Render(fmt.Sprintf("%d %s",
						check.StatusCode,
						getStatusText(check.StatusCode))))

				if check.Speed > 0 {
					resultLine += fmt.Sprintf(" (%s)",
						metricValueStyle.Render(fmt.Sprintf("%dms", check.Speed.Milliseconds())))
				}
				str.WriteString(fmt.Sprintf("  %s\n", resultLine))

				if check.Error != "" {
					str.WriteString(fmt.Sprintf("    %s\n", errorStyle.Render(check.Error)))
				}
			}
		} else {
			str.WriteString(fmt.Sprintf("  %s\n", infoStyle.Render("No checks completed yet")))
		}

		// Add an extra line for spacing between proxies
		str.WriteString("\n")
	}

	return str.String()
}

// Helper function to get HTTP status text
func getStatusText(code int) string {
	switch {
	case code >= 100 && code < 200:
		return "Informational"
	case code >= 200 && code < 300:
		return "OK"
	case code >= 300 && code < 400:
		return "Redirect"
	case code == 401:
		return "Unauthorized"
	case code == 403:
		return "Forbidden"
	case code == 404:
		return "Not Found"
	case code >= 400 && code < 500:
		return "Client Error"
	case code >= 500:
		return "Server Error"
	default:
		return "Unknown"
	}
}

// Helper methods
func (v *View) getActiveChecksList() []struct {
	proxy  string
	status *CheckStatus
} {
	var activeList []struct {
		proxy  string
		status *CheckStatus
	}

	for proxy, status := range v.ActiveChecks {
		if status.IsActive && time.Since(status.LastUpdate) < 5*time.Second {
			activeList = append(activeList, struct {
				proxy  string
				status *CheckStatus
			}{proxy, status})
		}
	}

	// Sort by position
	sort.Slice(activeList, func(i, j int) bool {
		return activeList[i].status.Position < activeList[j].status.Position
	})

	return activeList
}

func (v *View) getStatusIndicator(status *CheckStatus) string {
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
	} else if successCount == 0 && len(status.CheckResults) == status.TotalChecks {
		return errorStyle.Render("FAILED")
	} else if successCount == status.TotalChecks {
		return successStyle.Render("SUCCESS")
	} else if len(status.CheckResults) == status.TotalChecks {
		return warningStyle.Render("PARTIAL")
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render("CHECKING")
}

func (v *View) getCheckCount(status *CheckStatus) string {
	successCount := 0
	for _, check := range status.CheckResults {
		if check.Success {
			successCount++
		}
	}

	style := errorStyle
	if successCount > 0 {
		if successCount == status.TotalChecks {
			style = successStyle
		} else {
			style = warningStyle
		}
	}
	return style.Render(fmt.Sprintf("%d/%d", successCount, status.TotalChecks))
}
