# Regional Proxy Testing Guide

## Overview
This guide explains how to test ProxyHawk with free proxies from different geographic regions using Geonode's free proxy list.

## Quick Start

1. **Run the test script:**
   ```bash
   ./test_regional.sh
   ```

2. **Or manually test with regional config:**
   ```bash
   go run cmd/proxyhawk/main.go -l test_regional_proxies.txt -config regional_test_config.yaml -c 5 -d
   ```

## Getting Fresh Proxies from Geonode

1. Visit https://geonode.com/free-proxy-list
2. Filter by country/region:
   - **North America**: United States, Canada
   - **Europe**: UK, Germany, France, Netherlands
   - **Asia Pacific**: Japan, Singapore, India, Australia
   - **South America**: Brazil, Argentina
   - **Africa**: South Africa
   
3. Select proxies with:
   - High uptime (>90%)
   - Low latency
   - HTTP/HTTPS protocol support

4. Add to `test_regional_proxies.txt` in format:
   ```
   http://IP:PORT # Country-Region
   ```

## Regional Configuration

The `regional_test_config.yaml` includes:
- Lower concurrency (5) for free proxy stability
- Rate limiting (2s delay) to avoid bans
- Regional test URLs
- Increased timeout (15s) for slower proxies

## Region Mapping

| Region | Countries | Test URL |
|--------|-----------|----------|
| US | United States, Canada | httpbin.org/ip |
| EU | UK, Germany, France, Netherlands | ip-api.com/json |
| ASIA | Japan, Singapore, India, Australia | ipinfo.io/json |
| SA | Brazil, Argentina | httpbin.org/ip |
| AF | South Africa | httpbin.org/ip |

## Test Outputs

- `working_regional.txt` - All working proxies with speed
- `anonymous_regional.txt` - Only anonymous proxies
- Console output shows regional distribution

## Best Practices

1. **Free Proxy Limitations:**
   - Often slow and unreliable
   - May have rate limits
   - Can go offline quickly
   - Limited anonymity

2. **Testing Tips:**
   - Use low concurrency (2-5)
   - Enable rate limiting
   - Test small batches first
   - Update proxy list frequently

3. **Production Use:**
   - Consider premium proxy services
   - Implement proxy rotation
   - Monitor proxy health
   - Use regional failover

## Troubleshooting

**All proxies failing?**
- Check internet connection
- Verify proxies are still active on Geonode
- Increase timeout in config
- Lower concurrency

**Rate limiting issues?**
- Increase rate-delay in config
- Use fewer concurrent workers
- Test against different URLs

**Regional detection not working?**
- Some proxies may use VPN/tunneling
- Free proxies often have incorrect geo data
- Try premium services for accurate geo-location