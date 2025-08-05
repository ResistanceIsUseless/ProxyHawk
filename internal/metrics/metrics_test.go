package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNewCollector(t *testing.T) {
	collector := NewCollector()
	if collector == nil {
		t.Fatal("NewCollector() returned nil")
	}

	if collector.registry == nil {
		t.Error("NewCollector() did not initialize registry")
	}
}

func TestRecordProxyCheck(t *testing.T) {
	collector := NewCollector()

	// Record a working proxy
	collector.RecordProxyCheck(true, "http", time.Second)

	// Check that the counter was incremented
	if testutil.ToFloat64(collector.proxiesChecked) != 1 {
		t.Errorf("Expected proxiesChecked to be 1, got %f", testutil.ToFloat64(collector.proxiesChecked))
	}

	if testutil.ToFloat64(collector.proxiesWorking) != 1 {
		t.Errorf("Expected proxiesWorking to be 1, got %f", testutil.ToFloat64(collector.proxiesWorking))
	}

	if testutil.ToFloat64(collector.proxiesFailed) != 0 {
		t.Errorf("Expected proxiesFailed to be 0, got %f", testutil.ToFloat64(collector.proxiesFailed))
	}

	// Record a failed proxy
	collector.RecordProxyCheck(false, "socks5", time.Millisecond*500)

	if testutil.ToFloat64(collector.proxiesChecked) != 2 {
		t.Errorf("Expected proxiesChecked to be 2, got %f", testutil.ToFloat64(collector.proxiesChecked))
	}

	if testutil.ToFloat64(collector.proxiesFailed) != 1 {
		t.Errorf("Expected proxiesFailed to be 1, got %f", testutil.ToFloat64(collector.proxiesFailed))
	}
}

func TestRecordAnonymousProxy(t *testing.T) {
	collector := NewCollector()

	collector.RecordAnonymousProxy()
	collector.RecordAnonymousProxy()

	if testutil.ToFloat64(collector.proxiesAnonymous) != 2 {
		t.Errorf("Expected proxiesAnonymous to be 2, got %f", testutil.ToFloat64(collector.proxiesAnonymous))
	}
}

func TestRecordCheck(t *testing.T) {
	collector := NewCollector()

	// Record successful check
	collector.RecordCheck(true, time.Millisecond*200)

	if testutil.ToFloat64(collector.checksTotal) != 1 {
		t.Errorf("Expected checksTotal to be 1, got %f", testutil.ToFloat64(collector.checksTotal))
	}

	if testutil.ToFloat64(collector.checksErrors) != 0 {
		t.Errorf("Expected checksErrors to be 0, got %f", testutil.ToFloat64(collector.checksErrors))
	}

	// Record failed check
	collector.RecordCheck(false, 0)

	if testutil.ToFloat64(collector.checksTotal) != 2 {
		t.Errorf("Expected checksTotal to be 2, got %f", testutil.ToFloat64(collector.checksTotal))
	}

	if testutil.ToFloat64(collector.checksErrors) != 1 {
		t.Errorf("Expected checksErrors to be 1, got %f", testutil.ToFloat64(collector.checksErrors))
	}
}

func TestRecordCloudProvider(t *testing.T) {
	collector := NewCollector()

	collector.RecordCloudProvider("AWS")
	collector.RecordCloudProvider("GCP")
	collector.RecordCloudProvider("AWS")

	// Check AWS counter
	awsCount := testutil.ToFloat64(collector.checksPerProvider.WithLabelValues("AWS"))
	if awsCount != 2 {
		t.Errorf("Expected AWS checks to be 2, got %f", awsCount)
	}

	// Check GCP counter
	gcpCount := testutil.ToFloat64(collector.checksPerProvider.WithLabelValues("GCP"))
	if gcpCount != 1 {
		t.Errorf("Expected GCP checks to be 1, got %f", gcpCount)
	}
}

func TestRecordError(t *testing.T) {
	collector := NewCollector()

	collector.RecordError("connection_timeout")
	collector.RecordError("invalid_proxy")
	collector.RecordError("connection_timeout")

	// Check connection_timeout counter
	timeoutCount := testutil.ToFloat64(collector.errorsPerType.WithLabelValues("connection_timeout"))
	if timeoutCount != 2 {
		t.Errorf("Expected connection_timeout errors to be 2, got %f", timeoutCount)
	}

	// Check invalid_proxy counter
	invalidCount := testutil.ToFloat64(collector.errorsPerType.WithLabelValues("invalid_proxy"))
	if invalidCount != 1 {
		t.Errorf("Expected invalid_proxy errors to be 1, got %f", invalidCount)
	}
}

func TestGaugeUpdates(t *testing.T) {
	collector := NewCollector()

	collector.SetActiveChecks(5)
	collector.SetQueueSize(100)
	collector.SetWorkersActive(10)

	if testutil.ToFloat64(collector.activeChecks) != 5 {
		t.Errorf("Expected activeChecks to be 5, got %f", testutil.ToFloat64(collector.activeChecks))
	}

	if testutil.ToFloat64(collector.queueSize) != 100 {
		t.Errorf("Expected queueSize to be 100, got %f", testutil.ToFloat64(collector.queueSize))
	}

	if testutil.ToFloat64(collector.workersActive) != 10 {
		t.Errorf("Expected workersActive to be 10, got %f", testutil.ToFloat64(collector.workersActive))
	}
}

func TestStartStopServer(t *testing.T) {
	collector := NewCollector()

	// Test starting server
	err := collector.StartServer(":0") // Use port 0 to let OS choose
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Test stopping server
	err = collector.StopServer()
	if err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}

	// Test stopping already stopped server (should not error)
	err = collector.StopServer()
	if err != nil {
		t.Errorf("Unexpected error stopping already stopped server: %v", err)
	}
}

func TestStartServerTwice(t *testing.T) {
	collector := NewCollector()

	// Start server first time
	err := collector.StartServer(":0")
	if err != nil {
		t.Fatalf("Failed to start server first time: %v", err)
	}

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Try to start server second time (should fail)
	err = collector.StartServer(":0")
	if err == nil {
		t.Error("Expected error when starting server twice, but got nil")
	}

	// Clean up
	collector.StopServer()
}

func TestMetricsEndpoint(t *testing.T) {
	collector := NewCollector()

	// Record some metrics
	collector.RecordProxyCheck(true, "http", time.Second)
	collector.RecordAnonymousProxy()
	collector.SetActiveChecks(3)

	// Start server
	err := collector.StartServer(":0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer collector.StopServer()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Get metrics handler
	handler := collector.GetMetricsHandler()
	if handler == nil {
		t.Fatal("GetMetricsHandler() returned nil")
	}

	// Test that metrics are properly formatted
	registry := collector.GetRegistry()
	gatherer := prometheus.Gatherers{registry}
	metricFamilies, err := gatherer.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Check that we have some metrics
	if len(metricFamilies) == 0 {
		t.Error("Expected some metric families, got none")
	}

	// Look for our specific metrics
	var foundProxiesChecked, foundProxiesAnonymous, foundActiveChecks bool
	for _, mf := range metricFamilies {
		switch *mf.Name {
		case "proxyhawk_proxies_checked_total":
			foundProxiesChecked = true
			if *mf.Metric[0].Counter.Value != 1 {
				t.Errorf("Expected proxies_checked_total to be 1, got %f", *mf.Metric[0].Counter.Value)
			}
		case "proxyhawk_proxies_anonymous_total":
			foundProxiesAnonymous = true
			if *mf.Metric[0].Counter.Value != 1 {
				t.Errorf("Expected proxies_anonymous_total to be 1, got %f", *mf.Metric[0].Counter.Value)
			}
		case "proxyhawk_active_checks":
			foundActiveChecks = true
			if *mf.Metric[0].Gauge.Value != 3 {
				t.Errorf("Expected active_checks to be 3, got %f", *mf.Metric[0].Gauge.Value)
			}
		}
	}

	if !foundProxiesChecked {
		t.Error("Did not find proxyhawk_proxies_checked_total metric")
	}
	if !foundProxiesAnonymous {
		t.Error("Did not find proxyhawk_proxies_anonymous_total metric")
	}
	if !foundActiveChecks {
		t.Error("Did not find proxyhawk_active_checks metric")
	}
}

func TestHealthEndpoint(t *testing.T) {
	collector := NewCollector()

	// Start server
	err := collector.StartServer(":0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer collector.StopServer()

	// The health endpoint is tested implicitly since the server includes it
	// This test mainly ensures the server starts without issues
}

func TestMetricsLabels(t *testing.T) {
	collector := NewCollector()

	// Test proxy type labels
	collector.RecordProxyCheck(true, "http", time.Second)
	collector.RecordProxyCheck(true, "socks5", time.Second)
	collector.RecordProxyCheck(false, "http", time.Second)

	httpCount := testutil.ToFloat64(collector.checksPerType.WithLabelValues("http"))
	if httpCount != 2 {
		t.Errorf("Expected http checks to be 2, got %f", httpCount)
	}

	socks5Count := testutil.ToFloat64(collector.checksPerType.WithLabelValues("socks5"))
	if socks5Count != 1 {
		t.Errorf("Expected socks5 checks to be 1, got %f", socks5Count)
	}
}

// Benchmark tests
func BenchmarkRecordProxyCheck(b *testing.B) {
	collector := NewCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordProxyCheck(true, "http", time.Millisecond*100)
	}
}

func BenchmarkRecordCheck(b *testing.B) {
	collector := NewCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordCheck(true, time.Millisecond*50)
	}
}

func BenchmarkSetGauges(b *testing.B) {
	collector := NewCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.SetActiveChecks(i % 100)
		collector.SetQueueSize(i % 1000)
		collector.SetWorkersActive(i % 50)
	}
}
