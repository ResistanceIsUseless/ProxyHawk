package ui

import "github.com/charmbracelet/lipgloss"

// Color palette
const (
	ColorPrimary   = "87"  // Cyan
	ColorSecondary = "39"  // Blue
	ColorSuccess   = "42"  // Green
	ColorError     = "196" // Red
	ColorWarning   = "214" // Orange
	ColorInfo      = "244" // Gray
	ColorDebug     = "208" // Orange
	ColorMuted     = "252" // Light Gray
	ColorAccent    = "99"  // Purple
	ColorMetric    = "207" // Pink
	ColorSpinner   = "86"  // Bright Green
)

// Layout constants
const (
	DefaultWidth  = 50
	MinWidth      = 30
	MaxWidth      = 120
	WideWidth     = 80
	BorderPadding = 1
)

// Layout configuration
type LayoutConfig struct {
	TerminalWidth  int
	TerminalHeight int
	ContentWidth   int
	UseCompact     bool
}

// CalculateLayout determines the optimal layout based on terminal size
func CalculateLayout(termWidth, termHeight int) LayoutConfig {
	config := LayoutConfig{
		TerminalWidth:  termWidth,
		TerminalHeight: termHeight,
	}
	
	// Determine content width based on terminal size
	if termWidth <= MinWidth {
		config.ContentWidth = MinWidth
		config.UseCompact = true
	} else if termWidth >= MaxWidth {
		config.ContentWidth = MaxWidth
		config.UseCompact = false
	} else if termWidth >= WideWidth {
		config.ContentWidth = WideWidth
		config.UseCompact = false
	} else {
		config.ContentWidth = DefaultWidth
		config.UseCompact = termHeight < 20 // Use compact mode on short terminals
	}
	
	return config
}

// ApplyLayoutToStyle applies layout configuration to a lipgloss style
func ApplyLayoutToStyle(style lipgloss.Style, config LayoutConfig) lipgloss.Style {
	return style.Width(config.ContentWidth)
}

// GetStylesForLayout returns configured styles for the given layout
func GetStylesForLayout(config LayoutConfig) struct {
	Header     lipgloss.Style
	Progress   lipgloss.Style
	Status     lipgloss.Style
	Metric     lipgloss.Style
	Debug      lipgloss.Style
} {
	return struct {
		Header     lipgloss.Style
		Progress   lipgloss.Style
		Status     lipgloss.Style
		Metric     lipgloss.Style
		Debug      lipgloss.Style
	}{
		Header:     ApplyLayoutToStyle(HeaderStyle, config),
		Progress:   ApplyLayoutToStyle(ProgressStyle, config),
		Status:     ApplyLayoutToStyle(StatusBlockStyle, config),
		Metric:     ApplyLayoutToStyle(MetricBlockStyle, config),
		Debug:      ApplyLayoutToStyle(DebugBlockStyle, config),
	}
}

// Base styles
var (
	baseBlockStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, BorderPadding)

	baseTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, BorderPadding).
			Align(lipgloss.Center)
)

// Component styles
var (
	HeaderStyle = baseTitleStyle.Copy().
			Foreground(lipgloss.Color(ColorPrimary)).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorPrimary)).
			Width(DefaultWidth)

	ProgressStyle = baseBlockStyle.Copy().
			BorderForeground(lipgloss.Color(ColorSecondary)).
			Width(DefaultWidth)

	StatusBlockStyle = baseBlockStyle.Copy().
			BorderForeground(lipgloss.Color(ColorAccent)).
			Width(DefaultWidth)

	MetricBlockStyle = baseBlockStyle.Copy().
			BorderForeground(lipgloss.Color(ColorMetric)).
			Width(DefaultWidth)

	DebugBlockStyle = baseBlockStyle.Copy().
			BorderForeground(lipgloss.Color(ColorDebug)).
			Width(DefaultWidth)

	DebugHeaderStyle = baseTitleStyle.Copy().
			Foreground(lipgloss.Color(ColorDebug)).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorDebug)).
			Width(DefaultWidth)
)

// Text styles
var (
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSuccess)).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorError)).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorWarning)).
			Bold(true)

	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorInfo)).
			Italic(true)

	DebugTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted)).
			Bold(false)

	MetricLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorInfo)).
			Bold(true)

	MetricValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSpinner)).
			Bold(true)

	SpinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSpinner))

	ProxyURLStyle = lipgloss.NewStyle().
			Bold(true)

	StatusActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorInfo))

	StatusCompleteStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSuccess))
)

// Spinner animation frames
var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Status icons
const (
	IconSpinner = "⌛"
	IconSuccess = "✓"
	IconError   = "✗"
	IconWarning = "⚠"
	IconWorking = "🔧"
	IconSecure  = "🔒"
	IconCloud   = "☁️"
	IconAlert   = "⚠️"
)

// GetStatusIcon returns an appropriate icon for the given status
func GetStatusIcon(success, partial, complete bool) string {
	if !complete {
		return IconSpinner
	}
	if success {
		return IconSuccess
	}
	if partial {
		return IconWarning
	}
	return IconError
}

// GetStatusStyle returns an appropriate style for the given status
func GetStatusStyle(success, partial, complete bool) lipgloss.Style {
	if !complete {
		return InfoStyle
	}
	if success {
		return SuccessStyle
	}
	if partial {
		return WarningStyle
	}
	return ErrorStyle
}

// HTTP status code helpers
func GetHTTPStatusText(code int) string {
	switch {
	case code >= 100 && code < 200:
		return "Info"
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

func GetHTTPStatusStyle(code int) lipgloss.Style {
	switch {
	case code >= 200 && code < 300:
		return SuccessStyle
	case code >= 300 && code < 400:
		return WarningStyle
	case code >= 400:
		return ErrorStyle
	default:
		return InfoStyle
	}
}