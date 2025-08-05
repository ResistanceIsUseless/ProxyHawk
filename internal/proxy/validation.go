package proxy

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// validateResponse validates the HTTP response
func (c *Checker) validateResponse(resp *http.Response, body []byte) bool {
	if resp.StatusCode >= 400 {
		return false
	}

	if len(body) < c.config.MinResponseBytes {
		return false
	}

	for _, keyword := range c.config.DisallowedKeywords {
		if strings.Contains(string(body), keyword) {
			return false
		}
	}

	return true
}

// checkAnonymity checks if the proxy is anonymous
func (c *Checker) checkAnonymity(client *http.Client) (bool, string, string, error) {
	// Implementation for checking proxy anonymity
	return false, "", "", nil // Placeholder
}

// applyRateLimit applies rate limiting based on configuration
func (c *Checker) applyRateLimit(host string, result *ProxyResult) {
	if !c.config.RateLimitEnabled {
		return
	}

	// If per-proxy rate limiting is enabled, delegate to the proxy-specific function
	if c.config.RateLimitPerProxy && result.ProxyURL != "" {
		c.applyProxyRateLimit(result.ProxyURL, result)
		return
	}

	c.rateLimiterLock.Lock()
	defer c.rateLimiterLock.Unlock()

	// Determine the key for rate limiting
	rateLimitKey := "global"
	if c.config.RateLimitPerHost {
		rateLimitKey = host
	}

	// Check if we need to wait
	if lastTime, exists := c.rateLimiter[rateLimitKey]; exists {
		elapsed := time.Since(lastTime)
		if elapsed < c.config.RateLimitDelay {
			waitTime := c.config.RateLimitDelay - elapsed
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[DEBUG] Rate limiting: waiting %v before request to %s\n", waitTime, host)
			}
			time.Sleep(waitTime)
		}
	}

	// Update the last request time
	c.rateLimiter[rateLimitKey] = time.Now()

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[DEBUG] Rate limiting applied for %s\n", rateLimitKey)
	}
}

// applyProxyRateLimit applies rate limiting per individual proxy
func (c *Checker) applyProxyRateLimit(proxyURL string, result *ProxyResult) {
	if !c.config.RateLimitEnabled {
		return
	}

	c.rateLimiterLock.Lock()
	defer c.rateLimiterLock.Unlock()

	// Use the full proxy URL as the rate limiting key for per-proxy limiting
	rateLimitKey := proxyURL

	// Check if we need to wait
	if lastTime, exists := c.rateLimiter[rateLimitKey]; exists {
		elapsed := time.Since(lastTime)
		if elapsed < c.config.RateLimitDelay {
			waitTime := c.config.RateLimitDelay - elapsed
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[DEBUG] Per-proxy rate limiting: waiting %v for proxy %s\n", waitTime, proxyURL)
			}
			time.Sleep(waitTime)
		}
	}

	// Update the last request time for this specific proxy
	c.rateLimiter[rateLimitKey] = time.Now()

	if c.debug {
		result.DebugInfo += fmt.Sprintf("[DEBUG] Per-proxy rate limiting applied for %s\n", proxyURL)
	}
}