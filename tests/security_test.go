package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAdvancedChecks tests the advanced security check functionality
func TestAdvancedChecks(t *testing.T) {
	// Create a mock server that responds differently based on the request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/protocol-smuggling":
			w.WriteHeader(http.StatusBadRequest)
		case "/dns-rebinding":
			w.WriteHeader(http.StatusOK)
		case "/cache-poisoning":
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(http.StatusOK)
		case "/host-header-injection":
			w.Header().Set("Location", r.Host)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	// Create a test client
	client := server.Client()

	t.Run("Protocol Smuggling Check", func(t *testing.T) {
		isVulnerable, _ := checkProtocolSmuggling(client, true)
		if isVulnerable {
			t.Error("Expected no protocol smuggling vulnerability")
		}
	})

	t.Run("DNS Rebinding Check", func(t *testing.T) {
		isVulnerable, _ := checkDNSRebinding(client, true)
		if isVulnerable {
			t.Error("Expected no DNS rebinding vulnerability")
		}
	})

	t.Run("Cache Poisoning Check", func(t *testing.T) {
		isVulnerable, _ := checkCachePoisoning(client, true)
		if isVulnerable {
			t.Error("Expected no cache poisoning vulnerability")
		}
	})

	t.Run("Host Header Injection Check", func(t *testing.T) {
		isVulnerable, _ := checkHostHeaderInjection(client, true)
		if isVulnerable {
			t.Error("Expected no host header injection vulnerability")
		}
	})
}
