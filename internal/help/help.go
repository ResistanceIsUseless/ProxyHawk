package help

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

const (
	// Version information
	Version = "1.0.0"
	AppName = "ProxyHawk"
	
	// Colors for terminal output
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// HelpSection represents a section of help documentation
type HelpSection struct {
	Title   string
	Content string
}

// Example represents a usage example
type Example struct {
	Description string
	Command     string
	Explanation string
}

// GetBanner returns the application banner
func GetBanner(noColor bool) string {
	if noColor {
		return fmt.Sprintf(`
%s v%s - Advanced Proxy Checker and Validator
Security Testing | Performance Analysis | Cloud Detection
`, AppName, Version)
	}
	
	return fmt.Sprintf(`
%s%s%s v%s - %sAdvanced Proxy Checker and Validator%s
%sSecurity Testing | Performance Analysis | Cloud Detection%s
`, colorBold+colorBlue, AppName, colorReset, Version, colorBold, colorReset, colorCyan, colorReset)
}

// GetQuickStart returns quick start guide
func GetQuickStart(noColor bool) string {
	b := &strings.Builder{}
	
	header := "QUICK START"
	if !noColor {
		header = colorBold + colorGreen + header + colorReset
	}
	
	fmt.Fprintf(b, "\n%s\n\n", header)
	fmt.Fprintf(b, "1. Create a proxy list file (one proxy per line):\n")
	fmt.Fprintf(b, "   http://proxy1.example.com:8080\n")
	fmt.Fprintf(b, "   socks5://proxy2.example.com:1080\n")
	fmt.Fprintf(b, "   proxy3.example.com:3128\n\n")
	
	fmt.Fprintf(b, "2. Run ProxyHawk:\n")
	if noColor {
		fmt.Fprintf(b, "   proxyhawk -l proxy-list.txt\n\n")
	} else {
		fmt.Fprintf(b, "   %sproxyhawk -l proxy-list.txt%s\n\n", colorCyan, colorReset)
	}
	
	fmt.Fprintf(b, "3. Save results:\n")
	if noColor {
		fmt.Fprintf(b, "   proxyhawk -l proxy-list.txt -o results.txt -j results.json\n")
	} else {
		fmt.Fprintf(b, "   %sproxyhawk -l proxy-list.txt -o results.txt -j results.json%s\n", colorCyan, colorReset)
	}
	
	return b.String()
}

// GetFullHelp returns the complete help text
func GetFullHelp(noColor bool) string {
	b := &strings.Builder{}
	
	// Banner
	fmt.Fprint(b, GetBanner(noColor))
	
	// Synopsis
	sectionHeader(b, "SYNOPSIS", noColor)
	fmt.Fprintf(b, "  proxyhawk -l PROXY_LIST [OPTIONS]\n\n")
	
	// Core Options
	sectionHeader(b, "CORE OPTIONS", noColor)
	w := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	
	fmt.Fprintf(w, "  -l, --list FILE\t\tFile containing proxy list (required)\n")
	fmt.Fprintf(w, "  -config FILE\t\tConfiguration file path (default: config/default.yaml)\n")
	fmt.Fprintf(w, "  -c, --concurrency N\t\tNumber of concurrent checks\n")
	fmt.Fprintf(w, "  -t, --timeout SECONDS\t\tTimeout per proxy check\n")
	fmt.Fprintf(w, "  -v, --verbose\t\tEnable verbose output\n")
	fmt.Fprintf(w, "  -d, --debug\t\tEnable debug mode (detailed logs)\n")
	fmt.Fprintf(w, "  -h, --help\t\tShow this help message\n")
	fmt.Fprintf(w, "  --version\t\tShow version information\n")
	w.Flush()
	fmt.Fprintln(b)
	
	// Output Options
	sectionHeader(b, "OUTPUT OPTIONS", noColor)
	w = tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	
	fmt.Fprintf(w, "  -o, --output FILE\t\tSave results to text file\n")
	fmt.Fprintf(w, "  -j, --json FILE\t\tSave results as JSON\n")
	fmt.Fprintf(w, "  -wp FILE\t\tSave only working proxies\n")
	fmt.Fprintf(w, "  -wpa FILE\t\tSave only anonymous proxies\n")
	fmt.Fprintf(w, "  --no-ui\t\tDisable terminal UI (for automation)\n")
	w.Flush()
	fmt.Fprintln(b)
	
	// Progress Options
	sectionHeader(b, "PROGRESS OPTIONS (--no-ui mode)", noColor)
	w = tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	
	fmt.Fprintf(w, "  --progress TYPE\t\tProgress indicator type:\n")
	fmt.Fprintf(w, "  \t\t  none     - No progress output\n")
	fmt.Fprintf(w, "  \t\t  basic    - Text-based progress\n")
	fmt.Fprintf(w, "  \t\t  bar      - Progress bar (default)\n")
	fmt.Fprintf(w, "  \t\t  spinner  - Animated spinner\n")
	fmt.Fprintf(w, "  \t\t  dots     - Dot progress\n")
	fmt.Fprintf(w, "  \t\t  percent  - Percentage only\n")
	fmt.Fprintf(w, "  --progress-width N\t\tProgress bar width (default: 50)\n")
	fmt.Fprintf(w, "  --progress-no-color\t\tDisable colored progress output\n")
	w.Flush()
	fmt.Fprintln(b)
	
	// Security Options
	sectionHeader(b, "SECURITY & TESTING OPTIONS", noColor)
	w = tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	
	fmt.Fprintf(w, "  -r, --rdns\t\tUse reverse DNS for host headers\n")
	fmt.Fprintf(w, "  --rate-limit\t\tEnable rate limiting\n")
	fmt.Fprintf(w, "  --rate-delay DURATION\t\tDelay between requests (e.g., 500ms, 1s)\n")
	fmt.Fprintf(w, "  --rate-per-host\t\tApply rate limit per host (default)\n")
	fmt.Fprintf(w, "  --rate-per-proxy\t\tApply rate limit per proxy\n")
	w.Flush()
	fmt.Fprintln(b)
	
	// Advanced Options
	sectionHeader(b, "ADVANCED OPTIONS", noColor)
	w = tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	
	fmt.Fprintf(w, "  --hot-reload\t\tEnable config hot-reloading\n")
	fmt.Fprintf(w, "  --metrics\t\tEnable Prometheus metrics\n")
	fmt.Fprintf(w, "  --metrics-addr ADDR\t\tMetrics server address (default: :9090)\n")
	fmt.Fprintf(w, "  --metrics-path PATH\t\tMetrics endpoint path (default: /metrics)\n")
	w.Flush()
	fmt.Fprintln(b)
	
	// Examples
	examples := GetExamples()
	sectionHeader(b, "EXAMPLES", noColor)
	for i, ex := range examples {
		fmt.Fprintf(b, "  %d. %s\n", i+1, ex.Description)
		if noColor {
			fmt.Fprintf(b, "     $ %s\n", ex.Command)
		} else {
			fmt.Fprintf(b, "     $ %s%s%s\n", colorCyan, ex.Command, colorReset)
		}
		if ex.Explanation != "" {
			fmt.Fprintf(b, "     %s\n", ex.Explanation)
		}
		fmt.Fprintln(b)
	}
	
	// Security Features
	sectionHeader(b, "SECURITY FEATURES", noColor)
	fmt.Fprintln(b, "  • SSRF Detection - Tests for Server-Side Request Forgery vulnerabilities")
	fmt.Fprintln(b, "  • Host Header Injection - Multiple injection vectors and bypass techniques")
	fmt.Fprintln(b, "  • Protocol Smuggling - HTTP request smuggling detection")
	fmt.Fprintln(b, "  • Cloud Provider Detection - Identifies AWS, GCP, Azure, and others")
	fmt.Fprintln(b, "  • Anonymity Checking - Verifies if proxy hides your real IP")
	fmt.Fprintln(b, "  • Internal Network Scanning - Detects access to private networks")
	fmt.Fprintln(b, "  • XSS Prevention - Sanitizes all output to prevent code injection")
	fmt.Fprintln(b)
	
	// Configuration
	sectionHeader(b, "CONFIGURATION", noColor)
	fmt.Fprintln(b, "  ProxyHawk uses YAML configuration files for advanced settings.")
	fmt.Fprintln(b, "  Default config: config/default.yaml")
	fmt.Fprintln(b, "")
	fmt.Fprintln(b, "  Key configuration sections:")
	fmt.Fprintln(b, "  • concurrency     - Number of concurrent workers")
	fmt.Fprintln(b, "  • timeout         - Proxy check timeout")
	fmt.Fprintln(b, "  • validation      - Response validation rules")
	fmt.Fprintln(b, "  • cloud_providers - Cloud provider definitions")
	fmt.Fprintln(b, "  • advanced_checks - Security testing options")
	fmt.Fprintln(b, "  • retry           - Retry mechanism settings")
	fmt.Fprintln(b, "  • auth            - Proxy authentication settings")
	fmt.Fprintln(b)
	
	// Environment
	sectionHeader(b, "ENVIRONMENT", noColor)
	fmt.Fprintln(b, "  PROXYHAWK_CONFIG    Path to default configuration file")
	fmt.Fprintln(b, "  PROXYHAWK_NO_COLOR  Disable colored output (set to 1)")
	fmt.Fprintln(b, "  HTTP_PROXY          HTTP proxy for outbound connections")
	fmt.Fprintln(b, "  HTTPS_PROXY         HTTPS proxy for outbound connections")
	fmt.Fprintln(b, "  NO_PROXY            Comma-separated list of hosts to bypass proxy")
	fmt.Fprintln(b)
	
	// More Information
	sectionHeader(b, "MORE INFORMATION", noColor)
	fmt.Fprintln(b, "  GitHub:  https://github.com/ResistanceIsUseless/ProxyHawk")
	fmt.Fprintln(b, "  Issues:  https://github.com/ResistanceIsUseless/ProxyHawk/issues")
	fmt.Fprintln(b, "  Docs:    https://github.com/ResistanceIsUseless/ProxyHawk/wiki")
	fmt.Fprintln(b)
	
	return b.String()
}

// GetExamples returns usage examples
func GetExamples() []Example {
	return []Example{
		{
			Description: "Basic proxy checking",
			Command:     "proxyhawk -l proxies.txt",
			Explanation: "",
		},
		{
			Description: "Check with custom concurrency and timeout",
			Command:     "proxyhawk -l proxies.txt -c 50 -t 10",
			Explanation: "Uses 50 concurrent workers with 10-second timeout",
		},
		{
			Description: "Save results in multiple formats",
			Command:     "proxyhawk -l proxies.txt -o results.txt -j results.json -wp working.txt",
			Explanation: "Saves text report, JSON data, and working proxy list",
		},
		{
			Description: "Enable security testing with debug output",
			Command:     "proxyhawk -l proxies.txt -d --config security-config.yaml",
			Explanation: "Uses security configuration with detailed debug logs",
		},
		{
			Description: "Non-interactive mode for automation",
			Command:     "proxyhawk -l proxies.txt --no-ui --progress basic -o results.txt",
			Explanation: "Runs without TUI, shows basic progress, saves results",
		},
		{
			Description: "Rate-limited checking",
			Command:     "proxyhawk -l proxies.txt --rate-limit --rate-delay 2s",
			Explanation: "Adds 2-second delay between requests to avoid bans",
		},
		{
			Description: "Docker deployment with monitoring",
			Command:     "docker-compose up -d && docker-compose logs -f proxyhawk",
			Explanation: "Starts ProxyHawk with Prometheus and Grafana monitoring",
		},
		{
			Description: "Hot-reload configuration",
			Command:     "proxyhawk -l proxies.txt --hot-reload --config custom.yaml",
			Explanation: "Watches config file for changes and reloads automatically",
		},
		{
			Description: "Extract only anonymous proxies",
			Command:     "proxyhawk -l proxies.txt -wpa anonymous-only.txt --no-ui",
			Explanation: "Saves only proxies that hide your real IP",
		},
		{
			Description: "Production deployment with metrics",
			Command:     "proxyhawk -l proxies.txt --metrics --metrics-addr :9090 -c 100",
			Explanation: "Enables Prometheus metrics on port 9090 with high concurrency",
		},
	}
}

// PrintHelp prints help to the specified writer
func PrintHelp(w io.Writer, noColor bool) {
	fmt.Fprint(w, GetFullHelp(noColor))
}

// PrintQuickStart prints quick start guide
func PrintQuickStart(w io.Writer, noColor bool) {
	fmt.Fprint(w, GetBanner(noColor))
	fmt.Fprint(w, GetQuickStart(noColor))
}

// PrintVersion prints version information
func PrintVersion(w io.Writer, noColor bool) {
	if noColor {
		fmt.Fprintf(w, "%s version %s\n", AppName, Version)
	} else {
		fmt.Fprintf(w, "%s%s%s version %s%s%s\n", 
			colorBold+colorBlue, AppName, colorReset,
			colorGreen, Version, colorReset)
	}
	fmt.Fprintln(w, "Advanced proxy checker with security testing capabilities")
	fmt.Fprintln(w, "https://github.com/ResistanceIsUseless/ProxyHawk")
}

// PrintUsageError prints a usage error with suggestion
func PrintUsageError(w io.Writer, err error, noColor bool) {
	if noColor {
		fmt.Fprintf(w, "Error: %v\n\n", err)
		fmt.Fprintf(w, "Usage: proxyhawk -l PROXY_LIST [OPTIONS]\n")
		fmt.Fprintf(w, "Try 'proxyhawk --help' for more information.\n")
	} else {
		fmt.Fprintf(w, "%sError:%s %v\n\n", colorRed, colorReset, err)
		fmt.Fprintf(w, "Usage: %sproxyhawk -l PROXY_LIST [OPTIONS]%s\n", colorCyan, colorReset)
		fmt.Fprintf(w, "Try '%sproxyhawk --help%s' for more information.\n", colorYellow, colorReset)
	}
}

// sectionHeader adds a formatted section header
func sectionHeader(b *strings.Builder, title string, noColor bool) {
	if noColor {
		fmt.Fprintf(b, "%s\n", title)
	} else {
		fmt.Fprintf(b, "%s%s%s\n", colorBold+colorYellow, title, colorReset)
	}
}

// DetectNoColor checks if color should be disabled
func DetectNoColor() bool {
	// Check environment variable
	if os.Getenv("PROXYHAWK_NO_COLOR") == "1" {
		return true
	}
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	
	// Check if output is not a terminal
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		return true
	}
	
	return false
}