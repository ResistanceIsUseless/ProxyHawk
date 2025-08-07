# ProxyHawk CLI Examples

This document provides comprehensive examples of using ProxyHawk's command-line interface for various use cases.

## Table of Contents

- [Basic Usage](#basic-usage)  
- [Output Options](#output-options)
- [Performance Tuning](#performance-tuning)
- [Security Testing](#security-testing)
- [Automation & Scripting](#automation--scripting)
- [Monitoring & Metrics](#monitoring--metrics)
- [Advanced Configuration](#advanced-configuration)
- [Docker Deployment](#docker-deployment)
- [Troubleshooting](#troubleshooting)

## Basic Usage

### Simple proxy check
```bash
# Check proxies from a file
proxyhawk -l proxies.txt

# Check with verbose output
proxyhawk -l proxies.txt -v

# Check with debug information
proxyhawk -l proxies.txt -d
```

### Quick start
```bash
# Create a proxy list
echo "http://proxy1.example.com:8080" > my-proxies.txt
echo "socks5://proxy2.example.com:1080" >> my-proxies.txt

# Run basic check
proxyhawk -l my-proxies.txt
```

## Output Options

### Save results in different formats
```bash
# Save text report
proxyhawk -l proxies.txt -o results.txt

# Save JSON data
proxyhawk -l proxies.txt -j results.json

# Save both formats
proxyhawk -l proxies.txt -o results.txt -j results.json

# Save only working proxies
proxyhawk -l proxies.txt -wp working-proxies.txt

# Save only anonymous proxies  
proxyhawk -l proxies.txt -wpa anonymous-proxies.txt

# Save everything
proxyhawk -l proxies.txt -o full-report.txt -j data.json -wp working.txt -wpa anonymous.txt
```

### Non-interactive mode for scripts
```bash
# Disable TUI for automation
proxyhawk -l proxies.txt --no-ui -o results.txt

# Different progress indicators
proxyhawk -l proxies.txt --no-ui --progress bar
proxyhawk -l proxies.txt --no-ui --progress spinner  
proxyhawk -l proxies.txt --no-ui --progress dots
proxyhawk -l proxies.txt --no-ui --progress percent
proxyhawk -l proxies.txt --no-ui --progress none  # Silent
```

## Performance Tuning

### Concurrency and timeout settings
```bash
# High performance - many workers
proxyhawk -l proxies.txt -c 100 -t 5

# Conservative - fewer workers, longer timeout
proxyhawk -l proxies.txt -c 10 -t 30

# Maximum speed (use with caution)
proxyhawk -l proxies.txt -c 200 -t 3
```

### Rate limiting to avoid bans
```bash
# Basic rate limiting (1 second delay)
proxyhawk -l proxies.txt --rate-limit

# Custom delay
proxyhawk -l proxies.txt --rate-limit --rate-delay 2s

# Rate limit per proxy (instead of per host)
proxyhawk -l proxies.txt --rate-limit --rate-per-proxy --rate-delay 500ms

# Rate limit per host (good for testing many proxies from same provider)
proxyhawk -l proxies.txt --rate-limit --rate-per-host --rate-delay 1s
```

## Security Testing

### Enable comprehensive security checks
```bash
# Use security-focused configuration
proxyhawk -l proxies.txt --config config/security.yaml -d

# Enable reverse DNS lookups
proxyhawk -l proxies.txt -r -d

# Verbose security testing
proxyhawk -l proxies.txt -d -v --config config/advanced-security.yaml
```

### Custom security configuration
```yaml
# security-config.yaml
advanced_checks:
  test_ssrf: true
  test_host_header_injection: true  
  test_protocol_smuggling: true
  test_dns_rebinding: true
  test_ipv6: true
  test_cache_poisoning: true

cloud_providers:
  - name: "Custom Cloud"
    metadata_ips: ["169.254.169.254"]
    internal_ranges: ["10.0.0.0/8"]
```

```bash
# Use custom security config
proxyhawk -l proxies.txt --config security-config.yaml -d
```

## Automation & Scripting

### Automated proxy validation pipeline
```bash
#!/bin/bash
# proxy-validation.sh

PROXY_LIST="proxies.txt"
DATE=$(date +%Y%m%d_%H%M%S)
RESULTS_DIR="results_${DATE}"

mkdir -p "${RESULTS_DIR}"

echo "Starting proxy validation at $(date)"

# Run comprehensive check
proxyhawk -l "${PROXY_LIST}" \
    --no-ui \
    --progress bar \
    -c 50 \
    -t 10 \
    -o "${RESULTS_DIR}/full-report.txt" \
    -j "${RESULTS_DIR}/data.json" \
    -wp "${RESULTS_DIR}/working-proxies.txt" \
    -wpa "${RESULTS_DIR}/anonymous-proxies.txt" \
    --rate-limit \
    --rate-delay 1s

echo "Validation completed at $(date)"
echo "Results saved to ${RESULTS_DIR}/"

# Process results
WORKING_COUNT=$(wc -l < "${RESULTS_DIR}/working-proxies.txt" 2>/dev/null || echo "0")
ANONYMOUS_COUNT=$(wc -l < "${RESULTS_DIR}/anonymous-proxies.txt" 2>/dev/null || echo "0") 
TOTAL_COUNT=$(wc -l < "${PROXY_LIST}")

echo "Summary: ${WORKING_COUNT}/${TOTAL_COUNT} working proxies, ${ANONYMOUS_COUNT} anonymous"
```

### Continuous monitoring
```bash
#!/bin/bash
# monitor-proxies.sh

while true; do
    echo "$(date): Starting proxy check..."
    
    proxyhawk -l proxies.txt \
        --no-ui \
        --progress percent \
        -o "results/$(date +%H%M).txt" \
        --rate-limit \
        --hot-reload \
        --config monitor-config.yaml
    
    echo "$(date): Check completed, sleeping 30 minutes..."
    sleep 1800
done
```

## Monitoring & Metrics

### Enable Prometheus metrics
```bash
# Basic metrics on default port
proxyhawk -l proxies.txt --metrics

# Custom metrics configuration
proxyhawk -l proxies.txt \
    --metrics \
    --metrics-addr :9090 \
    --metrics-path /metrics \
    --no-ui

# Check metrics endpoint
curl http://localhost:9090/metrics
```

### Docker monitoring stack
```bash
# Start full monitoring with docker-compose
docker-compose up -d

# View logs
docker-compose logs -f proxyhawk

# Check Grafana dashboard
open http://localhost:3000  # admin/admin
```

## Advanced Configuration

### Hot-reloading configuration
```bash
# Enable hot-reload
proxyhawk -l proxies.txt --hot-reload --config dynamic-config.yaml

# In another terminal, edit the config
echo "concurrency: 20" >> dynamic-config.yaml
# ProxyHawk will automatically reload the config
```

### Custom configuration examples
```yaml
# high-performance.yaml
concurrency: 100
timeout: 5
user_agent: "ProxyHawk-HighPerf/1.0"

connection_pool:
  max_idle_conns: 200  
  max_conns_per_host: 50

retry:
  enabled: true
  max_retries: 2
  initial_delay: 1s
```

```yaml
# stealth-mode.yaml  
concurrency: 5
timeout: 30
user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

rate_limit:
  enabled: true
  delay: 3s
  per_host: true

retry:
  enabled: false
```

```bash
# Use custom configs
proxyhawk -l proxies.txt --config high-performance.yaml
proxyhawk -l proxies.txt --config stealth-mode.yaml --rate-limit
```

## Docker Deployment

### Basic Docker usage
```bash
# Build image
docker build -t proxyhawk .

# Run container
docker run --rm \
    -v $(pwd)/proxies.txt:/app/proxies.txt:ro \
    -v $(pwd)/output:/app/output \
    proxyhawk -l proxies.txt -o output/results.txt --no-ui

# With custom config
docker run --rm \
    -v $(pwd)/proxies.txt:/app/proxies.txt:ro \
    -v $(pwd)/config:/app/config:ro \
    -v $(pwd)/output:/app/output \
    proxyhawk --config config/custom.yaml -l proxies.txt --no-ui
```

### Production deployment
```bash
# Using deployment script
./scripts/deploy.sh setup
./scripts/deploy.sh run-basic

# With monitoring
./scripts/deploy.sh compose-up

# Security testing  
./scripts/deploy.sh run-security
```

### Kubernetes deployment
```yaml
# proxyhawk-job.yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: proxyhawk-check
spec:
  template:
    spec:
      containers:
      - name: proxyhawk
        image: proxyhawk:latest
        command: ["proxyhawk"]
        args:
          - "-l"
          - "/config/proxies.txt"
          - "--no-ui"
          - "--progress"
          - "percent"
          - "-o"
          - "/output/results.txt"
          - "-j"
          - "/output/results.json"
        volumeMounts:
        - name: proxy-list
          mountPath: /config
        - name: output
          mountPath: /output
      volumes:
      - name: proxy-list
        configMap:
          name: proxy-list-config
      - name: output
        persistentVolumeClaim:
          claimName: proxyhawk-output
      restartPolicy: Never
```

```bash
kubectl apply -f proxyhawk-job.yaml
kubectl logs job/proxyhawk-check
```

## Troubleshooting

### Debug common issues
```bash
# Debug connection issues
proxyhawk -l proxies.txt -d -v -t 30

# Test single proxy
echo "http://test-proxy.com:8080" | proxyhawk -l /dev/stdin -d

# Check configuration
proxyhawk --quickstart
proxyhawk --help | grep config

# Verbose rate limiting
proxyhawk -l proxies.txt --rate-limit --rate-delay 2s -d -v
```

### Performance debugging
```bash
# Check with minimal concurrency
proxyhawk -l proxies.txt -c 1 -d

# Test with longer timeout
proxyhawk -l proxies.txt -t 60 -v

# Profile with metrics
proxyhawk -l proxies.txt --metrics &
PID=$!
sleep 10
curl -s http://localhost:9090/metrics | grep -E "(proxyhawk_|go_)"
kill $PID
```

### Environment debugging
```bash
# Check environment
export PROXYHAWK_NO_COLOR=1
export PROXYHAWK_CONFIG=./custom-config.yaml

# Debug with environment
env | grep -i proxy
proxyhawk -l proxies.txt -d
```

### Common solutions
```bash
# Too many failures - increase timeout
proxyhawk -l proxies.txt -t 30

# Getting banned - add rate limiting  
proxyhawk -l proxies.txt --rate-limit --rate-delay 5s

# High memory usage - reduce concurrency
proxyhawk -l proxies.txt -c 10

# Config issues - validate first
proxyhawk --quickstart > test-config.txt
proxyhawk -l test-proxies.txt --config test-config.yaml -d
```

## Best Practices

### 1. Start Conservative
```bash
# Begin with low concurrency and high timeout
proxyhawk -l proxies.txt -c 5 -t 30 --rate-limit
```

### 2. Use Appropriate Output
```bash
# For analysis - use JSON
proxyhawk -l proxies.txt -j analysis.json

# For reuse - extract working proxies  
proxyhawk -l proxies.txt -wp good-proxies.txt
```

### 3. Monitor Performance
```bash
# Enable metrics for production
proxyhawk -l proxies.txt --metrics --hot-reload
```

### 4. Security First
```bash
# Always test security if using for security research  
proxyhawk -l proxies.txt --config security.yaml -d
```

### 5. Automation Ready
```bash
# Use --no-ui for scripts
proxyhawk -l proxies.txt --no-ui --progress basic -o results.txt
```

For more information, see the [README.md](../README.md) or run `proxyhawk --help`.