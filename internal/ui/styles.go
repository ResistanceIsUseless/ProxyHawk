package ui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
)

// Modern color palette using 256-color terminal codes
// Based on a clean, professional dark theme
const (
	// Primary colors
	ColorPrimary   = "#61AFEF" // Soft blue
	ColorSecondary = "#98C379" // Soft green
	ColorAccent    = "#C678DD" // Soft purple

	// Status colors
	ColorSuccess = "#98C379" // Green
	ColorWarning = "#E5C07B" // Yellow
	ColorError   = "#E06C75" // Red
	ColorInfo    = "#61AFEF" // Blue

	// Neutral colors
	ColorText      = "#ABB2BF" // Light gray
	ColorTextDim   = "#5C6370" // Dim gray
	ColorBorder    = "#3E4451" // Dark gray
	ColorBorderDim = "#2C323C" // Darker gray

	// Special status
	ColorActive  = "#61AFEF" // Blue
	ColorPending = "#E5C07B" // Yellow
)

// Layout constants
const (
	DefaultWidth  = 80
	MinWidth      = 60
	MaxWidth      = 120
	ContentPadding = 1
	SectionSpacing = 1
)

// Base styles for consistent look
var (
	// Base text styles
	baseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorText))

	boldStyle = baseStyle.Copy().Bold(true)

	dimStyle = baseStyle.Copy().
			Foreground(lipgloss.Color(ColorTextDim))

	// Border styles
	borderStyle = lipgloss.RoundedBorder()

	thinBorderStyle = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "╰",
		BottomRight: "╯",
	}
)

// Component styles
var (
	// Header - App title
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorPrimary)).
			Border(thinBorderStyle).
			BorderForeground(lipgloss.Color(ColorBorder)).
			Padding(0, 2).
			Width(DefaultWidth).
			Align(lipgloss.Center)

	// Stats bar - Key metrics at a glance
	StatsBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorText)).
			Border(thinBorderStyle, false, false, true, false).
			BorderForeground(lipgloss.Color(ColorBorderDim)).
			Padding(0, 1).
			Width(DefaultWidth)

	// Progress section
	ProgressStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Width(DefaultWidth)

	// Active checks section
	ChecksSectionStyle = lipgloss.NewStyle().
				Border(thinBorderStyle, true, false, false, false).
				BorderForeground(lipgloss.Color(ColorBorderDim)).
				Padding(1, 1).
				Width(DefaultWidth)

	// Footer - Controls and hints
	FooterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorTextDim)).
			Border(thinBorderStyle, true, false, false, false).
			BorderForeground(lipgloss.Color(ColorBorderDim)).
			Padding(0, 1).
			Width(DefaultWidth).
			Align(lipgloss.Center)
)

// Text semantic styles
var (
	// Status colors
	SuccessStyle = baseStyle.Copy().
			Foreground(lipgloss.Color(ColorSuccess)).
			Bold(true)

	WarningStyle = baseStyle.Copy().
			Foreground(lipgloss.Color(ColorWarning)).
			Bold(true)

	ErrorStyle = baseStyle.Copy().
			Foreground(lipgloss.Color(ColorError)).
			Bold(true)

	InfoStyle = baseStyle.Copy().
			Foreground(lipgloss.Color(ColorInfo))

	// Metric styles
	MetricLabelStyle = dimStyle.Copy()

	MetricValueStyle = boldStyle.Copy().
				Foreground(lipgloss.Color(ColorPrimary))

	MetricGoodStyle = boldStyle.Copy().
			Foreground(lipgloss.Color(ColorSuccess))

	MetricBadStyle = boldStyle.Copy().
			Foreground(lipgloss.Color(ColorError))

	// Proxy display
	ProxyURLStyle = boldStyle.Copy().
			Foreground(lipgloss.Color(ColorText))

	ProxyTypeStyle = baseStyle.Copy().
			Foreground(lipgloss.Color(ColorAccent))

	ProxySpeedStyle = baseStyle.Copy().
			Foreground(lipgloss.Color(ColorSecondary))

	// Active status
	ActiveStyle = baseStyle.Copy().
			Foreground(lipgloss.Color(ColorActive))

	PendingStyle = baseStyle.Copy().
			Foreground(lipgloss.Color(ColorPending))
)

// Status indicators - Clean, minimal symbols
const (
	IconSpinner = "⠿"
	IconSuccess = "✓"
	IconError   = "✗"
	IconWarning = "!"
	IconCheck   = "●"
	IconEmpty   = "○"
	IconActive  = "▶"
	IconDone    = "■"
)

// Spinner frames - Simple and smooth
var SpinnerFrames = []string{
	"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
}

// GetStatusIcon returns a clean status icon
func GetStatusIcon(working, failed, inProgress bool) string {
	if inProgress {
		return IconActive
	}
	if working {
		return IconSuccess
	}
	if failed {
		return IconError
	}
	return IconEmpty
}

// GetStatusStyle returns appropriate styling for status
func GetStatusStyle(working, failed, inProgress bool) lipgloss.Style {
	if inProgress {
		return ActiveStyle
	}
	if working {
		return SuccessStyle
	}
	if failed {
		return ErrorStyle
	}
	return dimStyle
}

// FormatDuration formats a duration for display
func FormatDuration(d lipgloss.Style, ms int64) string {
	if ms < 1000 {
		return d.Render(lipgloss.NewStyle().Render(fmt.Sprintf("%dms", ms)))
	}
	return d.Render(lipgloss.NewStyle().Render(fmt.Sprintf("%.2fs", float64(ms)/1000.0)))
}

// FormatPercentage formats a percentage for display
func FormatPercentage(value, total int) string {
	if total == 0 {
		return "0%"
	}
	pct := float64(value) / float64(total) * 100
	return fmt.Sprintf("%.1f%%", pct)
}

// FormatCount formats a count comparison
func FormatCount(value, total int) string {
	return fmt.Sprintf("%d/%d", value, total)
}
