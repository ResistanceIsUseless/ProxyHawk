# ProxyHawk Configuration

ProxyHawk uses a clean, XDG-compliant configuration system that separates templates from user configurations.

## 📁 Directory Structure

```
ProxyHawk/
├── config/                    # Templates and examples (don't edit)
│   ├── README.md             # Complete configuration guide
│   ├── server.template.yaml  # Template for new installations
│   ├── server.example.yaml   # Complete example with all options
│   ├── production.yaml       # Production-ready template
│   └── [other examples]      # Specialized configuration examples
├── scripts/
│   └── setup-config.sh       # Automated setup script
└── ~/.config/proxyhawk/      # User configurations (default location)
    └── server.yaml           # Your personal config
```

## 🚀 Quick Setup

### 1. Automated Setup (Recommended)
```bash
# Run the setup script
./scripts/setup-config.sh

# Edit the generated config
nano ~/.config/proxyhawk/server.yaml

# Start ProxyHawk
./proxyhawk-server
```

### 2. Manual Setup
```bash
# Create config directory
mkdir -p ~/.config/proxyhawk

# Copy template
cp config/server.template.yaml ~/.config/proxyhawk/server.yaml

# Edit with your proxy settings
nano ~/.config/proxyhawk/server.yaml

# Start server
./proxyhawk-server
```

## ⚙️ Configuration Locations

ProxyHawk looks for configuration files in this order:

1. **Command line**: `-config /path/to/config.yaml`
2. **Default user config**: `~/.config/proxyhawk/server.yaml`  
3. **Fallback**: Uses built-in defaults

## 📖 Key Configuration Sections

```yaml
# Server mode and network addresses
mode: dual                    # proxy, agent, or dual
socks5_addr: ":1080"         # SOCKS5 proxy port
http_addr: ":8080"           # HTTP proxy port
api_addr: ":8888"            # WebSocket API port

# Proxy regions (CUSTOMIZE THIS)
regions:
  us-west:
    name: "US West Coast"
    proxies:
      - url: "socks5://your-proxy.com:1080"
        weight: 10
        health_check_url: "http://httpbin.org/ip"

# Geographic testing
round_robin_detection:
  enabled: true
  min_samples: 5

# Performance and monitoring
cache:
  enabled: true
  ttl: 5m
metrics:
  enabled: false
  addr: ":9090"
```

## 🔧 Usage Examples

```bash
# Use default config location
./proxyhawk-server

# Use specific config file
./proxyhawk-server -config /path/to/custom.yaml

# Start only as proxy server
./proxyhawk-server -mode proxy

# Start with metrics enabled
./proxyhawk-server -metrics -metrics-addr :9090
```

## 🐳 Docker Configuration

Docker deployments use the configurations from the `config/` directory:

```bash
# Uses config/production.yaml by default
docker-compose up -d

# Override with environment variables
PROXYHAWK_LOG_LEVEL=debug docker-compose up -d
```

## 📚 More Information

- **Complete guide**: `config/README.md`
- **Example configs**: `config/` directory
- **Docker setup**: `docker-compose.yml`
- **Client libraries**: `clients/` directory

---

**Note**: The `config/` directory contains templates and examples only. Always place your actual configuration files in `~/.config/proxyhawk/` to avoid conflicts during updates.