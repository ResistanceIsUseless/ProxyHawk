# ProxyHawk Check Modes Proposal

## Executive Summary

Proposed 3-tier check system with optional Interactsh enhancement:
1. **Basic** - Fast proxy validation (current behavior)
2. **Intense** - Thorough proxy testing (anonymity, cloud detection, protocol variants)
3. **Vulns** - Security vulnerability testing (SSRF, header injection, smuggling)

Plus: **Interactsh flag** - Enables out-of-band detection for all modes (when applicable)

## Current State Analysis

### What We Have Now

**Configuration Structure:**
- Config file: `config/default.yaml`
- Advanced checks defined but never executed
- Cloud checks available but optional
- Anonymity checking stubbed out (placeholder)

**Command Line Flags:**
- `-v` (verbose) - More output
- `-d` (debug) - Debug logging
- `-config` - Config file path
- Various output flags, rate limiting, etc.

**Missing:**
- No mode selection mechanism
- No integration of advanced checks
- No way to enable intensive testing
- Interactsh configured but unused

### What We Need

A clear, user-friendly way to select check depth without overwhelming users with granular options.

## Proposed Solution: 3-Tier Check Modes

### Mode 1: Basic (Default)

**Purpose:** Fast proxy validation for bulk checking

**What It Does:**
- Protocol detection (HTTP/HTTPS/SOCKS4/SOCKS5)
- Basic connectivity test
- Response validation (status, size, keywords)
- Speed measurement
- Protocol support detection (SupportsHTTP, SupportsHTTPS)

**What It Skips:**
- Anonymity checking
- Cloud provider detection
- Advanced protocol testing (HTTP/2, HTTP/3)
- Security vulnerability tests
- Multiple test URLs

**Performance:**
- ~3-7 requests per proxy
- ~1-15 seconds per proxy
- Suitable for 500+ proxy lists

**Use Cases:**
- Bulk proxy list validation
- Quick filtering of dead proxies
- Finding working proxies fast
- Automated proxy rotation systems

**Command:**
```bash
# Explicit (default behavior)
./proxyhawk -l proxies.txt --mode basic

# Implicit (no mode flag = basic)
./proxyhawk -l proxies.txt
```

---

### Mode 2: Intense

**Purpose:** Thorough proxy testing for quality assessment

**What It Adds to Basic:**
- **Anonymity Detection:**
  - Real IP detection
  - Headers leak checking (X-Forwarded-For, Via, X-Real-IP)
  - Multiple IP check services
  - Anonymity level classification (transparent/anonymous/elite)

- **Cloud Provider Detection:**
  - Tests against AWS, GCP, Azure, DigitalOcean metadata endpoints
  - Detects if proxy is in cloud infrastructure
  - Checks for internal network access
  - Cloud provider classification

- **Advanced Protocol Testing:**
  - HTTP/2 support detection (if enabled)
  - HTTP/3 support detection (if enabled)
  - Multiple test URL validation
  - Protocol upgrade capabilities

- **Extended Validation:**
  - Tests multiple validation URLs from config
  - Requires majority success (not just first success)
  - Connection stability testing
  - Timeout behavior analysis

**Performance:**
- ~15-25 requests per proxy (without HTTP/2/3)
- ~20-40 seconds per proxy
- Suitable for 50-200 proxy lists

**Use Cases:**
- Quality proxy vetting
- Finding elite anonymous proxies
- Cloud infrastructure mapping
- Proxy service validation

**Command:**
```bash
./proxyhawk -l proxies.txt --mode intense

# With HTTP/2 and HTTP/3
./proxyhawk -l proxies.txt --mode intense --http2 --http3
```

**Configuration Integration:**
Uses existing config settings:
- `enable_cloud_checks: true` (auto-enabled in intense mode)
- `enable_anonymity_check: true` (auto-enabled in intense mode)
- `test_urls.urls` (tests multiple URLs instead of just default)
- `cloud_providers` (from config file)

---

### Mode 3: Vulns (Vulnerability Testing)

**Purpose:** Security vulnerability assessment

**What It Adds to Intense:**
- **SSRF Testing:**
  - 60+ internal target tests (localhost, metadata services, private IPs)
  - Port scanning detection (15 common ports)
  - DNS rebinding protection tests
  - IPv6 localhost variations
  - Cloud metadata endpoint access
  - Alternative IP encoding (octal, hex, decimal)

- **Host Header Injection:**
  - 62+ injection target tests
  - 11 different header injection vectors
  - CRLF injection attempts
  - Null byte injection
  - HTTP/1.0 bypass techniques
  - Conflicting header tests

- **Protocol Smuggling:**
  - Content-Length vs Transfer-Encoding conflicts
  - HTTP request splitting tests
  - Chunked encoding manipulation

- **DNS Rebinding:**
  - DNS resolution manipulation tests
  - Time-based DNS attacks
  - Host header DNS rebinding

- **Cache Poisoning:**
  - Cache control header manipulation
  - Unkeyed input injection
  - Cache key poisoning attempts

- **Advanced HTTP Method Testing:**
  - Tests PUT, DELETE, PATCH, OPTIONS, TRACE, CONNECT
  - Method override header tests
  - Dangerous method exposure

**Performance:**
- ~130-150 requests per proxy
- ~60-120 seconds per proxy
- Suitable for 5-20 proxy lists (targeted testing)

**Use Cases:**
- Security auditing
- Pentest reconnaissance
- Bug bounty research
- Security compliance checking
- Red team operations

**Command:**
```bash
./proxyhawk -l proxies.txt --mode vulns

# With verbose output to see findings
./proxyhawk -l proxies.txt --mode vulns -v

# With debug for full details
./proxyhawk -l proxies.txt --mode vulns -d
```

**Configuration Integration:**
Uses `advanced_checks` from config:
```yaml
advanced_checks:
  test_protocol_smuggling: true   # Auto-enabled in vulns mode
  test_dns_rebinding: true         # Auto-enabled in vulns mode
  test_ipv6: true                  # Auto-enabled in vulns mode
  test_http_methods: ["GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"]
  test_cache_poisoning: true       # Auto-enabled in vulns mode
  test_host_header_injection: true # Auto-enabled in vulns mode
  test_ssrf: true                  # Auto-enabled in vulns mode
```

---

## Interactsh Integration (Cross-Mode Enhancement)

**Purpose:** Out-of-band interaction detection for vulnerability confirmation

### What Interactsh Does

Interactsh is a service for detecting out-of-band interactions:
1. Generates unique subdomain (e.g., `abc123.interact.sh`)
2. ProxyHawk injects this into requests
3. If proxy/target makes DNS/HTTP requests to that domain, it's detected
4. Confirms vulnerabilities that might be blind

### When It Helps

**In Intense Mode:**
- Confirms DNS leaks in anonymity tests
- Detects proxy chaining/nesting
- Identifies proxy middleware

**In Vulns Mode:**
- Confirms blind SSRF vulnerabilities
- Validates DNS rebinding attacks
- Detects blind command injection
- Proves HTTP smuggling impact
- Confirms cache poisoning

### How It Works

**Without Interactsh (Default):**
- Tests rely on response codes and timing
- Some vulnerabilities may be missed (blind SSRF)
- Faster but less accurate

**With Interactsh:**
- Generates unique callback URLs
- Injects into payloads
- Polls for interactions
- Confirms vulnerabilities definitively
- Slower but more accurate

### Usage

**Enable for any mode:**
```bash
# Basic mode with Interactsh (not useful, no applicable tests)
./proxyhawk -l proxies.txt --interactsh

# Intense mode with Interactsh (useful for anonymity tests)
./proxyhawk -l proxies.txt --mode intense --interactsh

# Vulns mode with Interactsh (highly recommended)
./proxyhawk -l proxies.txt --mode vulns --interactsh
```

**Configuration:**
```yaml
interactsh_url: "https://interact.sh"  # Public server
interactsh_token: ""                   # Optional for auth

# Or use private server
interactsh_url: "https://my-interactsh.company.com"
interactsh_token: "secret-token-here"
```

**Per-Mode Behavior:**
- **Basic:** Interactsh flag ignored (no applicable tests)
- **Intense:** Uses Interactsh for anonymity/DNS tests if available
- **Vulns:** Uses Interactsh for all vulnerability tests (fallback to basic tests if unavailable)

### Performance Impact

With Interactsh enabled:
- Adds 2-5 seconds per check (polling delay)
- More accurate results
- Fewer false positives
- Recommended for vulns mode

---

## Implementation Plan

### Phase 1: Core Mode System

#### 1.1 Add Mode Flag

**File:** `cmd/proxyhawk/main.go`

Add new flag:
```go
checkMode := flag.String("mode", "basic", "Check mode: basic, intense, vulns")
interactsh := flag.Bool("interactsh", false, "Enable Interactsh for out-of-band detection")
```

#### 1.2 Add Mode Type

**File:** `internal/proxy/types.go`

```go
// CheckMode represents the depth of proxy checking
type CheckMode string

const (
	CheckModeBasic  CheckMode = "basic"   // Fast validation only
	CheckModeIntense CheckMode = "intense" // + Anonymity, cloud, protocols
	CheckModeVulns   CheckMode = "vulns"   // + Security vulnerability tests
)

// Add to Config struct:
CheckMode        CheckMode
EnableInteractsh bool
```

#### 1.3 Update Config Structure

**File:** `internal/config/config.go`

```go
// Add to Config struct:
CheckMode        string `yaml:"check_mode"`        // Default mode
EnableInteractsh bool   `yaml:"enable_interactsh"` // Enable Interactsh by default
```

### Phase 2: Implement Check Logic

#### 2.1 Update Checker.Check()

**File:** `internal/proxy/checker.go`

Modify the `Check()` function to call different check levels:

```go
func (c *Checker) Check(proxyURL string) *ProxyResult {
	result := &ProxyResult{
		ProxyURL:     proxyURL,
		Type:         ProxyTypeUnknown,
		CheckResults: []CheckResult{},
	}

	// Phase 1: Basic checks (always run)
	if err := c.performBasicChecks(result); err != nil {
		return result
	}

	// Phase 2: Intense checks (if mode >= intense)
	if c.config.CheckMode >= CheckModeIntense {
		if err := c.performIntenseChecks(result); err != nil {
			// Log but don't fail - intense checks are supplementary
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[INTENSE] Some checks failed: %v\n", err)
			}
		}
	}

	// Phase 3: Vulnerability checks (if mode == vulns)
	if c.config.CheckMode == CheckModeVulns {
		if err := c.performAdvancedChecks(client, result); err != nil {
			// Log but don't fail - vuln checks are supplementary
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[VULNS] Some checks failed: %v\n", err)
			}
		}
	}

	return result
}
```

#### 2.2 Refactor Basic Checks

Extract current `Check()` logic into `performBasicChecks()`:

```go
func (c *Checker) performBasicChecks(result *ProxyResult) error {
	// Current logic from Check() function
	// - determineProxyType()
	// - performChecks() (validation)
	return nil
}
```

#### 2.3 Implement Intense Checks

**File:** `internal/proxy/intense_checks.go` (NEW FILE)

```go
package proxy

func (c *Checker) performIntenseChecks(client *http.Client, result *ProxyResult) error {
	// Anonymity checking
	if c.config.EnableAnonymityCheck {
		if err := c.checkAnonymity(client, result); err != nil {
			// Log error but continue
		}
	}

	// Cloud provider detection
	if c.config.EnableCloudChecks {
		if err := c.checkCloudProvider(client, result); err != nil {
			// Log error but continue
		}
	}

	// Multiple URL validation
	if err := c.checkMultipleURLs(client, result); err != nil {
		// Log error but continue
	}

	// HTTP/2 and HTTP/3 if enabled
	if c.config.EnableHTTP2 {
		c.checkHTTP2Support(client, result)
	}
	if c.config.EnableHTTP3 {
		c.checkHTTP3Support(client, result)
	}

	return nil
}

func (c *Checker) checkAnonymity(client *http.Client, result *ProxyResult) error {
	// Test multiple IP check services
	services := []string{
		"https://api.ipify.org?format=json",
		"https://httpbin.org/ip",
		"http://ip-api.com/json/",
	}

	var realIP string
	var proxyIP string

	// Get real IP (direct connection)
	realIP, _ = c.getRealIP()

	// Get IP through proxy
	proxyIP, _ = c.getProxyIP(client, services)

	// Check for leaking headers
	leakyHeaders := c.checkHeaderLeaks(client)

	// Determine anonymity level
	if proxyIP == realIP {
		result.IsAnonymous = false
		result.AnonymityLevel = "transparent"
	} else if len(leakyHeaders) > 0 {
		result.IsAnonymous = false
		result.AnonymityLevel = "anonymous"
	} else {
		result.IsAnonymous = true
		result.AnonymityLevel = "elite"
	}

	result.RealIP = realIP
	result.ProxyIP = proxyIP

	return nil
}

func (c *Checker) checkCloudProvider(client *http.Client, result *ProxyResult) error {
	// Test each configured cloud provider
	for _, provider := range c.config.CloudProviders {
		// Try to access metadata endpoints
		for _, metadataURL := range provider.MetadataURLs {
			if c.testMetadataAccess(client, metadataURL, provider.MetadataHeaders) {
				result.CloudProvider = provider.Name
				result.MetadataAccess = true
				return nil
			}
		}

		// Test internal network access
		for _, ipRange := range provider.InternalRanges {
			if c.testInternalAccess(client, ipRange) {
				result.InternalAccess = true
			}
		}
	}

	return nil
}

func (c *Checker) checkMultipleURLs(client *http.Client, result *ProxyResult) error {
	successCount := 0
	requiredSuccess := len(c.config.TestURLs.TestURLs) / 2 + 1 // Majority

	for _, testURL := range c.config.TestURLs.TestURLs {
		checkResult, err := c.performSingleCheck(client, testURL.URL, result)
		if err == nil && checkResult.Success {
			successCount++
		}
		result.CheckResults = append(result.CheckResults, *checkResult)
	}

	if successCount < requiredSuccess {
		return fmt.Errorf("only %d/%d test URLs succeeded", successCount, len(c.config.TestURLs.TestURLs))
	}

	return nil
}
```

#### 2.4 Integrate Advanced Checks

**File:** `internal/proxy/advanced_checks.go`

Update `performAdvancedChecks()` to use Interactsh if enabled:

```go
func (c *Checker) performAdvancedChecks(client *http.Client, result *ProxyResult) error {
	if !c.hasAdvancedChecks() {
		return nil
	}

	// Initialize Interactsh if enabled and not disabled in config
	var tester *InteractshTester
	var err error
	if c.config.EnableInteractsh && !c.config.AdvancedChecks.DisableInteractsh {
		tester, err = NewInteractshTester()
		if err != nil {
			if c.debug {
				result.DebugInfo += fmt.Sprintf("[INTERACTSH] Failed to init: %v. Using basic tests.\n", err)
			}
			tester = nil // Fallback to basic tests
		}
		if tester != nil {
			defer tester.Close()
		}
	}

	// Rest of existing logic...
	// Each test checks if tester != nil and uses it, otherwise falls back
}
```

### Phase 3: Configuration Updates

#### 3.1 Update default.yaml

**File:** `config/default.yaml`

```yaml
# Check mode configuration
check_mode: basic  # Options: basic, intense, vulns

# Interactsh settings for out-of-band detection
enable_interactsh: false  # Enable by default (can be overridden with --interactsh flag)
interactsh_url: "https://interact.sh"
interactsh_token: ""

# These are now auto-enabled based on mode, but can be explicitly disabled
enable_cloud_checks: false     # Auto true in intense/vulns modes
enable_anonymity_check: false  # Auto true in intense/vulns modes

# Advanced checks - auto-enabled in vulns mode
advanced_checks:
  test_protocol_smuggling: false
  test_dns_rebinding: false
  test_ipv6: false
  test_http_methods: ["GET"]
  test_cache_poisoning: false
  test_host_header_injection: false
  test_ssrf: false
  disable_interactsh: false
```

#### 3.2 Mode-Based Config Overrides

**File:** `cmd/proxyhawk/main.go`

Apply automatic config overrides based on mode:

```go
// Apply mode-based configuration overrides
switch *checkMode {
case "basic":
	// Use config as-is (or force disable extras)
	cfg.EnableCloudChecks = false
	cfg.EnableAnonymityCheck = false
	// Don't override advanced_checks (leave all false)

case "intense":
	// Enable comprehensive checking
	cfg.EnableCloudChecks = true
	cfg.EnableAnonymityCheck = true
	// Advanced checks stay off

case "vulns":
	// Enable everything
	cfg.EnableCloudChecks = true
	cfg.EnableAnonymityCheck = true
	// Enable all advanced checks
	cfg.AdvancedChecks.TestProtocolSmuggling = true
	cfg.AdvancedChecks.TestDNSRebinding = true
	cfg.AdvancedChecks.TestIPv6 = true
	cfg.AdvancedChecks.TestHTTPMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "TRACE"}
	cfg.AdvancedChecks.TestCachePoisoning = true
	cfg.AdvancedChecks.TestHostHeaderInjection = true
	cfg.AdvancedChecks.TestSSRF = true

default:
	logger.Error("Invalid check mode", "mode", *checkMode)
	fmt.Fprintf(os.Stderr, "Invalid mode: %s. Must be basic, intense, or vulns\n", *checkMode)
	os.Exit(1)
}

// Apply Interactsh flag (overrides config)
if *interactsh {
	cfg.EnableInteractsh = true
}
```

### Phase 4: Output Enhancements

#### 4.1 Add Mode-Specific Output Fields

**File:** `internal/proxy/types.go`

```go
type ProxyResult struct {
	// Existing fields...

	// Intense mode fields
	IsAnonymous      bool
	AnonymityLevel   string // "transparent", "anonymous", "elite"
	RealIP           string
	ProxyIP          string
	CloudProvider    string
	InternalAccess   bool
	MetadataAccess   bool

	// Vulns mode fields
	VulnerabilityFindings []VulnerabilityFinding
	SecurityScore         int // 0-100, lower is worse
}

type VulnerabilityFinding struct {
	Type        string // "SSRF", "HeaderInjection", "ProtocolSmuggling", etc.
	Severity    string // "Critical", "High", "Medium", "Low"
	Description string
	Evidence    string
	Remediation string
}
```

#### 4.2 Update Output Formatters

**File:** `internal/output/formatter.go`

Update JSON and text output to include new fields based on mode.

### Phase 5: Documentation

#### 5.1 Update Help Text

**File:** `cmd/proxyhawk/help/help.go`

Add comprehensive mode documentation.

#### 5.2 Create Mode Comparison Guide

**File:** `CHECK_MODES.md` (NEW)

User-facing documentation explaining modes and when to use each.

#### 5.3 Update README

Add mode examples to README.md.

---

## Usage Examples

### Basic Mode (Default - Fast)

```bash
# Fastest - just connectivity
./proxyhawk -l free_proxies.txt

# With output
./proxyhawk -l free_proxies.txt -o working.txt -j results.json

# With rate limiting for safety
./proxyhawk -l free_proxies.txt --rate-limit --rate-delay 1s
```

**Expected Output:**
```
Checking 968 proxies in basic mode...
Progress: 968/968 • Working: 234 • Failed: 734 • Avg: 2.3s

Results:
- 234 working proxies found (24.2%)
- Types: 156 HTTP, 45 HTTPS, 21 SOCKS5, 12 SOCKS4
- Average speed: 2.3s
```

### Intense Mode (Thorough)

```bash
# Quality vetting
./proxyhawk -l premium_proxies.txt --mode intense

# Find elite proxies
./proxyhawk -l proxies.txt --mode intense -wpa elite.txt

# With protocol detection
./proxyhawk -l proxies.txt --mode intense --http2 --http3

# With Interactsh
./proxyhawk -l proxies.txt --mode intense --interactsh
```

**Expected Output:**
```
Checking 50 proxies in intense mode...
Progress: 50/50 • Working: 32 • Elite: 18 • Cloud: 12 • Avg: 15.2s

Results:
- 32 working proxies (64%)
- Anonymity: 18 elite, 9 anonymous, 5 transparent
- Cloud: 8 AWS, 3 GCP, 1 Azure
- Protocols: 28 HTTP/HTTPS, 4 SOCKS5
- HTTP/2: 12 supported
```

### Vulns Mode (Security Testing)

```bash
# Security audit
./proxyhawk -l target_proxies.txt --mode vulns

# With full verbosity
./proxyhawk -l targets.txt --mode vulns -v

# With Interactsh for confirmation
./proxyhawk -l targets.txt --mode vulns --interactsh -d

# Rate limited for stealth
./proxyhawk -l targets.txt --mode vulns --rate-limit --rate-delay 5s
```

**Expected Output:**
```
Checking 10 proxies in vulns mode...
⚠️  Security testing mode - this will be slow!

Progress: 10/10 • Working: 7 • Vulnerable: 4 • Avg: 87.3s

Security Findings:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[CRITICAL] http://192.168.1.5:8080
  ├─ SSRF: Can access internal networks
  │  └─ Evidence: 169.254.169.254 returned 200 OK
  ├─ Host Header Injection: Accepts malicious headers
  │  └─ Evidence: X-Forwarded-Host: 127.0.0.1 reached localhost
  └─ Security Score: 23/100

[HIGH] http://10.0.0.15:3128
  ├─ Cache Poisoning: Vulnerable to cache manipulation
  │  └─ Evidence: Accepts unkeyed inputs in cache control
  └─ Security Score: 45/100

[MEDIUM] http://proxy.example.com:8888
  ├─ Protocol Smuggling: Possible CL.TE desync
  │  └─ Evidence: Ambiguous Content-Length handling
  └─ Security Score: 67/100

Summary:
- 7/10 proxies working
- 4/7 have security vulnerabilities
- 1 Critical, 1 High, 2 Medium severity
- Recommend: Isolate critical findings from production use
```

---

## Migration Path

### For Existing Users

**No Breaking Changes:**
- Default behavior is `--mode basic` (same as current)
- All existing flags work as before
- Config files compatible (new fields optional)

**Gradual Adoption:**
1. Use basic mode (current behavior)
2. Try intense mode on small lists
3. Use vulns mode only for security testing

### Backward Compatibility

```bash
# Old way (still works)
./proxyhawk -l proxies.txt -d

# New explicit way (same result)
./proxyhawk -l proxies.txt --mode basic -d

# Old flags still work with new modes
./proxyhawk -l proxies.txt --mode intense -v -o results.txt
```

---

## Performance Comparison Table

| Mode | Requests/Proxy | Time/Proxy | Suitable For | Recommended List Size |
|------|----------------|------------|--------------|----------------------|
| **Basic** | 3-7 | 1-15s | Bulk validation | 500-5000 |
| **Intense** | 15-25 | 20-40s | Quality vetting | 50-200 |
| **Vulns** | 130-150 | 60-120s | Security testing | 5-20 |

**With Interactsh:**
- Intense: +2-5s per proxy
- Vulns: +5-10s per proxy

---

## Configuration Examples

### Basic Production Use

```yaml
# config/production.yaml
check_mode: basic
concurrency: 20
timeout: 10
rate_limit_enabled: true
rate_limit_delay: 500ms
enable_cloud_checks: false
enable_anonymity_check: false
```

### Quality Assurance

```yaml
# config/quality.yaml
check_mode: intense
concurrency: 5
timeout: 30
enable_cloud_checks: true
enable_anonymity_check: true
enable_http2: true
test_urls:
  urls:
    - url: "https://api.ipify.org?format=json"
    - url: "https://httpbin.org/ip"
    - url: "http://ip-api.com/json/"
```

### Security Testing

```yaml
# config/security.yaml
check_mode: vulns
concurrency: 2  # Low concurrency for careful testing
timeout: 60
enable_interactsh: true
rate_limit_enabled: true
rate_limit_delay: 5s
advanced_checks:
  test_ssrf: true
  test_host_header_injection: true
  test_protocol_smuggling: true
  test_dns_rebinding: true
  test_cache_poisoning: true
  test_http_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS", "TRACE"]
```

---

## Implementation Checklist

### Phase 1: Foundation (Week 1)
- [ ] Add `--mode` flag to main.go
- [ ] Add `--interactsh` flag to main.go
- [ ] Add CheckMode type to types.go
- [ ] Add mode validation logic
- [ ] Update config struct with mode field

### Phase 2: Basic Refactor (Week 1-2)
- [ ] Extract performBasicChecks() from Check()
- [ ] Test basic mode maintains current behavior
- [ ] Update tests for basic mode

### Phase 3: Intense Mode (Week 2-3)
- [ ] Create intense_checks.go
- [ ] Implement checkAnonymity()
- [ ] Implement checkCloudProvider()
- [ ] Implement checkMultipleURLs()
- [ ] Add HTTP/2 and HTTP/3 checks
- [ ] Add mode-based config overrides
- [ ] Test intense mode thoroughly

### Phase 4: Vulns Mode (Week 3-4)
- [ ] Integrate performAdvancedChecks() into Check()
- [ ] Update Interactsh initialization logic
- [ ] Add vulnerability severity scoring
- [ ] Add VulnerabilityFinding struct
- [ ] Test vulns mode on safe targets
- [ ] Add security findings output

### Phase 5: Output & Documentation (Week 4)
- [ ] Update output formatters for new fields
- [ ] Add mode indicator to TUI
- [ ] Create CHECK_MODES.md guide
- [ ] Update README with mode examples
- [ ] Update help text
- [ ] Add configuration examples

### Phase 6: Testing & Polish (Week 5)
- [ ] Unit tests for all modes
- [ ] Integration tests with real proxies
- [ ] Performance benchmarks
- [ ] Security test validation
- [ ] Documentation review
- [ ] Example configs for each mode

---

## Security Considerations

### Rate Limiting

**Critical for vulns mode:**
- Default to slower rate limits
- Recommend 3-5s delay between requests
- Per-host rate limiting to avoid detection

### Ethical Usage

**Add disclaimer to vulns mode:**
```
⚠️  WARNING: Vulnerability testing mode enabled
    Only use on proxies you own or have permission to test.
    Unauthorized security testing may be illegal.
    Use responsibly and ethically.
```

### Data Handling

**Vulnerability findings are sensitive:**
- Don't log full requests/responses by default
- Sanitize output of internal IPs
- Add `--sanitize` flag for safe sharing
- Add warnings about output redaction

---

## Future Enhancements

### Mode Extensions

**Phase 2 (Future):**
- `--mode benchmark` - Performance testing
- `--mode geolocation` - Geographic distribution analysis
- `--mode rotation` - Test for proxy rotation detection

### Fine-Grained Control

**Advanced users:**
```bash
# Custom check selection
./proxyhawk --mode custom \
  --enable-checks ssrf,header-injection \
  --disable-checks protocol-smuggling
```

### Report Generation

```bash
# Security report
./proxyhawk -l proxies.txt --mode vulns --report vulns-report.html

# Quality report
./proxyhawk -l proxies.txt --mode intense --report quality-report.pdf
```

---

## Summary

**Recommendation: Implement 3-tier system exactly as proposed**

**Benefits:**
1. **User-Friendly:** Single flag controls complexity level
2. **Backward Compatible:** Default behavior unchanged
3. **Flexible:** Interactsh can enhance any mode
4. **Performant:** Users choose speed vs thoroughness
5. **Clear:** Mode names are self-explanatory

**Next Steps:**
1. Review and approve this proposal
2. Start with Phase 1 (foundation)
3. Implement modes incrementally
4. Test thoroughly at each phase
5. Document comprehensively

**Timeline:** 5 weeks for full implementation with testing
**Effort:** ~40-60 hours of development + testing
