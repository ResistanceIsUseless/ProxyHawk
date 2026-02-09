package ui

import (
	"strings"
)

// Render renders the complete view based on the current mode
func (v *View) Render() string {
	if !v.IsValid() {
		return ErrorStyle.Render("⚠ Invalid view state")
	}

	var sections []string

	// Header - always visible
	header := &HeaderComponent{
		Title: "ProxyHawk",
		Mode:  v.Mode,
	}
	sections = append(sections, header.Render())

	// Stats bar - always visible
	statsBar := &StatsBarComponent{
		Current:  v.Current,
		Total:    v.Total,
		Working:  v.Working,
		Failed:   v.Failed,
		Active:   v.CountActive(),
		AvgSpeed: v.AvgSpeed,
	}
	sections = append(sections, statsBar.Render())

	// Progress bar - always visible
	progress := &ProgressComponent{
		Progress: v.Progress,
		Current:  v.Current,
		Total:    v.Total,
	}
	if progressView := progress.Render(); progressView != "" {
		sections = append(sections, progressView)
	}

	// Active checks - visible based on mode
	activeChecks := &ActiveChecksComponent{
		Checks:     v.ActiveChecks,
		SpinnerIdx: v.SpinnerIdx,
		Mode:       v.Mode,
		MaxVisible: v.getMaxVisible(),
	}
	if checksView := activeChecks.Render(); checksView != "" {
		sections = append(sections, checksView)
	}

	// Debug log - only in debug mode
	if v.Mode == ModeDebug && len(v.DebugMessages) > 0 {
		debugLog := &DebugLogComponent{
			Messages: v.DebugMessages,
			MaxLines: 15,
		}
		if debugView := debugLog.Render(); debugView != "" {
			sections = append(sections, debugView)
		}
	}

	// Footer - always visible
	footer := &FooterComponent{
		Hints:   v.getFooterHints(),
		Version: v.Version,
	}
	sections = append(sections, footer.Render())

	return strings.Join(sections, "\n")
}

// RenderDefault provides backward compatibility
func (v *View) RenderDefault() string {
	oldMode := v.Mode
	v.Mode = ModeDefault
	result := v.Render()
	v.Mode = oldMode
	return result
}

// RenderVerbose provides backward compatibility
func (v *View) RenderVerbose() string {
	oldMode := v.Mode
	v.Mode = ModeVerbose
	result := v.Render()
	v.Mode = oldMode
	return result
}

// RenderDebug provides backward compatibility
func (v *View) RenderDebug() string {
	oldMode := v.Mode
	v.Mode = ModeDebug
	result := v.Render()
	v.Mode = oldMode
	return result
}

// Helper methods

func (v *View) getMaxVisible() int {
	switch v.Mode {
	case ModeDebug:
		return 5 // Show fewer in debug mode to make room for logs
	case ModeVerbose:
		return 8
	default:
		return 10
	}
}

func (v *View) getFooterHints() []string {
	hints := []string{"press q to quit"}

	if v.Mode == ModeDefault {
		hints = append(hints, "use -v for verbose", "use -d for debug")
	}

	return hints
}
