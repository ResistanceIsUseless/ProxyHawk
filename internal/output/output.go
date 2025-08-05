package output

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/proxy"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/sanitizer"
)

// ProxyResultOutput represents a proxy result for output formatting
type ProxyResultOutput struct {
	Proxy          string        `json:"proxy"`
	Working        bool          `json:"working"`
	Speed          time.Duration `json:"speed_ns"`
	InteractshTest bool          `json:"interactsh_test"`
	RealIP         string        `json:"real_ip,omitempty"`
	ProxyIP        string        `json:"proxy_ip,omitempty"`
	IsAnonymous    bool          `json:"is_anonymous"`
	CloudProvider  string        `json:"cloud_provider,omitempty"`
	InternalAccess bool          `json:"internal_access"`
	MetadataAccess bool          `json:"metadata_access"`
	Timestamp      time.Time     `json:"timestamp"`
	Error          string        `json:"error,omitempty"`
	Type           string        `json:"type,omitempty"`
	
	// Protocol support information
	ProtocolSupport ProtocolSupport `json:"protocol_support"`
}

// ProtocolSupport represents which protocols a proxy supports
type ProtocolSupport struct {
	HTTP   bool `json:"http"`
	HTTPS  bool `json:"https"`
	HTTP2  bool `json:"http2"`
	HTTP3  bool `json:"http3"`
	SOCKS4 bool `json:"socks4"`
	SOCKS5 bool `json:"socks5"`
}

// SummaryOutput represents summary statistics for output
type SummaryOutput struct {
	TotalProxies        int                 `json:"total_proxies"`
	WorkingProxies      int                 `json:"working_proxies"`
	InteractshProxies   int                 `json:"interactsh_proxies"`
	AnonymousProxies    int                 `json:"anonymous_proxies"`
	CloudProxies        int                 `json:"cloud_proxies"`
	InternalAccessCount int                 `json:"internal_access_count"`
	MetadataAccessCount int                 `json:"metadata_access_count"`
	SuccessRate         float64             `json:"success_rate"`
	AverageSpeed        time.Duration       `json:"average_speed_ns"`
	Results             []ProxyResultOutput `json:"results"`
}

// ConvertToOutputFormat converts internal proxy results to output format with sanitization
func ConvertToOutputFormat(results []*proxy.ProxyResult) []ProxyResultOutput {
	return ConvertToOutputFormatWithSanitizer(results, sanitizer.DefaultSanitizer())
}

// ConvertToOutputFormatWithSanitizer converts internal proxy results to output format with custom sanitization
func ConvertToOutputFormatWithSanitizer(results []*proxy.ProxyResult, s *sanitizer.Sanitizer) []ProxyResultOutput {
	output := make([]ProxyResultOutput, len(results))
	for i, result := range results {
		errorMsg := ""
		if result.Error != nil {
			errorMsg = s.SanitizeError(result.Error.Error())
		}

		output[i] = ProxyResultOutput{
			Proxy:          s.SanitizeURL(result.ProxyURL),
			Working:        result.Working,
			Speed:          result.Speed,
			InteractshTest: false, // Will be set if interactsh tests were run
			RealIP:         s.SanitizeIP(result.RealIP),
			ProxyIP:        s.SanitizeIP(result.ProxyIP),
			IsAnonymous:    result.IsAnonymous,
			CloudProvider:  s.SanitizeString(result.CloudProvider),
			InternalAccess: result.InternalAccess,
			MetadataAccess: result.MetadataAccess,
			Timestamp:      time.Now(),
			Error:          errorMsg,
			Type:           s.SanitizeString(string(result.Type)),
			ProtocolSupport: ProtocolSupport{
				HTTP:   result.SupportsHTTP,
				HTTPS:  result.SupportsHTTPS,
				HTTP2:  result.SupportsHTTP2,
				HTTP3:  result.SupportsHTTP3,
				SOCKS4: result.Type == proxy.ProxyTypeSOCKS4,
				SOCKS5: result.Type == proxy.ProxyTypeSOCKS5,
			},
		}
	}
	return output
}

// GenerateSummary creates a summary from proxy results
func GenerateSummary(results []*proxy.ProxyResult) SummaryOutput {
	output := ConvertToOutputFormat(results)

	summary := SummaryOutput{
		TotalProxies: len(results),
		Results:      output,
	}

	var totalSpeed time.Duration
	var speedCount int

	for _, result := range results {
		if result.Working {
			summary.WorkingProxies++
			if result.Speed > 0 {
				totalSpeed += result.Speed
				speedCount++
			}
		}

		if result.IsAnonymous {
			summary.AnonymousProxies++
		}

		if result.CloudProvider != "" {
			summary.CloudProxies++
		}

		if result.InternalAccess {
			summary.InternalAccessCount++
		}

		if result.MetadataAccess {
			summary.MetadataAccessCount++
		}
	}

	if summary.TotalProxies > 0 {
		summary.SuccessRate = float64(summary.WorkingProxies) / float64(summary.TotalProxies) * 100
	}

	if speedCount > 0 {
		summary.AverageSpeed = totalSpeed / time.Duration(speedCount)
	}

	return summary
}

// WriteTextOutput writes results to a text file with sanitization
func WriteTextOutput(filename string, results []ProxyResultOutput, summary SummaryOutput) error {
	return WriteTextOutputWithSanitizer(filename, results, summary, sanitizer.DefaultSanitizer())
}

// WriteTextOutputWithSanitizer writes results to a text file with custom sanitization
func WriteTextOutputWithSanitizer(filename string, results []ProxyResultOutput, summary SummaryOutput, s *sanitizer.Sanitizer) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write header
	fmt.Fprintf(file, "ProxyHawk Results - %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "=====================================\n\n")

	// Write individual results
	for _, result := range results {
		status := "❌"
		if result.Working {
			status = "✅"
			if result.IsAnonymous {
				status += "🔒"
			}
			if result.CloudProvider != "" {
				status += "☁️"
			}
			if result.InternalAccess {
				status += "⚠️"
			}
		}

		// Results are already sanitized, but we apply additional text-specific sanitization
		proxy := s.SanitizeString(result.Proxy)
		fmt.Fprintf(file, "%s %s", status, proxy)

		if result.Working {
			fmt.Fprintf(file, " - %.2fs", result.Speed.Seconds())
			if result.Type != "" {
				proxyType := s.SanitizeString(result.Type)
				fmt.Fprintf(file, " (%s)", proxyType)
			}
			if result.CloudProvider != "" {
				cloudProvider := s.SanitizeString(result.CloudProvider)
				fmt.Fprintf(file, " [%s]", cloudProvider)
			}
		} else if result.Error != "" {
			errorMsg := s.SanitizeError(result.Error)
			fmt.Fprintf(file, " - Error: %s", errorMsg)
		}

		fmt.Fprintf(file, "\n")
	}

	// Write summary
	fmt.Fprintf(file, "\n=====================================\n")
	fmt.Fprintf(file, "SUMMARY\n")
	fmt.Fprintf(file, "=====================================\n")
	fmt.Fprintf(file, "Total proxies tested: %d\n", summary.TotalProxies)
	fmt.Fprintf(file, "Working proxies: %d\n", summary.WorkingProxies)
	fmt.Fprintf(file, "Anonymous proxies: %d\n", summary.AnonymousProxies)
	fmt.Fprintf(file, "Cloud proxies: %d\n", summary.CloudProxies)
	fmt.Fprintf(file, "Success rate: %.2f%%\n", summary.SuccessRate)

	if summary.AverageSpeed > 0 {
		fmt.Fprintf(file, "Average speed: %.2fs\n", summary.AverageSpeed.Seconds())
	}

	return nil
}

// WriteJSONOutput writes results to a JSON file with sanitization
func WriteJSONOutput(filename string, summary SummaryOutput) error {
	return WriteJSONOutputWithSanitizer(filename, summary, sanitizer.DefaultSanitizer())
}

// WriteJSONOutputWithSanitizer writes results to a JSON file with custom sanitization
func WriteJSONOutputWithSanitizer(filename string, summary SummaryOutput, s *sanitizer.Sanitizer) error {
	// Sanitize the summary before writing
	sanitizedSummary := sanitizeSummaryOutput(summary, s)

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(true) // Ensure HTML escaping is enabled
	return encoder.Encode(sanitizedSummary)
}

// sanitizeSummaryOutput applies sanitization to all string fields in summary
func sanitizeSummaryOutput(summary SummaryOutput, s *sanitizer.Sanitizer) SummaryOutput {
	// The results are already sanitized by ConvertToOutputFormatWithSanitizer
	// This function is for future extensibility if more fields need sanitization
	return summary
}

// WriteWorkingProxiesOutput writes only working proxies to a file with sanitization
func WriteWorkingProxiesOutput(filename string, results []ProxyResultOutput) error {
	return WriteWorkingProxiesOutputWithSanitizer(filename, results, sanitizer.DefaultSanitizer())
}

// WriteWorkingProxiesOutputWithSanitizer writes only working proxies to a file with custom sanitization
func WriteWorkingProxiesOutputWithSanitizer(filename string, results []ProxyResultOutput, s *sanitizer.Sanitizer) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "# Working Proxies - Generated %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "# Format: proxy - speed\n\n")

	for _, result := range results {
		if result.Working {
			proxy := s.SanitizeURL(result.Proxy)
			fmt.Fprintf(file, "%s - %.2fs", proxy, result.Speed.Seconds())
			if result.Type != "" {
				proxyType := s.SanitizeString(result.Type)
				fmt.Fprintf(file, " (%s)", proxyType)
			}
			fmt.Fprintf(file, "\n")
		}
	}

	return nil
}

// WriteAnonymousProxiesOutput writes only working anonymous proxies to a file with sanitization
func WriteAnonymousProxiesOutput(filename string, results []ProxyResultOutput) error {
	return WriteAnonymousProxiesOutputWithSanitizer(filename, results, sanitizer.DefaultSanitizer())
}

// WriteAnonymousProxiesOutputWithSanitizer writes only working anonymous proxies to a file with custom sanitization
func WriteAnonymousProxiesOutputWithSanitizer(filename string, results []ProxyResultOutput, s *sanitizer.Sanitizer) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "# Working Anonymous Proxies - Generated %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "# Format: proxy - speed\n\n")

	for _, result := range results {
		if result.Working && result.IsAnonymous {
			proxy := s.SanitizeURL(result.Proxy)
			fmt.Fprintf(file, "%s - %.2fs", proxy, result.Speed.Seconds())
			if result.Type != "" {
				proxyType := s.SanitizeString(result.Type)
				fmt.Fprintf(file, " (%s)", proxyType)
			}
			fmt.Fprintf(file, "\n")
		}
	}

	return nil
}
