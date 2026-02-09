# ProxyHawk Documentation

Complete documentation for ProxyHawk - advanced proxy checker, validator, and security testing tool.

## 📚 Documentation Index

### 🚀 Getting Started
- **[README](../README.md)** - Project overview, features, and quick start
- **[CLAUDE.md](../CLAUDE.md)** - Development guidance for Claude Code
- **[Configuration Guide](guides/CONFIGURATION.md)** - Complete setup instructions
- **[CLI Examples](guides/CLI_EXAMPLES.md)** - Command-line usage examples

### 📋 Reference Documentation
- **[Implementation Status](IMPLEMENTATION_STATUS.md)** - Current feature status and completeness
- **[Project Structure](PROJECT_STRUCTURE.md)** - Codebase organization
- **[Proxy Checking Flow](PROXY_CHECKING_FLOW.md)** - How proxy validation works
- **[Proxy Security Analysis](PROXY_SECURITY_ANALYSIS.md)** - Security features and testing
- **[Integration Guide](INTEGRATION_GUIDE.md)** - Integration instructions

### 🏗️ Deployment
- **[Deployment Overview](../deployments/README.md)** - All deployment options
- **[Docker Guide](../deployments/docker/README.md)** - Containerized deployment
- **[Regional Testing](REGIONAL_TESTING.md)** - Geographic testing setup

### 🔧 Development
- **[API Reference](api/)** - WebSocket and REST API documentation
- **[Client Libraries](../clients/)** - Python and other client libraries
- **[Examples](../examples/)** - Code examples and demos

### 📖 Guides and Tutorials
- **[Configuration](guides/CONFIGURATION.md)** - Detailed configuration options
- **[CLI Usage](guides/CLI_EXAMPLES.md)** - Command-line interface guide
- **[Monitoring](guides/monitoring.md)** - Metrics and observability (future)
- **[Security](guides/security.md)** - Security best practices (future)

## 🎯 Quick Navigation

### For Users
- New to ProxyHawk? Start with the [README](../README.md)
- Need to configure? See [Configuration Guide](guides/CONFIGURATION.md)
- Want to deploy? Check [Deployment Options](../deployments/README.md)

### For Developers  
- Building integrations? Use [Client Libraries](../clients/)
- Need API docs? See [API Reference](api/)
- Want examples? Browse [Examples](../examples/)

### For Operators
- Deploying in production? See [Docker Guide](../deployments/docker/README.md)
- Need monitoring? Check [Monitoring Guide](guides/monitoring.md) (future)
- Security concerns? Read [Security Guide](guides/security.md) (future)

### 🗂️ Archive
- **[Historical Documentation](archive/)** - Proposals and implementation notes (completed features)

## 📋 Feature Documentation

### Core Features
- **Three-Tier Check Modes**: Basic (fast), Intense (security), Vulns (comprehensive)
- **Multi-Protocol Support**: HTTP, HTTPS, SOCKS4, SOCKS5, HTTP/2, HTTP/3
- **Proxy Discovery**: Shodan, Censys, free lists, web scraping with honeypot detection
- **Terminal UI**: Real-time progress tracking with component-based architecture

### Security Features
- **SSRF Testing**: 60+ internal targets including cloud metadata services
- **Header Injection**: 62+ injection vectors with CRLF/null byte testing
- **Protocol Smuggling**: CL.TE desync detection
- **DNS Rebinding**: Protection testing
- **Cache Poisoning**: Detection and testing
- **Anonymity Detection**: Elite/Anonymous/Transparent classification with 10+ header checks

### Advanced Features
- **Configuration Hot-Reloading**: Live config updates without restart
- **Rate Limiting**: Three modes (global, per-host, per-proxy)
- **Progress Indicators**: 6 types for automation (bar, spinner, dots, percent, basic, none)
- **Metrics & Monitoring**: Prometheus integration with custom endpoint
- **Output Formats**: Text, JSON, working proxies, anonymous proxies
- **Interactsh Integration**: Out-of-band detection for vulnerability confirmation

## 🛠️ Configuration Quick Reference

```bash
# Basic setup
./scripts/setup-config.sh
./build/proxyhawk-server

# Docker deployment  
cd deployments/docker
docker-compose up -d

# Custom configuration
./build/proxyhawk-server -config ~/.config/proxyhawk/server.yaml
```

## 🐛 Troubleshooting

Common issues and solutions:

1. **Configuration Issues**: See [Configuration Guide](guides/CONFIGURATION.md)
2. **Docker Problems**: Check [Docker Guide](../deployments/docker/README.md)
3. **API Errors**: Review [API Reference](api/)
4. **Performance Issues**: See [Monitoring Guide](guides/monitoring.md) (future)

## 🤝 Contributing

- **Issues**: [GitHub Issues](https://github.com/ResistanceIsUseless/ProxyHawk/issues)
- **Discussions**: [GitHub Discussions](https://github.com/ResistanceIsUseless/ProxyHawk/discussions)
- **Code**: See [CONTRIBUTING.md](../CONTRIBUTING.md) (future)

## 📄 Reference

- **[API Documentation](api/)** - Complete API reference
- **[Configuration Schema](../config/)** - All configuration options
- **[Examples](../examples/)** - Working code examples
- **[Client Libraries](../clients/)** - Integration libraries

---

**Need help?** Check the relevant guide above or open an issue on GitHub.

## 📊 Project Status

- ✅ **Feature Complete**: All documented features implemented (99% complete)
- ✅ **45 Command-Line Flags**: All verified and working
- ✅ **Security Testing**: Comprehensive vulnerability detection suite
- ✅ **Terminal UI**: Full component-based system with real-time updates
- ✅ **Documentation**: Comprehensive guides and examples
- ⚠️ **Interactsh**: 100% complete (standalone flag added 2026-02-09)

**Last Updated**: 2026-02-09
**Version**: 1.0.0