package ui

import (
	"fmt"
	"strings"
	"time"
)

// Note: View, CheckStatus and CheckResult types are defined in types.go

// Render renders the view based on the current display mode
func (v *View) Render() string {
	mode := v.determineMode()
	return v.renderWithMode(mode)
}

// RenderDefault renders the standard view (kept for backward compatibility)
func (v *View) RenderDefault() string {
	return v.renderWithMode(ModeDefault)
}

// RenderVerbose renders the verbose view (kept for backward compatibility)
func (v *View) RenderVerbose() string {
	return v.renderWithMode(ModeVerbose)
}

// RenderDebug renders the debug view (kept for backward compatibility)
func (v *View) RenderDebug() string {
	return v.renderWithMode(ModeDebug)
}

// renderWithMode is the unified rendering function that eliminates code duplication
func (v *View) renderWithMode(mode ViewMode) string {
	// Validate view state before rendering
	if !v.IsValid() {
		return ErrorStyle.Render("Invalid view state - cannot render")
	}
	
	var sections []string
	
	// Title section
	sections = append(sections, v.renderTitle(mode))
	
	// Core components based on mode
	if mode == ModeVerbose || mode == ModeDebug {
		if metricsSection := v.renderMetrics(); metricsSection != "" {
			sections = append(sections, metricsSection)
		}
	}
	
	if progressSection := v.renderProgress(); progressSection != "" {
		sections = append(sections, progressSection)
	}
	
	if mode == ModeDebug {
		if summarySection := v.renderSummaryStats(); summarySection != "" {
			sections = append(sections, summarySection)
		}
	}
	
	// Active checks
	if checksSection := v.renderActiveChecksSection(mode); checksSection != "" {
		sections = append(sections, checksSection)
	}
	
	// Debug log (only in debug mode)
	if mode == ModeDebug && strings.TrimSpace(v.DebugInfo) != "" {
		debugLog := &DebugLogComponent{
			DebugInfo: v.DebugInfo,
			MaxLines:  20,
		}
		if debugSection := debugLog.Render(); debugSection != "" {
			sections = append(sections, debugSection)
		}
	}
	
	// Controls
	sections = append(sections, InfoStyle.Render("Press q to quit"))
	
	// Ensure we have at least something to show
	if len(sections) == 0 {
		return InfoStyle.Render("No content to display")
	}
	
	return strings.Join(sections, "\n\n")
}

// Helper methods for rendering individual sections
func (v *View) determineMode() ViewMode {
	if v.DisplayMode.IsDebug {
		return ModeDebug
	}
	if v.DisplayMode.IsVerbose {
		return ModeVerbose
	}
	return ModeDefault
}

func (v *View) renderTitle(mode ViewMode) string {
	switch mode {
	case ModeVerbose:
		return HeaderStyle.Render("ProxyHawk Progress (Verbose Mode)")
	case ModeDebug:
		return HeaderStyle.Render("ProxyHawk Debug Mode")
	default:
		return HeaderStyle.Render("ProxyHawk Progress")
	}
}

func (v *View) renderProgress() string {
	progressComp := &ProgressComponent{
		Progress: v.Progress,
		Current:  v.Current,
		Total:    v.Total,
		Layout:   v.Layout,
	}
	return progressComp.Render()
}

func (v *View) renderMetrics() string {
	workingCount := v.calculateWorkingCount()
	metricsComp := &MetricsComponent{
		ActiveJobs:   v.Metrics.ActiveJobs,
		QueueSize:    v.Metrics.QueueSize,
		SuccessRate:  v.Metrics.SuccessRate,
		AvgSpeed:     v.Metrics.AvgSpeed,
		WorkingCount: workingCount,
		TotalCount:   v.Current,
	}
	return metricsComp.Render()
}

// Helper methods
func (v *View) renderActiveChecksSection(mode ViewMode) string {
	var builder strings.Builder
	
	// Section header
	activeCount := v.countActiveChecks()
	
	switch mode {
	case ModeDefault:
		builder.WriteString("Current Checks:\n")
	case ModeVerbose:
		builder.WriteString("Detailed Check Status:\n")
	case ModeDebug:
		builder.WriteString("DEBUG: Active Checks\n")
	}
	
	// Status summary
	builder.WriteString(fmt.Sprintf("Active: %s | Completed: %s | Total: %s\n",
		MetricValueStyle.Render(fmt.Sprintf("%d", activeCount)),
		MetricValueStyle.Render(fmt.Sprintf("%d", v.Current)),
		MetricValueStyle.Render(fmt.Sprintf("%d", v.Total))))
	
	// Handle empty state
	if activeCount == 0 {
		if v.Current < v.Total {
			builder.WriteString(InfoStyle.Render("Waiting for checks to start..."))
		} else {
			builder.WriteString(SuccessStyle.Render("All checks completed!"))
		}
		return builder.String()
	}
	
	// Render active checks
	activeChecksComp := &ActiveChecksComponent{
		ActiveChecks: v.ActiveChecks,
		SpinnerIdx:   v.SpinnerIdx,
		Mode:         mode,
	}
	
	builder.WriteString("\n")
	builder.WriteString(activeChecksComp.Render())
	
	return builder.String()
}

func (v *View) renderSummaryStats() string {
	var builder strings.Builder
	
	// Calculate statistics
	workingCount := v.calculateWorkingCount()
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
	
	// Working proxies summary
	successRate := 0.0
	if v.Current > 0 {
		successRate = float64(workingCount) / float64(v.Current) * 100
	}
	
	builder.WriteString(fmt.Sprintf("%s %s\n",
		MetricLabelStyle.Render("Working Proxies:"),
		SuccessStyle.Render(fmt.Sprintf("%d/%d (%.1f%%)",
			workingCount, v.Current, successRate))))
	
	// Protocol support
	builder.WriteString(fmt.Sprintf("%s %s, %s %s\n",
		MetricLabelStyle.Render("HTTP Support:"),
		SuccessStyle.Render(fmt.Sprintf("%d", httpSupport)),
		MetricLabelStyle.Render("HTTPS Support:"),
		SuccessStyle.Render(fmt.Sprintf("%d", httpsSupport))))
	
	// Proxy types breakdown
	builder.WriteString(MetricLabelStyle.Render("Proxy Types: "))
	if len(proxyTypes) == 0 {
		builder.WriteString(InfoStyle.Render("None detected yet"))
	} else {
		var typeStrings []string
		for pType, count := range proxyTypes {
			typeStrings = append(typeStrings, fmt.Sprintf("%s: %d", pType, count))
		}
		builder.WriteString(SuccessStyle.Render(strings.Join(typeStrings, ", ")))
	}
	
	return StatusBlockStyle.Render(builder.String())
}

func (v *View) calculateWorkingCount() int {
	workingCount := 0
	for _, status := range v.ActiveChecks {
		successCount := 0
		for _, check := range status.CheckResults {
			if check.Success {
				successCount++
			}
		}
		if successCount > 0 {
			workingCount++
		}
	}
	return workingCount
}

func (v *View) countActiveChecks() int {
	count := 0
	for _, status := range v.ActiveChecks {
		if status.IsActive && time.Since(status.LastUpdate) < 5*time.Second {
			count++
		}
	}
	return count
}