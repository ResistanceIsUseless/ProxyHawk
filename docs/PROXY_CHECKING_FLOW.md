# ProxyHawk Proxy Checking Flow - Deep Dive

## Overview

ProxyHawk has a two-tier checking system:
1. **Basic Proxy Checks** - Always executed, validates proxy functionality
2. **Advanced Security Checks** - Optional, tests for vulnerabilities (currently NOT called in main flow)

## Check Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│ User runs: ./proxyhawk -l proxies.txt [-v] [-d]                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ Load Configuration (config/default.yaml)                        │
│ - Basic settings (timeout, concurrency, validation URL)        │
│ - Advanced checks config (all disabled by default)             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ Create Checker with debug flag                                  │
│ debug = -d flag OR any advanced check enabled                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ For each proxy: checker.Check(proxyURL)                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 1: Proxy Type Detection (determineProxyType)             │
│ - Parse proxy URL                                               │
│ - Try schemes in order: HTTP → HTTPS → SOCKS5 → SOCKS4        │
│ - For each scheme:                                              │
│   • Create HTTP client with proxy                               │
│   • Test with http://api.ipify.org?format=json                 │
│   • Test with https://api.ipify.org?format=json                │
│   • Record which protocols work (HTTP/HTTPS)                   │
│ - Return first working proxy type and client                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 2: Basic Validation (performChecks)                      │
│ - Make request to validation URL                                │
│ - Check response status code (expect 200)                       │
│ - Validate response size (min 50 bytes)                         │
│ - Check for disallowed keywords                                 │
│ - Verify required content/headers                               │
│ - Record speed/latency                                          │
│ - Mark proxy as working if all checks pass                     │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ ⚠️  MISSING: Advanced Checks NOT Called Here                   │
│                                                                  │
│ Should call: checker.performAdvancedChecks()                   │
│ But currently this is never invoked!                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ Return ProxyResult                                              │
│ - Working: true/false                                           │
│ - Type: http/https/socks4/socks5                               │
│ - Speed: duration                                               │
│ - SupportsHTTP/SupportsHTTPS: true/false                       │
│ - CheckResults: array of individual test results               │
└─────────────────────────────────────────────────────────────────┘
```

## 1. Basic Proxy Checks (Always Executed)

### Location
- File: [internal/proxy/checker.go](internal/proxy/checker.go)
- Function: `Check()` → `determineProxyType()` → `performChecks()`

### What Gets Checked

#### Phase 1: Protocol Detection
```go
// Lines 102-533: determineProxyType()
```

1. **URL Scheme Detection**
   - If proxy URL has a scheme (http://, socks5://), try that first
   - Test both HTTP and HTTPS endpoints to determine protocol support

2. **HTTP/HTTPS Proxy Testing**
   - Try as HTTP proxy → Test http://api.ipify.org → Test https://api.ipify.org
   - Try as HTTPS proxy → Same tests
   - Record which protocols work (sets SupportsHTTP, SupportsHTTPS flags)

3. **HTTP/2 and HTTP/3 Testing** (if enabled in config)
   - Only runs if `enable_http2: true` or `enable_http3: true` in config
   - Currently disabled by default

4. **SOCKS Proxy Testing**
   - Try SOCKS5 first → Test HTTP and HTTPS endpoints
   - Try SOCKS4 fallback → Same tests
   - Prefer SOCKS5 when both work

#### Phase 2: Basic Validation
```go
// Lines 536-669: performChecks()
```

1. **Request Validation**
   - Makes GET request to validation URL (default: https://api.ipify.org?format=json)
   - Includes configured headers and User-Agent
   - Records response time (Speed field)

2. **Response Validation**
   - **Status Code**: Must match `require_status_code` (default: 200)
   - **Response Size**: Must be >= `min_response_bytes` (default: 50 bytes)
   - **Disallowed Keywords**: Must NOT contain error strings like:
     - "Access Denied", "Proxy Error", "Bad Gateway"
     - "Gateway Timeout", "Service Unavailable"
     - "403 Forbidden", "502 Bad Gateway", etc.
   - **Required Content**: Must contain `require_content_match` if specified
   - **Required Headers**: Must have all headers in `require_header_fields`

3. **Result Recording**
   - Sets `Working = true` only if ALL validation checks pass
   - Records speed, status code, body size
   - Stores CheckResult for each test performed

### When These Run
**Always** - Every proxy check executes these basic checks, regardless of flags.

## 2. Advanced Security Checks (NOT Currently Executing!)

### Location
- File: [internal/proxy/advanced_checks.go](internal/proxy/advanced_checks.go)
- Function: `performAdvancedChecks()` (lines 36-216)

### Critical Issue
**⚠️ The `performAdvancedChecks()` function is defined but NEVER called from `Check()` or `performChecks()`!**

The advanced checks code exists but is not integrated into the main checking flow.

### What Should Be Checked (If It Were Called)

Configuration in [config/default.yaml](config/default.yaml:134-142):
```yaml
advanced_checks:
  test_protocol_smuggling: false
  test_dns_rebinding: false
  test_ipv6: false
  test_http_methods: ["GET"]
  test_cache_poisoning: false
  test_host_header_injection: false
  disable_interactsh: false
```

#### 1. Protocol Smuggling (test_protocol_smuggling)
```go
// Lines 68-89
```
- Sends POST request with ambiguous Content-Length headers
- Tests: `Content-Length: 4` + `Transfer-Encoding: chunked`
- Uses Interactsh for out-of-band detection (if available)
- Detects if proxy mishandles HTTP/1.1 request splitting

#### 2. DNS Rebinding (test_dns_rebinding)
```go
// Lines 92-113
```
- Sends request with manipulated Host headers
- Sets `X-Forwarded-Host` and `Host` to same value
- Tests if proxy follows DNS changes that could point to internal IPs
- Uses Interactsh for callback verification

#### 3. IPv6 Support (test_ipv6)
```go
// Lines 116-131
```
- Tests if proxy can reach IPv6 addresses
- Uses `http://[ipv6_address]` format
- Verifies IPv6 connectivity through proxy

#### 4. HTTP Methods (test_http_methods)
```go
// Lines 134-154
```
- Tests each method in the list (default: just GET)
- Can test: GET, POST, PUT, DELETE, PATCH, OPTIONS, HEAD, TRACE, CONNECT
- Identifies which HTTP methods the proxy allows

#### 5. Cache Poisoning (test_cache_poisoning)
```go
// Lines 157-178
```
- Sends request with cache control headers
- Tests: `Cache-Control: public, max-age=31536000`
- Checks if proxy caches responses inappropriately

#### 6. Host Header Injection (test_host_header_injection)
```go
// Lines 181-205, 444-494, 687-767
```
**Most comprehensive security test - tests 62+ internal targets!**

Tests internal network access through various injection techniques:

**Internal Targets Tested** (lines 406-419):
- `127.0.0.1` - Localhost
- `127.0.0.1:22` - SSH port
- `127.0.0.1:3306` - MySQL
- `192.168.1.1` - Common gateway
- `10.0.0.1`, `172.16.0.1` - RFC 1918 private ranges
- `169.254.169.254` - AWS metadata service
- `169.254.169.254:80` - AWS explicit port
- `localhost`, `0.0.0.0` - Alternative localhost
- `[::1]` - IPv6 localhost
- `metadata.google.internal` - GCP metadata

**Header Injection Vectors Tested** (lines 448-464):
- `Host` header manipulation
- `X-Forwarded-Host`
- `X-Host`
- `X-Forwarded-Server`
- `X-HTTP-Host-Override`
- `X-Real-IP`
- `X-Originating-IP`
- `X-Remote-IP`
- `X-Client-IP`
- `CF-Connecting-IP`
- `True-Client-IP`

**Advanced Techniques** (lines 687-767):
- Conflicting Host headers
- HTTP/1.0 Host header bypass
- CRLF injection: `127.0.0.1\r\nX-Injected: true`
- LF injection: `127.0.0.1\nX-Injected: true`
- Null byte injection: `127.0.0.1:80\x00`
- Whitespace manipulation
- Tabs in headers

#### 7. SSRF (test_ssrf)
```go
// Lines 208-213, 497-587
```
**Most extensive security test - tests 60+ attack vectors!**

**Internal Targets** (lines 510-562):
- All localhost variations (127.0.0.1, 127.1, localhost, 0.0.0.0, [::1])
- Common service ports (22 SSH, 3306 MySQL, 5432 PostgreSQL, 6379 Redis, 8080, 9200)
- Cloud metadata services:
  - AWS: 169.254.169.254, /latest/meta-data/, /latest/dynamic/instance-identity/
  - GCP: metadata.google.internal, metadata:80
  - Azure: /metadata/instance
  - DigitalOcean: /metadata/v1/maintenance
- Private network ranges:
  - 192.168.x.x, 10.x.x.x, 172.16-31.x.x
  - AWS VPC: 172.31.255.254
  - Carrier-grade NAT: 100.64.0.1
- Alternative localhost notations:
  - Octal: 0177.0.0.1
  - Hex: 0x7f.0x0.0x0.0x1
  - Decimal: 2130706433
- IPv6 variations:
  - Link-local: [fe80::1]
  - Unique local: [fc00::1], [fd00::1]
  - IPv4-mapped: [::ffff:127.0.0.1]
  - Documentation: [2001:db8::1]
  - Multicast: [ff02::1]

**Port Scanning** (lines 621-652):
Tests common internal service ports: 22, 23, 25, 53, 80, 110, 143, 443, 993, 995, 3306, 5432, 6379, 8080, 9200

**DNS Rebinding Protection** (lines 654-685):
Tests domains that might resolve to internal IPs:
- localhost.example.com
- 127.0.0.1.example.com
- 192.168.1.1.example.com

## Configuration Control

### Command Line Flags

```bash
# Basic mode (no advanced checks)
./proxyhawk -l proxies.txt

# Verbose mode (more output, no advanced checks)
./proxyhawk -l proxies.txt -v

# Debug mode (enables debug logging, but NOT advanced checks)
./proxyhawk -l proxies.txt -d
```

### Debug Flag Behavior
From [cmd/proxyhawk/main.go:369](cmd/proxyhawk/main.go#L369):
```go
debug := *debug || cfg.AdvancedChecks.TestProtocolSmuggling || cfg.AdvancedChecks.TestDNSRebinding
```

The `-d` flag OR any enabled advanced check enables debug mode, which:
- Adds detailed logging to `result.DebugInfo`
- Does NOT automatically enable advanced security checks
- Just makes the checker verbose about what it's doing

### Enabling Advanced Checks

To enable advanced security checks, modify [config/default.yaml](config/default.yaml:134-142):

```yaml
advanced_checks:
  test_protocol_smuggling: true     # Enable HTTP smuggling tests
  test_dns_rebinding: true          # Enable DNS rebinding tests
  test_ipv6: true                   # Enable IPv6 tests
  test_http_methods: ["GET", "POST", "PUT", "DELETE"]  # Test these methods
  test_cache_poisoning: true        # Enable cache poisoning tests
  test_host_header_injection: true  # Enable host header injection tests (62+ targets)
  test_ssrf: true                   # Enable SSRF tests (60+ attack vectors)
  disable_interactsh: false         # Use Interactsh for out-of-band detection
```

**However**, even with these enabled in config, the checks won't run because `performAdvancedChecks()` is never called!

## Integration Gap

### The Problem

The advanced checks function exists and is fully implemented, but it's never integrated into the main checking flow.

**Evidence:**
1. `performAdvancedChecks()` is defined in [internal/proxy/advanced_checks.go:36](internal/proxy/advanced_checks.go#L36)
2. It's only called in test files, never in production code
3. `grep performAdvancedChecks internal/proxy/*.go` shows:
   - `advanced_checks.go`: function definition
   - `advanced_checks_test.go`: test calls only
   - `checker.go`: NOT called anywhere!

### Where It Should Be Called

In [internal/proxy/checker.go:81](internal/proxy/checker.go#L81), after basic checks pass:

```go
// Perform checks using the determined client
if err := c.performChecks(client, result); err != nil {
    result.Error = errors.NewProxyError(errors.ErrorProxyValidationFailed, "validation failed", proxyURL, err)
    return result
}

// ⚠️ MISSING: Should add here:
// if err := c.performAdvancedChecks(client, result); err != nil {
//     // Log error but don't fail the proxy - advanced checks are supplementary
//     if c.debug {
//         result.DebugInfo += fmt.Sprintf("[ADVANCED] Some advanced checks failed: %v\n", err)
//     }
// }

result.Working = true // This happens at line 666 currently
```

## Check Performance Expectations

### Basic Checks (What You're Currently Getting)

For a list of 968 proxies with concurrency=10:

**Per Proxy:**
- Type detection: 2-6 HTTP requests (tries different protocols)
- Basic validation: 1 HTTP request
- Total: 3-7 requests per proxy
- Time: 1-15 seconds per proxy (depending on timeout and response)

**Total for 968 proxies:**
- Expected time: 2-25 minutes (depending on success rate)
- Working proxies: 20-40% typically for free proxy lists
- Expected results: 190-390 working proxies

### Advanced Checks (If They Were Enabled and Integrated)

**Per Proxy (if all checks enabled):**
- Protocol smuggling: 1 request
- DNS rebinding: 1 request
- IPv6: 1 request
- HTTP methods: N requests (N = number of methods to test)
- Cache poisoning: 1 request
- Host header injection: 62 requests (12 targets × 11 injection vectors + advanced techniques)
- SSRF: 60+ requests (internal targets + port scanning + DNS rebinding)

**Total per proxy with all advanced checks:**
- ~130-150 additional requests per proxy
- ~20-60 seconds additional time per proxy
- Would increase total scan time by 4-10x

## Current State Summary

### What Works Now ✅

1. **Proxy Type Detection**
   - Automatically detects HTTP, HTTPS, SOCKS4, SOCKS5
   - Tests both HTTP and HTTPS protocol support
   - Returns first working proxy type

2. **Basic Validation**
   - Status code checking
   - Response size validation
   - Error keyword detection
   - Content/header verification
   - Speed measurement

3. **Debug Output**
   - Detailed logging when `-d` flag is used
   - Shows each step of proxy detection
   - Records all test results

### What Doesn't Work ❌

1. **Advanced Security Checks**
   - Code is written but never called
   - Configuration exists but is ignored
   - No vulnerability testing is performed
   - No SSRF, header injection, or protocol smuggling detection

2. **Interactsh Integration**
   - Interactsh tester exists but can't be used
   - Out-of-band detection is unavailable
   - Configuration for interactsh_url is unused

## Recommendations

### For Basic Proxy Checking (Current Use Case)

Your current setup is **working as expected** for basic proxy validation:
- Detecting proxy types correctly
- Validating connectivity
- Measuring speed
- Filtering out broken proxies

The slow progress you're seeing is normal for free proxy lists (20-40% success rate).

### For Security Testing (If Needed)

If you want to test proxies for vulnerabilities:

1. **Integrate Advanced Checks**
   - Add call to `performAdvancedChecks()` in [checker.go:86](internal/proxy/checker.go#L86)
   - Make it non-blocking (don't fail proxy if advanced checks fail)
   - Only run if advanced checks are enabled in config

2. **Enable Specific Checks**
   - Start with just one: `test_ssrf: true` or `test_host_header_injection: true`
   - Test on small proxy list first (5-10 proxies)
   - Expect 10x longer scan time with all checks enabled

3. **Add Rate Limiting**
   - Enable `rate_limit_enabled: true` in config
   - Increase `rate_limit_delay` to avoid bans when testing vulnerabilities
   - Consider using `rate_limit_per_host: true`

## Code References

- **Basic Checks**: [internal/proxy/checker.go](internal/proxy/checker.go) lines 34-669
- **Advanced Checks**: [internal/proxy/advanced_checks.go](internal/proxy/advanced_checks.go) lines 36-767
- **Configuration**: [config/default.yaml](config/default.yaml)
- **Main Integration**: [cmd/proxyhawk/main.go](cmd/proxyhawk/main.go) lines 338-408
- **Types**: [internal/proxy/types.go](internal/proxy/types.go) lines 12-76

---

## TL;DR

**Current Behavior:**
- ProxyHawk performs comprehensive **basic proxy validation** (type detection, connectivity, speed)
- Advanced security checks (SSRF, header injection, smuggling) are **fully implemented but never executed**
- The `-d` debug flag only enables verbose logging, not vulnerability testing

**Why Scans Are Slow:**
- Testing 968 proxies with 3-7 requests each
- Free proxies typically have 20-40% success rate
- Many proxies timeout (default 15s each)
- This is normal and expected behavior

**To Enable Vulnerability Testing:**
- Need to integrate `performAdvancedChecks()` call in checker.go
- Would add 130-150 requests per proxy
- Would increase scan time by 4-10x
- Currently not recommended unless specifically doing security audits
