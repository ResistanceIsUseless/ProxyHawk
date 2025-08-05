package output

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/proxy"
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
}

// SummaryOutput represents summary statistics for output
type SummaryOutput struct {
	TotalProxies         int                  `json:"total_proxies"`
	WorkingProxies       int                  `json:"working_proxies"`
	InteractshProxies    int                  `json:"interactsh_proxies"`
	AnonymousProxies     int                  `json:"anonymous_proxies"`
	CloudProxies         int                  `json:"cloud_proxies"`
	InternalAccessCount  int                  `json:"internal_access_count"`
	MetadataAccessCount  int                  `json:"metadata_access_count"`
	SuccessRate          float64              `json:"success_rate"`
	AverageSpeed         time.Duration        `json:"average_speed_ns"`
	Results              []ProxyResultOutput  `json:"results"`
}

// ConvertToOutputFormat converts internal proxy results to output format
func ConvertToOutputFormat(results []*proxy.ProxyResult) []ProxyResultOutput {
	output := make([]ProxyResultOutput, len(results))
	for i, result := range results {
		output[i] = ProxyResultOutput{
			Proxy:          result.ProxyURL,
			Working:        result.Working,
			Speed:          result.Speed,
			InteractshTest: false, // Will be set if interactsh tests were run
			RealIP:         result.RealIP,
			ProxyIP:        result.ProxyIP,
			IsAnonymous:    result.IsAnonymous,
			CloudProvider:  result.CloudProvider,
			InternalAccess: result.InternalAccess,
			MetadataAccess: result.MetadataAccess,
			Timestamp:      time.Now(),
			Error:          result.Error,
			Type:           string(result.Type),
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

// WriteTextOutput writes results to a text file
func WriteTextOutput(filename string, results []ProxyResultOutput, summary SummaryOutput) error {
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

		fmt.Fprintf(file, "%s %s", status, result.Proxy)
		
		if result.Working {
			fmt.Fprintf(file, " - %.2fs", result.Speed.Seconds())
			if result.Type != "" {
				fmt.Fprintf(file, " (%s)", result.Type)
			}
			if result.CloudProvider != "" {
				fmt.Fprintf(file, " [%s]", result.CloudProvider)
			}
		} else if result.Error != "" {
			fmt.Fprintf(file, " - Error: %s", result.Error)
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

// WriteJSONOutput writes results to a JSON file
func WriteJSONOutput(filename string, summary SummaryOutput) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(summary)
}

// WriteWorkingProxiesOutput writes only working proxies to a file
func WriteWorkingProxiesOutput(filename string, results []ProxyResultOutput) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "# Working Proxies - Generated %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "# Format: proxy - speed\n\n")

	for _, result := range results {
		if result.Working {
			fmt.Fprintf(file, "%s - %.2fs", result.Proxy, result.Speed.Seconds())
			if result.Type != "" {
				fmt.Fprintf(file, " (%s)", result.Type)
			}
			fmt.Fprintf(file, "\n")
		}
	}

	return nil
}

// WriteAnonymousProxiesOutput writes only working anonymous proxies to a file
func WriteAnonymousProxiesOutput(filename string, results []ProxyResultOutput) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "# Working Anonymous Proxies - Generated %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "# Format: proxy - speed\n\n")

	for _, result := range results {
		if result.Working && result.IsAnonymous {
			fmt.Fprintf(file, "%s - %.2fs", result.Proxy, result.Speed.Seconds())
			if result.Type != "" {
				fmt.Fprintf(file, " (%s)", result.Type)
			}
			fmt.Fprintf(file, "\n")
		}
	}

	return nil
}