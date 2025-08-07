# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Versioning Guidance
- Always update the version number in help text and relevant version tracking files when making changes
- Follow semantic versioning (MAJOR.MINOR.PATCH)
- Increment version number based on the type of changes:
  * MAJOR version for incompatible API changes
  * MINOR version for backwards-compatible new features
  * PATCH version for backwards-compatible bug fixes

## Key Commands

### Building
```bash
# Build the main proxyhawk binary
go build -o proxyhawk cmd/proxyhawk/main.go

# Build all binaries
go build -v ./...

# Build for external systems (e.g., kubernetes)
GOOS=linux GOARCH=amd64 go build -o proxyhawk cmd/proxyhawk/main.go
```

### Testing
```bash
# Run all tests
make test

# Run tests with coverage
make coverage

# Run specific test
make test-one test=TestName

# Run tests in verbose mode
make test-verbose

# Run short tests (skip long-running tests)
make test-short
```

### Linting and Code Quality
```bash
# Run linting (uses golangci-lint with config in .golangci.yml)
make lint
golangci-lint run ./...

# Format code
go fmt ./...
gofmt -s -w .

# Security check
gosec ./...
```

### Development
```bash
# Run proxy checker with default options
go run cmd/proxyhawk/main.go -l test_proxies.txt -o results.txt -j results.json -d

# Clean build artifacts and test files
make clean

# Install dependencies
go mod download
go mod tidy
```

## Architecture Overview

### Project Structure
- **cmd/proxyhawk/**: Main application entry point with CLI flags, configuration loading, and concurrent proxy checking orchestration
- **internal/proxy/**: Core proxy checking logic including protocol detection (HTTP/HTTPS/SOCKS4/SOCKS5), advanced security checks, and rate limiting
- **internal/ui/**: Terminal UI implementation using bubbletea/bubbles for real-time progress visualization
- **cloudcheck/**: Cloud provider detection and metadata access validation
- **tests/**: Comprehensive test suite with Makefile for easy test execution

### Core Components

1. **Proxy Checker (internal/proxy/checker.go)**
   - Handles multi-protocol proxy validation with automatic protocol detection
   - Implements rate limiting (per-host or global) to prevent target server overload
   - Performs advanced security checks (protocol smuggling, DNS rebinding, cache poisoning)
   - Supports cloud provider detection and internal network access testing

2. **Configuration System**
   - YAML-based configuration (config.yaml) for cloud providers, validation rules, and advanced checks
   - Command-line flags override config file settings
   - Supports custom headers, user agents, and response validation criteria

3. **Concurrent Architecture**
   - Worker pool pattern with configurable concurrency (default: 10)
   - Channel-based job distribution with timeout protection
   - Real-time UI updates via tea.Model implementation
   - Thread-safe result collection with mutex protection

4. **Output Formats**
   - Text output with status icons (✅ Working, 🔒 Anonymous, ☁️ Cloud, ⚠️ Internal, ❌ Failed)
   - Structured JSON output with detailed results and advanced check findings
   - Working proxies list (-wp flag) and anonymous proxies list (-wpa flag)

### Key Features
- Concurrent proxy checking with configurable worker pool
- Support for HTTP, HTTPS, SOCKS4, and SOCKS5 proxies
- Advanced security checks (when enabled with debug mode or specific flags)
- Cloud provider detection (AWS, GCP, Azure, etc.)
- Rate limiting to prevent IP bans
- Real-time terminal UI with progress tracking
- Multiple output formats for different use cases

### Testing Strategy
- Unit tests for core proxy checking logic
- Integration tests for end-to-end proxy validation
- Security tests for advanced checks
- Output format tests
- Configuration loading tests
- Test helpers in internal/testhelpers and tests/testhelpers

### Important Notes
- The project uses Go 1.22+ with toolchain go1.23.6
- External dependencies include bubbletea for UI, interactsh for OOB testing, and h12.io/socks for SOCKS support
- Rate limiting is crucial when testing against production servers
- Debug mode (-d flag) enables verbose output and advanced security checks