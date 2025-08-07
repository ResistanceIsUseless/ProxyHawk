# ProxyHawk Python Client

A Python client library for interacting with ProxyHawk's WebSocket API for geographic DNS testing.

## Features

- **Async/Sync Support**: Both asynchronous and synchronous interfaces
- **Geographic Testing**: Test domains for geographic DNS differences
- **Batch Processing**: Efficiently test multiple domains at once
- **Real-time Updates**: Subscribe to domain changes and updates
- **Auto-reconnection**: Automatic connection recovery
- **Type Safety**: Full type hints and dataclass support

## Installation

```bash
# Install from source
pip install -e .

# Install dependencies only
pip install -r requirements.txt
```

## Quick Start

### Asynchronous Usage (Recommended)

```python
import asyncio
from proxyhawk_client import ProxyHawkClient, ProxyHawkConfig, TestMode

async def main():
    # Configure client
    config = ProxyHawkConfig(
        url="ws://localhost:8888/ws",
        regions=["us-west", "us-east", "eu-west"],
        test_mode=TestMode.BASIC,
        timeout=30.0
    )
    
    client = ProxyHawkClient(config)
    
    try:
        # Connect to ProxyHawk
        await client.connect()
        
        # Test single domain
        result = await client.test_domain("example.com")
        print(f"Geographic differences: {result.has_geographic_differences}")
        print(f"Regions tested: {len(result.region_results)}")
        
        # Test multiple domains
        domains = ["google.com", "github.com", "stackoverflow.com"]
        results = await client.batch_test(domains)
        
        for result in results:
            print(f"{result.domain}: {result.confidence:.2f} confidence")
        
    finally:
        await client.disconnect()

# Run the example
asyncio.run(main())
```

### Synchronous Usage

```python
from proxyhawk_client import SyncProxyHawkClient, ProxyHawkConfig

# Configure and connect
config = ProxyHawkConfig(url="ws://localhost:8888/ws")
client = SyncProxyHawkClient(config)

try:
    client.connect()
    
    # Test domain
    result = client.test_domain("example.com")
    print(f"Domain: {result.domain}")
    print(f"Has geographic differences: {result.has_geographic_differences}")
    
finally:
    client.disconnect()
```

## Advanced Usage

### Domain Subscriptions

```python
async def on_domain_update(domain, data):
    print(f"Update for {domain}: {data}")

# Subscribe to domain updates
await client.subscribe("example.com", on_domain_update)

# Global update handler
client.on_domain_update(lambda domain, data: print(f"Global: {domain}"))
```

### Custom Configuration

```python
from proxyhawk_client import ProxyHawkConfig, TestMode

config = ProxyHawkConfig(
    url="ws://your-proxyhawk-server:8888/ws",
    regions=["us-west", "us-east", "eu-west", "asia", "au"],
    test_mode=TestMode.COMPREHENSIVE,  # More detailed testing
    timeout=60.0,
    reconnect_delay=5.0,
    max_retries=3
)
```

### Event Handlers

```python
client = ProxyHawkClient(config)

# Connection events
client.on_connected = lambda: print("Connected to ProxyHawk!")
client.on_disconnected = lambda: print("Disconnected from ProxyHawk")
client.on_error = lambda e: print(f"Error: {e}")
```

## API Reference

### ProxyHawkClient

Main asynchronous client for ProxyHawk WebSocket API.

#### Methods

- `connect() -> bool`: Connect to ProxyHawk server
- `disconnect()`: Disconnect from server
- `test_domain(domain, regions=None) -> GeoTestResult`: Test single domain
- `batch_test(domains, regions=None) -> List[GeoTestResult]`: Test multiple domains
- `subscribe(domain, callback=None)`: Subscribe to domain updates
- `unsubscribe(domain)`: Unsubscribe from domain updates
- `get_regions() -> List[str]`: Get available testing regions

### SyncProxyHawkClient

Synchronous wrapper for non-async applications.

#### Methods

Same as `ProxyHawkClient` but all methods are synchronous.

### Data Classes

#### GeoTestResult

Complete geographic test result for a domain.

```python
@dataclass
class GeoTestResult:
    domain: str
    tested_at: str
    has_geographic_differences: bool
    is_round_robin: bool
    confidence: float
    region_results: Dict[str, RegionResult]
    summary: TestSummary
```

#### RegionResult

Test results for a specific geographic region.

```python
@dataclass
class RegionResult:
    region: str
    proxy_used: str
    dns_results: List[DNSResult]
    http_results: List[HTTPResult]
    response_time_ms: int
    success: bool
    error: str = ""
```

## Error Handling

```python
from websockets.exceptions import ConnectionClosed

try:
    result = await client.test_domain("example.com")
except ConnectionClosed:
    print("Connection lost, attempting reconnection...")
    await client.connect()
except TimeoutError:
    print("Test timed out")
except RuntimeError as e:
    print(f"Client error: {e}")
```

## Testing

Run the example script to test connectivity:

```bash
# Make sure ProxyHawk server is running on localhost:8888
python proxyhawk_client.py
```

## Integration Examples

### With SubScope

```python
# Use ProxyHawk results in SubScope workflow
result = await client.test_domain("target.com")
if result.has_geographic_differences:
    print(f"Domain {result.domain} has geographic differences!")
    print(f"Unique IPs found: {result.summary.unique_ips}")
```

### With Security Testing

```python
# Check for CDN/geo-blocking
domains = ["target.com", "admin.target.com", "api.target.com"]
results = await client.batch_test(domains)

for result in results:
    if result.has_geographic_differences:
        print(f"⚠️  {result.domain} may have geo-restrictions")
    if result.is_round_robin:
        print(f"🔄 {result.domain} uses round-robin DNS")
```

## Requirements

- Python 3.7+
- websockets >= 10.0
- ProxyHawk server running with WebSocket API enabled

## License

This project is licensed under the MIT License - see the LICENSE file for details.