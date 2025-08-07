# ProxyHawk Deployments

This directory contains deployment configurations and orchestration files for ProxyHawk across different environments.

## 📁 Directory Structure

```
deployments/
├── docker/              # Docker containerization
│   ├── Dockerfile       # Multi-stage build for ProxyHawk
│   ├── docker-compose.yml    # Production deployment
│   ├── docker-compose.test.yml  # Testing environment
│   ├── README.md        # Docker-specific documentation
│   ├── torrc           # Tor configuration
│   ├── prometheus.yml  # Prometheus monitoring config
│   └── grafana-datasources.yml  # Grafana dashboard config
├── kubernetes/          # Kubernetes manifests (future)
└── systemd/            # Systemd service files (future)
```

## 🚀 Quick Deployment

### Docker (Recommended)

```bash
# Production deployment
cd deployments/docker
docker-compose up -d

# Development/testing
docker-compose -f docker-compose.test.yml up -d
```

### Local Development

```bash
# Build binaries
make build

# Run with default config
./build/proxyhawk-server

# Run with custom config
./build/proxyhawk-server -config ~/.config/proxyhawk/server.yaml
```

## 🐳 Docker Deployment

The Docker deployment includes:

- **ProxyHawk Server** - Dual-mode proxy and geographic testing
- **Tor Proxy** - Anonymous proxy routing
- **Prometheus** - Metrics collection
- **Grafana** - Monitoring dashboards

### Configuration

1. **Environment Variables**:
   ```bash
   # Copy environment template
   cp deployments/docker/.env.example .env
   
   # Edit with your settings
   nano .env
   ```

2. **Custom Configuration**:
   ```bash
   # Override default config
   cp config/production.yaml deployments/docker/proxyhawk-config.yaml
   # Edit as needed
   ```

3. **Deploy**:
   ```bash
   cd deployments/docker
   docker-compose up -d
   ```

## 📊 Monitoring

### Prometheus Metrics

- **Endpoint**: http://localhost:9090
- **Config**: `deployments/docker/prometheus.yml`
- **Metrics**: Proxy health, request counts, geographic test statistics

### Grafana Dashboard

- **Endpoint**: http://localhost:3000
- **Default Credentials**: admin/admin
- **Datasources**: Pre-configured for Prometheus

## 🔧 Service Management

```bash
# View logs
docker-compose logs -f proxyhawk-server

# Restart service
docker-compose restart proxyhawk-server

# Update configuration
docker-compose down
# Edit configs
docker-compose up -d

# Scale services (if applicable)
docker-compose scale proxyhawk-server=3
```

## 🔒 Security Considerations

### Production Deployment

1. **TLS Termination**: Use nginx/Traefik for HTTPS
2. **Network Security**: Configure firewall rules
3. **Secret Management**: Use Docker secrets or external vaults
4. **Resource Limits**: Set appropriate CPU/memory limits

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PROXYHAWK_MODE` | Operation mode | `dual` |
| `PROXYHAWK_LOG_LEVEL` | Logging level | `info` |
| `PROXYHAWK_METRICS_ENABLED` | Enable metrics | `true` |
| `TOR_ENABLED` | Enable Tor integration | `false` |

## 🚀 Future Deployment Options

### Kubernetes (Planned)
- Helm charts for easy deployment
- Horizontal pod autoscaling
- Service mesh integration
- Persistent storage for configurations

### Systemd Services (Planned)
- Native Linux service integration
- Automatic startup and recovery
- Log rotation and management
- Resource control with cgroups

## 📚 Additional Resources

- [Docker Documentation](docker/README.md)
- [Configuration Guide](../docs/guides/CONFIGURATION.md)
- [API Documentation](../docs/api/)
- [Monitoring Setup](../docs/guides/monitoring.md)

---

Choose the deployment method that best fits your infrastructure and requirements. Docker is recommended for most use cases due to its simplicity and included monitoring stack.