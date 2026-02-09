package ui

import "time"

// Backward compatibility layer for existing code

// ViewDisplayMode provides backward compatibility
type ViewDisplayMode struct {
	IsVerbose bool
	IsDebug   bool
}

// DisplayMode provides backward compatibility - maps to internal Mode
func (v *View) DisplayModeCompat() ViewDisplayMode {
	return ViewDisplayMode{
		IsVerbose: v.Mode == ModeVerbose,
		IsDebug:   v.Mode == ModeDebug,
	}
}

// SetDisplayMode provides backward compatibility
func (v *View) SetDisplayMode(dm ViewDisplayMode) {
	v.SetMode(dm.IsVerbose, dm.IsDebug)
}

// ViewMetrics provides backward compatibility for metrics
type ViewMetrics struct {
	ActiveJobs  int
	QueueSize   int
	SuccessRate float64
	AvgSpeed    time.Duration
}

// Metrics provides backward compatibility - calculates from View state
func (v *View) MetricsCompat() ViewMetrics {
	successRate := 0.0
	if v.Current > 0 {
		successRate = float64(v.Working) / float64(v.Current) * 100
	}

	return ViewMetrics{
		ActiveJobs:  v.CountActive(),
		QueueSize:   v.Total - v.Current - v.CountActive(),
		SuccessRate: successRate,
		AvgSpeed:    v.AvgSpeed,
	}
}

// DebugInfo provides backward compatibility - returns concatenated debug messages
func (v *View) DebugInfoString() string {
	if len(v.DebugMessages) == 0 {
		return ""
	}
	result := ""
	for _, msg := range v.DebugMessages {
		result += msg + "\n"
	}
	return result
}

// SetDebugInfo provides backward compatibility
func (v *View) SetDebugInfoFromString(info string) {
	v.DebugMessages = []string{info}
}

// AppendDebugInfo provides backward compatibility
func (v *View) AppendDebugInfo(info string) {
	v.AddDebugMessage(info)
}
