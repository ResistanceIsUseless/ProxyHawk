# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview
ProxyHawk is a comprehensive proxy checker and validator with advanced security testing capabilities. It supports HTTP, HTTPS, HTTP/2, HTTP/3, SOCKS4, and SOCKS5 proxies with features for discovery, validation, and security analysis.

## Versioning Guidance
- Always update the version number in help text and relevant version tracking files when making changes
- Version strings appear in [internal/help/help.go](internal/help/help.go) and [cmd/proxyhawk-server/main.go](cmd/proxyhawk-server/main.go)
- Follow semantic versioning (MAJOR.MINOR.PATCH)
- Increment version number based on the type of changes:
  * MAJOR version for incompatible API changes
  * MINOR version for backwards-compatible new features
  * PATCH version for backwards-compatible bug fixes

## Key Commands

### Building
```bash
# Build both proxyhawk and proxyhawk-server binaries (outputs to ./build/)
make build

# Build all binaries including utilities
go build -v ./...

# Build for all platforms (Linux, macOS, Windows - outputs to ./build/dist/)
make build-all

# Build for specific platform (e.g., kubernetes)
GOOS=linux GOARCH=amd64 go build -o proxyhawk cmd/proxyhawk/main.go
```

### Testing
```bash
# Run all tests
make test
go test -v ./...

# Run specific test
go test -v ./internal/proxy -run TestCheckerBasic

# Run tests with coverage (generates coverage.html)
make test-coverage

# Run tests with race detection
make test-race

# Run short tests (skip long-running tests)
make test-short

# Run benchmarks
make benchmark
go test -bench=. -benchmem ./...
```

### Linting and Code Quality
```bash
# Run linting (uses golangci-lint with config in .golangci.yml)
make lint

# Format code
make fmt

# Run go vet
make vet

# Security vulnerability scan (requires nancy)
make security-scan
```

### Development
```bash
# Run proxy checker with default options
make run

# Run with debug output
make run-debug

# Run with metrics enabled
make run-metrics

# Set up development environment (installs tools)
make dev-setup

# Clean build artifacts and test files
make clean

# Update dependencies
make deps-update

# Install dependencies
go mod download
go mod tidy
```

## Architecture Overview

### Project Structure
- **cmd/proxyhawk/**: Main CLI application for checking proxies
- **cmd/proxyhawk-server/**: Proxy server with SOCKS5/HTTP proxy and geographic testing API
- **cmd/proxyfetch/**: Utility for discovering and fetching proxy lists
- **cmd/viewtest/**: Test utility for UI component development
- **internal/proxy/**: Core proxy checking logic including protocol detection (HTTP/HTTPS/HTTP2/HTTP3/SOCKS4/SOCKS5), advanced security checks, rate limiting, and retry logic
- **internal/discovery/**: Proxy discovery system with Shodan, Censys, free lists, web scraper, and honeypot detection
- **internal/ui/**: Terminal UI implementation using bubbletea/bubbles with multiple view modes (default, verbose, debug)
- **internal/worker/**: Worker pool manager for concurrent proxy checking
- **internal/cloudcheck/**: Cloud provider detection and metadata access validation
- **internal/config/**: Configuration loading, validation, and hot-reloading via file watcher
- **internal/output/**: Result formatting and file output handlers with XSS sanitization
- **internal/metrics/**: Prometheus metrics collection and export
- **internal/logging/**: Structured logging system using slog
- **internal/errors/**: Comprehensive error handling with 30+ error codes
- **internal/validation/**: Input validation for proxy URLs and addresses
- **internal/sanitizer/**: XSS and security sanitization for output
- **internal/progress/**: Progress indicators for non-UI mode
- **internal/pool/**: HTTP connection pooling for performance optimization
- **pkg/server/**: Reusable server components for proxy and API functionality
- **tests/**: Comprehensive test suite including security, output, and integration tests
- **config/**: Configuration files (default.yaml, development.yaml, production.yaml, etc.)

### Core Components

1. **Proxy Checker ([internal/proxy/checker.go](internal/proxy/checker.go))**
   - Three-phase checking process: Type Detection → Validation → Security Checks
   - Multi-protocol proxy validation with automatic protocol detection
   - Rate limiting (per-proxy, per-host, or global) to prevent target server overload
   - Advanced security checks (SSRF, host header injection, protocol smuggling, DNS rebinding, cache poisoning)
   - Cloud provider detection and internal network access testing
   - Retry mechanism with exponential backoff for transient failures
   - Authentication support (basic and digest methods) for HTTP and SOCKS proxies

2. **Configuration System ([internal/config/](internal/config/))**
   - YAML-based configuration (default: [config/default.yaml](config/default.yaml))
   - Multiple example configs: development.yaml, production.yaml, auth-example.yaml, discovery-example.yaml, etc.
   - Command-line flags override config file settings
   - Hot-reloading via file watcher ([internal/config/watcher.go](internal/config/watcher.go)) when --hot-reload flag is enabled
   - Validator ([internal/config/validator.go](internal/config/validator.go)) ensures configuration integrity at load time
   - Supports custom headers, user agents, response validation criteria, and discovery API credentials
   - Three-tier check modes: basic (connectivity), intense (core security), vulns (full scanning)

3. **Concurrent Architecture ([internal/worker/manager.go](internal/worker/manager.go))**
   - Worker pool pattern with configurable concurrency (default: 10)
   - Channel-based job distribution with timeout protection and context-based cancellation
   - Real-time UI updates via tea.Model implementation with bubbletea message passing
   - Thread-safe result collection with mutex protection
   - Graceful shutdown with signal handling (SIGINT, SIGTERM) in main.go
   - Connection pooling ([internal/pool/pool.go](internal/pool/pool.go)) to reuse HTTP clients across checks

4. **Output Formats ([internal/output/output.go](internal/output/output.go))**
   - Text output with status icons (✅ Working, 🔒 Anonymous, ☁️ Cloud, ⚠️ Internal, ❌ Failed)
   - Structured JSON output with detailed results and advanced check findings
   - XSS sanitization ([internal/sanitizer/sanitizer.go](internal/sanitizer/sanitizer.go)) for all output
   - Working proxies list (-wp flag) and anonymous proxies list (-wpa flag)
   - Multiple progress indicator types for non-UI mode: bar, spinner, dots, percent, basic, none

5. **Discovery System ([internal/discovery/](internal/discovery/))**
   - Multiple discovery sources: Shodan, Censys, free proxy lists, web scraping
   - Manager ([internal/discovery/manager.go](internal/discovery/manager.go)) coordinates across sources
   - Intelligent scoring system based on confidence, speed, location, and availability
   - Honeypot detection ([internal/discovery/honeypot.go](internal/discovery/honeypot.go)) to filter monitoring systems
   - Deduplication across sources
   - Country filtering and preset queries optimized for each source

6. **Error Handling ([internal/errors/errors.go](internal/errors/errors.go))**
   - 30+ specific error codes for different failure scenarios
   - Structured error types with context and metadata
   - Error categorization (Config, File I/O, Network, HTTP, Proxy, Validation, Security, System)
   - Error classification (retryable, critical, category-based)
   - Full error wrapping and unwrapping support

7. **Security Features**
   - **Anonymity Detection**: Elite/Anonymous/Transparent/Compromised classification with 10+ header leak checks
   - **Proxy Chain Detection**: Detects proxy-behind-proxy via Via and X-Forwarded-For headers
   - **SSRF Testing**: 60+ targets including cloud metadata (AWS, GCP, Azure), internal networks (RFC 1918, 6598, 3927), localhost variants
   - **Host Header Injection**: Tests X-Forwarded-Host, X-Real-IP, malformed headers, HTTP/1.0 bypasses
   - **Protocol Smuggling**: Detects Content-Length/Transfer-Encoding conflicts
   - **DNS Rebinding Protection**: Tests DNS resolution validation
   - **Cache Poisoning**: Tests cache manipulation vulnerabilities

### Key Features
- Concurrent proxy checking with configurable worker pool and context-based cancellation
- Support for HTTP, HTTPS, HTTP/2, HTTP/3 (experimental), SOCKS4, and SOCKS5 proxies
- Automatic proxy type detection and protocol negotiation
- Three-tier check modes: basic (connectivity), intense (core security), vulns (full vulnerability scanning)
- Advanced security checks: SSRF detection, host header injection, protocol smuggling, DNS rebinding protection, cache poisoning
- Proxy discovery from multiple sources (Shodan, Censys, free lists, web scraping)
- Honeypot detection and filtering for discovered proxies
- Cloud provider detection (AWS, GCP, Azure, etc.) with ASN and organization matching
- Proxy authentication support (basic and digest methods)
- Retry mechanism with exponential backoff for transient failures
- Rate limiting (per-proxy, per-host, or global) to prevent IP bans
- Real-time terminal UI with multiple view modes (default, verbose, debug)
- Progress indicators for non-UI/automation mode
- Metrics collection and Prometheus export
- Configuration hot-reloading for live updates
- Multiple output formats: text, JSON, working proxies, anonymous proxies
- Comprehensive XSS sanitization and security hardening

### Testing Strategy
- Unit tests for core proxy checking logic
- Integration tests for end-to-end proxy validation ([tests/init_test.go](tests/init_test.go))
- Security tests for advanced checks ([tests/security_test.go](tests/security_test.go))
- Output format tests ([tests/output_test.go](tests/output_test.go))
- Configuration loading and validation tests
- Benchmark tests for performance profiling ([tests/proxy/benchmark_test.go](tests/proxy/benchmark_test.go))
- Test helpers in [internal/testhelpers](internal/testhelpers) and [tests/testhelpers](tests/testhelpers)
- Race detection tests to catch concurrency issues (make test-race)

### Important Notes
- **Configuration is never hardcoded**: All paths, patterns, and settings are in config files
- The project uses Go 1.23.0+ with toolchain go1.24.2
- External dependencies include:
  - bubbletea/bubbles/lipgloss for terminal UI
  - interactsh for out-of-band (OOB) security testing
  - h12.io/socks for SOCKS proxy support
  - armon/go-socks5 for SOCKS5 server implementation
  - prometheus/client_golang for metrics collection
  - fsnotify for configuration file watching
- Rate limiting is crucial when testing against production servers to avoid IP bans
- Debug mode (-d flag) enables verbose output and detailed security check information
- Advanced security checks use Interactsh for OOB detection (configure in config.yaml)
- Discovery mode requires API keys for Shodan/Censys (configure in config.yaml or use free sources)
- Connection pooling is enabled by default for better performance across repeated checks
- All output is sanitized to prevent XSS vulnerabilities
- Structured logging with slog is used throughout (no fmt.Print* statements)

### Docker and Deployment
```bash
# Build Docker image
make docker-build

# Run with Docker Compose (includes Prometheus and Grafana)
make docker-compose-up

# Access services:
# - ProxyHawk API: http://localhost:8888/api/health
# - Prometheus: http://localhost:9090
# - Grafana: http://localhost:3000 (admin/admin)

# Stop services
make docker-compose-down
```

### Common Development Patterns

**Adding a new proxy protocol:**
1. Add ProxyType constant to [internal/proxy/types.go](internal/proxy/types.go)
2. Implement protocol detection in [internal/proxy/client.go](internal/proxy/client.go)
3. Add client creation logic in [internal/proxy/checker.go](internal/proxy/checker.go) determineProxyType()
4. Update protocol support flags in ProxyResult struct
5. Add tests in [tests/](tests/) directory

**Adding a new discovery source:**
1. Create new file in [internal/discovery/](internal/discovery/) (e.g., newsource.go)
2. Implement the Discoverer interface with Discover() method
3. Register in [internal/discovery/manager.go](internal/discovery/manager.go) NewManager()
4. Add configuration fields to [internal/config/config.go](internal/config/config.go) DiscoveryConfig
5. Update honeypot detection patterns in [internal/discovery/honeypot.go](internal/discovery/honeypot.go) if needed

**Adding a new security check:**
1. Add check configuration to [internal/proxy/types.go](internal/proxy/types.go) AdvancedChecks struct
2. Implement check logic in [internal/proxy/advanced_checks.go](internal/proxy/advanced_checks.go)
3. Call from [internal/proxy/checker.go](internal/proxy/checker.go) performChecks() method
4. Update ProxyResult SecurityCheckResults in [internal/proxy/types.go](internal/proxy/types.go)
5. Add tests in [tests/security_test.go](tests/security_test.go)

**Adding a new configuration option:**
1. Add field to Config struct in [internal/config/config.go](internal/config/config.go)
2. Update config examples in [config/](config/) directory (default.yaml, etc.)
3. Add validation in [internal/config/validator.go](internal/config/validator.go) if needed
4. Add command-line flag in [cmd/proxyhawk/main.go](cmd/proxyhawk/main.go) if exposing to CLI
5. Document in README.md configuration section

**Adding a new error type:**
1. Define error code constant in [internal/errors/errors.go](internal/errors/errors.go)
2. Create constructor function (e.g., NewNetworkError, NewProxyError)
3. Use structured error in calling code with proper context
4. Add test case in [internal/errors/errors_test.go](internal/errors/errors_test.go)

### Code Quality Standards
- All new code must have test coverage
- Use structured logging ([internal/logging/logger.go](internal/logging/logger.go)) - no fmt.Print* statements
- All errors must use the error system in [internal/errors/errors.go](internal/errors/errors.go)
- All user-facing output must be sanitized via [internal/sanitizer/sanitizer.go](internal/sanitizer/sanitizer.go)
- All configuration must be in YAML files, never hardcoded
- Follow the existing pattern: read multiple files to understand the architecture before making changes
- Run `make lint` before committing to ensure code quality
- Run `make test-race` to check for concurrency issues
