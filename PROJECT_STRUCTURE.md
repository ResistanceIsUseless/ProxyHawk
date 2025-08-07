# ProxyHawk Project Structure

This document describes the organized directory structure of the ProxyHawk project.

## 📁 Repository Organization

```
ProxyHawk/
├── build/                     # Build artifacts (ignored by git)
│   ├── proxyhawk              # Proxy checker binary
│   ├── proxyhawk-server       # Geographic testing server binary
│   └── dist/                  # Multi-platform builds
├── 
├── clients/                   # Client libraries and SDKs
│   └── python/                # Python WebSocket client library
│       ├── proxyhawk_client.py
│       ├── setup.py
│       └── README.md
├── 
├── cmd/                       # Application entry points
│   ├── proxyhawk/             # Main proxy checker CLI
│   ├── proxyhawk-server/      # Geographic testing server
│   ├── proxyfetch/            # Proxy discovery utility
│   └── viewtest/              # Testing utilities
├── 
├── config/                    # Configuration templates (XDG compliant)
│   ├── README.md              # Configuration guide
│   ├── server.template.yaml   # Template for new users
│   ├── server.example.yaml    # Complete example config
│   ├── production.yaml        # Production-ready config
│   └── [specialized configs]  # Auth, metrics, etc.
├── 
├── deployments/               # Deployment configurations
│   ├── README.md              # Deployment guide
│   └── docker/                # Docker containerization
│       ├── Dockerfile         # Multi-stage build
│       ├── docker-compose.yml # Production stack
│       ├── torrc              # Tor configuration
│       └── README.md          # Docker deployment guide
├── 
├── docs/                      # All documentation
│   ├── README.md              # Documentation index
│   ├── REGIONAL_TESTING.md    # Geographic testing guide
│   ├── api/                   # API documentation (future)
│   └── guides/                # User guides
│       ├── CONFIGURATION.md   # Setup and configuration
│       └── CLI_EXAMPLES.md    # Command-line usage
├── 
├── examples/                  # Code examples and demos
│   └── proxy-chaining-demo.go # Proxy chaining example
├── 
├── internal/                  # Private application code
│   ├── config/                # Configuration management
│   ├── discovery/             # Proxy discovery engines
│   ├── logging/               # Structured logging
│   ├── metrics/               # Prometheus metrics
│   ├── proxy/                 # Core proxy testing logic
│   ├── ui/                    # Terminal UI components
│   └── [other modules]        # Various internal packages
├── 
├── pkg/                       # Public library code
│   └── server/                # ProxyHawk server components
│       ├── server.go          # Main dual-mode server
│       ├── websocket_service.go # WebSocket API
│       ├── proxy_router.go    # SOCKS5/HTTP proxy
│       ├── proxy_chain.go     # Proxy chaining logic
│       ├── geographic_tester.go # Geographic DNS testing
│       └── [other modules]    # Pool management, DNS cache
├── 
├── scripts/                   # Utility scripts
│   ├── setup-config.sh        # Configuration setup
│   ├── deploy.sh              # Deployment script
│   └── completions/           # Shell completions
├── 
├── tests/                     # Test suites
│   ├── README.md              # Testing guide
│   ├── integration/           # Integration tests
│   ├── proxy/                 # Proxy-specific tests
│   └── testhelpers/           # Test utilities
├── 
├── CLAUDE.md                  # Claude Code development guide
├── CONFIGURATION.md           # Quick configuration reference
├── Makefile                   # Build automation
├── README.md                  # Project overview
├── go.mod                     # Go module definition
├── go.sum                     # Go dependency checksums
└── VERSION                    # Current version
```

## 🎯 Key Design Principles

### 1. **Separation of Concerns**
- **Source code**: `cmd/`, `internal/`, `pkg/`
- **Configuration**: `config/` (templates), `~/.config/proxyhawk/` (user configs)
- **Deployment**: `deployments/` (Docker, K8s, etc.)
- **Documentation**: `docs/` (all documentation)
- **Build artifacts**: `build/` (ignored by git)

### 2. **XDG Base Directory Compliance**
- Configuration templates in repository: `config/`
- User configurations: `~/.config/proxyhawk/server.yaml`
- Build artifacts: `build/` (not in home directory)

### 3. **Clear Public/Private API Boundaries**
- **`pkg/`**: Stable public APIs for external consumption
- **`internal/`**: Private implementation details
- **`cmd/`**: Application entry points only

### 4. **Deployment-First Organization**
- All deployment configs in `deployments/`
- Environment-specific configurations grouped together
- Clear separation between development and production configs

### 5. **Documentation Co-location**
- Each major component has its own README
- Comprehensive documentation index in `docs/README.md`
- Configuration guides with templates

## 📚 Directory Purposes

### Core Application Code
- **`cmd/`**: Main applications and CLI entry points
- **`internal/`**: Private application logic and utilities
- **`pkg/server/`**: Public server components for ProxyHawk

### Configuration & Deployment
- **`config/`**: Configuration templates and examples
- **`deployments/`**: All deployment scenarios (Docker, K8s, etc.)
- **`scripts/`**: Automation and utility scripts

### Development & Testing
- **`build/`**: Generated binaries and build artifacts
- **`tests/`**: All test code and test data
- **`examples/`**: Working code examples and demonstrations

### Documentation & Clients
- **`docs/`**: Comprehensive documentation with guides
- **`clients/`**: Client libraries for different languages

## 🚀 Usage Patterns

### Development Workflow
```bash
# Setup
git clone <repo>
cd ProxyHawk
make dev-setup

# Build
make build

# Run
./build/proxyhawk-server
./build/proxyhawk -l proxies.txt
```

### Configuration Management
```bash
# Initial setup
./scripts/setup-config.sh

# Edit configuration
nano ~/.config/proxyhawk/server.yaml

# Use custom config
./build/proxyhawk-server -config /path/to/config.yaml
```

### Deployment Options
```bash
# Docker (recommended)
cd deployments/docker
docker-compose up -d

# Local binary
make build
./build/proxyhawk-server
```

## 🔄 Migration Benefits

### Before (Issues)
- Docker files scattered between root and `./docker/`
- Build artifacts cluttering repository root
- Configuration files mixed with source code
- Documentation spread across multiple locations

### After (Improvements)
- **Cleaner root directory**: Only essential files at top level
- **Logical grouping**: Related files organized together
- **Clear ownership**: Each directory has a specific purpose
- **Better maintenance**: Easier to find and update related files
- **Standard compliance**: Follows XDG and Go project conventions

## 📖 Navigation Guide

### New Users
1. Start with [README.md](README.md) for project overview
2. Follow [Configuration Guide](docs/guides/CONFIGURATION.md) for setup
3. Use [deployment instructions](deployments/README.md) for deployment

### Developers
1. Review [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md) (this file)
2. Explore `internal/` and `pkg/` for code organization
3. Check `examples/` for integration patterns

### Operators
1. Use `deployments/` for deployment configurations
2. Reference `docs/guides/` for operational procedures
3. Monitor using configurations in `deployments/docker/`

---

This structure follows Go project layout standards while optimizing for operational excellence and developer experience.