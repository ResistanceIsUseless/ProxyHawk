# ProxyCheck Tests

This directory contains the test suite for the ProxyCheck application. The tests are organized by functionality and use Go's built-in testing framework.

## Test Structure

- `init_test.go`: Test environment setup and cleanup
- `types.go`: Common types and interfaces used across tests
- `proxy_test.go`: Tests for basic proxy validation functionality
- `security_test.go`: Tests for advanced security checks
- `output_test.go`: Tests for output formatting
- `config_test.go`: Tests for configuration loading
- `working_proxies_test.go`: Tests for working proxies output

## Running Tests

From the project root directory:

```bash
# Run all tests
make test

# Run tests with coverage report
make coverage

# Run a specific test
make test-one test=TestName

# Run tests in verbose mode
make test-verbose

# Run short tests (skip long running tests)
make test-short
```

## Test Environment

The test suite uses:
- Mock HTTP servers for proxy testing
- Temporary files for output testing
- Test configuration file
- Cleanup of test artifacts after completion

## Writing New Tests

When adding new tests:

1. Choose the appropriate test file based on functionality
2. Follow the existing test patterns
3. Use table-driven tests where appropriate
4. Add cleanup code for any test artifacts
5. Update this README if adding new test categories

## Test Coverage

The test suite aims to cover:
- Basic proxy validation
- Security checks
  - Protocol smuggling
  - DNS rebinding
  - Cache poisoning
  - Host header injection
- Output formatting
  - Text output
  - JSON output
  - Working proxies output
- Configuration loading
  - Valid configurations
  - Invalid configurations
- Error handling
  - Network errors
  - Invalid input
  - Edge cases

## Mocking

The test suite uses several mocking strategies:
- `httptest.Server` for mock HTTP servers
- In-memory file system for config testing
- Mock proxy responses
- Simulated security vulnerabilities

## Continuous Integration

Tests are automatically run on:
- Every push to the repository
- Pull request creation
- Pull request updates

The GitHub Actions workflow runs:
1. Code linting
2. Unit tests
3. Coverage reporting
4. Security checks 