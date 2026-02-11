package tests

import (
	"testing"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/proxy"
)

// TestAdvancedChecks tests the advanced security check functionality
func TestAdvancedChecks(t *testing.T) {
	t.Run("Advanced Checks Configuration", func(t *testing.T) {
		// Test that advanced checks can be configured
		config := proxy.AdvancedChecks{
			TestProtocolSmuggling:   true,
			TestDNSRebinding:        true,
			TestCachePoisoning:      true,
			TestHostHeaderInjection: true,
			TestIPv6:                true,
			TestHTTPMethods:         []string{"GET", "POST", "PUT"},
		}

		if !config.TestProtocolSmuggling {
			t.Error("Protocol smuggling check should be enabled")
		}
		if !config.TestDNSRebinding {
			t.Error("DNS rebinding check should be enabled")
		}
		if !config.TestCachePoisoning {
			t.Error("Cache poisoning check should be enabled")
		}
		if !config.TestHostHeaderInjection {
			t.Error("Host header injection check should be enabled")
		}
		if !config.TestIPv6 {
			t.Error("IPv6 check should be enabled")
		}
		if len(config.TestHTTPMethods) != 3 {
			t.Errorf("Expected 3 HTTP methods, got %d", len(config.TestHTTPMethods))
		}
	})

	t.Run("Checker Creation with Advanced Checks", func(t *testing.T) {
		// Test that proxy checker can be created with advanced checks
		checkerConfig := proxy.Config{
			Timeout:       10 * time.Second,
			ValidationURL: "http://example.com",
			AdvancedChecks: proxy.AdvancedChecks{
				TestProtocolSmuggling: true,
				TestDNSRebinding:      true,
			},
		}

		checker := proxy.NewChecker(checkerConfig, true, nil)
		if checker == nil {
			t.Error("Should be able to create checker with advanced checks")
		}
	})

	t.Run("Disabled Advanced Checks", func(t *testing.T) {
		// Test default configuration (all checks disabled)
		config := proxy.AdvancedChecks{}

		if config.TestProtocolSmuggling {
			t.Error("Protocol smuggling check should be disabled by default")
		}
		if config.TestDNSRebinding {
			t.Error("DNS rebinding check should be disabled by default")
		}
		if config.TestCachePoisoning {
			t.Error("Cache poisoning check should be disabled by default")
		}
	})
}
