package validation

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// ValidationError represents a validation error with details
type ValidationError struct {
	Field   string
	Value   string
	Message string
	Code    ValidationErrorCode
}

// ValidationErrorCode represents different types of validation errors
type ValidationErrorCode int

const (
	ErrorInvalidURL ValidationErrorCode = iota
	ErrorInvalidHost
	ErrorInvalidPort
	ErrorInvalidScheme
	ErrorPrivateIP
	ErrorInvalidFormat
	ErrorUnsupportedProtocol
)

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error in %s: %s (value: %s)", e.Field, e.Message, e.Value)
}

// ProxyValidator provides comprehensive proxy URL validation
type ProxyValidator struct {
	allowPrivateIPs    bool
	supportedSchemes   []string
	maxHostnameLength  int
	maxPortNumber      int
}

// NewProxyValidator creates a new proxy validator with default settings
func NewProxyValidator() *ProxyValidator {
	return &ProxyValidator{
		allowPrivateIPs:   true, // Allow private IPs for internal infrastructure scanning
		supportedSchemes:  []string{"http", "https", "socks4", "socks5"},
		maxHostnameLength: 253,
		maxPortNumber:     65535,
	}
}

// WithAllowPrivateIPs configures whether private IPs are allowed
func (v *ProxyValidator) WithAllowPrivateIPs(allow bool) *ProxyValidator {
	v.allowPrivateIPs = allow
	return v
}

// WithSupportedSchemes sets the supported proxy schemes
func (v *ProxyValidator) WithSupportedSchemes(schemes []string) *ProxyValidator {
	v.supportedSchemes = make([]string, len(schemes))
	copy(v.supportedSchemes, schemes)
	return v
}

// ValidateProxyURL validates a complete proxy URL
func (v *ProxyValidator) ValidateProxyURL(proxyURL string) error {
	if strings.TrimSpace(proxyURL) == "" {
		return ValidationError{
			Field:   "proxy_url",
			Value:   proxyURL,
			Message: "proxy URL cannot be empty",
			Code:    ErrorInvalidURL,
		}
	}

	// Parse the URL
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return ValidationError{
			Field:   "proxy_url",
			Value:   proxyURL,
			Message: fmt.Sprintf("failed to parse URL: %v", err),
			Code:    ErrorInvalidURL,
		}
	}

	// Validate scheme
	if err := v.validateScheme(parsed.Scheme); err != nil {
		return err
	}

	// Validate host and port
	if err := v.validateHost(parsed.Host); err != nil {
		return err
	}

	// Additional checks for specific schemes
	if err := v.validateSchemeSpecific(parsed); err != nil {
		return err
	}

	return nil
}

// ValidateProxyAddress validates a host:port combination
func (v *ProxyValidator) ValidateProxyAddress(address string) error {
	if strings.TrimSpace(address) == "" {
		return ValidationError{
			Field:   "proxy_address",
			Value:   address,
			Message: "proxy address cannot be empty",
			Code:    ErrorInvalidFormat,
		}
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		// Try to parse as host without port
		if !strings.Contains(address, ":") {
			return v.validateHostname(address)
		}
		return ValidationError{
			Field:   "proxy_address",
			Value:   address,
			Message: fmt.Sprintf("invalid host:port format: %v", err),
			Code:    ErrorInvalidFormat,
		}
	}

	// Validate host
	if err := v.validateHostname(host); err != nil {
		return err
	}

	// Validate port
	if err := v.validatePort(port); err != nil {
		return err
	}

	return nil
}

// validateScheme checks if the URL scheme is supported
func (v *ProxyValidator) validateScheme(scheme string) error {
	if scheme == "" {
		return ValidationError{
			Field:   "scheme",
			Value:   scheme,
			Message: "URL scheme is required",
			Code:    ErrorInvalidScheme,
		}
	}

	scheme = strings.ToLower(scheme)
	for _, supported := range v.supportedSchemes {
		if scheme == supported {
			return nil
		}
	}

	return ValidationError{
		Field:   "scheme",
		Value:   scheme,
		Message: fmt.Sprintf("unsupported scheme (supported: %v)", v.supportedSchemes),
		Code:    ErrorUnsupportedProtocol,
	}
}

// validateHost validates the host portion of a URL
func (v *ProxyValidator) validateHost(host string) error {
	if host == "" {
		return ValidationError{
			Field:   "host",
			Value:   host,
			Message: "host is required",
			Code:    ErrorInvalidHost,
		}
	}

	// Split host and port if they're combined
	hostname, port, err := net.SplitHostPort(host)
	if err != nil {
		// Host without port
		hostname = host
	} else {
		// Validate port if present
		if err := v.validatePort(port); err != nil {
			return err
		}
	}

	return v.validateHostname(hostname)
}

// validateHostname validates a hostname or IP address
func (v *ProxyValidator) validateHostname(hostname string) error {
	if hostname == "" {
		return ValidationError{
			Field:   "hostname",
			Value:   hostname,
			Message: "hostname cannot be empty",
			Code:    ErrorInvalidHost,
		}
	}

	if len(hostname) > v.maxHostnameLength {
		return ValidationError{
			Field:   "hostname",
			Value:   hostname,
			Message: fmt.Sprintf("hostname too long (max: %d characters)", v.maxHostnameLength),
			Code:    ErrorInvalidHost,
		}
	}

	// Check if it's an IP address
	if ip := net.ParseIP(hostname); ip != nil {
		return v.validateIPAddress(ip)
	}

	// Validate as hostname
	return v.validateHostnameFormat(hostname)
}

// validateIPAddress validates an IP address
func (v *ProxyValidator) validateIPAddress(ip net.IP) error {
	// Check for private IP addresses if not allowed
	if !v.allowPrivateIPs && isPrivateIP(ip) {
		return ValidationError{
			Field:   "ip_address",
			Value:   ip.String(),
			Message: "private IP addresses are not allowed",
			Code:    ErrorPrivateIP,
		}
	}

	// Check for loopback, multicast, etc.
	if ip.IsLoopback() {
		return ValidationError{
			Field:   "ip_address",
			Value:   ip.String(),
			Message: "loopback IP addresses are not allowed",
			Code:    ErrorPrivateIP,
		}
	}

	if ip.IsMulticast() {
		return ValidationError{
			Field:   "ip_address",
			Value:   ip.String(),
			Message: "multicast IP addresses are not allowed",
			Code:    ErrorInvalidHost,
		}
	}

	return nil
}

// validateHostnameFormat validates hostname format according to RFC specifications
func (v *ProxyValidator) validateHostnameFormat(hostname string) error {
	// Basic hostname validation regex
	// RFC 1123 compliant hostname regex
	hostnameRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`)
	
	if !hostnameRegex.MatchString(hostname) {
		return ValidationError{
			Field:   "hostname",
			Value:   hostname,
			Message: "invalid hostname format",
			Code:    ErrorInvalidHost,
		}
	}

	// Check for consecutive dots
	if strings.Contains(hostname, "..") {
		return ValidationError{
			Field:   "hostname",
			Value:   hostname,
			Message: "hostname cannot contain consecutive dots",
			Code:    ErrorInvalidHost,
		}
	}

	// Check if hostname starts or ends with a dot
	if strings.HasPrefix(hostname, ".") || strings.HasSuffix(hostname, ".") {
		return ValidationError{
			Field:   "hostname",
			Value:   hostname,
			Message: "hostname cannot start or end with a dot",
			Code:    ErrorInvalidHost,
		}
	}

	return nil
}

// validatePort validates a port number
func (v *ProxyValidator) validatePort(portStr string) error {
	if portStr == "" {
		return ValidationError{
			Field:   "port",
			Value:   portStr,
			Message: "port is required",
			Code:    ErrorInvalidPort,
		}
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return ValidationError{
			Field:   "port",
			Value:   portStr,
			Message: "port must be a number",
			Code:    ErrorInvalidPort,
		}
	}

	if port <= 0 || port > v.maxPortNumber {
		return ValidationError{
			Field:   "port",
			Value:   portStr,
			Message: fmt.Sprintf("port must be between 1 and %d", v.maxPortNumber),
			Code:    ErrorInvalidPort,
		}
	}

	return nil
}

// validateSchemeSpecific performs scheme-specific validation
func (v *ProxyValidator) validateSchemeSpecific(parsed *url.URL) error {
	switch strings.ToLower(parsed.Scheme) {
	case "socks4", "socks5":
		// SOCKS proxies shouldn't have path, query, or fragment
		if parsed.Path != "" && parsed.Path != "/" {
			return ValidationError{
				Field:   "path",
				Value:   parsed.Path,
				Message: "SOCKS URLs should not contain a path",
				Code:    ErrorInvalidURL,
			}
		}
		if parsed.RawQuery != "" {
			return ValidationError{
				Field:   "query",
				Value:   parsed.RawQuery,
				Message: "SOCKS URLs should not contain query parameters",
				Code:    ErrorInvalidURL,
			}
		}
		if parsed.Fragment != "" {
			return ValidationError{
				Field:   "fragment",
				Value:   parsed.Fragment,
				Message: "SOCKS URLs should not contain fragments",
				Code:    ErrorInvalidURL,
			}
		}
	}

	return nil
}

// NormalizeProxyURL normalizes a proxy URL for consistent handling
func (v *ProxyValidator) NormalizeProxyURL(proxyURL string) (string, error) {
	// Trim whitespace
	proxyURL = strings.TrimSpace(proxyURL)
	
	// Remove trailing slashes
	proxyURL = strings.TrimRight(proxyURL, "/")

	// Add scheme if missing
	if !strings.Contains(proxyURL, "://") {
		proxyURL = "http://" + proxyURL
	}

	// Parse to validate and reformat
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return "", ValidationError{
			Field:   "proxy_url",
			Value:   proxyURL,
			Message: fmt.Sprintf("failed to parse URL: %v", err),
			Code:    ErrorInvalidURL,
		}
	}

	// Validate the parsed URL
	if err := v.ValidateProxyURL(parsed.String()); err != nil {
		return "", err
	}

	// Return normalized URL
	return parsed.String(), nil
}

// isPrivateIP checks if an IP address is private or reserved
func isPrivateIP(ip net.IP) bool {
	// Handle both IPv4 and IPv6
	if ip.To4() != nil {
		return isPrivateIPv4(ip)
	}
	return isPrivateIPv6(ip)
}

// isPrivateIPv4 checks if an IPv4 address is private or reserved
func isPrivateIPv4(ip net.IP) bool {
	// RFC 1918 private address ranges
	privateRanges := []string{
		"10.0.0.0/8",     // Private-use networks
		"172.16.0.0/12",  // Private-use networks
		"192.168.0.0/16", // Private-use networks
	}

	// RFC 6598 Carrier-grade NAT
	carrierGradeNAT := "100.64.0.0/10"

	// RFC 3927 Link-local
	linkLocal := "169.254.0.0/16"

	// RFC 5737 IPv4 test networks (documentation/examples)
	testNetworks := []string{
		"192.0.2.0/24",    // TEST-NET-1
		"198.51.100.0/24", // TEST-NET-2
		"203.0.113.0/24",  // TEST-NET-3
	}

	// Additional reserved ranges
	// NOTE: We only block truly private/internal ranges, not Class E (240.0.0.0/4)
	// as some of these addresses may be allocated for public use
	reservedRanges := []string{
		"0.0.0.0/8",       // Current network (RFC 1122)
		"127.0.0.0/8",     // Loopback (RFC 1122)
		"224.0.0.0/4",     // Multicast (RFC 5771)
		// Note: 240.0.0.0/4 (Class E) historically reserved but may be used
		// We're being more permissive here to allow potentially valid proxies
		"255.255.255.255/32", // Limited broadcast (RFC 919)
	}

	// Combine all ranges
	allRanges := []string{}
	allRanges = append(allRanges, privateRanges...)
	allRanges = append(allRanges, carrierGradeNAT, linkLocal)
	allRanges = append(allRanges, testNetworks...)
	allRanges = append(allRanges, reservedRanges...)

	for _, cidr := range allRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// isPrivateIPv6 checks if an IPv6 address is private or reserved
func isPrivateIPv6(ip net.IP) bool {
	// RFC 4193 Unique Local Addresses
	uniqueLocal := "fc00::/7"

	// RFC 4291 Link-local addresses
	linkLocal := "fe80::/10"

	// RFC 3849 IPv6 documentation prefix
	documentation := "2001:db8::/32"

	// Additional reserved IPv6 ranges
	reservedRanges := []string{
		"::/128",         // Unspecified address
		"::1/128",        // Loopback address
		"::ffff:0:0/96",  // IPv4-mapped addresses
		"100::/64",       // Discard prefix
		"2001::/32",      // Teredo tunneling
		"2001:10::/28",   // Deprecated (ORCHID)
		"2001:20::/28",   // ORCHIDv2
		"2002::/16",      // 6to4
		"ff00::/8",       // Multicast
	}

	// Combine all ranges
	allRanges := []string{uniqueLocal, linkLocal, documentation}
	allRanges = append(allRanges, reservedRanges...)

	for _, cidr := range allRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// BatchValidateProxies validates multiple proxy URLs and returns results
func (v *ProxyValidator) BatchValidateProxies(proxies []string) map[string]error {
	results := make(map[string]error)
	for _, proxy := range proxies {
		results[proxy] = v.ValidateProxyURL(proxy)
	}
	return results
}