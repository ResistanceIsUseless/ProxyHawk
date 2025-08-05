package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// ProgressIndicator provides different types of progress indication for CLI
type ProgressIndicator interface {
	Start(total int)
	Update(current int, message string)
	Finish(message string)
	SetOutput(writer io.Writer)
}

// ProgressType represents different types of progress indicators
type ProgressType string

const (
	ProgressTypeNone     ProgressType = "none"     // No progress indication
	ProgressTypeBasic    ProgressType = "basic"    // Simple text progress
	ProgressTypeBar      ProgressType = "bar"      // Progress bar
	ProgressTypeSpinner  ProgressType = "spinner"  // Spinner with status
	ProgressTypeDots     ProgressType = "dots"     // Dot progress
	ProgressTypePercent  ProgressType = "percent"  // Percentage only
)

// Config holds configuration for progress indicators
type Config struct {
	Type        ProgressType
	Width       int           // Width of progress bar
	UpdateRate  time.Duration // How often to update spinner/dots
	ShowETA     bool          // Show estimated time of arrival
	ShowStats   bool          // Show statistics (success rate, etc.)
	NoColor     bool          // Disable colored output
	Output      io.Writer     // Output destination (default: os.Stderr)
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		Type:       ProgressTypeBar,
		Width:      50,
		UpdateRate: 250 * time.Millisecond,
		ShowETA:    true,
		ShowStats:  true,
		NoColor:    false,
		Output:     os.Stderr,
	}
}

// NewProgressIndicator creates a progress indicator based on the configuration
func NewProgressIndicator(config Config) ProgressIndicator {
	if config.Output == nil {
		config.Output = os.Stderr
	}

	switch config.Type {
	case ProgressTypeNone:
		return &NoneIndicator{}
	case ProgressTypeBasic:
		return &BasicIndicator{config: config}
	case ProgressTypeBar:
		return &BarIndicator{config: config}
	case ProgressTypeSpinner:
		return &SpinnerIndicator{config: config}
	case ProgressTypeDots:
		return &DotsIndicator{config: config}
	case ProgressTypePercent:
		return &PercentIndicator{config: config}
	default:
		return &BasicIndicator{config: config}
	}
}

// Stats holds progress statistics
type Stats struct {
	Total       int
	Current     int
	Working     int
	Failed      int
	StartTime   time.Time
	LastUpdate  time.Time
	ETA         time.Duration
	Rate        float64 // proxies per second
}

// NoneIndicator provides no progress indication
type NoneIndicator struct{}

func (n *NoneIndicator) Start(total int)                     {}
func (n *NoneIndicator) Update(current int, message string) {}
func (n *NoneIndicator) Finish(message string)              {}
func (n *NoneIndicator) SetOutput(writer io.Writer)         {}

// BasicIndicator provides simple text-based progress
type BasicIndicator struct {
	config Config
	stats  Stats
	mutex  sync.Mutex
}

func (b *BasicIndicator) Start(total int) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	
	b.stats = Stats{
		Total:     total,
		StartTime: time.Now(),
	}
	
	fmt.Fprintf(b.config.Output, "Starting proxy tests: %d proxies to check\n", total)
}

func (b *BasicIndicator) Update(current int, message string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	
	b.stats.Current = current
	b.stats.LastUpdate = time.Now()
	
	if message != "" {
		// Parse message to extract success/failure info
		if strings.Contains(strings.ToLower(message), "success") || 
		   strings.Contains(strings.ToLower(message), "working") {
			b.stats.Working++
		} else if strings.Contains(strings.ToLower(message), "fail") ||
		          strings.Contains(strings.ToLower(message), "error") {
			b.stats.Failed++
		}
	}
	
	progress := float64(current) / float64(b.stats.Total) * 100
	elapsed := time.Since(b.stats.StartTime)
	
	if current > 0 {
		b.stats.Rate = float64(current) / elapsed.Seconds()
		if b.stats.Rate > 0 {
			remaining := float64(b.stats.Total - current)
			b.stats.ETA = time.Duration(remaining/b.stats.Rate) * time.Second
		}
	}
	
	statusLine := fmt.Sprintf("Progress: %d/%d (%.1f%%)", current, b.stats.Total, progress)
	
	if b.config.ShowStats && (b.stats.Working > 0 || b.stats.Failed > 0) {
		statusLine += fmt.Sprintf(" | Working: %d, Failed: %d", b.stats.Working, b.stats.Failed)
	}
	
	if b.config.ShowETA && b.stats.ETA > 0 {
		statusLine += fmt.Sprintf(" | ETA: %v", b.stats.ETA.Round(time.Second))
	}
	
	if message != "" && len(message) < 50 {
		statusLine += fmt.Sprintf(" | %s", message)
	}
	
	fmt.Fprintf(b.config.Output, "\r%s", statusLine)
	
	// Add newline every 10% or on significant events
	if current%max(b.stats.Total/10, 1) == 0 || current == b.stats.Total {
		fmt.Fprintf(b.config.Output, "\n")
	}
}

func (b *BasicIndicator) Finish(message string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	
	elapsed := time.Since(b.stats.StartTime)
	avgRate := float64(b.stats.Current) / elapsed.Seconds()
	
	fmt.Fprintf(b.config.Output, "\nCompleted: %d proxies tested in %v (%.2f proxies/sec)\n", 
		b.stats.Current, elapsed.Round(time.Second), avgRate)
	
	if b.config.ShowStats {
		successRate := float64(b.stats.Working) / float64(b.stats.Current) * 100
		fmt.Fprintf(b.config.Output, "Results: %d working (%.1f%%), %d failed\n", 
			b.stats.Working, successRate, b.stats.Failed)
	}
	
	if message != "" {
		fmt.Fprintf(b.config.Output, "%s\n", message)
	}
}

func (b *BasicIndicator) SetOutput(writer io.Writer) {
	b.config.Output = writer
}

// BarIndicator provides a visual progress bar
type BarIndicator struct {
	config Config
	stats  Stats
	mutex  sync.Mutex
}

func (b *BarIndicator) Start(total int) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	
	b.stats = Stats{
		Total:     total,
		StartTime: time.Now(),
	}
	
	fmt.Fprintf(b.config.Output, "ProxyHawk: Testing %d proxies\n", total)
}

func (b *BarIndicator) Update(current int, message string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	
	b.stats.Current = current
	b.stats.LastUpdate = time.Now()
	
	if message != "" {
		if strings.Contains(strings.ToLower(message), "success") || 
		   strings.Contains(strings.ToLower(message), "working") {
			b.stats.Working++
		} else if strings.Contains(strings.ToLower(message), "fail") ||
		          strings.Contains(strings.ToLower(message), "error") {
			b.stats.Failed++
		}
	}
	
	progress := float64(current) / float64(b.stats.Total)
	elapsed := time.Since(b.stats.StartTime)
	
	if current > 0 {
		b.stats.Rate = float64(current) / elapsed.Seconds()
		if b.stats.Rate > 0 {
			remaining := float64(b.stats.Total - current)
			b.stats.ETA = time.Duration(remaining/b.stats.Rate) * time.Second
		}
	}
	
	// Create progress bar
	filledWidth := int(progress * float64(b.config.Width))
	bar := strings.Repeat("█", filledWidth) + strings.Repeat("░", b.config.Width-filledWidth)
	
	// Add colors if enabled
	if !b.config.NoColor {
		bar = fmt.Sprintf("\033[32m%s\033[37m%s\033[0m", 
			strings.Repeat("█", filledWidth), 
			strings.Repeat("░", b.config.Width-filledWidth))
	}
	
	statusLine := fmt.Sprintf("\r[%s] %d/%d (%.1f%%)", 
		bar, current, b.stats.Total, progress*100)
	
	if b.config.ShowStats && (b.stats.Working > 0 || b.stats.Failed > 0) {
		successRate := float64(b.stats.Working) / float64(current) * 100
		if !b.config.NoColor {
			statusLine += fmt.Sprintf(" | \033[32m✓%d\033[0m \033[31m✗%d\033[0m (%.1f%%)", 
				b.stats.Working, b.stats.Failed, successRate)
		} else {
			statusLine += fmt.Sprintf(" | ✓%d ✗%d (%.1f%%)", 
				b.stats.Working, b.stats.Failed, successRate)
		}
	}
	
	if b.config.ShowETA && b.stats.ETA > 0 {
		statusLine += fmt.Sprintf(" | ETA: %v", b.stats.ETA.Round(time.Second))
	}
	
	if b.stats.Rate > 0 {
		statusLine += fmt.Sprintf(" | %.1f/s", b.stats.Rate)
	}
	
	fmt.Fprint(b.config.Output, statusLine)
}

func (b *BarIndicator) Finish(message string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	
	elapsed := time.Since(b.stats.StartTime)
	avgRate := float64(b.stats.Current) / elapsed.Seconds()
	successRate := float64(b.stats.Working) / float64(b.stats.Current) * 100
	
	fmt.Fprintf(b.config.Output, "\n\nCompleted in %v (%.2f proxies/sec)\n", 
		elapsed.Round(time.Second), avgRate)
	
	if b.config.ShowStats {
		if !b.config.NoColor {
			fmt.Fprintf(b.config.Output, "Results: \033[32m%d working\033[0m (\033[32m%.1f%%\033[0m), \033[31m%d failed\033[0m\n", 
				b.stats.Working, successRate, b.stats.Failed)
		} else {
			fmt.Fprintf(b.config.Output, "Results: %d working (%.1f%%), %d failed\n", 
				b.stats.Working, successRate, b.stats.Failed)
		}
	}
	
	if message != "" {
		fmt.Fprintf(b.config.Output, "%s\n", message)
	}
}

func (b *BarIndicator) SetOutput(writer io.Writer) {
	b.config.Output = writer
}

// SpinnerIndicator provides a spinning indicator with status
type SpinnerIndicator struct {
	config      Config
	stats       Stats
	mutex       sync.Mutex
	ticker      *time.Ticker
	spinnerIdx  int
	lastMessage string
}

var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (s *SpinnerIndicator) Start(total int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.stats = Stats{
		Total:     total,
		StartTime: time.Now(),
	}
	
	fmt.Fprintf(s.config.Output, "ProxyHawk: Starting tests for %d proxies\n", total)
	
	s.ticker = time.NewTicker(s.config.UpdateRate)
	go s.spin()
}

func (s *SpinnerIndicator) spin() {
	for range s.ticker.C {
		s.mutex.Lock()
		s.spinnerIdx = (s.spinnerIdx + 1) % len(spinnerChars)
		s.updateDisplay()
		s.mutex.Unlock()
	}
}

func (s *SpinnerIndicator) updateDisplay() {
	if s.stats.Current == 0 {
		return
	}
	
	progress := float64(s.stats.Current) / float64(s.stats.Total) * 100
	elapsed := time.Since(s.stats.StartTime)
	
	if s.stats.Current > 0 {
		s.stats.Rate = float64(s.stats.Current) / elapsed.Seconds()
		if s.stats.Rate > 0 {
			remaining := float64(s.stats.Total - s.stats.Current)
			s.stats.ETA = time.Duration(remaining/s.stats.Rate) * time.Second
		}
	}
	
	spinner := spinnerChars[s.spinnerIdx]
	if s.config.NoColor {
		spinner = "|/-\\"[s.spinnerIdx%4 : s.spinnerIdx%4+1]
	}
	
	statusLine := fmt.Sprintf("\r%s Testing proxies... %d/%d (%.1f%%)", 
		spinner, s.stats.Current, s.stats.Total, progress)
	
	if s.config.ShowStats && (s.stats.Working > 0 || s.stats.Failed > 0) {
		statusLine += fmt.Sprintf(" | ✓%d ✗%d", s.stats.Working, s.stats.Failed)
	}
	
	if s.config.ShowETA && s.stats.ETA > 0 {
		statusLine += fmt.Sprintf(" | ETA: %v", s.stats.ETA.Round(time.Second))
	}
	
	if s.lastMessage != "" {
		msgLen := min(30, len(s.lastMessage))
		statusLine += fmt.Sprintf(" | %s", s.lastMessage[:msgLen])
	}
	
	fmt.Fprint(s.config.Output, statusLine)
}

func (s *SpinnerIndicator) Update(current int, message string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.stats.Current = current
	s.stats.LastUpdate = time.Now()
	s.lastMessage = message
	
	if message != "" {
		if strings.Contains(strings.ToLower(message), "success") || 
		   strings.Contains(strings.ToLower(message), "working") {
			s.stats.Working++
		} else if strings.Contains(strings.ToLower(message), "fail") ||
		          strings.Contains(strings.ToLower(message), "error") {
			s.stats.Failed++
		}
	}
}

func (s *SpinnerIndicator) Finish(message string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	if s.ticker != nil {
		s.ticker.Stop()
	}
	
	elapsed := time.Since(s.stats.StartTime)
	avgRate := float64(s.stats.Current) / elapsed.Seconds()
	
	fmt.Fprintf(s.config.Output, "\r✓ Completed: %d proxies tested in %v (%.2f/sec)\n", 
		s.stats.Current, elapsed.Round(time.Second), avgRate)
	
	if s.config.ShowStats {
		successRate := float64(s.stats.Working) / float64(s.stats.Current) * 100
		fmt.Fprintf(s.config.Output, "Results: %d working (%.1f%%), %d failed\n", 
			s.stats.Working, successRate, s.stats.Failed)
	}
	
	if message != "" {
		fmt.Fprintf(s.config.Output, "%s\n", message)
	}
}

func (s *SpinnerIndicator) SetOutput(writer io.Writer) {
	s.config.Output = writer
}

// DotsIndicator shows progress with dots
type DotsIndicator struct {
	config Config
	stats  Stats
	mutex  sync.Mutex
	dots   int
}

func (d *DotsIndicator) Start(total int) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	d.stats = Stats{
		Total:     total,
		StartTime: time.Now(),
	}
	
	fmt.Fprintf(d.config.Output, "Testing %d proxies: ", total)
}

func (d *DotsIndicator) Update(current int, message string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	d.stats.Current = current
	
	// Show a dot every 5% or minimum every 10 proxies
	step := max(d.stats.Total/20, 10)
	if current%step == 0 {
		fmt.Fprint(d.config.Output, ".")
		d.dots++
		
		// New line every 50 dots
		if d.dots%50 == 0 {
			progress := float64(current) / float64(d.stats.Total) * 100
			fmt.Fprintf(d.config.Output, " %d/%d (%.1f%%)\n", current, d.stats.Total, progress)
		}
	}
	
	if message != "" {
		if strings.Contains(strings.ToLower(message), "success") || 
		   strings.Contains(strings.ToLower(message), "working") {
			d.stats.Working++
		} else if strings.Contains(strings.ToLower(message), "fail") ||
		          strings.Contains(strings.ToLower(message), "error") {
			d.stats.Failed++
		}
	}
}

func (d *DotsIndicator) Finish(message string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	elapsed := time.Since(d.stats.StartTime)
	fmt.Fprintf(d.config.Output, " Done! (%v)\n", elapsed.Round(time.Second))
	
	if d.config.ShowStats {
		successRate := float64(d.stats.Working) / float64(d.stats.Current) * 100
		fmt.Fprintf(d.config.Output, "%d working (%.1f%%), %d failed\n", 
			d.stats.Working, successRate, d.stats.Failed)
	}
	
	if message != "" {
		fmt.Fprintf(d.config.Output, "%s\n", message)
	}
}

func (d *DotsIndicator) SetOutput(writer io.Writer) {
	d.config.Output = writer
}

// PercentIndicator shows only percentage progress
type PercentIndicator struct {
	config      Config
	stats       Stats
	mutex       sync.Mutex
	lastPercent int
}

func (p *PercentIndicator) Start(total int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	p.stats = Stats{
		Total:     total,
		StartTime: time.Now(),
	}
	
	fmt.Fprintf(p.config.Output, "Testing %d proxies: 0%%", total)
}

func (p *PercentIndicator) Update(current int, message string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	p.stats.Current = current
	percent := int(float64(current) / float64(p.stats.Total) * 100)
	
	// Only update when percentage changes
	if percent != p.lastPercent {
		p.lastPercent = percent
		fmt.Fprintf(p.config.Output, "\rTesting %d proxies: %d%%", p.stats.Total, percent)
		
		// Show details every 10%
		if percent%10 == 0 && percent > 0 {
			elapsed := time.Since(p.stats.StartTime)
			rate := float64(current) / elapsed.Seconds()
			fmt.Fprintf(p.config.Output, " (%d/%d, %.1f/s)", current, p.stats.Total, rate)
		}
	}
	
	if message != "" {
		if strings.Contains(strings.ToLower(message), "success") || 
		   strings.Contains(strings.ToLower(message), "working") {
			p.stats.Working++
		} else if strings.Contains(strings.ToLower(message), "fail") ||
		          strings.Contains(strings.ToLower(message), "error") {
			p.stats.Failed++
		}
	}
}

func (p *PercentIndicator) Finish(message string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	elapsed := time.Since(p.stats.StartTime)
	avgRate := float64(p.stats.Current) / elapsed.Seconds()
	
	fmt.Fprintf(p.config.Output, "\rCompleted: 100%% (%d proxies in %v, %.2f/sec)\n", 
		p.stats.Current, elapsed.Round(time.Second), avgRate)
	
	if p.config.ShowStats {
		successRate := float64(p.stats.Working) / float64(p.stats.Current) * 100
		fmt.Fprintf(p.config.Output, "Results: %d working (%.1f%%), %d failed\n", 
			p.stats.Working, successRate, p.stats.Failed)
	}
	
	if message != "" {
		fmt.Fprintf(p.config.Output, "%s\n", message)
	}
}

func (p *PercentIndicator) SetOutput(writer io.Writer) {
	p.config.Output = writer
}

// Utility functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}