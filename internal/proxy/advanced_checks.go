package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AdvancedChecks represents advanced security check settings
type AdvancedChecks struct {
	TestProtocolSmuggling   bool     `yaml:"test_protocol_smuggling"`
	TestDNSRebinding        bool     `yaml:"test_dns_rebinding"`
	TestIPv6                bool     `yaml:"test_ipv6"`
	TestHTTPMethods         []string `yaml:"test_http_methods"`
	TestCachePoisoning      bool     `yaml:"test_cache_poisoning"`
	TestHostHeaderInjection bool     `yaml:"test_host_header_injection"`
	DisableInteractsh       bool     `yaml:"disable_interactsh"` // Set to true to disable Interactsh and use basic checks
}

// AdvancedCheckResult represents the result of advanced security checks
type AdvancedCheckResult struct {
	ProtocolSmuggling   *CheckResult
	DNSRebinding        *CheckResult
	IPv6                *CheckResult
	HTTPMethods         []*CheckResult
	CachePoisoning      *CheckResult
	HostHeaderInjection *CheckResult
}

// performAdvancedChecks runs all configured advanced security checks
func (c *Checker) performAdvancedChecks(client *http.Client, result *ProxyResult) error {
	if !c.hasAdvancedChecks() {
		return nil
	}

	advancedResults := &AdvancedCheckResult{}

	// Initialize Interactsh tester unless explicitly disabled
	var tester *InteractshTester
	var err error
	if !c.config.AdvancedChecks.DisableInteractsh {
		tester, err = NewInteractshTester()
		if err != nil {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("\nFailed to initialize Interactsh tester: %v\nFalling back to basic checks.", err)
			}
		}
		if tester != nil {
			defer tester.Close()
		}
	}

	// Use validation URL as fallback test domain
	testDomain := c.config.ValidationURL
	if testDomain == "" {
		testDomain = "http://www.google.com"
	}
	if u, err := url.Parse(testDomain); err == nil {
		testDomain = u.Host
	}

	// Protocol Smuggling Test
	if c.config.AdvancedChecks.TestProtocolSmuggling {
		if tester != nil {
			res, err := tester.PerformInteractshTest(client, func(url string) (*http.Request, error) {
				req, err := http.NewRequest("POST", fmt.Sprintf("http://%s", url), strings.NewReader("test"))
				if err != nil {
					return nil, err
				}
				req.Header.Add("Content-Length", "4")
				req.Header.Add("Transfer-Encoding", "chunked")
				return req, nil
			})
			if err == nil {
				advancedResults.ProtocolSmuggling = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		} else {
			if res, err := c.checkProtocolSmuggling(client, testDomain); err == nil {
				advancedResults.ProtocolSmuggling = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		}
	}

	// DNS Rebinding Test
	if c.config.AdvancedChecks.TestDNSRebinding {
		if tester != nil {
			res, err := tester.PerformInteractshTest(client, func(url string) (*http.Request, error) {
				req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", url), nil)
				if err != nil {
					return nil, err
				}
				req.Header.Set("X-Forwarded-Host", url)
				req.Header.Set("Host", url)
				return req, nil
			})
			if err == nil {
				advancedResults.DNSRebinding = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		} else {
			if res, err := c.checkDNSRebinding(client, testDomain); err == nil {
				advancedResults.DNSRebinding = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		}
	}

	// IPv6 Test
	if c.config.AdvancedChecks.TestIPv6 {
		if tester != nil {
			res, err := tester.PerformInteractshTest(client, func(url string) (*http.Request, error) {
				return http.NewRequest("GET", fmt.Sprintf("http://[%s]", url), nil)
			})
			if err == nil {
				advancedResults.IPv6 = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		} else {
			if res, err := c.checkIPv6Support(client, testDomain); err == nil {
				advancedResults.IPv6 = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		}
	}

	// HTTP Methods Test
	if len(c.config.AdvancedChecks.TestHTTPMethods) > 0 {
		var results []*CheckResult
		if tester != nil {
			for _, method := range c.config.AdvancedChecks.TestHTTPMethods {
				res, err := tester.PerformInteractshTest(client, func(url string) (*http.Request, error) {
					return http.NewRequest(method, fmt.Sprintf("http://%s", url), nil)
				})
				if err == nil {
					results = append(results, res)
				}
			}
		} else {
			results, err = c.checkHTTPMethods(client, testDomain)
		}
		if err == nil && len(results) > 0 {
			advancedResults.HTTPMethods = results
			for _, res := range results {
				result.CheckResults = append(result.CheckResults, *res)
			}
		}
	}

	// Cache Poisoning Test
	if c.config.AdvancedChecks.TestCachePoisoning {
		if tester != nil {
			res, err := tester.PerformInteractshTest(client, func(url string) (*http.Request, error) {
				req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", url), nil)
				if err != nil {
					return nil, err
				}
				req.Header.Set("Cache-Control", "public, max-age=31536000")
				req.Header.Set("X-Cache-Control", "public, max-age=31536000")
				return req, nil
			})
			if err == nil {
				advancedResults.CachePoisoning = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		} else {
			if res, err := c.checkCachePoisoning(client, testDomain); err == nil {
				advancedResults.CachePoisoning = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		}
	}

	// Host Header Injection Test
	if c.config.AdvancedChecks.TestHostHeaderInjection {
		if tester != nil {
			res, err := tester.PerformInteractshTest(client, func(url string) (*http.Request, error) {
				req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", url), nil)
				if err != nil {
					return nil, err
				}
				req.Host = url
				req.Header.Set("X-Forwarded-Host", url)
				req.Header.Set("X-Host", url)
				req.Header.Set("X-Forwarded-Server", url)
				req.Header.Set("X-HTTP-Host-Override", url)
				return req, nil
			})
			if err == nil {
				advancedResults.HostHeaderInjection = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		} else {
			if res, err := c.checkHostHeaderInjection(client, testDomain); err == nil {
				advancedResults.HostHeaderInjection = res
				result.CheckResults = append(result.CheckResults, *res)
			}
		}
	}

	return nil
}

func (c *Checker) hasAdvancedChecks() bool {
	checks := c.config.AdvancedChecks
	return checks.TestProtocolSmuggling ||
		checks.TestDNSRebinding ||
		checks.TestIPv6 ||
		len(checks.TestHTTPMethods) > 0 ||
		checks.TestCachePoisoning ||
		checks.TestHostHeaderInjection
}

// Individual check implementations
func (c *Checker) checkProtocolSmuggling(client *http.Client, testDomain string) (*CheckResult, error) {
	result := &CheckResult{
		URL:     fmt.Sprintf("http://%s", testDomain),
		Success: false,
	}

	// Send a request with ambiguous Content-Length headers
	req, err := http.NewRequest("POST", result.URL, strings.NewReader("test"))
	if err != nil {
		return result, err
	}

	req.Header.Add("Content-Length", "4")
	req.Header.Add("Transfer-Encoding", "chunked")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	defer resp.Body.Close()

	result.Speed = time.Since(start)
	result.StatusCode = resp.StatusCode
	result.Success = resp.StatusCode < 400

	return result, nil
}

func (c *Checker) checkDNSRebinding(client *http.Client, testDomain string) (*CheckResult, error) {
	result := &CheckResult{
		URL:     fmt.Sprintf("http://%s", testDomain),
		Success: false,
	}

	req, err := http.NewRequest("GET", result.URL, nil)
	if err != nil {
		return result, err
	}

	// Add headers to test DNS rebinding
	req.Header.Set("X-Forwarded-Host", testDomain)
	req.Header.Set("Host", testDomain)

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	defer resp.Body.Close()

	result.Speed = time.Since(start)
	result.StatusCode = resp.StatusCode
	result.Success = resp.StatusCode < 400

	return result, nil
}

func (c *Checker) checkIPv6Support(client *http.Client, testDomain string) (*CheckResult, error) {
	result := &CheckResult{
		URL:     fmt.Sprintf("http://[%s]", testDomain),
		Success: false,
	}

	req, err := http.NewRequest("GET", result.URL, nil)
	if err != nil {
		return result, err
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	defer resp.Body.Close()

	result.Speed = time.Since(start)
	result.StatusCode = resp.StatusCode
	result.Success = resp.StatusCode < 400

	return result, nil
}

func (c *Checker) checkHTTPMethods(client *http.Client, testDomain string) ([]*CheckResult, error) {
	var results []*CheckResult
	baseURL := fmt.Sprintf("http://%s", testDomain)

	for _, method := range c.config.AdvancedChecks.TestHTTPMethods {
		result := &CheckResult{
			URL: baseURL,
		}

		req, err := http.NewRequest(method, baseURL, nil)
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}

		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		resp.Body.Close()

		result.Speed = time.Since(start)
		result.StatusCode = resp.StatusCode
		result.Success = resp.StatusCode < 400

		results = append(results, result)
	}

	return results, nil
}

func (c *Checker) checkCachePoisoning(client *http.Client, testDomain string) (*CheckResult, error) {
	result := &CheckResult{
		URL:     fmt.Sprintf("http://%s", testDomain),
		Success: false,
	}

	req, err := http.NewRequest("GET", result.URL, nil)
	if err != nil {
		return result, err
	}

	// Add cache control headers
	req.Header.Set("Cache-Control", "public, max-age=31536000")
	req.Header.Set("X-Cache-Control", "public, max-age=31536000")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	defer resp.Body.Close()

	result.Speed = time.Since(start)
	result.StatusCode = resp.StatusCode
	result.Success = resp.StatusCode < 400

	return result, nil
}

func (c *Checker) checkHostHeaderInjection(client *http.Client, testDomain string) (*CheckResult, error) {
	result := &CheckResult{
		URL:     fmt.Sprintf("http://%s", testDomain),
		Success: false,
	}

	req, err := http.NewRequest("GET", result.URL, nil)
	if err != nil {
		return result, err
	}

	// Test host header injection with various headers
	req.Host = testDomain
	req.Header.Set("X-Forwarded-Host", testDomain)
	req.Header.Set("X-Host", testDomain)
	req.Header.Set("X-Forwarded-Server", testDomain)
	req.Header.Set("X-HTTP-Host-Override", testDomain)

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	defer resp.Body.Close()

	result.Speed = time.Since(start)
	result.StatusCode = resp.StatusCode
	result.Success = resp.StatusCode < 400

	return result, nil
}
