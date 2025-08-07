# ProxyHawk Configuration Guide

This directory contains configuration templates and examples for ProxyHawk. 

**User configurations should be placed in `~/.config/proxyhawk/`** following XDG Base Directory specification.

## 📁 Configuration Files Overview

### Server Configuration
- **`server.example.yaml`** - Complete example for ProxyHawk server with all options documented
- **`server.default.yaml`** - Default server configuration for development
- **`production.yaml`** - Production-ready configuration with security hardening
- **`development.yaml`** - Development configuration with debug features enabled

### Specialized Configurations
- **`multi-host.example.yaml`** - Multi-host deployment configuration
- **`proxy-chaining.yaml`** - Advanced proxy chaining configuration examples
- **`auth-example.yaml`** - Authentication and security configuration
- **`metrics-example.yaml`** - Prometheus metrics and monitoring setup
- **`connection-pool-example.yaml`** - Connection pooling and performance tuning
- **`retry-example.yaml`** - Retry policies and error handling
- **`discovery-example.yaml`** - Service discovery and health checking

## 🚀 Quick Start

### Basic Server Setup

```bash
# Run the setup script to initialize config
./scripts/setup-config.sh

# Edit with your settings
nano ~/.config/proxyhawk/server.yaml

# Start the server (uses default config location)
./proxyhawk-server
```

### Docker Deployment

```bash
# Use the production configuration
docker-compose up -d
# Configuration is automatically mounted from config/production.yaml
```

## 📖 Configuration Reference

### Server Modes

ProxyHawk supports three operational modes:

- **`proxy`** - Traditional proxy server only (SOCKS5 + HTTP)
- **`agent`** - Geographic testing service only (WebSocket API)
- **`dual`** - Both proxy and testing capabilities (recommended)

### Core Settings

```yaml
# Server operation mode
mode: dual

# Network addresses
socks5_addr: ":1080"    # SOCKS5 proxy port
http_addr: ":8080"      # HTTP proxy port  
api_addr: ":8888"       # WebSocket API port

# Proxy selection strategy
selection_strategy: smart  # random, round_robin, smart, weighted
```

### Regional Proxy Configuration

```yaml
regions:
  us-west:
    name: "US West Coast"
    proxies:
      - url: "socks5://proxy1.example.com:1080"
        weight: 10
        health_check_url: "http://httpbin.org/ip"
      - url: "http://proxy2.example.com:8080"  
        weight: 8
        health_check_url: "http://httpbin.org/ip"
  
  eu-west:
    name: "Western Europe"
    proxies:
      - url: "socks5://eu-proxy.example.com:1080"
        weight: 10
        health_check_url: "http://httpbin.org/ip"
```

### Advanced Features

#### Round-Robin DNS Detection
```yaml
round_robin_detection:
  enabled: true
  min_samples: 5
  sample_interval: 2s
  confidence_threshold: 0.85
```

#### Health Checking  
```yaml
health_check:
  enabled: true
  interval: 1m
  timeout: 10s
  failure_threshold: 3
  success_threshold: 2
```

#### DNS Caching
```yaml
cache:
  enabled: true
  ttl: 5m
  max_entries: 10000
```

#### Metrics and Monitoring
```yaml
metrics:
  enabled: true
  addr: ":9090"
  path: "/metrics"
```

### Security Configuration

#### Authentication (Future Feature)
```yaml
auth:
  enabled: false
  method: "token"  # token, basic, oauth
  tokens:
    - "your-secure-token-here"
```

#### TLS Configuration
```yaml
tls:
  enabled: false
  cert_file: "/path/to/cert.pem"
  key_file: "/path/to/key.pem"
  ca_file: "/path/to/ca.pem"  # For client cert validation
```

## 🔧 Environment-Specific Configurations

### Development Environment
- Debug logging enabled
- Relaxed timeouts
- Mock proxies for testing
- Hot reload capabilities

### Production Environment  
- Security hardening
- Performance optimization
- Monitoring and alerting
- Backup and recovery

### Testing Environment
- Isolated test proxies
- Extended logging
- Chaos engineering support
- Load testing configurations

## 📊 Configuration Validation

ProxyHawk validates all configuration files on startup:

```bash
# Validate using default config location
./proxyhawk-server -help

# Validate specific config file
./proxyhawk-server -config ~/.config/proxyhawk/server.yaml -help
```

## 🐳 Docker Configuration

### Environment Variables

All YAML configuration options can be overridden with environment variables:

```bash
# Override server mode
PROXYHAWK_MODE=dual

# Override addresses
PROXYHAWK_SOCKS5_ADDR=:1080
PROXYHAWK_HTTP_ADDR=:8080
PROXYHAWK_API_ADDR=:8888

# Override regions (JSON format)
PROXYHAWK_REGIONS='{"us-west":{"name":"US West","proxies":[...]}}'

# Override logging
PROXYHAWK_LOG_LEVEL=debug
PROXYHAWK_LOG_FORMAT=json
```

### Docker Compose Override

```yaml
# docker-compose.override.yml
version: '3.8'
services:
  proxyhawk-server:
    environment:
      - PROXYHAWK_LOG_LEVEL=debug
      - PROXYHAWK_METRICS_ENABLED=true
    volumes:
      - ./config/development.yaml:/app/config.yaml:ro
```

## 🔍 Configuration Examples by Use Case

### 1. Basic Proxy Server
```yaml
mode: proxy
socks5_addr: ":1080"
http_addr: ":8080"
log_level: info
```

### 2. Geographic Testing Service
```yaml
mode: agent
api_addr: ":8888"
regions:
  us-west: {...}
  eu-west: {...}
round_robin_detection:
  enabled: true
```

### 3. Enterprise Deployment
```yaml
mode: dual
selection_strategy: smart
health_check:
  enabled: true
  interval: 30s
metrics:
  enabled: true
auth:
  enabled: true
tls:
  enabled: true
```

### 4. High-Performance Setup
```yaml
connection_pool:
  max_idle_conns: 100
  idle_timeout: 90s
  keep_alive: 30s
cache:
  enabled: true
  max_entries: 50000
  ttl: 10m
```

## 🚨 Security Best Practices

### 1. Production Deployment
- Use TLS encryption for all connections
- Enable authentication for WebSocket API
- Restrict API access with firewall rules
- Regular security updates and monitoring

### 2. Proxy Configuration
- Validate proxy URLs and credentials
- Use health checks to detect compromised proxies
- Rotate proxy credentials regularly
- Monitor for unusual traffic patterns

### 3. Logging and Monitoring
- Enable structured logging in production
- Monitor proxy performance and availability
- Set up alerting for failures and security events
- Regular log analysis and cleanup

## 🔧 Troubleshooting

### Common Configuration Issues

1. **Invalid YAML syntax**
   ```bash
   # Validate YAML syntax
   yamllint config/server.yaml
   ```

2. **Port conflicts**
   ```bash
   # Check port availability
   netstat -tulpn | grep :1080
   ```

3. **Proxy connectivity**
   ```bash
   # Test proxy manually
   curl --proxy socks5://proxy.example.com:1080 http://httpbin.org/ip
   ```

### Configuration Debugging

```yaml
# Enable debug logging
log_level: debug
log_format: json

# Enable configuration validation
validate_config: true

# Test mode for development
test_mode:
  enabled: true
  mock_proxies: true
  skip_health_checks: false
```

## 📚 Additional Resources

- [ProxyHawk Architecture Guide](../docs/architecture.md)
- [API Documentation](../docs/api.md)
- [Docker Deployment Guide](../docs/docker.md)
- [Client Libraries](../clients/README.md)
- [Contributing Guidelines](../CONTRIBUTING.md)

## 📧 Support

For configuration help and support:
- GitHub Issues: [ProxyHawk Issues](https://github.com/ResistanceIsUseless/ProxyHawk/issues)
- Documentation: [docs/](../docs/)
- Examples: [examples/](../examples/)

---

**Last Updated**: 2024-08-07  
**Version**: 1.0.0