# ProxyCheck Tests

This directory contains the test suite for the ProxyCheck application. The tests are organized by functionality and use Go's built-in testing framework.

## Test Structure

- `proxy/benchmark_test.go`: Benchmark tests for proxy validation using real-world proxy lists

## Running Tests

From the project root directory:

```bash
# Run all tests
go test ./tests/...

# Run benchmark tests
go test ./tests/proxy -bench=.

# Run benchmark tests with verbose output
go test ./tests/proxy -bench=. -v

# Run benchmark tests with custom timeout (default is 10m)
go test ./tests/proxy -bench=. -timeout 20m
```

## Benchmark Tests

The benchmark suite includes:

### ProxyScrape API Benchmark (`BenchmarkProxyScrapeChecking`)

Tests proxy validation using the ProxyScrape API. The benchmark:

1. Fetches proxies from ProxyScrape's free proxy list API
2. Validates each proxy against a public endpoint (Google)
3. Reports detailed statistics:
   - Number of working proxies and success rate
   - Average response time for working proxies
   - Sample of errors for failed proxies

Configuration:
- Timeout: 10 seconds per proxy
- Max concurrent checks: 50
- Validation URL: https://www.google.com
- Proxy protocols: HTTP/HTTPS

Example output:
```
Fetched 763 proxies from ProxyScrape API
Working proxies: 60/763 (7.86%)
Average response time: 4.97s
```

## Test Environment

The test suite uses:
- Real proxy lists from ProxyScrape API
- Concurrent proxy checking
- Public endpoints for validation
- Automatic cleanup of resources

## Writing New Tests

When adding new tests:

1. Create a new test file in the appropriate package
2. Follow Go's testing conventions
3. Use benchmarks for performance-critical components
4. Include proper error handling and logging
5. Update this README with new test details

## Test Coverage

The test suite covers:
- Proxy validation
  - Connection testing
  - Response validation
  - Timeout handling
- Concurrency handling
  - Rate limiting
  - Resource cleanup
- Error scenarios
  - Network errors
  - Invalid proxy formats
  - Timeout conditions

## Continuous Integration

Tests are automatically run on:
- Every push to the repository
- Pull request creation
- Pull request updates

The CI workflow includes:
1. Code linting
2. Unit tests
3. Benchmark tests (skipped for quick checks)
4. Coverage reporting 