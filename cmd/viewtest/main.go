package main

import (
	"fmt"
	"os"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/ui"
	"github.com/charmbracelet/bubbles/progress"
)

func main() {
	// Check if an argument was provided to specify which view to show
	viewType := "all"
	if len(os.Args) > 1 {
		viewType = os.Args[1]
	}

	// Create mock data for UI testing
	view := createMockView()

	// Display the appropriate view(s)
	switch viewType {
	case "default":
		fmt.Println(view.RenderDefault())
	case "verbose":
		fmt.Println(view.RenderVerbose())
	case "debug":
		fmt.Println(view.RenderDebug())
	case "all":
		// Display all views for comparison
		fmt.Println("====================== DEFAULT VIEW ======================")
		fmt.Println(view.RenderDefault())
		fmt.Println("\n\n====================== VERBOSE VIEW ======================")
		fmt.Println(view.RenderVerbose())
		fmt.Println("\n\n====================== DEBUG VIEW ======================")
		fmt.Println(view.RenderDebug())
	default:
		fmt.Printf("Unknown view type: %s\nValid options: default, verbose, debug, all\n", viewType)
	}
}

func createMockView() *ui.View {
	// Create progress bar
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)
	p.SetPercent(0.35) // 35% completed

	// Create the view with mock data
	view := &ui.View{
		Progress:     p,
		Total:        30,
		Current:      10,
		ActiveJobs:   5,
		QueueSize:    15,
		SuccessRate:  60.0,
		AvgSpeed:     320 * time.Millisecond,
		DebugInfo:    createMockDebugInfo(),
		SpinnerIdx:   2,
		ActiveChecks: createMockChecks(),
		IsVerbose:    false,
		IsDebug:      false,
	}

	return view
}

func createMockChecks() map[string]*ui.CheckStatus {
	now := time.Now()
	checks := make(map[string]*ui.CheckStatus)

	// Add some sample proxies with different statuses
	checks["http://192.168.1.1:8080"] = &ui.CheckStatus{
		Proxy:          "http://192.168.1.1:8080",
		TotalChecks:    2,
		DoneChecks:     2,
		LastUpdate:     now,
		Speed:          250 * time.Millisecond,
		ProxyType:      "HTTP",
		IsActive:       true,
		Position:       1,
		SupportsHTTP:   true,
		SupportsHTTPS:  false,
		InternalAccess: false,
		MetadataAccess: false,
		CheckResults: []ui.CheckResult{
			{
				URL:        "http://example.com",
				Success:    true,
				Speed:      200 * time.Millisecond,
				StatusCode: 200,
				BodySize:   1024,
			},
			{
				URL:        "https://example.com",
				Success:    false,
				Speed:      300 * time.Millisecond,
				StatusCode: 403,
				Error:      "Access forbidden",
				BodySize:   256,
			},
		},
	}

	checks["socks5://10.0.0.1:1080"] = &ui.CheckStatus{
		Proxy:          "socks5://10.0.0.1:1080",
		TotalChecks:    2,
		DoneChecks:     2,
		LastUpdate:     now,
		Speed:          150 * time.Millisecond,
		ProxyType:      "SOCKS5",
		IsActive:       true,
		Position:       2,
		SupportsHTTP:   true,
		SupportsHTTPS:  true,
		CloudProvider:  "AWS",
		InternalAccess: true,
		MetadataAccess: true,
		CheckResults: []ui.CheckResult{
			{
				URL:        "http://example.com",
				Success:    true,
				Speed:      120 * time.Millisecond,
				StatusCode: 200,
				BodySize:   1024,
			},
			{
				URL:        "https://example.com",
				Success:    true,
				Speed:      180 * time.Millisecond,
				StatusCode: 200,
				BodySize:   1024,
			},
		},
	}

	checks["socks4://172.16.1.5:4145"] = &ui.CheckStatus{
		Proxy:       "socks4://172.16.1.5:4145",
		TotalChecks: 2,
		DoneChecks:  1,
		LastUpdate:  now,
		ProxyType:   "SOCKS4",
		IsActive:    true,
		Position:    3,
		CheckResults: []ui.CheckResult{
			{
				URL:        "http://example.com",
				Success:    true,
				Speed:      350 * time.Millisecond,
				StatusCode: 200,
				BodySize:   1024,
			},
		},
	}

	checks["http://11.22.33.44:3128"] = &ui.CheckStatus{
		Proxy:         "http://11.22.33.44:3128",
		TotalChecks:   2,
		DoneChecks:    2,
		LastUpdate:    now,
		Speed:         450 * time.Millisecond,
		ProxyType:     "HTTP",
		IsActive:      true,
		Position:      4,
		SupportsHTTP:  false,
		SupportsHTTPS: false,
		CheckResults: []ui.CheckResult{
			{
				URL:        "http://example.com",
				Success:    false,
				Error:      "Connection refused",
				StatusCode: 0,
			},
			{
				URL:        "https://example.com",
				Success:    false,
				Error:      "Connection timeout",
				StatusCode: 0,
			},
		},
	}

	checks["http://veryveryverylongproxyhostname.com:8888"] = &ui.CheckStatus{
		Proxy:       "http://veryveryverylongproxyhostname.com:8888",
		TotalChecks: 2,
		DoneChecks:  0,
		LastUpdate:  now,
		IsActive:    true,
		Position:    5,
	}

	return checks
}

func createMockDebugInfo() string {
	return `[DEBUG] Starting proxy checks with concurrency: 5
[DEBUG] Total proxies to check: 30
[DEBUG] Worker 1 started
[DEBUG] Worker 2 started
[DEBUG] Worker 3 started
[DEBUG] Worker 4 started
[DEBUG] Worker 5 started
[DEBUG] Worker 1 checking: http://192.168.1.1:8080
[DEBUG] Proxy URL: http://192.168.1.1:8080
[DEBUG] Using original scheme: http
[DEBUG] Testing as HTTP proxy
[DEBUG] Making request to: http://example.com
[DEBUG] Response received in 200ms: Status: 200 OK
[DEBUG] Success! Working as HTTP proxy with HTTP endpoint
[DEBUG] Testing with HTTPS endpoint
[DEBUG] Making request to: https://example.com
[DEBUG] Error: Access forbidden
[DEBUG] HTTP proxy supports HTTP but not HTTPS
[DEBUG] Worker 1 success: http://192.168.1.1:8080 (HTTP)
[DEBUG] Worker 2 checking: socks5://10.0.0.1:1080
[DEBUG] Proxy URL: socks5://10.0.0.1:1080
[DEBUG] Using original scheme: socks5
[DEBUG] Testing as SOCKS5 proxy
[DEBUG] Making request to: http://example.com
[DEBUG] Response received in 120ms: Status: 200 OK
[DEBUG] Success! Working as SOCKS5 proxy with HTTP endpoint
[DEBUG] Testing with HTTPS endpoint
[DEBUG] Making request to: https://example.com
[DEBUG] Response received in 180ms: Status: 200 OK
[DEBUG] Success! Working as SOCKS5 proxy with HTTPS endpoint
[DEBUG] SOCKS5 proxy supports both HTTP and HTTPS
[DEBUG] Cloud provider detected: AWS
[DEBUG] Internal IP access: YES
[DEBUG] Metadata access: YES
[DEBUG] Worker 2 success: socks5://10.0.0.1:1080 (SOCKS5)
[DEBUG] Worker 3 checking: socks4://172.16.1.5:4145
[DEBUG] Proxy URL: socks4://172.16.1.5:4145
[DEBUG] Using original scheme: socks4
[DEBUG] Testing as SOCKS4 proxy
[DEBUG] Making request to: http://example.com
[DEBUG] Response received in 350ms: Status: 200 OK
[DEBUG] Success! Working as SOCKS4 proxy with HTTP endpoint
[DEBUG] Testing with HTTPS endpoint
[DEBUG] Worker 4 checking: http://11.22.33.44:3128
[DEBUG] Proxy URL: http://11.22.33.44:3128
[DEBUG] Using original scheme: http
[DEBUG] Testing as HTTP proxy
[DEBUG] Making request to: http://example.com
[DEBUG] Error: Connection refused
[DEBUG] Failed to use as HTTP proxy: Connection refused
[DEBUG] Testing as HTTPS proxy
[DEBUG] Making request to: http://example.com
[DEBUG] Error: Connection refused
[DEBUG] Failed to use as HTTPS proxy: Connection refused
[DEBUG] Testing as SOCKS4 proxy
[DEBUG] Making request to: http://example.com
[DEBUG] Error: Connection timeout
[DEBUG] Failed to use as SOCKS4 proxy: Connection timeout
[DEBUG] Testing as SOCKS5 proxy
[DEBUG] Making request to: http://example.com
[DEBUG] Error: Connection timeout
[DEBUG] Failed to use as SOCKS5 proxy: Connection timeout
[DEBUG] Worker 4 failed: http://11.22.33.44:3128 - Could not determine proxy type
[DEBUG] Worker 5 checking: http://veryveryverylongproxyhostname.com:8888`
}
