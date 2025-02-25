# ProxyHawk

A comprehensive proxy checker and validator with advanced features for testing HTTP, HTTPS, SOCKS4, and SOCKS5 proxies.

**This tool is still a work in progress and not all features are available.**
The following issues need to be addressed:
- [ ] Get tests working
- [ ] Tune checking parameters
- [ ] Clean up output to be more concise
- [ ] Test advanced security checks

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

## Features

- Concurrent proxy checking using goroutines
- Configurable number of concurrent checks
- Support for HTTP, HTTPS, and SOCKS proxies
- Detailed output with success rate and timing information
- Optional debug output for troubleshooting
- Configurable timeouts and retry attempts
- Custom HTTP headers and User-Agent support
- Multiple output formats (text, JSON)
- Proxy anonymity detection
- Cloud provider detection
- Internal network testing
- Advanced security checks

## Configuration

The application uses a YAML configuration file (default: `config.yaml`) to define cloud provider settings and validation rules. The configuration includes:

- Cloud provider definitions
- Default HTTP headers
- User-Agent settings
- Response validation rules
- Advanced security check configurations

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
go run main.go -l proxies.txt
```

### Command Line Options
```
- -l: File containing proxy list (one per line) (required)
- -u: URL to test proxies against (default: https://www.google.com)
- -i: Enable Interactsh validation for additional OOB testing
- -p: Enable IPInfo validation to check proxy anonymity
- -cloud: Enable cloud provider detection and internal network testing
- -config: Path to configuration file (default: config.yaml)
- -c: Number of concurrent checks (default: 10)
- -t`: Timeout for each proxy check (default: 10s)
- -d: Enable debug output (shows full request/response details)
- -o: Output results to text file (includes all details and summary)
- -j: Output results to JSON file (structured format for programmatic use)
- -wp: Output only working proxies to a text file (format: proxy - speed)
- -wpa: Output only working anonymous proxies to a text file (format: proxy - speed)

# Rate Limiting Options
- -rate-limit: Enable rate limiting to prevent overwhelming target servers
- -rate-delay: Delay between requests (e.g. 500ms, 1s, 2s) (default: 1s)
- -rate-per-host: Apply rate limiting per host instead of globally (default: true)
```
### Example Commands

Basic check against default URL:
```bash
go run main.go -l proxies.txt
```

Check against specific URL with increased concurrency and longer timeout:
```bash
go run main.go -l proxies.txt -u https://example.com -c 20 -t 15s
```

Enable all validation methods with debug output:
```bash
go run main.go -l proxies.txt -i -p -cloud -d
```

Save results to text and JSON files:
```bash
go run main.go -l proxies.txt -o results.txt -j results.json
```

Test with custom configuration and all advanced checks:
```bash
go run main.go -l proxies.txt -config custom_config.yaml -i -p -cloud -d -t 20s
```

Check anonymity and cloud provider detection only:
```bash
go run main.go -l proxies.txt -p -cloud -c 15
```

Maximum performance testing:
```bash
go run main.go -l proxies.txt -c 50 -t 5s
```

Debug mode with specific URL and timeout:
```bash
go run main.go -l proxies.txt -d -u https://api.example.com/test -t 30s
```

Interactsh validation only:
```bash
go run main.go -l proxies.txt -i -c 25
```

Full validation with custom output files:
```bash
go run main.go -l proxies.txt -i -p -cloud -d -o custom_results.txt -j custom_results.json -t 25s -c 30
```

Check proxies and save working ones to a separate file:
```bash
go run main.go -l proxies.txt -wp working_proxies.txt
```

Check proxies and save working anonymous ones to a separate file:
```bash
go run main.go -l proxies.txt -p -wpa working_anonymous_proxies.txt
```

Check proxies with rate limiting to avoid IP bans:
```bash
go run main.go -l proxies.txt -rate-limit -rate-delay 2s
```

### Output Formats

#### Text Output (-o)
The text output file includes:
- Status icons for each proxy:
  - ✅ Working proxy
  - 🔒 Anonymous proxy
  - ☁️ Cloud provider proxy
  - ⚠️ Internal network access
  - ❌ Failed proxy
- Response time and error messages
- IP information when available
- Cloud provider details
- Advanced security check results
- Timestamp of each check
- Summary statistics

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
      "advanced_checks": {
        "protocol_smuggling": false,
        "dns_rebinding": false,
        "cache_poisoning": false,
        "nonstandard_ports": {
          "8080": true,
          "3128": false
        },
        "ipv6_supported": true,
        "method_support": {
          "GET": true,
          "POST": true,
          "PUT": false
        },
        "path_traversal": false,
        "vuln_details": {}
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
