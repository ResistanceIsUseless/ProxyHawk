package cloudcheck

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"
)

// CloudProvider represents a cloud provider configuration
type CloudProvider struct {
	Name            string            `yaml:"name"`
	MetadataIPs     []string          `yaml:"metadata_ips"`
	MetadataURLs    []string          `yaml:"metadata_urls"`
	MetadataHeaders map[string]string `yaml:"metadata_headers"`
	InternalRanges  []string          `yaml:"internal_ranges"`
	ASNs            []string          `yaml:"asns"`
	OrgNames        []string          `yaml:"org_names"`
	Timeout         int               `yaml:"timeout"` // Timeout in seconds for network requests
}

// Result represents the result of cloud provider checks
type Result struct {
	Provider       *CloudProvider
	InternalAccess bool
	MetadataAccess bool
	DebugInfo      string
}

// DetectFromWhois attempts to detect a cloud provider from WHOIS data
func DetectFromWhois(whoisData string, providers []CloudProvider) *CloudProvider {
	whoisUpper := strings.ToUpper(whoisData)
	for _, provider := range providers {
		for _, orgName := range provider.OrgNames {
			if strings.Contains(whoisUpper, strings.ToUpper(orgName)) {
				return &provider
			}
		}
	}
	return nil
}

// GetWhoisInfo performs a WHOIS lookup for an IP
func GetWhoisInfo(ip string) (string, error) {
	conn, err := net.Dial("tcp", "whois.iana.org:43")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	fmt.Fprintf(conn, "%s\n", ip)
	result, err := io.ReadAll(conn)
	if err != nil {
		return "", err
	}

	// Check if we need to query a specific WHOIS server
	whoisServer := ""
	for _, line := range strings.Split(string(result), "\n") {
		if strings.HasPrefix(line, "whois:") {
			whoisServer = strings.TrimSpace(strings.TrimPrefix(line, "whois:"))
			break
		}
	}

	if whoisServer != "" {
		conn, err = net.Dial("tcp", whoisServer+":43")
		if err != nil {
			return "", err
		}
		defer conn.Close()

		fmt.Fprintf(conn, "%s\n", ip)
		result, err = io.ReadAll(conn)
		if err != nil {
			return "", err
		}
	}

	return string(result), nil
}

// GetRandomInternalIPs generates random IPs from the provider's internal ranges
func GetRandomInternalIPs(provider *CloudProvider, count int) []string {
	var ips []string
	for _, cidr := range provider.InternalRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}

		// Get first and last IP of range
		firstIP := network.IP
		lastIP := getLastIP(network)

		// Generate random IPs in this range
		for i := 0; i < count && len(ips) < count; i++ {
			randomIP := generateRandomIP(firstIP, lastIP)
			ips = append(ips, randomIP.String())
		}
	}
	return ips
}

// CheckInternalAccess tests if the proxy can access internal cloud resources
func CheckInternalAccess(client *http.Client, provider *CloudProvider, debug bool) (*Result, error) {
	debugInfo := ""
	if provider == nil {
		return &Result{DebugInfo: "No cloud provider detected"}, nil
	}

	// Set timeout if specified in provider config
	if provider.Timeout > 0 {
		client.Timeout = time.Duration(provider.Timeout) * time.Second
	}

	// Test multiple random internal IPs
	internalAccess := false
	randomIPs := GetRandomInternalIPs(provider, 5)

	for _, ip := range randomIPs {
		url := fmt.Sprintf("http://%s/", ip)
		if debug {
			debugInfo += fmt.Sprintf("\nTrying internal IP: %s\n", url)
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			if debug {
				debugInfo += fmt.Sprintf("Error creating request for %s: %v\n", url, err)
			}
			continue
		}
		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if debug {
				debugInfo += fmt.Sprintf("Internal access successful to %s!\n", ip)
			}
			internalAccess = true
			break
		}
	}

	// Test metadata endpoints
	metadataAccess := false

	// First try metadata IPs
	for _, metadataIP := range provider.MetadataIPs {
		for _, scheme := range []string{"http", "https"} {
			metadataURL := fmt.Sprintf("%s://%s/", scheme, metadataIP)
			if debug {
				debugInfo += fmt.Sprintf("\nTrying metadata IP: %s\n", metadataURL)
			}

			req, err := http.NewRequest("GET", metadataURL, nil)
			if err != nil {
				if debug {
					debugInfo += fmt.Sprintf("Error creating request for %s: %v\n", metadataURL, err)
				}
				continue
			}
			// Add provider-specific metadata headers
			for key, value := range provider.MetadataHeaders {
				req.Header.Set(key, value)
			}

			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					if debug {
						body, _ := io.ReadAll(resp.Body)
						debugInfo += fmt.Sprintf("Metadata access successful via IP! Response:\n%s\n", string(body))
					}
					metadataAccess = true
					break
				}
			}
		}
		if metadataAccess {
			break
		}
	}

	// If metadata IP access failed, try metadata URLs
	if !metadataAccess {
		for _, metadataURL := range provider.MetadataURLs {
			if debug {
				debugInfo += fmt.Sprintf("\nTrying metadata URL: %s\n", metadataURL)
			}

			req, err := http.NewRequest("GET", metadataURL, nil)
			if err != nil {
				if debug {
					debugInfo += fmt.Sprintf("Error creating request for %s: %v\n", metadataURL, err)
				}
				continue
			}
			// Add provider-specific metadata headers
			for key, value := range provider.MetadataHeaders {
				req.Header.Set(key, value)
			}

			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					if debug {
						body, _ := io.ReadAll(resp.Body)
						debugInfo += fmt.Sprintf("Metadata access successful via URL! Response:\n%s\n", string(body))
					}
					metadataAccess = true
					break
				}
			}
		}
	}

	return &Result{
		Provider:       provider,
		InternalAccess: internalAccess,
		MetadataAccess: metadataAccess,
		DebugInfo:      debugInfo,
	}, nil
}

// Helper functions
func ipToInt(ip net.IP) int64 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return int64(ip[0])<<24 | int64(ip[1])<<16 | int64(ip[2])<<8 | int64(ip[3])
}

func intToIP(i int64) net.IP {
	ip := make(net.IP, 4)
	ip[0] = byte(i >> 24)
	ip[1] = byte(i >> 16)
	ip[2] = byte(i >> 8)
	ip[3] = byte(i)
	return ip
}

func generateRandomIP(first, last net.IP) net.IP {
	firstInt := ipToInt(first)
	lastInt := ipToInt(last)
	random := firstInt + rand.Int63n(lastInt-firstInt+1)
	return intToIP(random)
}

func getLastIP(network *net.IPNet) net.IP {
	lastIP := make(net.IP, len(network.IP))
	copy(lastIP, network.IP)
	for i := len(lastIP) - 1; i >= 0; i-- {
		lastIP[i] |= ^network.Mask[i]
	}
	return lastIP
}
