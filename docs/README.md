# ProxyHawk Documentation

Complete documentation for ProxyHawk dual-mode proxy server and geographic testing service.

## 📚 Documentation Index

### 🚀 Getting Started
- **[README](../README.md)** - Project overview and quick start
- **[Configuration Guide](guides/CONFIGURATION.md)** - Complete setup instructions
- **[CLI Examples](guides/CLI_EXAMPLES.md)** - Command-line usage examples

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

## 📋 Feature Documentation

### Core Features
- **Dual-Mode Operation**: Proxy server + Geographic testing service
- **Multi-Protocol Support**: SOCKS5, HTTP, WebSocket API
- **Geographic Testing**: Round-robin DNS detection across regions
- **Proxy Chaining**: Multi-hop proxy support with Tor integration

### Advanced Features
- **Health Checking**: Automatic proxy pool management
- **DNS Caching**: Performance optimization with TTL-based caching
- **Metrics & Monitoring**: Prometheus integration
- **Client Libraries**: Python WebSocket client with async/sync support

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

**Last Updated**: 2024-08-07  
**Version**: 1.0.0