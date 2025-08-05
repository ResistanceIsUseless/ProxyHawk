package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Collector manages all ProxyHawk metrics
type Collector struct {
	// Counters
	proxiesChecked   prometheus.Counter
	proxiesWorking   prometheus.Counter
	proxiesFailed    prometheus.Counter
	proxiesAnonymous prometheus.Counter
	checksTotal      prometheus.Counter
	checksErrors     prometheus.Counter

	// Histograms
	checkDuration prometheus.Histogram
	responseTime  prometheus.Histogram

	// Gauges
	activeChecks  prometheus.Gauge
	queueSize     prometheus.Gauge
	workersActive prometheus.Gauge

	// Labels
	checksPerType     *prometheus.CounterVec
	checksPerProvider *prometheus.CounterVec
	errorsPerType     *prometheus.CounterVec

	registry *prometheus.Registry
	server   *http.Server
	mutex    sync.RWMutex
}

// NewCollector creates a new metrics collector
func NewCollector() *Collector {
	c := &Collector{
		registry: prometheus.NewRegistry(),
	}

	c.initMetrics()
	c.registerMetrics()

	return c
}

// initMetrics initializes all Prometheus metrics
func (c *Collector) initMetrics() {
	// Counters
	c.proxiesChecked = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "proxyhawk_proxies_checked_total",
		Help: "Total number of proxies checked",
	})

	c.proxiesWorking = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "proxyhawk_proxies_working_total",
		Help: "Total number of working proxies found",
	})

	c.proxiesFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "proxyhawk_proxies_failed_total",
		Help: "Total number of failed proxy checks",
	})

	c.proxiesAnonymous = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "proxyhawk_proxies_anonymous_total",
		Help: "Total number of anonymous proxies found",
	})

	c.checksTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "proxyhawk_checks_total",
		Help: "Total number of individual checks performed",
	})

	c.checksErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "proxyhawk_checks_errors_total",
		Help: "Total number of check errors",
	})

	// Histograms
	c.checkDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "proxyhawk_check_duration_seconds",
		Help:    "Duration of proxy checks in seconds",
		Buckets: prometheus.DefBuckets,
	})

	c.responseTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "proxyhawk_response_time_seconds",
		Help:    "HTTP response time through proxy in seconds",
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 25, 50, 100},
	})

	// Gauges
	c.activeChecks = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "proxyhawk_active_checks",
		Help: "Number of currently active proxy checks",
	})

	c.queueSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "proxyhawk_queue_size",
		Help: "Number of proxies waiting to be checked",
	})

	c.workersActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "proxyhawk_workers_active",
		Help: "Number of active worker goroutines",
	})

	// Counter vectors with labels
	c.checksPerType = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxyhawk_checks_per_type_total",
			Help: "Total number of checks per proxy type",
		},
		[]string{"proxy_type"},
	)

	c.checksPerProvider = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxyhawk_checks_per_provider_total",
			Help: "Total number of checks per cloud provider",
		},
		[]string{"provider"},
	)

	c.errorsPerType = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxyhawk_errors_per_type_total",
			Help: "Total number of errors per error type",
		},
		[]string{"error_type"},
	)
}

// registerMetrics registers all metrics with the Prometheus registry
func (c *Collector) registerMetrics() {
	c.registry.MustRegister(
		c.proxiesChecked,
		c.proxiesWorking,
		c.proxiesFailed,
		c.proxiesAnonymous,
		c.checksTotal,
		c.checksErrors,
		c.checkDuration,
		c.responseTime,
		c.activeChecks,
		c.queueSize,
		c.workersActive,
		c.checksPerType,
		c.checksPerProvider,
		c.errorsPerType,
	)
}

// StartServer starts the metrics HTTP server
func (c *Collector) StartServer(addr string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.server != nil {
		return fmt.Errorf("metrics server already running")
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	c.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		server := c.server // Capture server in closure to avoid race
		if server != nil {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				// Log error but don't crash the main application
			}
		}
	}()

	return nil
}

// StopServer stops the metrics HTTP server
func (c *Collector) StopServer() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.server.Shutdown(ctx)
	c.server = nil
	return err
}

// Metrics recording methods

// RecordProxyCheck records a completed proxy check
func (c *Collector) RecordProxyCheck(working bool, proxyType string, duration time.Duration) {
	c.proxiesChecked.Inc()
	c.checkDuration.Observe(duration.Seconds())
	c.checksPerType.WithLabelValues(proxyType).Inc()

	if working {
		c.proxiesWorking.Inc()
	} else {
		c.proxiesFailed.Inc()
	}
}

// RecordAnonymousProxy records an anonymous proxy discovery
func (c *Collector) RecordAnonymousProxy() {
	c.proxiesAnonymous.Inc()
}

// RecordCheck records an individual check operation
func (c *Collector) RecordCheck(success bool, responseTime time.Duration) {
	c.checksTotal.Inc()
	if responseTime > 0 {
		c.responseTime.Observe(responseTime.Seconds())
	}
	if !success {
		c.checksErrors.Inc()
	}
}

// RecordCloudProvider records a check for a specific cloud provider
func (c *Collector) RecordCloudProvider(provider string) {
	c.checksPerProvider.WithLabelValues(provider).Inc()
}

// RecordError records an error by type
func (c *Collector) RecordError(errorType string) {
	c.errorsPerType.WithLabelValues(errorType).Inc()
}

// Gauge update methods

// SetActiveChecks updates the active checks gauge
func (c *Collector) SetActiveChecks(count int) {
	c.activeChecks.Set(float64(count))
}

// SetQueueSize updates the queue size gauge
func (c *Collector) SetQueueSize(size int) {
	c.queueSize.Set(float64(size))
}

// SetWorkersActive updates the active workers gauge
func (c *Collector) SetWorkersActive(count int) {
	c.workersActive.Set(float64(count))
}

// GetRegistry returns the Prometheus registry for external use
func (c *Collector) GetRegistry() *prometheus.Registry {
	return c.registry
}

// GetMetricsHandler returns an HTTP handler for the /metrics endpoint
func (c *Collector) GetMetricsHandler() http.Handler {
	return promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{})
}
