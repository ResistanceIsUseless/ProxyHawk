# ProxyHawk

A comprehensive proxy checker and validator with **advanced security testing** capabilities for HTTP, HTTPS, SOCKS4, and SOCKS5 proxies.

[![Go Version](https://img.shields.io/badge/Go-1.23%2B-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

## Features

- **Multi-Protocol Support**: HTTP, HTTPS, HTTP/2, HTTP/3, SOCKS4, SOCKS5
- **Advanced SSRF Detection**: 16 advanced checks with 154 test cases covering all known attack vectors
- **Vulnerability Scanning**: 55+ CVE checks including 6 critical vulnerabilities (CVSS 9.0+)
- **Proxy Discovery**: Shodan, Censys, free lists, web scraping with honeypot filtering
- **Path-Based Fingerprinting**: Reverse proxy and API gateway detection
- **Anonymity Detection**: Elite/Anonymous/Transparent classification with 10+ header checks
- **Three Check Modes**: Basic (fast), Intense (security), Vulns (comprehensive)

## Quick Start

### Installation

```bash
# From source
git clone https://github.com/ResistanceIsUseless/ProxyHawk.git
cd ProxyHawk
make build

# Using Go
go install github.com/ResistanceIsUseless/ProxyHawk/cmd/proxyhawk@latest

# Docker
docker-compose up -d
```

### Basic Usage

```bash
# Test proxy list
./proxyhawk -l proxies.txt

# Test single proxy
./proxyhawk -host 192.168.1.100:8080

# Full security scan
./proxyhawk -l proxies.txt -mode vulns -d

# Discover proxies from Shodan
./proxyhawk -discover -discover-source shodan -discover-limit 50
```

## Command-Line Arguments

### Core Options
- `-l` - File with proxy list (one per line)
- `-host` - Single proxy to test (IP or hostname)
- `-cidr` - CIDR range to test
- `-config` - Config file path (default: config/default.yaml)
- `-c` - Concurrent checks (default: 10)
- `-t` - Timeout (default: 10s)
- `-v` - Verbose output
- `-d` - Debug mode

### Security Testing
- `-mode` - Check mode: `basic` (connectivity), `intense` (security), `vulns` (comprehensive)
- `-advanced` - Enable all security checks
- `-fingerprint` - Enable proxy fingerprinting
- `-path-fingerprint` - Path-based fingerprinting mode
- `-interactsh` - Enable out-of-band detection

### Output Options
- `-o` - Save results to text file
- `-j` - Save results to JSON file
- `-wp` - Save working proxies only
- `-wpa` - Save anonymous proxies only
- `-no-ui` - Disable terminal UI

### Discovery Options
- `-discover` - Enable discovery mode
- `-discover-source` - Source: `shodan`, `censys`, `freelists`, `webscraper`, `all`
- `-discover-limit` - Max candidates (default: 100)
- `-discover-validate` - Validate discovered proxies
- `-discover-countries` - Filter by countries (e.g., "US,GB,DE")

### Rate Limiting
- `-rate-limit` - Enable rate limiting
- `-rate-delay` - Delay between requests (default: 1s)
- `-rate-per-host` - Per-host rate limiting
- `-rate-per-proxy` - Per-proxy rate limiting

## Common Examples

```bash
# Quick connectivity test
./proxyhawk -l proxies.txt

# Security testing with fingerprinting
./proxyhawk -l proxies.txt -mode intense -fingerprint -v

# Comprehensive vulnerability scan
./proxyhawk -l proxies.txt -mode vulns -d -o results.txt -j results.json

# Discover and validate proxies
./proxyhawk -discover -discover-validate -wp working.txt

# Test reverse proxy/API gateway
./proxyhawk -path-fingerprint -host http://api.example.com

# Test with rate limiting
./proxyhawk -l proxies.txt -rate-limit -rate-delay 2s

# Automation mode
./proxyhawk -l proxies.txt -no-ui -progress bar -o results.txt
```

## Check Modes

| Mode | Speed | Tests | Use Case |
|------|-------|-------|----------|
| **basic** | Fast (~2s) | Connectivity only | Quick validation |
| **intense** | Medium (~10s) | Core security checks | Production vetting |
| **vulns** | Slow (~2.5min) | 154 advanced tests | Comprehensive audit |

## Configuration

Create `config.yaml` with your settings:

```yaml
# Discovery API credentials
discovery:
  shodan_api_key: "YOUR_KEY"
  censys_api_key: "YOUR_KEY"
  censys_secret: "YOUR_SECRET"

# Security testing
advanced_checks:
  test_ssrf: true
  test_host_header_injection: true
  test_protocol_smuggling: true

# Rate limiting
rate_limit_enabled: true
rate_limit_delay: "1s"
```

**⚠️ Security**: Never commit API keys to git. See [SECURITY_NOTICE.md](SECURITY_NOTICE.md) for safe practices.

## Output Formats

### Text Output
```
✅ http://proxy.example.com:8080 - 1.5s - Anonymous (Elite) - US
🔒 http://secure-proxy.com:3128 - 2.1s - Anonymous - GB
☁️ http://aws-proxy.com:8080 - 1.8s - AWS - US-EAST-1
❌ http://dead-proxy.com:8080 - Failed (timeout)
```

### JSON Output
```json
{
  "total_proxies": 4,
  "working_proxies": 3,
  "anonymous_proxies": 2,
  "success_rate": 75.0,
  "results": [...]
}
```

## Advanced SSRF Detection (v1.6.0)

ProxyHawk includes **154 advanced SSRF test cases** covering:

**Priority 1** (92 tests): URL parser differentials, IP obfuscation, redirect chains, protocol smuggling, header injection, proxy_pass traversal, host header SSRF

**Priority 2** (22 tests): SNI proxy routing, DNS rebinding, HTTP/2 header injection, AWS IMDSv2 bypass

**Priority 3** (40 tests): URL encoding bypass, multiple Host headers, cloud-specific headers, port tricks, fragment/query manipulation

See [docs/DETAILED_README.md](docs/DETAILED_README.md) for complete vulnerability coverage.

## Documentation

- [Detailed README](docs/DETAILED_README.md) - Complete feature documentation
- [Security Research](docs/SECURITY_RESEARCH_ANALYSIS.md) - SSRF vulnerability analysis
- [Implementation Status](docs/IMPLEMENTATION_STATUS.md) - Feature checklist
- [API Key Security](SECURITY_NOTICE.md) - Safe configuration practices
- [Development Guide](CLAUDE.md) - Contributing and development

## Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run linter
make lint
```

## Requirements

- Go 1.23+
- Optional: Shodan/Censys API keys for discovery

## Credits

Shout out to [@geeknik](https://github.com/geeknik) for the name and [@nullenc0de](https://github.com/nullenc0de) for the support!

## License

MIT License - see [LICENSE](LICENSE) file for details.
