#!/bin/bash
set -e

# ProxyHawk Configuration Setup Script
# This script helps set up ProxyHawk configuration for first-time users

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEMPLATE_DIR="$PROJECT_ROOT/config"
USER_CONFIG_DIR="$HOME/.config/proxyhawk"

echo "🚀 ProxyHawk Configuration Setup"
echo "================================="
echo ""

# Check if template directory exists
if [ ! -d "$TEMPLATE_DIR" ]; then
    echo "❌ Template directory not found: $TEMPLATE_DIR"
    exit 1
fi

# Create user config directory
if [ ! -d "$USER_CONFIG_DIR" ]; then
    echo "📁 Creating user config directory: $USER_CONFIG_DIR"
    mkdir -p "$USER_CONFIG_DIR"
fi

# Check if server.yaml already exists
if [ -f "$USER_CONFIG_DIR/server.yaml" ]; then
    echo "⚠️  Configuration file already exists: $USER_CONFIG_DIR/server.yaml"
    read -p "Do you want to overwrite it? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Setup cancelled."
        exit 0
    fi
fi

# Copy template to user config directory
echo "📋 Copying template configuration..."
cp "$TEMPLATE_DIR/server.template.yaml" "$USER_CONFIG_DIR/server.yaml"

echo "✅ Configuration template copied to: $USER_CONFIG_DIR/server.yaml"
echo ""
echo "📝 Next Steps:"
echo "1. Edit $USER_CONFIG_DIR/server.yaml with your proxy settings"
echo "2. Replace placeholder proxy URLs with your actual proxies"
echo "3. Configure regions according to your proxy locations"
echo "4. Start ProxyHawk server: ./proxyhawk-server"
echo ""
echo "📚 Documentation:"
echo "- Configuration guide: $TEMPLATE_DIR/README.md"
echo "- Full examples: $TEMPLATE_DIR/server.example.yaml"
echo "- Docker setup: docker-compose.yml"
echo ""
echo "🔧 Quick Test:"
echo "   # Uses default config location automatically"
echo "   ./proxyhawk-server"
echo ""
echo "   # Or specify config explicitly"
echo "   ./proxyhawk-server -config $USER_CONFIG_DIR/server.yaml"
echo ""

# Check if proxyhawk-server binary exists
if [ -f "$PROJECT_ROOT/proxyhawk-server" ]; then
    echo "✅ ProxyHawk server binary found"
else
    echo "⚠️  ProxyHawk server binary not found"
    echo "   Build it with: go build -o proxyhawk-server cmd/proxyhawk-server/main.go"
fi

echo "Setup complete! 🎉"