package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

// ViewMode represents the display mode
type ViewMode int

const (
	ModeDefault ViewMode = iota
	ModeVerbose
	ModeDebug
)

// Component represents a renderable UI component
type Component interface {
	Render() string
}

// ProgressComponent handles progress display
type ProgressComponent struct {
	Progress progress.Model
	Current  int
	Total    int
	Layout   LayoutConfig
}

func (p *ProgressComponent) Render() string {
	if p == nil {
		return ErrorStyle.Render("Progress component is nil")
	}

	var builder strings.Builder

	// Ensure we have valid progress values
	current := p.Current
	total := p.Total
	if current < 0 {
		current = 0
	}
	if total < 0 {
		total = 0
	}
	if current > total && total > 0 {
		current = total
	}

	builder.WriteString(fmt.Sprintf("%s %s/%s\n",
		MetricLabelStyle.Render("Progress:"),
		MetricValueStyle.Render(fmt.Sprintf("%d", current)),
		MetricValueStyle.Render(fmt.Sprintf("%d", total))))

	// Only show progress bar if we have valid progress model
	if p.Progress.Percent() >= 0 {
		builder.WriteString(p.Progress.View())
	} else {
		builder.WriteString(InfoStyle.Render("Calculating progress..."))
	}

	// Apply layout-aware styling
	style := ProgressStyle
	if p.Layout.ContentWidth > 0 {
		style = ApplyLayoutToStyle(style, p.Layout)
	}

	return style.Render(builder.String())
}

// MetricsComponent displays key metrics
type MetricsComponent struct {
	ActiveJobs   int
	QueueSize    int
	SuccessRate  float64
	AvgSpeed     time.Duration
	WorkingCount int
	TotalCount   int
}

func (m *MetricsComponent) Render() string {
	if m == nil {
		return ErrorStyle.Render("Metrics component is nil")
	}

	var builder strings.Builder

	// Ensure non-negative values
	activeJobs := m.ActiveJobs
	if activeJobs < 0 {
		activeJobs = 0
	}

	queueSize := m.QueueSize
	if queueSize < 0 {
		queueSize = 0
	}

	workingCount := m.WorkingCount
	if workingCount < 0 {
		workingCount = 0
	}

	totalCount := m.TotalCount
	if totalCount < 0 {
		totalCount = 0
	}

	successRate := m.SuccessRate
	if successRate < 0 {
		successRate = 0
	} else if successRate > 100 {
		successRate = 100
	}

	builder.WriteString(fmt.Sprintf("%s %d\n",
		MetricLabelStyle.Render("Active Jobs:"),
		activeJobs))

	builder.WriteString(fmt.Sprintf("%s %d\n",
		MetricLabelStyle.Render("Queue Size:"),
		queueSize))

	builder.WriteString(fmt.Sprintf("%s %s/%d (%.1f%%)\n",
		MetricLabelStyle.Render("Working:"),
		SuccessStyle.Render(fmt.Sprintf("%d", workingCount)),
		totalCount,
		successRate))

	// Handle potential negative or zero duration gracefully
	avgSpeedStr := "N/A"
	if m.AvgSpeed > 0 {
		avgSpeedStr = m.AvgSpeed.Round(time.Millisecond).String()
	}

	builder.WriteString(fmt.Sprintf("%s %v\n",
		MetricLabelStyle.Render("Avg Speed:"),
		MetricValueStyle.Render(avgSpeedStr)))

	return MetricBlockStyle.Render(builder.String())
}

// ActiveCheckItem represents a single active check
type ActiveCheckItem struct {
	Proxy        string
	Status       *CheckStatus
	SpinnerFrame string
}

func (a *ActiveCheckItem) Render(mode ViewMode) string {
	if a == nil {
		return ErrorStyle.Render("ActiveCheckItem is nil")
	}

	if a.Status == nil {
		return ErrorStyle.Render("CheckStatus is nil for proxy: " + a.Proxy)
	}

	var builder strings.Builder

	// Safely determine status
	successCount := 0
	totalResults := len(a.Status.CheckResults)
	for _, check := range a.Status.CheckResults {
		if check.Success {
			successCount++
		}
	}

	// Handle edge cases with total checks
	totalChecks := a.Status.TotalChecks
	if totalChecks < 0 {
		totalChecks = 0
	}

	isComplete := totalResults == totalChecks && totalChecks > 0
	isSuccess := successCount == totalChecks && isComplete
	isPartial := successCount > 0 && !isSuccess && isComplete

	statusIcon := GetStatusIcon(isSuccess, isPartial, isComplete)
	statusStyle := GetStatusStyle(isSuccess, isPartial, isComplete)

	// Safely format proxy URL
	displayProxy := strings.TrimSpace(a.Proxy)
	if displayProxy == "" {
		displayProxy = "Unknown proxy"
	}

	maxLen := 35
	if mode == ModeVerbose {
		maxLen = 50
	}
	if len(displayProxy) > maxLen {
		displayProxy = displayProxy[:maxLen-3] + "..."
	}

	// Main status line
	if isComplete {
		builder.WriteString(fmt.Sprintf("%s %s",
			statusStyle.Render(statusIcon),
			ProxyURLStyle.Render(displayProxy)))
	} else {
		builder.WriteString(fmt.Sprintf("%s %s",
			SpinnerStyle.Render(a.SpinnerFrame),
			ProxyURLStyle.Render(displayProxy)))
	}

	// Add proxy type
	if a.Status.ProxyType != "" {
		builder.WriteString(fmt.Sprintf(" [%s]",
			SuccessStyle.Render(a.Status.ProxyType)))
	}

	// Add protocol support indicators
	protocols := []string{}
	if a.Status.SupportsHTTP {
		protocols = append(protocols, "HTTP")
	}
	if a.Status.SupportsHTTPS {
		protocols = append(protocols, "HTTPS")
	}
	if len(protocols) > 0 {
		builder.WriteString(fmt.Sprintf(" (%s)",
			SuccessStyle.Render(strings.Join(protocols, "+"))))
	}

	// Add speed if available
	if a.Status.Speed > 0 {
		builder.WriteString(fmt.Sprintf(" %s",
			MetricValueStyle.Render(a.Status.Speed.Round(time.Millisecond).String())))
	}

	builder.WriteString("\n")

	// Add detailed info based on mode
	switch mode {
	case ModeVerbose:
		builder.WriteString(a.renderVerboseDetails())
	case ModeDebug:
		builder.WriteString(a.renderDebugDetails())
	}

	return builder.String()
}

func (a *ActiveCheckItem) renderVerboseDetails() string {
	var builder strings.Builder

	// Show individual check results
	if len(a.Status.CheckResults) > 0 {
		builder.WriteString("  Results:\n")
		for i, check := range a.Status.CheckResults {
			statusStyle := GetHTTPStatusStyle(check.StatusCode)
			statusText := GetHTTPStatusText(check.StatusCode)

			builder.WriteString(fmt.Sprintf("    %d. %s: %s",
				i+1,
				check.URL,
				statusStyle.Render(fmt.Sprintf("%d %s", check.StatusCode, statusText))))

			if check.Speed > 0 {
				builder.WriteString(fmt.Sprintf(" (%s)",
					MetricValueStyle.Render(check.Speed.Round(time.Millisecond).String())))
			}
			builder.WriteString("\n")

			if check.Error != "" {
				builder.WriteString(fmt.Sprintf("       %s\n",
					ErrorStyle.Render("Error: "+check.Error)))
			}
		}
	}

	return builder.String()
}

func (a *ActiveCheckItem) renderDebugDetails() string {
	var builder strings.Builder

	// Show compact check results
	if len(a.Status.CheckResults) > 0 {
		for _, check := range a.Status.CheckResults {
			statusStyle := GetHTTPStatusStyle(check.StatusCode)

			method := extractMethod(check.URL)

			resultLine := fmt.Sprintf("  * %s - %s",
				method,
				statusStyle.Render(fmt.Sprintf("%d %s",
					check.StatusCode,
					GetHTTPStatusText(check.StatusCode))))

			if check.Speed > 0 {
				resultLine += fmt.Sprintf(" (%s)",
					MetricValueStyle.Render(fmt.Sprintf("%dms", check.Speed.Milliseconds())))
			}

			builder.WriteString(resultLine + "\n")

			if check.Error != "" {
				builder.WriteString(fmt.Sprintf("    %s\n",
					ErrorStyle.Render(check.Error)))
			}
		}
	} else {
		builder.WriteString(fmt.Sprintf("  %s\n",
			InfoStyle.Render("No checks completed yet")))
	}

	return builder.String()
}

// ActiveChecksComponent manages the display of active proxy checks
type ActiveChecksComponent struct {
	ActiveChecks map[string]*CheckStatus
	SpinnerIdx   int
	Mode         ViewMode
}

func (ac *ActiveChecksComponent) Render() string {
	if ac == nil {
		return ErrorStyle.Render("ActiveChecks component is nil")
	}

	if ac.ActiveChecks == nil {
		return InfoStyle.Render("No active checks data available")
	}

	var builder strings.Builder

	activeList := ac.getActiveChecksList()

	// Header with counts
	activeCount := len(activeList)
	if activeCount == 0 {
		builder.WriteString(InfoStyle.Render("No active checks at the moment\n"))
		return builder.String()
	}

	// Render each active check with safe spinner index
	var spinnerFrame string
	if len(SpinnerFrames) > 0 {
		spinnerFrame = SpinnerFrames[ac.SpinnerIdx%len(SpinnerFrames)]
	} else {
		spinnerFrame = "⠋" // fallback spinner frame
	}

	for _, item := range activeList {
		if item.status == nil {
			builder.WriteString(ErrorStyle.Render("Invalid check status\n"))
			continue
		}

		checkItem := &ActiveCheckItem{
			Proxy:        item.proxy,
			Status:       item.status,
			SpinnerFrame: spinnerFrame,
		}

		if rendered := checkItem.Render(ac.Mode); rendered != "" {
			builder.WriteString(rendered)

			if ac.Mode == ModeDebug {
				builder.WriteString("\n") // Extra spacing in debug mode
			}
		}
	}

	result := builder.String()
	if result == "" {
		return InfoStyle.Render("No active checks to display")
	}

	return result
}

func (ac *ActiveChecksComponent) getActiveChecksList() []struct {
	proxy  string
	status *CheckStatus
} {
	var activeList []struct {
		proxy  string
		status *CheckStatus
	}

	for proxy, status := range ac.ActiveChecks {
		if status.IsActive && time.Since(status.LastUpdate) < 5*time.Second {
			activeList = append(activeList, struct {
				proxy  string
				status *CheckStatus
			}{proxy, status})
		}
	}

	// Sort by position or proxy name
	sort.Slice(activeList, func(i, j int) bool {
		if activeList[i].status.Position != activeList[j].status.Position {
			return activeList[i].status.Position < activeList[j].status.Position
		}
		return activeList[i].proxy < activeList[j].proxy
	})

	return activeList
}

// DebugLogComponent displays recent debug log entries
type DebugLogComponent struct {
	DebugInfo string
	MaxLines  int
}

func (d *DebugLogComponent) Render() string {
	if d.DebugInfo == "" {
		return ""
	}

	var builder strings.Builder

	builder.WriteString(HeaderStyle.Copy().
		Foreground(lipgloss.Color(ColorDebug)).
		BorderForeground(lipgloss.Color(ColorDebug)).
		Render("RECENT LOG EVENTS") + "\n\n")

	// Show only the most recent lines
	lines := strings.Split(d.DebugInfo, "\n")
	maxLines := d.MaxLines
	if maxLines <= 0 {
		maxLines = 15
	}

	start := 0
	if len(lines) > maxLines {
		start = len(lines) - maxLines
	}

	for _, line := range lines[start:] {
		if line == "" {
			continue
		}

		// Color-code based on content
		style := DebugTextStyle
		if strings.Contains(strings.ToLower(line), "success") ||
			strings.Contains(strings.ToLower(line), "working") {
			style = SuccessStyle
		} else if strings.Contains(strings.ToLower(line), "error") ||
			strings.Contains(strings.ToLower(line), "failed") {
			style = ErrorStyle
		} else if strings.Contains(strings.ToLower(line), "warning") {
			style = WarningStyle
		}

		builder.WriteString(style.Render(line) + "\n")
	}

	return builder.String()
}

// Helper functions
func extractMethod(url string) string {
	method := "REQUEST"
	urlLower := strings.ToLower(url)

	if strings.Contains(urlLower, "http://") {
		return "HTTP"
	} else if strings.Contains(urlLower, "https://") {
		return "HTTPS"
	} else if strings.Contains(urlLower, "socks4") {
		return "SOCKS4"
	} else if strings.Contains(urlLower, "socks5") {
		return "SOCKS5"
	}

	// Try to extract HTTP method from URL if it's formatted as "METHOD URL"
	urlParts := strings.SplitN(url, " ", 2)
	if len(urlParts) > 1 {
		return strings.ToUpper(urlParts[0])
	}

	return method
}
