package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

// Component interface for all renderable UI elements
type Component interface {
	Render() string
}

// =============================================================================
// HEADER COMPONENT
// =============================================================================

// HeaderComponent displays the application header
type HeaderComponent struct {
	Title string
	Mode  ViewMode
}

func (h *HeaderComponent) Render() string {
	title := "ProxyHawk"

	switch h.Mode {
	case ModeVerbose:
		title += " • Verbose Mode"
	case ModeDebug:
		title += " • Debug Mode"
	}

	return HeaderStyle.Render(title)
}

// =============================================================================
// STATS BAR COMPONENT
// =============================================================================

// StatsBarComponent shows key metrics in a horizontal bar
type StatsBarComponent struct {
	Current     int
	Total       int
	Working     int
	Failed      int
	Active      int
	AvgSpeed    time.Duration
}

func (s *StatsBarComponent) Render() string {
	if s.Total == 0 {
		return StatsBarStyle.Render("Initializing...")
	}

	// Calculate success rate
	completed := s.Current
	successRate := 0.0
	if completed > 0 {
		successRate = float64(s.Working) / float64(completed) * 100
	}

	// Build stats items
	var items []string

	// Progress
	items = append(items, fmt.Sprintf("%s %s",
		MetricLabelStyle.Render("Progress:"),
		MetricValueStyle.Render(FormatCount(s.Current, s.Total))))

	// Working (green if good, red if bad)
	workingStyle := MetricGoodStyle
	if successRate < 50 && completed > 5 {
		workingStyle = MetricBadStyle
	}
	items = append(items, fmt.Sprintf("%s %s",
		MetricLabelStyle.Render("Working:"),
		workingStyle.Render(fmt.Sprintf("%d", s.Working))))

	// Failed
	if s.Failed > 0 {
		items = append(items, fmt.Sprintf("%s %s",
			MetricLabelStyle.Render("Failed:"),
			MetricBadStyle.Render(fmt.Sprintf("%d", s.Failed))))
	}

	// Active
	if s.Active > 0 {
		items = append(items, fmt.Sprintf("%s %s",
			MetricLabelStyle.Render("Active:"),
			ActiveStyle.Render(fmt.Sprintf("%d", s.Active))))
	}

	// Average speed
	if s.AvgSpeed > 0 {
		speedStr := fmt.Sprintf("%.0fms", float64(s.AvgSpeed.Milliseconds()))
		items = append(items, fmt.Sprintf("%s %s",
			MetricLabelStyle.Render("Avg:"),
			ProxySpeedStyle.Render(speedStr)))
	}

	// Join with separator
	content := strings.Join(items, "  •  ")
	return StatsBarStyle.Render(content)
}

// =============================================================================
// PROGRESS COMPONENT
// =============================================================================

// ProgressComponent displays a progress bar
type ProgressComponent struct {
	Progress progress.Model
	Current  int
	Total    int
}

func (p *ProgressComponent) Render() string {
	if p.Total == 0 {
		return ""
	}

	var b strings.Builder

	// Progress bar
	b.WriteString(p.Progress.View())
	b.WriteString("\n")

	// Count and percentage on same line
	percentage := FormatPercentage(p.Current, p.Total)
	count := FormatCount(p.Current, p.Total)
	b.WriteString(dimStyle.Render(count))
	b.WriteString(" ")
	b.WriteString(MetricValueStyle.Render(percentage))

	return ProgressStyle.Render(b.String())
}

// =============================================================================
// ACTIVE CHECKS COMPONENT
// =============================================================================

// ActiveChecksComponent displays currently running checks
type ActiveChecksComponent struct {
	Checks      map[string]*CheckStatus
	SpinnerIdx  int
	Mode        ViewMode
	MaxVisible  int
}

func (a *ActiveChecksComponent) Render() string {
	if len(a.Checks) == 0 {
		return ChecksSectionStyle.Render(
			dimStyle.Render("No active checks"))
	}

	// Get active checks sorted by position
	active := a.getActiveSorted()
	if len(active) == 0 {
		return ChecksSectionStyle.Render(
			dimStyle.Render("Waiting for checks to start..."))
	}

	// Limit visible items
	maxVisible := a.MaxVisible
	if maxVisible <= 0 {
		maxVisible = 10
	}
	if len(active) > maxVisible {
		active = active[:maxVisible]
	}

	var b strings.Builder

	// Section header
	b.WriteString(dimStyle.Render(fmt.Sprintf("Active Checks (%d)", len(active))))
	b.WriteString("\n\n")

	// Render each check
	spinnerFrame := SpinnerFrames[a.SpinnerIdx%len(SpinnerFrames)]

	for i, check := range active {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(a.renderCheck(check, spinnerFrame))
	}

	return ChecksSectionStyle.Render(b.String())
}

func (a *ActiveChecksComponent) renderCheck(status *CheckStatus, spinner string) string {
	// Determine status
	isComplete := status.DoneChecks >= status.TotalChecks && status.TotalChecks > 0
	hasSuccess := false
	hasFailed := false

	for _, result := range status.CheckResults {
		if result.Success {
			hasSuccess = true
		} else {
			hasFailed = true
		}
	}

	// Format proxy URL
	proxyURL := status.Proxy
	if len(proxyURL) > 45 {
		proxyURL = proxyURL[:42] + "..."
	}

	var b strings.Builder

	// Status icon and URL
	if isComplete {
		icon := GetStatusIcon(hasSuccess, hasFailed, false)
		style := GetStatusStyle(hasSuccess, hasFailed, false)
		b.WriteString(style.Render(icon + " "))
	} else {
		b.WriteString(ActiveStyle.Render(spinner + " "))
	}

	b.WriteString(ProxyURLStyle.Render(proxyURL))

	// Protocol badges
	var badges []string
	if status.ProxyType != "" {
		badges = append(badges, ProxyTypeStyle.Render(status.ProxyType))
	}
	if status.SupportsHTTP {
		badges = append(badges, dimStyle.Render("HTTP"))
	}
	if status.SupportsHTTPS {
		badges = append(badges, dimStyle.Render("HTTPS"))
	}
	if len(badges) > 0 {
		b.WriteString(" " + strings.Join(badges, " "))
	}

	// Speed
	if status.Speed > 0 {
		speedMs := status.Speed.Milliseconds()
		speedStyle := ProxySpeedStyle
		if speedMs > 2000 {
			speedStyle = WarningStyle
		} else if speedMs < 500 {
			speedStyle = SuccessStyle
		}
		b.WriteString(" " + speedStyle.Render(fmt.Sprintf("%.0fms", float64(speedMs))))
	}

	// Detailed mode info
	if a.Mode == ModeVerbose || a.Mode == ModeDebug {
		b.WriteString("\n")
		b.WriteString(a.renderCheckDetails(status))
	}

	return b.String()
}

func (a *ActiveChecksComponent) renderCheckDetails(status *CheckStatus) string {
	var b strings.Builder

	// Check results summary
	if len(status.CheckResults) > 0 {
		success := 0
		failed := 0
		for _, r := range status.CheckResults {
			if r.Success {
				success++
			} else {
				failed++
			}
		}

		b.WriteString("  ")
		if success > 0 {
			b.WriteString(SuccessStyle.Render(fmt.Sprintf("✓ %d", success)))
		}
		if failed > 0 {
			if success > 0 {
				b.WriteString(" ")
			}
			b.WriteString(ErrorStyle.Render(fmt.Sprintf("✗ %d", failed)))
		}

		// Show individual results in debug mode
		if a.Mode == ModeDebug {
			for _, r := range status.CheckResults {
				b.WriteString("\n  ")
				if r.Success {
					b.WriteString(SuccessStyle.Render("✓"))
				} else {
					b.WriteString(ErrorStyle.Render("✗"))
				}
				b.WriteString(" " + dimStyle.Render(r.URL))
				if r.Error != "" {
					b.WriteString(" " + ErrorStyle.Render(r.Error))
				}
			}
		}
	}

	return b.String()
}

func (a *ActiveChecksComponent) getActiveSorted() []*CheckStatus {
	var active []*CheckStatus

	cutoff := time.Now().Add(-5 * time.Second)
	for _, status := range a.Checks {
		if status.IsActive && status.LastUpdate.After(cutoff) {
			active = append(active, status)
		}
	}

	// Sort by position
	sort.Slice(active, func(i, j int) bool {
		return active[i].Position < active[j].Position
	})

	return active
}

// =============================================================================
// FOOTER COMPONENT
// =============================================================================

// FooterComponent displays help text and controls
type FooterComponent struct {
	Hints   []string
	Version string
}

func (f *FooterComponent) Render() string {
	hints := f.Hints
	if len(hints) == 0 {
		hints = []string{"press q to quit"}
	}

	// Add version to hints if available
	if f.Version != "" {
		hints = append(hints, fmt.Sprintf("v%s", f.Version))
	}

	content := strings.Join(hints, "  •  ")
	return FooterStyle.Render(content)
}

// =============================================================================
// DEBUG LOG COMPONENT
// =============================================================================

// DebugLogComponent shows recent debug messages
type DebugLogComponent struct {
	Messages []string
	MaxLines int
}

func (d *DebugLogComponent) Render() string {
	if len(d.Messages) == 0 {
		return ""
	}

	maxLines := d.MaxLines
	if maxLines <= 0 {
		maxLines = 10
	}

	// Take last N messages
	start := 0
	if len(d.Messages) > maxLines {
		start = len(d.Messages) - maxLines
	}
	messages := d.Messages[start:]

	var b strings.Builder
	b.WriteString(dimStyle.Render("Debug Log"))
	b.WriteString("\n\n")

	for _, msg := range messages {
		// Colorize based on content
		style := dimStyle
		if strings.Contains(strings.ToLower(msg), "error") ||
		   strings.Contains(strings.ToLower(msg), "fail") {
			style = ErrorStyle
		} else if strings.Contains(strings.ToLower(msg), "success") ||
		          strings.Contains(strings.ToLower(msg), "working") {
			style = SuccessStyle
		} else if strings.Contains(strings.ToLower(msg), "warn") {
			style = WarningStyle
		}

		b.WriteString(style.Render("  " + msg))
		b.WriteString("\n")
	}

	return lipgloss.NewStyle().
		Border(thinBorderStyle).
		BorderForeground(lipgloss.Color(ColorBorderDim)).
		Padding(1, 1).
		Width(DefaultWidth).
		Render(b.String())
}
