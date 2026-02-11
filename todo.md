# ProxyHawk TODO List

Last updated: 2026-02-10

## ✅ Completed Tasks (v1.5.1)

### Priority 2 Advanced SSRF Checks
- [x] Implement SNI Proxy SSRF tests (TLS SNI field manipulation)
- [x] Implement real DNS rebinding tests (using public rebinding services)
- [x] Implement HTTP/2 header injection tests (CRLF in binary headers)
- [x] Implement AWS IMDSv2 token workflow bypass tests
- [x] Add 4 new fields to AdvancedSSRFResult struct
- [x] Integrate Priority 2 checks into performAdvancedSSRFChecks()
- [x] Update test summary to count new checks (+22 test cases)
- [x] Create vulns_ssrf_advanced_priority2.go (400+ lines)
- [x] Test against Nginx Kubernetes Gateway
- [x] Document implementation in V1.5.1_PRIORITY2_IMPLEMENTATION.md

## ✅ Completed Tasks (v1.5.0)

### Advanced SSRF Vulnerability Checks
- [x] Implement URL Parser Differential tests (Orange Tsai research)
  - 13 parser confusion patterns (@, backslash, null bytes, encoding)
- [x] Implement IP Obfuscation Bypass tests
  - 15+ IP representation formats (decimal, octal, hex, abbreviated)
- [x] Implement Redirect Chain SSRF tests
  - 3 scenarios targeting AWS/GCP metadata and localhost
- [x] Implement Protocol Smuggling tests
  - 9 non-HTTP protocol schemes (file://, gopher://, dict://, etc.)
- [x] Implement Header Injection SSRF tests
  - 40 combinations (10 headers × 4 internal targets)
- [x] Implement Nginx proxy_pass Traversal tests
  - 7 path traversal patterns
- [x] Implement Host Header SSRF tests
  - 5 internal host targets
- [x] Create vulns_ssrf_advanced.go (700+ lines)
- [x] Add AdvancedSSRFResult struct to types.go
- [x] Integrate advanced SSRF checks into checker.go fallback logic
- [x] Add test summary logging to show all checks performed
- [x] Create SECURITY_RESEARCH_ANALYSIS.md
- [x] Create V1.5.0_IMPLEMENTATION_SUMMARY.md
- [x] Test against Nginx Kubernetes Gateway (10.176.17.250)

### Fallback Mode Implementation
- [x] Modify checker.go to fallback if proxy connection fails
- [x] Add performDirectScan() function for direct vulnerability testing
- [x] Integrate all 55 standard vulnerability checks into fallback
- [x] Integrate advanced SSRF checks into fallback
- [x] Test all modes (basic, intense, vulns) with fallback

## ✅ Completed Tasks (v1.4.0)

### Path-Based Fingerprinting Implementation
- [x] Implement path-based fingerprinting for reverse proxies and API gateways
- [x] Add direct HTTP request capability (not through proxy)
- [x] Test multiple paths (/, /admin, /api, /v1, etc.) - 20 default paths
- [x] Compare server headers across different paths
- [x] Detect backend routing/rewriting from header differences
- [x] Identify proxy software from error page content (when Server header is hidden)
- [x] Detect backend frameworks (Django, Flask, Spring, Rails, etc.)
- [x] Identify routing patterns (API versioning, admin interfaces, health endpoints)
- [x] Add -path-fingerprint and -paths command-line flags
- [x] Test against Nginx Kubernetes Gateway (10.176.17.250)
- [x] Update README.md with path-based fingerprinting documentation

## ✅ Completed Tasks (v1.3.0)

### Configuration System Overhaul
- [x] Analyze current config structure and identify client vs server configs
- [x] Create ~/.config/proxyhawk directory structure with XDG compliance
- [x] Generate comprehensive default client config (config/client/default.yaml)
- [x] Move server configs to config/server/ directory
- [x] Move example configs to config/examples/ directory
- [x] Update old default.yaml to symlink to new client config
- [x] Implement config initialization in cmd/proxyhawk/main.go
  - Auto-generates ~/.config/proxyhawk/config.yaml on first run
  - Follows XDG Base Directory specification
  - Provides helpful user feedback
- [x] Test build to ensure no compilation errors
- [x] Update config/README.md for new structure
  - Complete documentation of all config files
  - Usage examples for common scenarios
  - Security best practices
  - Troubleshooting guide

### Vulnerability Scanning Implementation (COMPLETE - 55/55 checks)
- [x] Implement Priority 1 (Critical) vulnerability checks
  - Nginx CVE-2025-1974, WebSocket abuse, HTTP/2 smuggling
- [x] Implement Priority 2 (High-Impact) vulnerability checks
  - 10 checks for generic misconfigurations
- [x] Implement Priority 3 (Extended) vulnerability checks
  - 12 checks for Nginx/Apache (cache bypass, auth bypass, CVEs, SSRF, htaccess)
- [x] Implement Priority 4 (Vendor-Specific) vulnerability checks
  - 20+ checks for HAProxy, Squid, Traefik, Envoy, Caddy, Varnish, F5 BIG-IP, Nginx Plus
- [x] Complete all 55 vulnerability checks (100% coverage)
- [x] Update VERSION to 1.3.0
- [x] Update README with full vulnerability documentation

## 📋 Current Status

### Configuration Structure
```
config/
├── client/              # ProxyHawk CLI configurations
│   └── default.yaml     # Comprehensive default (auto-copied to ~/.config/)
├── server/              # ProxyHawk Server configurations
│   ├── server.default.yaml
│   ├── server.example.yaml
│   ├── production.yaml
│   └── development.yaml
├── examples/            # Feature-specific examples
│   ├── auth-example.yaml
│   ├── connection-pool-example.yaml
│   ├── discovery-example.yaml
│   ├── metrics-example.yaml
│   ├── retry-example.yaml
│   ├── multi-host.example.yaml
│   └── proxy-chaining.yaml
├── default.yaml         # Symlink to client/default.yaml
└── README.md            # Complete configuration guide
```

### User Experience
- ✅ First run automatically creates `~/.config/proxyhawk/config.yaml`
- ✅ Config precedence: CLI flags > Environment variables > User config > Defaults
- ✅ Clear logging when config is created or loaded
- ✅ XDG Base Directory compliant
- ✅ All 55 vulnerability checks available and documented

## 🚀 Future Enhancements

### High Priority (Next Release - v1.5.2 Config Improvements)
- [ ] Make config file optional (like Nuclei) - see CONFIG_IMPROVEMENTS_NEEDED.md
- [ ] Implement config merging (defaults → user config → CLI flags)
- [ ] Create internal/config/defaults.go with built-in defaults
- [ ] Create internal/config/merge.go for config merging
- [ ] Move validation warning to AFTER mode overrides
- [ ] Make test summary always visible via logger (not just DebugInfo)
- [ ] Add unit tests for config initialization logic
- [ ] Add integration tests for vulnerability checks
- [ ] Performance benchmarks for vuln scanning
- [ ] Docker test environments for vulnerable services

### Medium Priority
- [ ] Config migration tool for users with old config files
- [ ] Interactive config wizard (`proxyhawk --configure`)
- [ ] Config diff tool to show what changed between versions
- [ ] Web UI for config management
- [ ] Automated exploit chain discovery (path traversal → RCE)

### Low Priority
- [ ] Config templates for specific use cases (pentesting, monitoring, CI/CD)
- [ ] Config backup/restore functionality
- [ ] Config export to other formats (JSON, TOML)
- [ ] Machine learning-based proxy fingerprinting
- [ ] Integration with Metasploit modules
- [ ] Custom Nuclei template import

## 🐛 Known Issues

None currently identified.

## 📝 Notes

- Configuration system now follows best practices
- User configs are isolated from repository configs
- Server and client configs are clearly separated
- All 55 standard vulnerability checks implemented
- v1.5.0: 7 advanced SSRF checks (92 test cases) implemented
- v1.5.1: 4 Priority 2 SSRF checks (22 test cases) implemented
- Total: 11 advanced SSRF checks with 114 test cases
- Config system improvements needed (see CONFIG_IMPROVEMENTS_NEEDED.md)
- Ready for v1.5.1 release (pending config improvements)

## 🔗 Related Files

- [README.md](README.md) - Main documentation
- [CLAUDE.md](CLAUDE.md) - Architecture and development guide
- [config/README.md](config/README.md) - Configuration guide
- [config/client/default.yaml](config/client/default.yaml) - Default client config
- [VERSION](VERSION) - Current version (1.3.0)
- [internal/config/init.go](internal/config/init.go) - Config initialization logic
- [internal/proxy/vulns_*.go](internal/proxy/) - Vulnerability check implementations

## 📊 Project Statistics

- **Version**: 1.5.1
- **Standard Vulnerability Checks**: 55 (complete)
- **Advanced SSRF Checks**: 11 (114 test cases)
  - v1.5.0: 7 checks (92 test cases)
  - v1.5.1: 4 checks (22 test cases)
- **Total HTTP Requests in vulns mode**: ~264
- **Configuration Files**: 13 (organized by purpose)
- **Lines of Vulnerability Code**: ~5,000+
- **Supported Platforms**: Linux, macOS, Windows
- **Supported Protocols**: HTTP, HTTPS, HTTP/2, HTTP/3, SOCKS4, SOCKS5
