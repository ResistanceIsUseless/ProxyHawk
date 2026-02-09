# ProxyHawk
[proxyhawk.jpg]
A comprehensive proxy checker and validator with **advanced security testing** capabilities for HTTP, HTTPS, SOCKS4, and SOCKS5 proxies.

## 🚀 Major Updates
- ✅ **Production-ready** with comprehensive security testing
- ✅ **Three-Tier Check Modes** - Choose between basic, intense, or vulns scanning modes
- ✅ **Enhanced Anonymity Detection** - Elite/Anonymous/Transparent/Compromised classification with 10+ header checks
- ✅ **Proxy Chain Detection** - Automatic detection of proxy-behind-proxy configurations
- ✅ **Enhanced security testing** - SSRF (60+ targets), Host header injection, protocol smuggling detection
- ✅ **Structured logging** and error handling
- ✅ **Modular architecture** with 27% code reduction
- ✅ **Comprehensive input validation** with security hardening
- ✅ **100% test coverage** for core functionality

## Installation

### From Source
```bash
git clone https://github.com/ResistanceIsUseless/ProxyHawk.git
cd ProxyHawk
make build  # Builds both proxyhawk and proxyhawk-server in ./build/
```

### Using Go Install
```bash
go install github.com/ResistanceIsUseless/ProxyHawk/cmd/proxyhawk@latest
```

### Docker (Recommended for Production)
```bash
# Quick start with Docker
cd deployments/docker
docker-compose up -d

# Access services:
# - ProxyHawk API: http://localhost:8888/api/health
# - Prometheus: http://localhost:9090  
# - Grafana: http://localhost:3000 (admin/admin)
```

### Using Makefile
```bash
# Development workflow
make deps          # Install dependencies
make build         # Build binary
make test          # Run tests
make docker-build  # Build Docker image
make docker-run    # Run in container
```

## Features

### Core Proxy Testing
- Concurrent proxy checking using goroutines with configurable concurrency
- Support for HTTP, HTTPS, HTTP/2, HTTP/3, SOCKS4, and SOCKS5 proxies
- Automatic proxy type detection and validation
- HTTP/2 and HTTP/3 protocol support with automatic detection
- Detailed timing and performance metrics
- Multiple output formats (text, JSON, working proxies only)

### Proxy Discovery & Intelligence
- **Multiple Discovery Sources**: Discover proxies from various sources
  - **Shodan Integration**: Professional internet scanning database
  - **Censys Integration**: Comprehensive internet-wide asset discovery
  - **Free Proxy Lists**: Aggregated from ProxyList.geonode.com, FreeProxy.world
  - **Web Scraping**: Real-time scraping from public proxy sources
- **Smart Filtering**: Filter by country, confidence score, proxy type, and more
- **Intelligent Scoring**: Advanced scoring system based on multiple factors
- **Auto-Validation**: Automatically test discovered candidates
- **Deduplication**: Remove duplicate candidates across sources
- **Preset Queries**: Built-in search queries optimized for each source
- **🛡️ Honeypot Detection**: Advanced filtering to identify and remove honeypots/monitoring systems

### Security Testing & Validation
- **Three-Tier Check Modes**: Choose between basic (connectivity only), intense (core security checks), or vulns (full vulnerability scanning)
- **Advanced Anonymity Detection**: Classification of proxies into Elite, Anonymous, Transparent, or Compromised levels based on 10+ header checks
- **Proxy Chain Detection**: Automatic detection of proxy-behind-proxy configurations via Via header and X-Forwarded-For analysis
- **SSRF Detection**: Tests access to cloud metadata services (AWS, GCP, Azure), internal networks (RFC 1918, RFC 6598, RFC 3927), and localhost variants (60+ test targets)
- **Host Header Injection**: Advanced testing with multiple injection vectors including X-Forwarded-Host, X-Real-IP, malformed headers, and HTTP/1.0 bypasses
- **Protocol Smuggling**: Detection of HTTP request smuggling vulnerabilities using Content-Length/Transfer-Encoding conflicts
- **Internal Network Scanning**: Port scanning capabilities and DNS rebinding protection testing
- **Input Validation**: Comprehensive URL validation with security hardening against malicious inputs

### Advanced Features
- **Enhanced Anonymity Detection**: Elite/Anonymous/Transparent/Compromised classification with IP leak detection
- **Proxy Chain Detection**: Detection of proxy-behind-proxy configurations with detailed chain information
- **Header Leak Analysis**: Detection of 10+ header types that may leak real IP (X-Forwarded-For, Via, X-Real-IP, etc.)
- Cloud provider detection and classification
- Custom HTTP headers and User-Agent support
- Rate limiting with per-host or global controls
- Reverse DNS (rDNS) lookup for host headers
- Structured logging with multiple verbosity levels
- Graceful shutdown with signal handling
- Configurable timeouts and validation rules

## Configuration

The application uses a YAML configuration file (default: `config.yaml`) to define cloud provider settings and validation rules. The configuration includes:

- Cloud provider definitions and metadata service endpoints
- Default HTTP headers and User-Agent settings
- Response validation rules and content matching
- Advanced security check configurations (SSRF, host header injection, protocol smuggling)  
- Internal network ranges and reserved IP blocks
- Rate limiting and timeout settings
- Proxy discovery API credentials and filtering options

Example configuration:
```yaml
cloud_providers:
  - name: AWS
    metadata_ips:
      - "169.254.169.254"
    internal_ranges:
      - "172.31.0.0/16"
      - "10.0.0.0/16"
    asns:
      - "16509"
      - "14618"
    org_names:
      - "AMAZON"
      - "AWS"

# Discovery configuration
discovery:
  # API credentials for discovery sources
  shodan_api_key: "YOUR_SHODAN_API_KEY"
  censys_api_key: "YOUR_CENSYS_API_KEY"
  censys_secret: "YOUR_CENSYS_SECRET"
  
  # Search parameters
  max_results: 1000
  countries: ["US", "GB", "DE", "NL"]
  min_confidence: 0.3
  timeout: 30
  rate_limit: 60  # requests per minute
  
  # Filtering options
  exclude_residential: true
  exclude_cdn: true
  exclude_malicious: true
  deduplicate: true
  
  # Security options
  enable_honeypot_filter: true  # Enable honeypot detection
  honeypot_threshold: 0.4       # Suspicion threshold
```

You can specify a custom configuration file using the `-config` flag.

## Usage

1. Create a text file containing your proxy list, one proxy per line. The format should be:
```
http://proxy1.example.com:8080
proxy2.example.com:3128
https://proxy3.example.com:8443
socks5://proxy4.example.com:1080
```

Note: The scheme (http://, https://, socks5://) is optional. If not provided, http:// will be used by default.

2. Run the proxy checker:
```bash
./proxyhawk -l proxies.txt
```

### Command Line Options
```
Core Options:
- -l: File containing proxy list (one per line)
- -host: Single proxy host (IP, hostname, or IP:PORT) to test
- -cidr: CIDR range to test (e.g., 192.168.1.0/24 or 192.168.1.0/24:8080)
- -config: Path to configuration file (default: config/default.yaml)
- -c: Number of concurrent checks (overrides config)
- -t: Timeout in seconds (overrides config)
- -v: Enable verbose output
- -d: Enable debug mode (shows detailed request/response information)
- -hot-reload: Enable configuration hot-reloading (watches config file for changes)

Security Testing:
- -mode: Check mode: basic (connectivity only), intense (advanced security checks), vulns (vulnerability scanning) (default: basic)
- -advanced: Enable all advanced security checks (overrides -mode setting)
- -interactsh: Enable Interactsh for out-of-band detection (enhances security checks with external callback verification)
- -r: Use reverse DNS lookup for host headers
- Advanced security tests are configured via the YAML config file:
  * SSRF testing against internal networks and cloud metadata (60+ targets)
  * Host header injection with multiple attack vectors
  * Protocol smuggling detection
  * DNS rebinding protection testing
  * Enhanced anonymity detection with 10+ header checks
  * Proxy chain detection via Via and X-Forwarded-For analysis

Output Options:
- -o: Output results to text file (includes all details and summary)
- -j: Output results to JSON file (structured format for programmatic use)
- -wp: Output only working proxies to a text file (format: proxy - speed)
- -wpa: Output only working anonymous proxies to a text file (format: proxy - speed)
- -no-ui: Disable terminal UI (for automation/scripting)

Progress Options (for -no-ui mode):
- -progress: Progress indicator type (bar, spinner, dots, percent, basic, none) (default: bar)
- -progress-width: Width of progress bar (default: 50)
- -progress-no-color: Disable colored progress output

Rate Limiting:
- -rate-limit: Enable rate limiting to prevent overwhelming target servers
- -rate-delay: Delay between requests (e.g. 500ms, 1s, 2s) (default: 1s)
- -rate-per-host: Apply rate limiting per host instead of globally (default: true)
- -rate-per-proxy: Apply rate limiting per individual proxy (takes precedence over per-host)

Metrics:
- -metrics: Enable Prometheus metrics endpoint
- -metrics-addr: Address to serve metrics on (default: :9090)
- -metrics-path: Path for metrics endpoint (default: /metrics)

Protocol Support:
- -http2: Enable HTTP/2 protocol detection and support
- -http3: Enable HTTP/3 protocol detection and support (experimental)

Discovery Options:
- -discover: Enable discovery mode to find proxy candidates
- -discover-source: Discovery source to use (shodan, censys, freelists, webscraper, all) (default: all)
- -discover-query: Custom discovery query (uses preset if empty)
- -discover-limit: Maximum number of candidates to discover (default: 100)
- -discover-validate: Validate discovered candidates immediately
- -discover-countries: Comma-separated list of country codes to target
- -discover-min-confidence: Minimum confidence score for candidates (0.0-1.0, default: 0.0)
- -discover-no-honeypot-filter: Disable honeypot detection and filtering

Help Options:
- -help, -h: Show help message
- -version: Show version information
- -quickstart: Show quick start guide with examples
```
### Example Commands

Basic check against default URL (connectivity only):
```bash
./proxyhawk -l proxies.txt
```

Test a single proxy host:
```bash
./proxyhawk -host 192.168.1.100:8080
./proxyhawk -host proxy.example.com:3128
```

Test a CIDR range:
```bash
./proxyhawk -cidr 192.168.1.0/24:8080
./proxyhawk -cidr 10.0.0.0/24  # Tests common proxy ports on each IP
```

Check with increased concurrency and longer timeout:
```bash
./proxyhawk -l proxies.txt -c 20 -t 15s
```

Enable intense mode with core security checks:
```bash
./proxyhawk -l proxies.txt -mode intense
```

Enable full vulnerability scanning mode:
```bash
./proxyhawk -l proxies.txt -mode vulns -d
```

Enable all advanced checks (overrides mode):
```bash
./proxyhawk -l proxies.txt -advanced -d
```

Enable comprehensive security testing with debug output:
```bash
./proxyhawk -l proxies.txt -d -config security_config.yaml
```

Save results to text and JSON files:
```bash
./proxyhawk -l proxies.txt -o results.txt -j results.json
```

Test with custom security configuration:
```bash
./proxyhawk -l proxies.txt -config custom_config.yaml -d -t 20s
```

Run without UI for automation:
```bash
./proxyhawk -l proxies.txt -no-ui -v -o results.txt
```

Maximum performance testing:
```bash
./proxyhawk -l proxies.txt -c 50 -t 5s
```

Debug mode with specific URL and timeout:
```bash
./proxyhawk -l proxies.txt -d -t 30s
```

Advanced security testing with custom output files:
```bash
./proxyhawk -l proxies.txt -d -o security_results.txt -j security_results.json -t 25s -c 30
```

Check proxies and save working ones to a separate file:
```bash
./proxyhawk -l proxies.txt -wp working_proxies.txt
```

Check proxies and save working anonymous ones to a separate file:
```bash
./proxyhawk -l proxies.txt -wpa working_anonymous_proxies.txt
```

Check proxies with rate limiting to avoid IP bans:
```bash
./proxyhawk -l proxies.txt -rate-limit -rate-delay 2s
```

Test proxies with HTTP/2 and HTTP/3 support:
```bash
./proxyhawk -l proxies.txt -http2 -http3 -v
```

Run with hot-reloading config (for long-running checks):
```bash
./proxyhawk -l proxies.txt -hot-reload -config custom_config.yaml
```

Use granular per-proxy rate limiting (slowest but safest):
```bash
./proxyhawk -l proxies.txt -rate-limit -rate-per-proxy -rate-delay 3s
```

Run in automation mode with custom progress indicator:
```bash
./proxyhawk -l proxies.txt -no-ui -progress spinner -o results.txt
```

Enable metrics for monitoring:
```bash
./proxyhawk -l proxies.txt -metrics -metrics-addr :9090
```

### Check Mode Examples

Basic mode (connectivity only, fastest):
```bash
./proxyhawk -l proxies.txt -mode basic -c 50
```

Intense mode (core security checks, balanced):
```bash
./proxyhawk -l proxies.txt -mode intense -c 20 -t 15s
```

Vulnerability scanning mode (comprehensive, slowest):
```bash
./proxyhawk -l proxies.txt -mode vulns -c 10 -t 30s -d
```

Override mode with explicit advanced checks:
```bash
./proxyhawk -l proxies.txt -advanced -t 25s
```

Enable Interactsh for out-of-band vulnerability confirmation:
```bash
# With intense mode (recommended for anonymity testing)
./proxyhawk -l proxies.txt -mode intense -interactsh

# With vulns mode (recommended for full vulnerability testing)
./proxyhawk -l proxies.txt -mode vulns -interactsh -d
```

### Discovery Mode Examples

Discover proxies using Shodan (requires API key):
```bash
./proxyhawk -discover -discover-source shodan -discover-limit 50
```

Discover proxies using Censys (requires API credentials):
```bash
./proxyhawk -discover -discover-source censys -discover-limit 100 -v
```

Discover proxies from free lists (no API key required):
```bash
./proxyhawk -discover -discover-source freelists -discover-limit 200
```

Discover proxies using web scraping (no API key required):
```bash
./proxyhawk -discover -discover-source webscraper -discover-limit 150
```

Discover from all available sources:
```bash
./proxyhawk -discover -discover-source all -discover-limit 500 -v
```

Discover and validate proxies from specific countries:
```bash
./proxyhawk -discover -discover-countries "US,GB,DE" -discover-validate -o discovered.txt
```

Custom discovery query with confidence filtering:
```bash
./proxyhawk -discover -discover-query "squid proxy" -discover-min-confidence 0.7 -j results.json
```

Discover high-quality proxies and save working ones:
```bash
./proxyhawk -discover -discover-limit 200 -discover-validate -wp working-discovered.txt -v
```

Discover proxies by specific source with validation:
```bash
./proxyhawk -discover -discover-source censys -discover-query "services.service_name: http and services.port: 3128" -discover-validate -o censys-proxies.txt
```

Disable honeypot filtering (not recommended):
```bash
./proxyhawk -discover -discover-source shodan -discover-no-honeypot-filter -discover-limit 50
```

Enable verbose logging to see honeypot filtering in action:
```bash
./proxyhawk -discover -discover-source all -discover-limit 100 -v
```

### Check Modes

ProxyHawk supports three check modes to balance speed and thoroughness:

| Mode | Description | Tests Included | Performance | Use Case |
|------|-------------|----------------|-------------|----------|
| **basic** | Connectivity only | Connection validation, response validation | Fastest (~1-2s per proxy) | Quick proxy list validation, high-volume testing |
| **intense** | Core security checks | Basic + SSRF, Host Header Injection, Protocol Smuggling, IPv6, Anonymity Detection, Chain Detection | Moderate (~5-10s per proxy) | Production proxy vetting, security-conscious deployments |
| **vulns** | Full vulnerability scanning | Intense + DNS Rebinding, Cache Poisoning, Extended SSRF (60+ targets) | Slowest (~15-30s per proxy) | Security research, comprehensive auditing |

**Test Coverage by Mode:**

- **Basic Mode**:
  - ✅ Connectivity validation
  - ✅ Response validation
  - ✅ Protocol detection
  - ✅ Basic anonymity detection
  - ❌ Advanced security checks disabled

- **Intense Mode**:
  - ✅ All basic mode checks
  - ✅ SSRF testing (core targets)
  - ✅ Host Header Injection detection
  - ✅ Protocol Smuggling detection
  - ✅ IPv6 testing
  - ✅ Enhanced anonymity detection (10+ headers)
  - ✅ Proxy chain detection
  - ⚠️ DNS Rebinding disabled (slow)
  - ⚠️ Cache Poisoning disabled (slow)

- **Vulns Mode**:
  - ✅ All intense mode checks
  - ✅ DNS Rebinding testing
  - ✅ Cache Poisoning testing
  - ✅ Extended SSRF testing (60+ targets including cloud metadata)
  - ✅ Comprehensive security audit

**Anonymity Detection (All Modes):**
All modes include basic anonymity detection. Intense and Vulns modes include enhanced detection with:
- 10+ header leak checks (X-Forwarded-For, Via, X-Real-IP, etc.)
- IP extraction from leaked headers
- Proxy chain detection via Via and X-Forwarded-For analysis
- Classification: Elite, Anonymous, Transparent, Compromised, Unknown

### Output Formats

#### Text Output (-o)
The text output file includes:
- Status icons for each proxy:
  - ✅ Working proxy
  - 🔒 Anonymous proxy  
  - ☁️ Cloud provider proxy
  - ⚠️ Security vulnerability detected
  - 🚨 Internal network access possible
  - ❌ Failed proxy
- Response time and performance metrics
- IP information and geolocation when available
- Cloud provider classification
- Security vulnerability details (SSRF, host header injection, etc.)
- Protocol support information (HTTP/HTTPS/SOCKS)
- Timestamp of each check
- Comprehensive summary statistics

#### JSON Output (-j)
The JSON output includes a structured format with:
```json
{
  "total_proxies": 10,
  "working_proxies": 5,
  "interactsh_proxies": 3,
  "anonymous_proxies": 2,
  "cloud_proxies": 1,
  "internal_access_count": 0,
  "success_rate": 50.0,
  "results": [
    {
      "proxy": "http://example.com:8080",
      "working": true,
      "speed_ns": 1500000000,
      "interactsh_test": true,
      "real_ip": "1.2.3.4",
      "proxy_ip": "5.6.7.8",
      "is_anonymous": true,
      "anonymity_level": "elite",
      "detected_ip": "",
      "leaking_headers": [],
      "proxy_chain_detected": false,
      "proxy_chain_info": "",
      "cloud_provider": "AWS",
      "internal_access": false,
      "metadata_access": false,
      "timestamp": "2024-02-13T02:05:45Z",
      "security_checks": {
        "ssrf_vulnerability": false,
        "host_header_injection": false,
        "protocol_smuggling": false,
        "dns_rebinding": false,
        "internal_network_access": false,
        "cloud_metadata_access": false,
        "vulnerability_details": {
          "detected_issues": [],
          "internal_targets_accessible": [],
          "malformed_requests_accepted": false
        }
      },
      "protocol_support": {
        "http": true,
        "https": true,
        "socks4": false,
        "socks5": false
      }
    }
  ]
}
```

**New Fields in JSON Output:**
- `anonymity_level`: Classification (elite/anonymous/transparent/compromised/unknown)
- `detected_ip`: IP address detected during anonymity check (if any leak found)
- `leaking_headers`: Array of headers that leaked information
- `proxy_chain_detected`: Boolean indicating if proxy-behind-proxy was detected
- `proxy_chain_info`: Details about detected proxy chain

## Building

```bash
go build -o proxyhawk
```

## Requirements

- Go 1.21 or later
- Internet connection for proxy checking
- Valid proxy list in supported format

## Testing

The project includes a comprehensive test suite located in the `tests` directory. The tests cover:

- Basic proxy validation
- Advanced security checks
- Output formatting
- Configuration loading
- Working proxies output

### Running Tests

Run all tests:
```bash
make test
```

Run tests with coverage report:
```bash
make coverage
```

Run a specific test:
```bash
make test-one test=TestName
```

Run tests in verbose mode:
```bash
make test-verbose
```

Run short tests (skip long-running tests):
```bash
make test-short
```

### Test Structure

- `tests/proxy_test.go`: Tests for basic proxy validation
- `tests/security_test.go`: Tests for advanced security checks
- `tests/output_test.go`: Tests for output formatting
- `tests/config_test.go`: Tests for configuration loading
- `tests/working_proxies_test.go`: Tests for working proxies output
- `tests/types.go`: Common types used in tests
- `tests/init_test.go`: Test environment setup and cleanup

### Linting

The project uses golangci-lint for code quality checks. Run linting:
```bash
make lint
```

Linting configuration is in `.golangci.yml` and includes:
- Code formatting (gofmt)
- Code simplification (gosimple)
- Error checking (errcheck)
- Security checks (gosec)
- And many more... 

Shout out to @geeknik for helping me with the name and @nullenc0de for helping as well!!