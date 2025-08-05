# ProxyHawk
[proxyhawk.jpg]
A comprehensive proxy checker and validator with **advanced security testing** capabilities for HTTP, HTTPS, SOCKS4, and SOCKS5 proxies.

## 🚀 Major Updates
- ✅ **Production-ready** with comprehensive security testing
- ✅ **Enhanced security testing** - SSRF, Host header injection, protocol smuggling detection
- ✅ **Structured logging** and error handling
- ✅ **Modular architecture** with 27% code reduction
- ✅ **Comprehensive input validation** with security hardening
- ✅ **100% test coverage** for core functionality

## Installation

### From Source
```bash
git clone https://github.com/ResistanceIsUseless/ProxyHawk.git
cd ProxyHawk
go build -o proxyhawk cmd/proxyhawk/main.go
```

### Using Go Install
```bash
go install github.com/ResistanceIsUseless/proxyhawk@latest
```

### Docker (Recommended for Production)
```bash
# Quick start with Docker
docker build -t proxyhawk .
docker run --rm -v $(pwd)/proxies.txt:/app/proxies.txt:ro -v $(pwd)/output:/app/output proxyhawk -l proxies.txt -o output/results.txt --no-ui

# Or use the deployment script for easier management
./scripts/deploy.sh setup
./scripts/deploy.sh run-basic

# Full monitoring stack with Prometheus + Grafana
docker-compose up -d
# Access Grafana at http://localhost:3000 (admin/admin)
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

### Security Testing & Validation
- **SSRF Detection**: Tests access to cloud metadata services (AWS, GCP, Azure), internal networks (RFC 1918, RFC 6598, RFC 3927), and localhost variants
- **Host Header Injection**: Advanced testing with multiple injection vectors including X-Forwarded-Host, X-Real-IP, malformed headers, and HTTP/1.0 bypasses
- **Protocol Smuggling**: Detection of HTTP request smuggling vulnerabilities using Content-Length/Transfer-Encoding conflicts
- **Internal Network Scanning**: Port scanning capabilities and DNS rebinding protection testing
- **Input Validation**: Comprehensive URL validation with security hardening against malicious inputs

### Advanced Features
- Proxy anonymity detection with IP comparison
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
- -l: File containing proxy list (one per line) (required)
- -config: Path to configuration file (default: config/default.yaml)
- -c: Number of concurrent checks (overrides config)
- -t: Timeout in seconds (overrides config)
- -v: Enable verbose output
- -d: Enable debug mode (shows detailed request/response information)

Security Testing:
- -r: Use reverse DNS lookup for host headers
- Advanced security tests are configured via the YAML config file:
  * SSRF testing against internal networks and cloud metadata
  * Host header injection with multiple attack vectors
  * Protocol smuggling detection
  * DNS rebinding protection testing

Output Options:
- -o: Output results to text file (includes all details and summary)
- -j: Output results to JSON file (structured format for programmatic use)
- -wp: Output only working proxies to a text file (format: proxy - speed)
- -wpa: Output only working anonymous proxies to a text file (format: proxy - speed)
- -no-ui: Disable terminal UI (for automation/scripting)

Rate Limiting:
- -rate-limit: Enable rate limiting to prevent overwhelming target servers
- -rate-delay: Delay between requests (e.g. 500ms, 1s, 2s) (default: 1s)
- -rate-per-host: Apply rate limiting per host instead of globally (default: true)

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
- -discover-min-confidence: Minimum confidence score for candidates
```
### Example Commands

Basic check against default URL:
```bash
./proxyhawk -l proxies.txt
```

Check with increased concurrency and longer timeout:
```bash
./proxyhawk -l proxies.txt -c 20 -t 15s
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