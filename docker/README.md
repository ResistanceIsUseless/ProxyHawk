# ProxyHawk Docker Guide

This directory contains Docker configuration files for running ProxyHawk in containerized environments.

## Quick Start

### Build and Run Basic Container

```bash
# Build the Docker image
docker build -t proxyhawk .

# Run basic proxy checking
docker run --rm -v $(pwd)/test-proxies.txt:/app/proxies.txt:ro \
    -v $(pwd)/output:/app/output \
    proxyhawk -l proxies.txt -o output/results.txt --no-ui -v
```

### Using Docker Compose

```bash
# Start all services with monitoring
docker-compose up -d

# Start only basic ProxyHawk service
docker-compose up proxyhawk

# Start with metrics enabled
docker-compose up proxyhawk-metrics prometheus grafana

# View logs
docker-compose logs -f proxyhawk

# Stop all services
docker-compose down
```

## Available Services

### 1. Basic ProxyHawk (`proxyhawk`)
- Standard proxy checking functionality
- Outputs results to mounted volume
- No external dependencies

### 2. ProxyHawk with Metrics (`proxyhawk-metrics`)
- Includes Prometheus metrics endpoint
- Exposes port 9090 for metrics collection
- Configured for monitoring with Prometheus

### 3. ProxyHawk with Authentication (`proxyhawk-auth`)
- Configured for authenticated proxy testing
- Uses `config/auth-example.yaml` configuration
- Supports username/password proxy authentication

### 4. ProxyHawk Security Testing (`proxyhawk-security`)
- Advanced security testing enabled
- Includes SSRF, Host header injection, protocol smuggling tests
- Debug mode enabled for detailed output

### 5. Monitoring Stack
- **Prometheus**: Metrics collection and storage
- **Grafana**: Metrics visualization dashboard

## Configuration

### Volume Mounts

```bash
# Required volumes
-v $(pwd)/test-proxies.txt:/app/proxies.txt:ro     # Proxy list (read-only)
-v $(pwd)/output:/app/output                       # Output directory

# Optional volumes
-v $(pwd)/config:/app/config:ro                    # Custom configuration (read-only)
```

### Environment Variables

```bash
# Example with environment variables
docker run --rm \
    -e PROXYHAWK_CONFIG=config/production.yaml \
    -e PROXYHAWK_CONCURRENCY=100 \
    -e PROXYHAWK_TIMEOUT=30 \
    -v $(pwd)/test-proxies.txt:/app/proxies.txt:ro \
    -v $(pwd)/output:/app/output \
    proxyhawk
```

## Multi-Architecture Support

The Docker image supports both AMD64 and ARM64 architectures:

```bash
# Build for specific platform
docker build --platform linux/amd64 -t proxyhawk:amd64 .
docker build --platform linux/arm64 -t proxyhawk:arm64 .

# Build multi-architecture image
docker buildx build --platform linux/amd64,linux/arm64 -t proxyhawk:latest .
```

## Security Considerations

### Running as Non-Root User
The Docker image runs as a non-root user (`proxyhawk:1001`) for security:

```dockerfile
USER proxyhawk
```

### Network Security
- The container only exposes port 9090 for metrics (when enabled)
- Uses bridge networking for isolation
- No privileged mode required

### File Permissions
- Configuration files are mounted read-only
- Output directory has proper permissions for the proxyhawk user

## Advanced Usage Examples

### 1. Batch Processing with Volume Mounts

```bash
# Process multiple proxy lists
docker run --rm \
    -v $(pwd)/proxy-lists:/app/proxy-lists:ro \
    -v $(pwd)/results:/app/results \
    proxyhawk \
    -l proxy-lists/premium.txt \
    -o results/premium-results.txt \
    -j results/premium-results.json \
    --no-ui -v
```

### 2. Automated Scheduling with Cron

```bash
# Add to crontab for daily proxy checking
0 2 * * * docker run --rm -v /path/to/proxies:/app/proxies.txt:ro -v /path/to/output:/app/output proxyhawk -l proxies.txt -o output/daily-$(date +\%Y\%m\%d).txt --no-ui
```

### 3. Integration with CI/CD

```yaml
# GitHub Actions example
- name: Run ProxyHawk Tests
  run: |
    docker run --rm \
      -v ${{ github.workspace }}/proxies.txt:/app/proxies.txt:ro \
      -v ${{ github.workspace }}/output:/app/output \
      proxyhawk \
      -l proxies.txt \
      -j output/results.json \
      --no-ui -v
```

### 4. Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: proxyhawk
spec:
  replicas: 1
  selector:
    matchLabels:
      app: proxyhawk
  template:
    metadata:
      labels:
        app: proxyhawk
    spec:
      containers:
      - name: proxyhawk
        image: proxyhawk:latest
        args: ["--config", "config/production.yaml", "--no-ui", "-v"]
        volumeMounts:
        - name: proxy-list
          mountPath: /app/proxies.txt
          subPath: proxies.txt
        - name: output
          mountPath: /app/output
        - name: config
          mountPath: /app/config
      volumes:
      - name: proxy-list
        configMap:
          name: proxy-list-config
      - name: output
        persistentVolumeClaim:
          claimName: proxyhawk-output
      - name: config
        configMap:
          name: proxyhawk-config
```

## Monitoring and Alerting

### Accessing Grafana Dashboard
1. Start the full monitoring stack: `docker-compose up -d`
2. Open Grafana: http://localhost:3000
3. Login with admin/admin
4. Import ProxyHawk dashboard (create custom dashboard using ProxyHawk metrics)

### Available Metrics
- `proxyhawk_proxy_checks_total`: Total number of proxy checks performed
- `proxyhawk_proxy_checks_successful`: Number of successful proxy checks
- `proxyhawk_proxy_response_time_seconds`: Proxy response time distribution
- `proxyhawk_anonymous_proxies_total`: Number of anonymous proxies found
- `proxyhawk_cloud_proxies_total`: Number of cloud-based proxies detected

### Custom Alerts
Add to Prometheus `alerting_rules.yml`:

```yaml
groups:
  - name: proxyhawk
    rules:
      - alert: ProxyHawkHighFailureRate
        expr: rate(proxyhawk_proxy_checks_total[5m]) - rate(proxyhawk_proxy_checks_successful[5m]) > 0.8
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "ProxyHawk has high proxy failure rate"
```

## Troubleshooting

### Common Issues

1. **Permission Denied on Output Directory**
   ```bash
   # Fix permissions
   sudo chown -R 1001:1001 output/
   ```

2. **Configuration File Not Found**
   ```bash
   # Ensure config volume is mounted correctly
   -v $(pwd)/config:/app/config:ro
   ```

3. **Container Exits Immediately**
   ```bash
   # Check logs
   docker logs proxyhawk
   
   # Run with debug
   docker run --rm proxyhawk --help
   ```

4. **Metrics Not Available**
   ```bash
   # Ensure metrics port is exposed
   -p 9090:9090
   
   # Check if metrics are enabled in config
   ```

### Performance Tuning

```bash
# Increase memory limit for large proxy lists
docker run --memory=2g --cpus=2 proxyhawk

# Adjust concurrency based on container resources
docker run proxyhawk -c 200  # High concurrency for powerful containers
```

## Security Scanning

Scan the Docker image for vulnerabilities:

```bash
# Using Trivy
trivy image proxyhawk:latest

# Using Docker Scout
docker scout cves proxyhawk:latest
```