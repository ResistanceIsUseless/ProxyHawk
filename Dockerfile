# Multi-stage Dockerfile for ProxyHawk
# Supports both amd64 and arm64 architectures

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

RUN go build -ldflags="-w -s" -o proxyhawk cmd/proxyhawk/main.go

# Verify the binary was built correctly
RUN chmod +x proxyhawk

# Final stage - minimal runtime image
FROM --platform=$TARGETPLATFORM alpine:3.19

# Install ca-certificates for HTTPS requests and tzdata for timezone support
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user for security
RUN addgroup -g 1001 -S proxyhawk && \
    adduser -u 1001 -S proxyhawk -G proxyhawk

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/proxyhawk .

# Copy configuration files
COPY --from=builder /app/config ./config

# Create directories for output files
RUN mkdir -p /app/output && \
    chown -R proxyhawk:proxyhawk /app/output

# Switch to non-root user
USER proxyhawk

# Expose port for metrics (if enabled)
EXPOSE 9090

# Set default command
ENTRYPOINT ["./proxyhawk"]

# Default configuration
CMD ["--config", "config/default.yaml", "--no-ui"]

# Metadata
LABEL maintainer="ProxyHawk Development Team"
LABEL description="A comprehensive proxy checker and validator with advanced security testing capabilities"
LABEL version="1.0"
LABEL org.opencontainers.image.source="https://github.com/ResistanceIsUseless/ProxyHawk"
LABEL org.opencontainers.image.description="ProxyHawk - Advanced proxy testing and security validation tool"
LABEL org.opencontainers.image.licenses="MIT"