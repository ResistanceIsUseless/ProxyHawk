//** this is a simple tool to fetch proxies from the proxyfetch api and save them to a file for testing purposes **//

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Base API URL without type parameters
const proxyScrapeBaseAPI = "https://api.proxyscrape.com/v4/free-proxy-list/get?request=display_proxies&proxy_format=protocolipport&format=json"

type ProxyResponse struct {
	Data []struct {
		Proxy string `json:"proxy"`
	} `json:"data"`
}

func main() {
	// Define command-line flags
	outputFileFlag := flag.String("output", "proxies.txt", "Output file path")
	proxyTypeFlag := flag.String("type", "all", "Proxy type: http, socks4, socks5, or all")

	// Add help information
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}

	// Parse command-line flags
	flag.Parse()

	// Validate proxy type
	proxyType := strings.ToLower(*proxyTypeFlag)
	validTypes := map[string]bool{
		"all":    true,
		"http":   true,
		"socks4": true,
		"socks5": true,
	}

	if !validTypes[proxyType] {
		log.Fatalf("Invalid proxy type: %s. Must be one of: all, http, socks4, socks5", proxyType)
	}

	// Build API URL with proxy type parameter if needed
	apiURL := proxyScrapeBaseAPI
	if proxyType != "all" {
		apiURL += "&protocol=" + proxyType
	}

	fmt.Printf("Fetching %s proxies and saving to %s...\n", proxyType, *outputFileFlag)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make request to proxy API
	resp, err := client.Get(apiURL)
	if err != nil {
		log.Fatalf("Error fetching proxies: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Error: API returned status code %d", resp.StatusCode)
	}

	// Read and parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response: %v", err)
	}

	var proxyResp ProxyResponse
	if err := json.Unmarshal(body, &proxyResp); err != nil {
		log.Fatalf("Error parsing JSON response: %v", err)
	}

	// Open file for writing
	file, err := os.Create(*outputFileFlag)
	if err != nil {
		log.Fatalf("Error creating output file: %v", err)
	}
	defer file.Close()

	// Write proxies to file
	count := 0
	for _, proxy := range proxyResp.Data {
		_, err := fmt.Fprintln(file, proxy.Proxy)
		if err != nil {
			log.Fatalf("Error writing to file: %v", err)
		}
		count++
	}

	fmt.Printf("Successfully saved %d %s proxies to %s\n", count, proxyType, *outputFileFlag)
}
