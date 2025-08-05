#!/bin/bash

# ProxyHawk Docker Deployment Script
# This script demonstrates how to deploy ProxyHawk using Docker

set -e

# Configuration
PROXYHAWK_VERSION="latest"
OUTPUT_DIR="./output"
CONFIG_DIR="./config"
PROXY_FILE="./test-proxies.txt"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_requirements() {
    log_info "Checking requirements..."
    
    # Check if Docker is installed
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    # Check if Docker is running
    if ! docker info &> /dev/null; then
        log_error "Docker daemon is not running. Please start Docker."
        exit 1
    fi
    
    # Check if docker-compose is available
    if ! command -v docker-compose &> /dev/null; then
        log_warning "docker-compose not found, trying docker compose..."
        if ! docker compose version &> /dev/null; then
            log_error "Neither docker-compose nor docker compose is available."
            exit 1
        fi
        COMPOSE_CMD="docker compose"
    else
        COMPOSE_CMD="docker-compose"
    fi
    
    log_success "All requirements met"
}

create_directories() {
    log_info "Creating necessary directories..."
    mkdir -p "$OUTPUT_DIR"
    mkdir -p "$(dirname "$PROXY_FILE")"
    log_success "Directories created"
}

create_sample_proxy_file() {
    if [ ! -f "$PROXY_FILE" ]; then
        log_info "Creating sample proxy file..."
        cat > "$PROXY_FILE" << EOF
# Sample proxy list for testing
# Format: protocol://host:port or protocol://username:password@host:port
http://proxy1.example.com:8080
http://proxy2.example.com:3128
socks5://proxy3.example.com:1080
https://proxy4.example.com:8443
EOF
        log_success "Sample proxy file created at $PROXY_FILE"
        log_warning "Please replace with your actual proxy list before running"
    fi
}

build_image() {
    log_info "Building ProxyHawk Docker image..."
    docker build -t proxyhawk:$PROXYHAWK_VERSION .
    log_success "Docker image built successfully"
}

show_usage() {
    cat << EOF

ProxyHawk Docker Deployment Script

Usage: $0 [COMMAND] [OPTIONS]

Commands:
    setup           Set up environment and build image
    run-basic       Run basic proxy checking
    run-metrics     Run with metrics enabled
    run-auth        Run with authentication support
    run-security    Run with advanced security testing
    compose-up      Start all services with docker-compose
    compose-down    Stop all services
    clean           Clean up containers and images
    help            Show this help message

Options:
    -f, --proxy-file FILE   Specify proxy file (default: $PROXY_FILE)
    -o, --output-dir DIR    Specify output directory (default: $OUTPUT_DIR)
    -c, --config-dir DIR    Specify config directory (default: $CONFIG_DIR)
    -v, --version VERSION   Specify ProxyHawk version (default: $PROXYHAWK_VERSION)

Examples:
    $0 setup                                    # Initial setup
    $0 run-basic                               # Basic proxy checking
    $0 run-metrics                             # Run with monitoring
    $0 compose-up                              # Start full monitoring stack
    $0 run-basic -f my-proxies.txt             # Use custom proxy file

EOF
}

run_basic() {
    log_info "Running basic ProxyHawk container..."
    docker run --rm \
        -v "$(pwd)/$PROXY_FILE:/app/proxies.txt:ro" \
        -v "$(pwd)/$OUTPUT_DIR:/app/output" \
        -v "$(pwd)/$CONFIG_DIR:/app/config:ro" \
        proxyhawk:$PROXYHAWK_VERSION \
        -l proxies.txt \
        -o output/results.txt \
        -j output/results.json \
        --no-ui -v
    
    log_success "Basic proxy checking completed"
    log_info "Results saved to $OUTPUT_DIR/results.txt and $OUTPUT_DIR/results.json"
}

run_metrics() {
    log_info "Running ProxyHawk with metrics enabled..."
    log_info "Metrics will be available at http://localhost:9090/metrics"
    
    docker run --rm \
        -p 9090:9090 \
        -v "$(pwd)/$PROXY_FILE:/app/proxies.txt:ro" \
        -v "$(pwd)/$OUTPUT_DIR:/app/output" \
        -v "$(pwd)/$CONFIG_DIR:/app/config:ro" \
        proxyhawk:$PROXYHAWK_VERSION \
        -l proxies.txt \
        -o output/metrics-results.txt \
        -j output/metrics-results.json \
        --metrics --metrics-addr :9090 \
        --no-ui -v
    
    log_success "Metrics-enabled proxy checking completed"
}

run_auth() {
    log_info "Running ProxyHawk with authentication support..."
    
    if [ ! -f "$CONFIG_DIR/auth-example.yaml" ]; then
        log_error "Authentication config file not found: $CONFIG_DIR/auth-example.yaml"
        exit 1
    fi
    
    docker run --rm \
        -v "$(pwd)/$PROXY_FILE:/app/proxies.txt:ro" \
        -v "$(pwd)/$OUTPUT_DIR:/app/output" \
        -v "$(pwd)/$CONFIG_DIR:/app/config:ro" \
        proxyhawk:$PROXYHAWK_VERSION \
        --config config/auth-example.yaml \
        -l proxies.txt \
        -o output/auth-results.txt \
        -j output/auth-results.json \
        --no-ui -v
    
    log_success "Authentication-enabled proxy checking completed"
}

run_security() {
    log_info "Running ProxyHawk with advanced security testing..."
    
    docker run --rm \
        -v "$(pwd)/$PROXY_FILE:/app/proxies.txt:ro" \
        -v "$(pwd)/$OUTPUT_DIR:/app/output" \
        -v "$(pwd)/$CONFIG_DIR:/app/config:ro" \
        proxyhawk:$PROXYHAWK_VERSION \
        --config config/production.yaml \
        -l proxies.txt \
        -o output/security-results.txt \
        -j output/security-results.json \
        --no-ui -v -d
    
    log_success "Security testing completed"
    log_info "Detailed security analysis saved to $OUTPUT_DIR/security-results.json"
}

compose_up() {
    log_info "Starting ProxyHawk services with Docker Compose..."
    
    if [ ! -f "docker-compose.yml" ]; then
        log_error "docker-compose.yml not found in current directory"
        exit 1
    fi
    
    $COMPOSE_CMD up -d
    
    log_success "Services started successfully"
    log_info "Access points:"
    log_info "  - Grafana Dashboard: http://localhost:3000 (admin/admin)"  
    log_info "  - Prometheus: http://localhost:9091"
    log_info "  - ProxyHawk Metrics: http://localhost:9090/metrics"
    log_info ""
    log_info "View logs with: $COMPOSE_CMD logs -f"
    log_info "Stop services with: $0 compose-down"
}

compose_down() {
    log_info "Stopping ProxyHawk services..."
    $COMPOSE_CMD down
    log_success "Services stopped"
}

clean_up() {
    log_info "Cleaning up ProxyHawk containers and images..."
    
    # Stop and remove containers
    docker ps -a | grep proxyhawk | awk '{print $1}' | xargs -r docker rm -f
    
    # Remove images
    docker images | grep proxyhawk | awk '{print $3}' | xargs -r docker rmi -f
    
    # Clean up Docker system
    docker system prune -f
    
    log_success "Cleanup completed"
}

setup() {
    log_info "Setting up ProxyHawk Docker environment..."
    check_requirements
    create_directories
    create_sample_proxy_file
    build_image
    log_success "Setup completed successfully"
    log_info ""
    log_info "Next steps:"
    log_info "1. Edit $PROXY_FILE with your proxy list"
    log_info "2. Run '$0 run-basic' for basic proxy checking"
    log_info "3. Run '$0 compose-up' for full monitoring stack"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -f|--proxy-file)
            PROXY_FILE="$2"
            shift 2
            ;;
        -o|--output-dir)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        -c|--config-dir)
            CONFIG_DIR="$2"
            shift 2
            ;;
        -v|--version)
            PROXYHAWK_VERSION="$2"
            shift 2
            ;;
        setup)
            COMMAND="setup"
            shift
            ;;
        run-basic)
            COMMAND="run-basic"
            shift
            ;;
        run-metrics)
            COMMAND="run-metrics"
            shift
            ;;
        run-auth)
            COMMAND="run-auth"
            shift
            ;;
        run-security)
            COMMAND="run-security"
            shift
            ;;
        compose-up)
            COMMAND="compose-up"
            shift
            ;;
        compose-down)
            COMMAND="compose-down"
            shift
            ;;
        clean)
            COMMAND="clean"
            shift
            ;;
        help|--help|-h)
            show_usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Execute command
case ${COMMAND:-help} in
    setup)
        setup
        ;;
    run-basic)
        check_requirements
        run_basic
        ;;
    run-metrics)
        check_requirements
        run_metrics
        ;;
    run-auth)
        check_requirements
        run_auth
        ;;
    run-security)
        check_requirements
        run_security
        ;;
    compose-up)
        check_requirements
        compose_up
        ;;
    compose-down)
        check_requirements
        compose_down
        ;;
    clean)
        check_requirements
        clean_up
        ;;
    help)
        show_usage
        ;;
    *)
        log_error "No command specified"
        show_usage
        exit 1
        ;;
esac