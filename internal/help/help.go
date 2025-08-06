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
	
	// Usage
	fmt.Fprintf(b, "Usage:\n")
	fmt.Fprintf(b, "  proxyhawk [flags]\n\n")
	
	fmt.Fprintf(b, "Flags:\n")
	
	// TARGET section
	sectionHeader(b, "TARGET:", noColor)
	w := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "   -l string\ttarget proxy list file to scan (one proxy per line)\n")
	fmt.Fprintf(w, "   -config string\tconfiguration file path (default \"config/default.yaml\")\n")
	w.Flush()
	fmt.Fprintln(b)
	
	// DISCOVERY section
	sectionHeader(b, "DISCOVERY:", noColor)
	w = tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "   -discover\tenable discovery mode to find proxy candidates\n")
	fmt.Fprintf(w, "   -discover-source string\tdiscovery source (shodan,censys,freelists,webscraper,all) (default \"all\")\n")
	fmt.Fprintf(w, "   -discover-limit int\tmaximum number of candidates to discover (default 100)\n")
	fmt.Fprintf(w, "   -discover-validate\tvalidate discovered candidates immediately\n")
	w.Flush()
	fmt.Fprintln(b)
	
	// OUTPUT section  
	sectionHeader(b, "OUTPUT:", noColor)
	w = tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "   -o string\tfile to save text results\n")
	fmt.Fprintf(w, "   -j string\tfile to save JSON results\n")
	fmt.Fprintf(w, "   -wp string\tfile to save only working proxies\n")
	fmt.Fprintf(w, "   -v\tenable verbose output\n")
	fmt.Fprintf(w, "   -d\tenable debug mode with detailed logs\n")
	fmt.Fprintf(w, "   -no-ui\tdisable terminal UI (for automation/scripting)\n")
	w.Flush()
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
	fmt.Fprintf(b, "%s\n", title)
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