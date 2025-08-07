# Docker Deployment for ProxyHawk

Complete Docker containerization with monitoring stack for ProxyHawk geographic proxy testing service.

## 🎯 What's Included

- **ProxyHawk Server** - Dual-mode proxy and geographic testing service
- **Tor Proxy** - Anonymous routing and circuit management  
- **Prometheus** - Metrics collection and alerting
- **Grafana** - Monitoring dashboards and visualization

## 🚀 Quick Start

```bash
# Navigate to docker directory
cd deployments/docker

# Start the full stack
docker-compose up -d

# Check status
docker-compose ps

# Access services
# - ProxyHawk API: http://localhost:8888/api/health
# - Prometheus: http://localhost:9090
# - Grafana: http://localhost:3000 (admin/admin)
```

## 📊 Services

- **SOCKS5 Proxy**: `localhost:1080`
- **HTTP Proxy**: `localhost:8080`  
- **WebSocket API**: `localhost:8888`
- **Prometheus**: `localhost:9090`
- **Grafana**: `localhost:3000`

## 🛠️ Management

```bash
# View logs
docker-compose logs -f

# Restart service
docker-compose restart proxyhawk-server

# Update images
docker-compose pull && docker-compose up -d
```

For detailed configuration and troubleshooting, see the main deployment documentation.