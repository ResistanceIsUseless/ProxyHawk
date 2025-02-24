package ui

import (
	"fmt"
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
	str.WriteString(headerStyle.Render("ProxyCheck Progress") + "\n\n")

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
	str.WriteString(headerStyle.Render("ProxyCheck Progress (Verbose Mode)") + "\n\n")

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
	str := strings.Builder{}

	// Title
	str.WriteString(headerStyle.Render("ProxyCheck Progress (Debug Mode)") + "\n\n")

	// Progress information
	str.WriteString(v.renderProgressAndMetrics())

	// Current checks with debug information
	str.WriteString("\nCurrent Checks:\n")
	str.WriteString(v.renderActiveChecksDebug())

	// Debug Information
	if v.DebugInfo != "" {
		str.WriteString("\nDebug Information:\n")
		str.WriteString(debugBlockStyle.Render(v.DebugInfo) + "\n")
	}

	// Controls
	str.WriteString("\n" + infoStyle.Render("Press q to quit"))

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

	for _, ps := range activeList {
		proxy, status := ps.proxy, ps.status
		spinner := spinnerFrames[v.SpinnerIdx%len(spinnerFrames)]

		statusLine := fmt.Sprintf("%s %s [%s] %s (%s checks)\n",
			lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(spinner),
			lipgloss.NewStyle().Bold(true).Render(proxy),
			successStyle.Render(status.ProxyType),
			v.getStatusIndicator(status),
			v.getCheckCount(status))
		str.WriteString(statusLine)
	}

	return str.String()
}

func (v *View) renderActiveChecksVerbose() string {
	str := strings.Builder{}
	activeList := v.getActiveChecksList()

	for _, ps := range activeList {
		proxy, status := ps.proxy, ps.status
		spinner := spinnerFrames[v.SpinnerIdx%len(spinnerFrames)]

		// Main status line
		str.WriteString(fmt.Sprintf("%s %s\n",
			lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(spinner),
			lipgloss.NewStyle().Bold(true).Render(proxy)))

		// Detailed status information
		str.WriteString(fmt.Sprintf("  Type: %s\n", successStyle.Render(status.ProxyType)))
		str.WriteString(fmt.Sprintf("  Status: %s\n", v.getStatusIndicator(status)))
		str.WriteString(fmt.Sprintf("  Progress: %s checks\n", v.getCheckCount(status)))
		if status.Speed > 0 {
			str.WriteString(fmt.Sprintf("  Speed: %v\n", status.Speed.Round(time.Millisecond)))
		}

		// Cloud provider information
		if status.CloudProvider != "" {
			str.WriteString(fmt.Sprintf("  Cloud Provider: %s\n", status.CloudProvider))
			if status.InternalAccess {
				str.WriteString(fmt.Sprintf("  Internal Access: %s\n", successStyle.Render("Yes")))
			}
			if status.MetadataAccess {
				str.WriteString(fmt.Sprintf("  Metadata Access: %s\n", warningStyle.Render("Yes")))
			}
		}

		// Check results
		if len(status.CheckResults) > 0 {
			str.WriteString("  Check Results:\n")
			for _, check := range status.CheckResults {
				checkStatus := successStyle.Render("✓")
				if !check.Success {
					checkStatus = errorStyle.Render("✗")
				}

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
					check.URL,
					strings.Join(details, ", ")))
			}
		}
		str.WriteString("\n")
	}

	return str.String()
}

func (v *View) renderActiveChecksDebug() string {
	str := strings.Builder{}
	activeList := v.getActiveChecksList()

	for _, ps := range activeList {
		proxy, status := ps.proxy, ps.status
		spinner := spinnerFrames[v.SpinnerIdx%len(spinnerFrames)]

		// Main status line with debug information
		str.WriteString(fmt.Sprintf("%s %s [%s] (%d/%d checks)\n",
			lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(spinner),
			lipgloss.NewStyle().Bold(true).Render(proxy),
			successStyle.Render(status.ProxyType),
			status.DoneChecks,
			status.TotalChecks))

		// Cloud provider debug information
		if status.CloudProvider != "" {
			str.WriteString(fmt.Sprintf("  Cloud Provider: %s\n", status.CloudProvider))
			str.WriteString(fmt.Sprintf("  Internal Access: %v\n", status.InternalAccess))
			str.WriteString(fmt.Sprintf("  Metadata Access: %v\n", status.MetadataAccess))
		}

		// Detailed check results with debug information
		for _, check := range status.CheckResults {
			checkStatus := successStyle.Render("✓")
			if !check.Success {
				checkStatus = errorStyle.Render("✗")
			}

			str.WriteString(fmt.Sprintf("  %s %s\n", checkStatus, check.URL))
			str.WriteString(fmt.Sprintf("    Status Code: %d\n", check.StatusCode))
			str.WriteString(fmt.Sprintf("    Response Size: %d bytes\n", check.BodySize))
			str.WriteString(fmt.Sprintf("    Response Time: %v\n", check.Speed.Round(time.Millisecond)))

			if check.Error != "" {
				str.WriteString(fmt.Sprintf("    Error: %s\n", check.Error))
			}
		}

		// Add timing information
		if !status.LastUpdate.IsZero() {
			str.WriteString(fmt.Sprintf("    Last Update: %v ago\n",
				time.Since(status.LastUpdate).Round(time.Millisecond)))
		}

		str.WriteString("\n")
	}

	return str.String()
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
