#!/bin/bash

# Test script for TUI
cd "/Users/mgriffiths/Library/Mobile Documents/com~apple~CloudDocs/Projects/Code/ProxyHawk"

echo "==================================="
echo "ProxyHawk TUI Test"
echo "==================================="
echo ""
echo "Testing with build/proxies.txt (968 proxies)"
echo ""
echo "Instructions:"
echo "1. Watch for spinner animation (should be smooth)"
echo "2. Watch progress bar update (should increment)"
echo "3. Try resizing terminal window (progress bar should adjust)"
echo "4. Press 'q' to quit (should exit cleanly)"
echo ""
echo "Press Enter to start..."
read

./build/proxyhawk -l build/proxies.txt

echo ""
echo "Test complete!"
echo ""
echo "Did you observe:"
echo "- Smooth spinner animation? (Y/N)"
echo "- Progress updates? (Y/N)"
echo "- Responsive to window resize? (Y/N)"
echo "- Clean quit with 'q'? (Y/N)"
