# ProxyHawk Makefile
# Provides convenient commands for development, testing, and deployment

.PHONY: help build test clean docker-build docker-run docker-compose-up docker-compose-down docker-push lint fmt

# Default target
help: ## Show this help message
	@echo "ProxyHawk Development Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Build commands
build: ## Build the ProxyHawk binaries
	@echo "Building ProxyHawk binaries..."
	@mkdir -p build
	go build -ldflags="-w -s" -o build/proxyhawk cmd/proxyhawk/main.go
	go build -ldflags="-w -s" -o build/proxyhawk-server cmd/proxyhawk-server/main.go
	@echo "Build complete: ./build/proxyhawk, ./build/proxyhawk-server"

build-all: ## Build for all platforms (Linux, macOS, Windows)
	@echo "Building for all platforms..."
	@mkdir -p build/dist
	GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o build/dist/proxyhawk-linux-amd64 cmd/proxyhawk/main.go
	GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o build/dist/proxyhawk-server-linux-amd64 cmd/proxyhawk-server/main.go
	GOOS=linux GOARCH=arm64 go build -ldflags="-w -s" -o build/dist/proxyhawk-linux-arm64 cmd/proxyhawk/main.go
	GOOS=linux GOARCH=arm64 go build -ldflags="-w -s" -o build/dist/proxyhawk-server-linux-arm64 cmd/proxyhawk-server/main.go
	GOOS=darwin GOARCH=amd64 go build -ldflags="-w -s" -o build/dist/proxyhawk-darwin-amd64 cmd/proxyhawk/main.go
	GOOS=darwin GOARCH=amd64 go build -ldflags="-w -s" -o build/dist/proxyhawk-server-darwin-amd64 cmd/proxyhawk-server/main.go
	GOOS=darwin GOARCH=arm64 go build -ldflags="-w -s" -o build/dist/proxyhawk-darwin-arm64 cmd/proxyhawk/main.go
	GOOS=darwin GOARCH=arm64 go build -ldflags="-w -s" -o build/dist/proxyhawk-server-darwin-arm64 cmd/proxyhawk-server/main.go
	GOOS=windows GOARCH=amd64 go build -ldflags="-w -s" -o build/dist/proxyhawk-windows-amd64.exe cmd/proxyhawk/main.go
	GOOS=windows GOARCH=amd64 go build -ldflags="-w -s" -o build/dist/proxyhawk-server-windows-amd64.exe cmd/proxyhawk-server/main.go
	@echo "Multi-platform build complete in ./build/dist/"

# Test commands
test: ## Run all tests
	@echo "Running tests..."
	go test -v ./...

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-race: ## Run tests with race detection
	@echo "Running tests with race detection..."
	go test -v -race ./...

test-short: ## Run only short tests
	@echo "Running short tests..."
	go test -v -short ./...

benchmark: ## Run benchmark tests
	@echo "Running benchmarks..."
	go test -v -bench=. -benchmem ./...

# Code quality commands
lint: ## Run linter (requires golangci-lint)
	@echo "Running linter..."
	golangci-lint run

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

# Dependency management
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod verify

deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

deps-clean: ## Clean module cache
	@echo "Cleaning module cache..."
	go clean -modcache

# Docker commands
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -f deployments/docker/Dockerfile -t proxyhawk:latest .
	@echo "Docker image built: proxyhawk:latest"

docker-build-multi: ## Build multi-architecture Docker image
	@echo "Building multi-architecture Docker image..."
	docker buildx build --platform linux/amd64,linux/arm64 -f deployments/docker/Dockerfile -t proxyhawk:latest .

docker-run: ## Run Docker container with test configuration
	@echo "Running Docker container..."
	docker run --rm \
		-v $(PWD)/test-proxies.txt:/app/proxies.txt:ro \
		-v $(PWD)/output:/app/output \
		proxyhawk:latest \
		-l proxies.txt -o output/docker-results.txt --no-ui -v

docker-run-metrics: ## Run Docker container with metrics enabled
	@echo "Running Docker container with metrics..."
	docker run --rm -p 9090:9090 \
		-v $(PWD)/test-proxies.txt:/app/proxies.txt:ro \
		-v $(PWD)/output:/app/output \
		proxyhawk:latest \
		-l proxies.txt -o output/metrics-results.txt \
		--metrics --metrics-addr :9090 --no-ui -v

docker-compose-up: ## Start all Docker Compose services
	@echo "Starting Docker Compose services..."
	cd deployments/docker && docker-compose up -d
	@echo "Services started. Access Grafana at http://localhost:3000 (admin/admin)"

docker-compose-down: ## Stop all Docker Compose services
	@echo "Stopping Docker Compose services..."
	cd deployments/docker && docker-compose down

docker-compose-logs: ## Show Docker Compose logs
	cd deployments/docker && docker-compose logs -f

docker-push: ## Push Docker image to registry (requires REGISTRY variable)
ifndef REGISTRY
	$(error REGISTRY variable is not set. Use: make docker-push REGISTRY=your-registry.com/proxyhawk)
endif
	@echo "Pushing to registry: $(REGISTRY)"
	docker tag proxyhawk:latest $(REGISTRY):latest
	docker push $(REGISTRY):latest

# Development commands
dev-setup: ## Set up development environment
	@echo "Setting up development environment..."
	go mod download
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "Development environment ready"

run: ## Run ProxyHawk with default configuration
	@echo "Running ProxyHawk..."
	go run cmd/proxyhawk/main.go --config config/default.yaml -l test-proxies.txt -v

run-debug: ## Run ProxyHawk with debug output
	@echo "Running ProxyHawk in debug mode..."
	go run cmd/proxyhawk/main.go --config config/default.yaml -l test-proxies.txt -v -d

run-metrics: ## Run ProxyHawk with metrics enabled
	@echo "Running ProxyHawk with metrics..."
	go run cmd/proxyhawk/main.go --config config/metrics-example.yaml -l test-proxies.txt --metrics -v

# Clean commands
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf build/
	rm -f coverage.out coverage.html
	go clean ./...

clean-docker: ## Clean Docker images and containers
	@echo "Cleaning Docker resources..."
	docker system prune -f
	docker image prune -f

clean-all: clean clean-docker ## Clean everything
	@echo "All clean!"

# Release commands
release-check: test lint ## Check if ready for release
	@echo "Release readiness check complete"

# Create output directory
output:
	@mkdir -p output

# Installation commands  
install: build ## Install ProxyHawk binary to $GOPATH/bin
	@echo "Installing ProxyHawk..."
	go install cmd/proxyhawk/main.go

uninstall: ## Remove ProxyHawk binary from $GOPATH/bin
	@echo "Uninstalling ProxyHawk..."
	rm -f $(GOPATH)/bin/proxyhawk

# Documentation
docs: ## Generate documentation
	@echo "Generating documentation..."
	go doc -all ./... > docs/API.md
	@echo "API documentation generated: docs/API.md"

# Security
security-scan: ## Run security vulnerability scan
	@echo "Running security scan..."
	go list -json -deps ./... | nancy sleuth

# Integration tests
integration-test: docker-build ## Run integration tests with Docker
	@echo "Running integration tests..."
	./tests/integration/run_tests.sh

# Version info
version: ## Show version information
	@echo "ProxyHawk Build Information:"
	@echo "Go version: $(shell go version)"
	@echo "Git commit: $(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
	@echo "Build date: $(shell date -u +%Y-%m-%dT%H:%M:%SZ)"

# Default values
REGISTRY ?= proxyhawk
PLATFORMS ?= linux/amd64,linux/arm64