package proxy

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

const proxyScrapeAPI = "https://api.proxyscrape.com/v4/free-proxy-list/get?request=display_proxies&proxy_format=protocolipport&format=json"

type proxyHawkConfig struct {
	Timeout           time.Duration
	MaxConcurrent     int
	ValidationURL     string
	ValidationPattern string
}

type benchConfig struct {
	ProxyHawk proxyHawkConfig
}

type proxyResult struct {
	ProxyURL string
	Latency  time.Duration
	Working  bool
	Error    error
}

func fetchProxiesFromAPI() ([]string, error) {
	resp, err := http.Get(proxyScrapeAPI)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch proxies: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var proxies []string
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		proxy := scanner.Text()
		// Split by space in case there are multiple proxies per line
		for _, p := range strings.Fields(proxy) {
			// Convert socks4:// prefix to socks4: to match our format
			p = strings.ReplaceAll(p, "socks4://", "socks4:")
			// Convert http:// prefix to http: to match our format
			p = strings.ReplaceAll(p, "http://", "http:")
			proxies = append(proxies, p)
		}
	}

	return proxies, nil
}

func BenchmarkProxyScrapeChecking(b *testing.B) {
	// Create test config
	cfg := &benchConfig{
		ProxyHawk: proxyHawkConfig{
			Timeout:           10 * time.Second,
			MaxConcurrent:     50,
			ValidationURL:     "https://www.google.com",
			ValidationPattern: ".*",
		},
	}

	// Fetch proxies from ProxyScrape API
	resp, err := http.Get("https://api.proxyscrape.com/v2/?request=displayproxies&protocol=http&timeout=10000&country=all&ssl=all&anonymity=all")
	if err != nil {
		b.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		b.Fatal(err)
	}

	// Parse proxies and clean the URLs
	proxyList := strings.Split(string(body), "\n")
	cleanProxyList := make([]string, 0, len(proxyList))
	for _, proxy := range proxyList {
		proxy = strings.TrimSpace(proxy)
		if proxy != "" {
			cleanProxyList = append(cleanProxyList, proxy)
		}
	}
	b.Logf("Fetched %d proxies from ProxyScrape API", len(cleanProxyList))

	// Reset timer before starting proxy checks
	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		// Create channels for results and concurrency control
		results := make(chan *proxyResult, len(cleanProxyList))
		sem := make(chan struct{}, cfg.ProxyHawk.MaxConcurrent)
		var wg sync.WaitGroup

		// Check each proxy
		workingProxies := 0
		totalProxies := len(cleanProxyList)
		var totalLatency time.Duration
		var errors []error

		// Start a goroutine to close results channel when all workers are done
		wg.Add(len(cleanProxyList))
		go func() {
			wg.Wait()
			close(results)
		}()

		// Launch proxy checkers
		for _, proxy := range cleanProxyList {
			sem <- struct{}{} // Acquire semaphore

			go func(proxyStr string) {
				defer wg.Done()
				defer func() { <-sem }() // Release semaphore

				// Create proxy URL
				proxyURL, err := url.Parse("http://" + proxyStr)
				if err != nil {
					results <- &proxyResult{
						ProxyURL: proxyStr,
						Error:    err,
					}
					return
				}

				// Create HTTP client with proxy
				client := &http.Client{
					Timeout: 5 * time.Second,
					CheckRedirect: func(req *http.Request, via []*http.Request) error {
						return http.ErrUseLastResponse
					},
					Transport: &http.Transport{
						DisableKeepAlives: true,
						Proxy:             http.ProxyURL(proxyURL),
						TLSClientConfig: &tls.Config{
							InsecureSkipVerify: true,
						},
					},
				}

				// Test proxy
				start := time.Now()
				resp, err := client.Get(cfg.ProxyHawk.ValidationURL)
				if err != nil {
					results <- &proxyResult{
						ProxyURL: proxyStr,
						Error:    err,
					}
					return
				}
				defer resp.Body.Close()

				latency := time.Since(start)

				// Check response
				if resp.StatusCode != http.StatusOK {
					results <- &proxyResult{
						ProxyURL: proxyStr,
						Error:    fmt.Errorf("unexpected status code: %d", resp.StatusCode),
					}
					return
				}

				results <- &proxyResult{
					ProxyURL: proxyStr,
					Latency:  latency,
					Working:  true,
				}
			}(proxy)
		}

		// Process results
		for result := range results {
			if result.Working {
				workingProxies++
				totalLatency += result.Latency
			} else if result.Error != nil {
				errors = append(errors, fmt.Errorf("Proxy %s error: %v", result.ProxyURL, result.Error))
			}
		}

		// Log results
		b.Logf("Working proxies: %d/%d (%.2f%%)", workingProxies, totalProxies, float64(workingProxies)/float64(totalProxies)*100)
		if workingProxies > 0 {
			avgLatency := totalLatency / time.Duration(workingProxies)
			b.Logf("Average response time: %v", avgLatency)
		}

		// Log sample of errors if no working proxies found
		if workingProxies == 0 && len(errors) > 0 {
			b.Log("Sample of proxy check errors:")
			for i := 0; i < 5 && i < len(errors); i++ {
				b.Log(errors[i])
			}
		}
	}
}
