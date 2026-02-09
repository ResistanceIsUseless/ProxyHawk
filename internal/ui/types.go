package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/progress"
)

// ViewMode represents the display mode
type ViewMode int

const (
	ModeDefault ViewMode = iota
	ModeVerbose
	ModeDebug
)

// View represents the main UI state
type View struct {
	// Progress tracking
	Progress progress.Model
	Total    int
	Current  int

	// Counters
	Working int
	Failed  int

	// Performance metrics
	AvgSpeed time.Duration

	// Active state
	ActiveChecks map[string]*CheckStatus
	SpinnerIdx   int

	// Display mode
	Mode ViewMode

	// Debug messages
	DebugMessages []string

	// Version information
	Version string
}

// NewView creates a new View with sensible defaults
func NewView() *View {
	return &View{
		Progress:      progress.New(progress.WithDefaultGradient()),
		ActiveChecks:  make(map[string]*CheckStatus),
		DebugMessages: make([]string, 0),
		Mode:          ModeDefault,
	}
}

// SetMode sets the display mode
func (v *View) SetMode(verbose, debug bool) {
	if debug {
		v.Mode = ModeDebug
	} else if verbose {
		v.Mode = ModeVerbose
	} else {
		v.Mode = ModeDefault
	}
}

// AddDebugMessage adds a debug message to the log
func (v *View) AddDebugMessage(msg string) {
	v.DebugMessages = append(v.DebugMessages, msg)
	// Keep only last 50 messages
	if len(v.DebugMessages) > 50 {
		v.DebugMessages = v.DebugMessages[len(v.DebugMessages)-50:]
	}
}

// UpdateProgress updates the progress bar
func (v *View) UpdateProgress(current, total int) {
	v.Current = current
	v.Total = total
	if total > 0 {
		v.Progress.SetPercent(float64(current) / float64(total))
	}
}

// CountActive returns the number of currently active checks
func (v *View) CountActive() int {
	count := 0
	cutoff := time.Now().Add(-5 * time.Second)
	for _, status := range v.ActiveChecks {
		if status.IsActive && status.LastUpdate.After(cutoff) {
			count++
		}
	}
	return count
}

// IsValid checks if the View state is valid
func (v *View) IsValid() bool {
	return v.Total >= 0 &&
		v.Current >= 0 &&
		v.Current <= v.Total &&
		v.ActiveChecks != nil
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
