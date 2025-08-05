package main

import (
	"fmt"
	"os"
	"strings"
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

	// Check if animation is requested
	animate := false
	for _, arg := range os.Args {
		if arg == "--animate" || arg == "-a" {
			animate = true
			break
		}
	}

	// If animation is requested and we're in debug mode, show animated view
	if animate && (viewType == "debug" || viewType == "all") {
		animateDebugView(view)
		return
	}

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
		fmt.Println("Add --animate or -a to see the animated debug view")
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
		Progress:  p,
		Total:     30,
		Current:   10,
		Metrics: ui.ViewMetrics{
			ActiveJobs:  5,
			QueueSize:   15,
			SuccessRate: 60.0,
			AvgSpeed:    320 * time.Millisecond,
		},
		DebugInfo:    createMockDebugInfo(),
		SpinnerIdx:   2,
		ActiveChecks: createMockChecks(),
		DisplayMode: ui.ViewDisplayMode{
			IsVerbose: false,
			IsDebug:   false,
		},
	}

	return view
}

func createMockChecks() map[string]*ui.CheckStatus {
	now := time.Now()
	checks := make(map[string]*ui.CheckStatus)

	// Add some sample proxies with different statuses
	checks["http://192.168.1.1:8080"] = &ui.CheckStatus{
		Proxy:          "http://192.168.1.1:8080",
		TotalChecks:    4,
		DoneChecks:     4,
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
				URL:        "GET http://example.com",
				Success:    true,
				Speed:      200 * time.Millisecond,
				StatusCode: 200,
				BodySize:   1024,
			},
			{
				URL:        "HEAD https://example.com",
				Success:    false,
				Speed:      300 * time.Millisecond,
				StatusCode: 403,
				Error:      "Access forbidden",
				BodySize:   256,
			},
			{
				URL:        "POST http://api.example.com/login",
				Success:    false,
				Speed:      150 * time.Millisecond,
				StatusCode: 401,
				Error:      "Authentication required",
				BodySize:   128,
			},
			{
				URL:        "CONNECT example.com:443",
				Success:    true,
				Speed:      175 * time.Millisecond,
				StatusCode: 200,
				BodySize:   0,
			},
		},
	}

	checks["socks5://10.0.0.1:1080"] = &ui.CheckStatus{
		Proxy:          "socks5://10.0.0.1:1080",
		TotalChecks:    3,
		DoneChecks:     3,
		LastUpdate:     now,
		Speed:          150 * time.Millisecond,
		ProxyType:      "SOCKS5",
		IsActive:       true,
		Position:       2,
		SupportsHTTP:   true,
		SupportsHTTPS:  true,
		CloudProvider:  "AWS",
		InternalAccess: true,
		MetadataAccess: false,
		CheckResults: []ui.CheckResult{
			{
				URL:        "HTTPS https://secure.example.com",
				Success:    true,
				Speed:      120 * time.Millisecond,
				StatusCode: 200,
				BodySize:   2048,
			},
			{
				URL:        "SOCKS5 example.com:22",
				Success:    true,
				Speed:      180 * time.Millisecond,
				StatusCode: 200,
				BodySize:   1024,
			},
			{
				URL:        "HTTP http://internal-service.aws.com",
				Success:    true,
				Speed:      90 * time.Millisecond,
				StatusCode: 200,
				BodySize:   4096,
			},
		},
	}

	// Add a proxy with no completed checks
	checks["http://10.10.10.10:3128"] = &ui.CheckStatus{
		Proxy:         "http://10.10.10.10:3128",
		TotalChecks:   2,
		DoneChecks:    0,
		LastUpdate:    now,
		Speed:         0,
		ProxyType:     "HTTP",
		IsActive:      true,
		Position:      3,
		SupportsHTTP:  false,
		SupportsHTTPS: false,
		CheckResults:  []ui.CheckResult{},
	}

	// Add a very long proxy URL to test display truncation
	checks["socks4://very-long-proxy-address-that-should-be-truncated.example.com:1080"] = &ui.CheckStatus{
		Proxy:         "socks4://very-long-proxy-address-that-should-be-truncated.example.com:1080",
		TotalChecks:   2,
		DoneChecks:    1,
		LastUpdate:    now,
		Speed:         350 * time.Millisecond,
		ProxyType:     "SOCKS4",
		IsActive:      true,
		Position:      4,
		SupportsHTTP:  true,
		SupportsHTTPS: false,
		CheckResults: []ui.CheckResult{
			{
				URL:        "SOCKS4 target.example.org:21",
				Success:    false,
				Speed:      350 * time.Millisecond,
				StatusCode: 404,
				Error:      "Destination unreachable",
				BodySize:   0,
			},
		},
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

// animateDebugView displays the debug view with an animated spinner
func animateDebugView(view *ui.View) {
	fmt.Println("Press Ctrl+C to exit the animated view")

	for i := 0; i < 60; i++ {
		// Clear screen by printing newlines
		fmt.Print(strings.Repeat("\n", 50))

		// Update spinner index
		view.SpinnerIdx = i

		// For every 10 frames, add a new check result to one of the proxies
		if i > 0 && i%10 == 0 {
			// Pick a proxy with the fewest check results
			var targetProxy string
			minResults := 999

			for proxy, status := range view.ActiveChecks {
				if len(status.CheckResults) < minResults {
					minResults = len(status.CheckResults)
					targetProxy = proxy
				}
			}

			if targetProxy != "" && view.ActiveChecks[targetProxy] != nil {
				status := view.ActiveChecks[targetProxy]

				// Add a new check result
				result := ui.CheckResult{
					URL:        fmt.Sprintf("GET http://example.com/path%d", i/10),
					Success:    i%20 != 0, // Every other batch has an error
					Speed:      time.Duration(100+i*5) * time.Millisecond,
					StatusCode: 200,
					BodySize:   int64(512 + i*100),
				}

				// Set error message for failed checks
				if i%20 == 0 {
					result.Error = "Connection reset"
				}

				status.CheckResults = append(status.CheckResults, result)

				status.DoneChecks++
				view.Current++
			}
		}

		// Render and display the debug view
		fmt.Println("====================== ANIMATED DEBUG VIEW ======================")
		fmt.Println(view.RenderDebug())

		// Sleep before next frame
		time.Sleep(200 * time.Millisecond)
	}
}
