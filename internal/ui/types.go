package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/progress"
)

// View represents the main UI state and orchestrates component rendering
type View struct {
	// Progress tracking
	Progress progress.Model
	Total    int
	Current  int
	
	// Performance metrics
	Metrics ViewMetrics
	
	// Active state
	ActiveChecks map[string]*CheckStatus
	SpinnerIdx   int
	
	// Display configuration
	DisplayMode ViewDisplayMode
	
	// Layout configuration
	Layout LayoutConfig
	
	// Debug information
	DebugInfo string
}

// ViewMetrics contains performance and execution metrics
type ViewMetrics struct {
	ActiveJobs   int
	QueueSize    int
	SuccessRate  float64
	AvgSpeed     time.Duration
}

// ViewDisplayMode contains display configuration
type ViewDisplayMode struct {
	IsVerbose bool
	IsDebug   bool
}

// NewView creates a new View with sensible defaults
func NewView() *View {
	return &View{
		Progress:     progress.New(),
		ActiveChecks: make(map[string]*CheckStatus),
		Metrics:      ViewMetrics{},
		DisplayMode:  ViewDisplayMode{},
		Layout:       CalculateLayout(DefaultWidth+20, 30), // Default layout
	}
}

// UpdateLayout updates the view's layout configuration
func (v *View) UpdateLayout(termWidth, termHeight int) {
	v.Layout = CalculateLayout(termWidth, termHeight)
}

// IsValid checks if the View state is valid
func (v *View) IsValid() bool {
	return v.Total >= 0 && 
		   v.Current >= 0 && 
		   v.Current <= v.Total &&
		   v.ActiveChecks != nil
}

// GetCompletionPercentage returns the completion percentage
func (v *View) GetCompletionPercentage() float64 {
	if v.Total == 0 {
		return 0
	}
	return float64(v.Current) / float64(v.Total) * 100
}

// HasActiveChecks returns true if there are any active checks
func (v *View) HasActiveChecks() bool {
	return len(v.ActiveChecks) > 0
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