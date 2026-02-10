# ProxyHawk Configuration Guide

ProxyHawk uses a flexible configuration system that supports multiple deployment scenarios. Configuration files are organized by purpose to make it easy to find and customize settings.

## 📁 Directory Structure

```
config/
├── client/              # ProxyHawk CLI configurations
│   └── default.yaml     # Default client configuration (comprehensive)
├── server/              # ProxyHawk Server configurations
│   ├── server.default.yaml   # Default server configuration
│   ├── server.example.yaml   # Fully documented server example
│   ├── production.yaml       # Production-ready settings
│   └── development.yaml      # Development settings
├── examples/            # Example configurations for specific features
│   ├── auth-example.yaml
│   ├── connection-pool-example.yaml
│   ├── discovery-example.yaml
│   ├── metrics-example.yaml
│   ├── retry-example.yaml
│   ├── multi-host.example.yaml
│   └── proxy-chaining.yaml
├── default.yaml         # Symlink to client/default.yaml (for backward compatibility)
└── README.md            # This file
```

## 🚀 Quick Start

### ProxyHawk CLI (Proxy Checking)

ProxyHawk automatically creates a user configuration file on first run:

```bash
# First run - auto-generates ~/.config/proxyhawk/config.yaml
./proxyhawk -l proxies.txt

# The config is created at:
# - Linux/macOS: ~/.config/proxyhawk/config.yaml
# - Or: $XDG_CONFIG_HOME/proxyhawk/config.yaml if XDG_CONFIG_HOME is set
```

**To customize your settings:**

```bash
# Edit your user config
nano ~/.config/proxyhawk/config.yaml

# Or use a custom config file
./proxyhawk -config /path/to/custom.yaml -l proxies.txt
```

**Configuration precedence (highest to lowest):**
1. Command-line flags (`-t`, `-c`, `-rate-limit`, etc.)
2. Environment variables (`SHODAN_API_KEY`, `CENSYS_API_ID`, etc.)
3. User config file (`~/.config/proxyhawk/config.yaml`)
4. Built-in defaults

### ProxyHawk Server (Proxy/Agent Service)

```bash
# Use default server config
./proxyhawk-server

# Use custom config
./proxyhawk-server -config config/server/production.yaml

# Override with environment variables
PROXYHAWK_MODE=dual PROXYHAWK_SOCKS5_ADDR=:1080 ./proxyhawk-server
```

## 📖 Configuration Files Explained

### Client Configurations

#### `client/default.yaml`
- **Purpose**: Comprehensive default configuration for ProxyHawk CLI
- **Use case**: Reference for all available options
- **Auto-generated**: Copied to `~/.config/proxyhawk/config.yaml` on first run
- **Features**: Proxy checking, vulnerability scanning, discovery, rate limiting, retries, cloud detection

**Key sections:**
- General settings (timeouts, TLS verification, concurrency)
- Rate limiting (per-host, per-proxy, global)
- Retry mechanism (exponential backoff, error patterns)
- Authentication (basic, digest)
- HTTP headers and user agent
- Test URLs for validation
- Response validation rules
- Interactsh (OOB testing for vulnerabilities)
- Advanced security checks (smuggling, cache poisoning, DNS rebinding)
- Cloud provider detection (AWS, GCP, Azure, DigitalOcean)
- Protocol support (HTTP/2, HTTP/3)
- Fingerprinting
- Connection pooling
- Metrics (Prometheus)
- Discovery settings (Shodan, Censys, honeypot filtering)

### Server Configurations

#### `server/server.default.yaml`
- **Purpose**: Minimal working server configuration
- **Use case**: Quick testing and development
- **Features**: Basic dual-mode setup (SOCKS5 + HTTP proxy + WebSocket API)

#### `server/server.example.yaml`
- **Purpose**: Fully documented server configuration with all options
- **Use case**: Learning what's configurable, creating custom configs
- **Features**: Regional proxies, health checks, round-robin DNS detection, caching, metrics

#### `server/production.yaml`
- **Purpose**: Production-ready configuration with security hardening
- **Use case**: Deploying to production environments
- **Features**: Optimized timeouts, health monitoring, metrics enabled, security settings

#### `server/development.yaml`
- **Purpose**: Development-friendly configuration
- **Use case**: Local testing and debugging
- **Features**: Debug logging, relaxed timeouts, mock proxies

### Example Configurations

#### `examples/auth-example.yaml`
- **Purpose**: Proxy authentication configuration
- **Features**: Basic auth, digest auth, default credentials

#### `examples/connection-pool-example.yaml`
- **Purpose**: HTTP connection pool tuning
- **Features**: Connection limits, timeouts, keep-alives

#### `examples/discovery-example.yaml`
- **Purpose**: Proxy discovery with Shodan/Censys
- **Features**: API credentials, search filters, honeypot detection

#### `examples/metrics-example.yaml`
- **Purpose**: Prometheus metrics configuration
- **Features**: Metrics endpoint, exporters, custom metrics

#### `examples/retry-example.yaml`
- **Purpose**: Retry logic and error handling
- **Features**: Exponential backoff, retryable errors, circuit breakers

#### `examples/multi-host.example.yaml`
- **Purpose**: Multi-region proxy configuration
- **Features**: Regional proxy pools, load balancing

#### `examples/proxy-chaining.yaml`
- **Purpose**: Advanced proxy chaining setup
- **Features**: Multi-hop proxies, Tor integration

## 🔧 Common Configuration Tasks

### Enable Vulnerability Scanning

Edit `~/.config/proxyhawk/config.yaml`:

```yaml
# Enable advanced security checks
advanced_checks:
  test_protocol_smuggling: true     # HTTP request smuggling
  test_dns_rebinding: true          # DNS rebinding attacks
  test_cache_poisoning: true        # Cache poisoning
  test_host_header_injection: true  # Host header injection
  test_ipv6: true                   # IPv6 support testing
  test_http_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS", "TRACE"]

# Enable fingerprinting to detect proxy software
enable_fingerprint: true

# Configure Interactsh for out-of-band testing
interactsh_url: "https://interact.sh"
interactsh_token: ""  # Optional: use your own Interactsh server
```

### Set Up Proxy Discovery

```yaml
discovery:
  # API credentials (use environment variables for security)
  shodan_api_key: ""  # Or set SHODAN_API_KEY env var
  censys_api_key: ""  # Or set CENSYS_API_ID env var
  censys_secret: ""   # Or set CENSYS_SECRET env var

  # Search parameters
  max_results: 1000
  countries: ["US", "GB", "DE"]  # Target specific countries
  min_confidence: 0.5            # Higher = fewer but better results

  # Security filters
  enable_honeypot_filter: true   # Automatically detect and filter honeypots
  honeypot_threshold: 0.4        # Suspicion threshold (0.0-1.0)
  exclude_residential: true      # Exclude residential IPs
  exclude_cdn: true              # Exclude CDN/cloud IPs
```

**Use discovery:**
```bash
# Discover proxies using Shodan (requires API key)
export SHODAN_API_KEY="your-key-here"
./proxyhawk -discover -country US -max-results 500

# Discover from free sources (no API key required)
./proxyhawk -discover -source free
```

### Enable Rate Limiting

```yaml
# Prevent IP bans by rate limiting requests
rate_limit_enabled: true
rate_limit_delay: 2s           # 2 seconds between requests
rate_limit_per_host: true      # Rate limit per target host
rate_limit_per_proxy: false    # Don't rate limit per proxy (faster checking)
```

### Configure Retry Logic

```yaml
retry_enabled: true
max_retries: 3
initial_retry_delay: 1s
max_retry_delay: 30s
backoff_factor: 2.0
retryable_errors:
  - "connection refused"
  - "connection timed out"
  - "i/o timeout"
```

### Enable Cloud Detection

```yaml
enable_cloud_checks: true

# Cloud providers are already configured by default
# (AWS, GCP, Azure, DigitalOcean)
```

### Enable Metrics Export

```yaml
metrics:
  enabled: true
  listen_addr: ":9090"
  path: "/metrics"
```

Then access metrics at: `http://localhost:9090/metrics`

### Optimize Performance

```yaml
# Increase concurrency for faster scanning
concurrency: 50

# Connection pooling for better performance
connection_pool:
  max_idle_conns: 200
  max_idle_conns_per_host: 20
  max_conns_per_host: 100
  idle_conn_timeout: "90s"
  keep_alive_timeout: "30s"

# Enable HTTP/2 for modern proxies
enable_http2: true
```

## 🔐 Security Best Practices

### 1. API Credentials

**Never commit API keys to version control!**

Use environment variables:
```bash
export SHODAN_API_KEY="your-key-here"
export CENSYS_API_ID="your-id-here"
export CENSYS_SECRET="your-secret-here"
```

Or use a separate credentials file outside the repo:
```bash
./proxyhawk -config ~/.config/proxyhawk/config.yaml -l proxies.txt
```

### 2. TLS Verification

For production use, enable TLS verification:
```yaml
insecure_skip_verify: false  # Verify TLS certificates
```

For testing/development only:
```yaml
insecure_skip_verify: true   # WARNING: insecure, for testing only
```

### 3. Rate Limiting

Always use rate limiting when scanning public targets:
```yaml
rate_limit_enabled: true
rate_limit_delay: 2s  # Adjust based on target sensitivity
```

### 4. Proxy Authentication

Store credentials securely (not in config files):
```yaml
auth_enabled: true
default_username: ""  # Set via environment variable
default_password: ""  # Set via environment variable
```

```bash
export PROXY_USERNAME="user"
export PROXY_PASSWORD="pass"
```

## 🐳 Docker Configuration

Docker deployments use volume mounts for configuration:

```yaml
# docker-compose.yml
version: '3.8'
services:
  proxyhawk-server:
    image: proxyhawk:latest
    volumes:
      - ./config/server/production.yaml:/app/config.yaml:ro
    ports:
      - "1080:1080"  # SOCKS5
      - "8080:8080"  # HTTP
      - "8888:8888"  # WebSocket API
```

Override with environment variables:
```bash
docker run -e PROXYHAWK_MODE=dual \
           -e PROXYHAWK_LOG_LEVEL=debug \
           proxyhawk:latest
```

## 🔍 Troubleshooting

### Config file not found

```bash
# Check which config is being used
./proxyhawk -h

# Verify user config location
ls -la ~/.config/proxyhawk/config.yaml

# Force regenerate user config
rm ~/.config/proxyhawk/config.yaml
./proxyhawk -l proxies.txt
```

### Invalid YAML syntax

```bash
# Validate YAML
yamllint ~/.config/proxyhawk/config.yaml

# Check for common issues:
# - Tabs instead of spaces (YAML requires spaces)
# - Misaligned indentation
# - Missing colons or quotes
```

### Configuration not taking effect

Check precedence:
1. CLI flags override everything
2. Environment variables override config file
3. Config file overrides defaults

```bash
# See what config is loaded
./proxyhawk -d -l proxies.txt 2>&1 | grep "Config loaded"
```

### Permission denied on ~/.config/proxyhawk

```bash
# Fix permissions
mkdir -p ~/.config/proxyhawk
chmod 755 ~/.config/proxyhawk
```

## 📚 Additional Resources

- [Main README](../README.md) - Full ProxyHawk documentation
- [CLAUDE.md](../CLAUDE.md) - Architecture and development guide
- [Examples](examples/) - Feature-specific configuration examples
- [GitHub Issues](https://github.com/ResistanceIsUseless/ProxyHawk/issues) - Report problems or request features

## 🎓 Configuration Examples by Use Case

### Basic Proxy Testing
```yaml
timeout: 10
concurrency: 10
enable_cloud_checks: false
enable_anonymity_check: true
```

### Security Research
```yaml
timeout: 30
concurrency: 5
enable_fingerprint: true
advanced_checks:
  test_protocol_smuggling: true
  test_cache_poisoning: true
  test_host_header_injection: true
```

### Large-Scale Discovery
```yaml
concurrency: 100
rate_limit_enabled: true
rate_limit_delay: 500ms
discovery:
  max_results: 10000
  enable_honeypot_filter: true
```

### Production Monitoring
```yaml
retry_enabled: true
max_retries: 3
metrics:
  enabled: true
connection_pool:
  max_idle_conns: 200
```

---

**Last Updated**: 2026-02-09
**Version**: 1.3.0
**ProxyHawk**: https://github.com/ResistanceIsUseless/ProxyHawk
