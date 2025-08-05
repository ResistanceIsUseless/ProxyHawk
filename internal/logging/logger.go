package logging

import (
	"io"
	"log/slog"
	"os"
)

// Logger provides structured logging capabilities
type Logger struct {
	*slog.Logger
}

// LogLevel represents log level constants
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Config represents logger configuration
type Config struct {
	Level  LogLevel
	Format string // "json" or "text"
	Output io.Writer
}

// NewLogger creates a new structured logger
func NewLogger(config Config) *Logger {
	var level slog.Level
	switch config.Level {
	case LevelDebug:
		level = slog.LevelDebug
	case LevelInfo:
		level = slog.LevelInfo
	case LevelWarn:
		level = slog.LevelWarn
	case LevelError:
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	output := config.Output
	if output == nil {
		output = os.Stdout
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	if config.Format == "json" {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	return &Logger{
		Logger: slog.New(handler),
	}
}

// GetDefaultLogger returns a logger with sensible defaults
func GetDefaultLogger() *Logger {
	return NewLogger(Config{
		Level:  LevelInfo,
		Format: "text",
		Output: os.Stdout,
	})
}

// WithContext adds contextual fields to the logger
func (l *Logger) WithContext(args ...any) *Logger {
	return &Logger{
		Logger: l.With(args...),
	}
}

// WithWorker adds worker ID context
func (l *Logger) WithWorker(workerID int) *Logger {
	return l.WithContext("worker", workerID)
}

// WithProxy adds proxy context
func (l *Logger) WithProxy(proxy string) *Logger {
	return l.WithContext("proxy", proxy)
}

// WithDuration adds duration context
func (l *Logger) WithDuration(key string, duration float64) *Logger {
	return l.WithContext(key, duration)
}

// ConfigLoaded logs successful configuration loading
func (l *Logger) ConfigLoaded(file string) {
	l.Info("Configuration loaded", "file", file)
}

// ConfigNotFound logs when config file is not found
func (l *Logger) ConfigNotFound(file string) {
	l.Warn("Config file not found, using defaults", "file", file)
}

// ProxiesLoaded logs successful proxy loading
func (l *Logger) ProxiesLoaded(count int, file string) {
	l.Info("Proxies loaded", "count", count, "file", file)
}

// ProxyCheckStart logs start of proxy checking
func (l *Logger) ProxyCheckStart(total int, concurrency int) {
	l.Info("Starting proxy checks", "total", total, "concurrency", concurrency)
}

// ProxyCheckComplete logs completion of proxy checking
func (l *Logger) ProxyCheckComplete() {
	l.Info("Proxy checking complete")
}

// ProxySuccess logs successful proxy check
func (l *Logger) ProxySuccess(proxy string, duration float64, anonymous bool, cloudProvider string) {
	logger := l.WithProxy(proxy).WithDuration("duration_seconds", duration)
	if anonymous {
		logger = logger.WithContext("anonymous", true)
	}
	if cloudProvider != "" {
		logger = logger.WithContext("cloud_provider", cloudProvider)
	}
	logger.Info("Proxy check successful")
}

// ProxyFailure logs failed proxy check
func (l *Logger) ProxyFailure(proxy string, err error) {
	l.WithProxy(proxy).Error("Proxy check failed", "error", err)
}

// WorkerStart logs worker startup
func (l *Logger) WorkerStart(workerID int) {
	l.WithWorker(workerID).Debug("Worker started")
}

// WorkerStop logs worker shutdown
func (l *Logger) WorkerStop(workerID int) {
	l.WithWorker(workerID).Debug("Worker stopped")
}

// ShutdownReceived logs shutdown signal
func (l *Logger) ShutdownReceived() {
	l.Info("Shutdown signal received, cleaning up...")
}

// ShutdownComplete logs shutdown completion
func (l *Logger) ShutdownComplete() {
	l.Info("Shutdown complete")
}

// ResultsSaved logs when results are saved to file
func (l *Logger) ResultsSaved(file string, format string) {
	l.Info("Results saved", "file", file, "format", format)
}

// SummaryStats logs summary statistics
func (l *Logger) SummaryStats(total, working, anonymous int, successRate float64) {
	l.Info("Summary statistics",
		"total_proxies", total,
		"working_proxies", working,
		"anonymous_proxies", anonymous,
		"success_rate_percent", successRate,
	)
}