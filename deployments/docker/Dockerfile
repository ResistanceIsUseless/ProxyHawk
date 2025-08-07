# Multi-stage Dockerfile for ProxyHawk
# Supports both amd64 and arm64 architectures with dual-mode server and Tor integration

# Build stage
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder

# Set build arguments for cross-compilation
ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

# Install necessary packages for building
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the application with proper cross-compilation support
ENV CGO_ENABLED=0
ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}

# Build both the main checker and the new server
RUN go build -ldflags="-w -s" -o proxyhawk cmd/proxyhawk/main.go
RUN go build -ldflags="-w -s" -o proxyhawk-server cmd/proxyhawk-server/main.go

# Verify the binaries were built correctly
RUN chmod +x proxyhawk proxyhawk-server

# Final stage - runtime image with Tor support
FROM --platform=$TARGETPLATFORM alpine:3.19

# Install runtime dependencies including Tor for proxy chaining
RUN apk --no-cache add ca-certificates tzdata tor curl

# Create non-root user for security
RUN addgroup -g 1001 -S proxyhawk && \
    adduser -u 1001 -S proxyhawk -G proxyhawk

# Create necessary directories
RUN mkdir -p /app/output /app/config /app/logs /var/lib/tor && \
    chown -R proxyhawk:proxyhawk /app /var/lib/tor

# Set working directory
WORKDIR /app

# Copy binaries from builder stage
COPY --from=builder /app/proxyhawk .
COPY --from=builder /app/proxyhawk-server .

# Copy configuration files
COPY --from=builder /app/config ./config

# Copy Tor configuration
COPY docker/torrc /etc/tor/torrc

# Switch to non-root user
USER proxyhawk

# Expose ports for dual-mode server
EXPOSE 1080 8080 8888 9090
# 1080 - SOCKS5 proxy
# 8080 - HTTP proxy  
# 8888 - WebSocket API
# 9090 - Metrics

# Health check for server mode
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:9090/health || curl -f http://localhost:8888/health || exit 1

# Set default command to dual-mode server
ENTRYPOINT ["./proxyhawk-server"]

# Default configuration for dual-mode
CMD ["--config", "config/examples/proxy-chaining.yaml"]

# Metadata
LABEL maintainer="ProxyHawk Development Team"
LABEL description="A comprehensive proxy checker and validator with advanced security testing capabilities"
LABEL version="1.0"
LABEL org.opencontainers.image.source="https://github.com/ResistanceIsUseless/ProxyHawk"
LABEL org.opencontainers.image.description="ProxyHawk - Advanced proxy testing and security validation tool"
LABEL org.opencontainers.image.licenses="MIT"