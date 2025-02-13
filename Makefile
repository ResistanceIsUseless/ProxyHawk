.PHONY: all build test lint clean

# Default target
all: lint test build

# Build the application
build:
	go build -v ./...

# Run tests with coverage
test:
	go test -v -race -cover ./tests/...

# Run linting
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	go clean
	rm -f test_*.txt test_*.json
	rm -rf test_output

# Install dependencies
deps:
	go mod download
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run the proxy checker with default options
run:
	go run main.go -l test_proxies.txt -o results.txt -j results.json -d

# Run all tests and generate coverage report
coverage:
	go test -v -race -coverprofile=coverage.out ./tests/...
	go tool cover -html=coverage.out -o coverage.html

# Format code
fmt:
	go fmt ./...
	gofmt -s -w .

# Verify dependencies
verify:
	go mod verify
	go mod tidy

# Security check
security:
	gosec ./...

# Run tests in verbose mode
test-verbose:
	go test -v ./tests/...

# Run specific test
test-one:
	@if [ "$(test)" = "" ]; then \
		echo "Usage: make test-one test=TestName"; \
	else \
		go test -v ./tests/... -run $(test); \
	fi

# Run tests with short flag (skip long running tests)
test-short:
	go test -v -short ./tests/... 