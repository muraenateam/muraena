#!/bin/bash
#
# Test script for Muraena + mitmproxy integration
# This script verifies that mitmproxy is properly configured and accessible
#
# Usage: ./test-mitmproxy.sh
#

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

PROXY_PORT="8080"
WEB_PORT="8081"

echo "Testing Muraena + mitmproxy Integration"
echo "========================================"
echo ""

# Test 1: Check if mitmproxy is installed
echo -n "1. Checking if mitmproxy is installed... "
if command -v mitmproxy &> /dev/null; then
    echo -e "${GREEN}✓${NC}"
    mitmproxy --version | head -n 1
else
    echo -e "${RED}✗${NC}"
    echo "   mitmproxy not found. Please install it first."
    exit 1
fi

echo ""

# Test 2: Check if mitmweb service is running
echo -n "2. Checking if mitmweb service is running... "
if systemctl is-active --quiet mitmweb; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC}"
    echo "   mitmweb service is not running"
    echo "   Start it with: sudo systemctl start mitmweb"
    exit 1
fi

echo ""

# Test 3: Check if proxy port is listening
echo -n "3. Checking if proxy port ($PROXY_PORT) is listening... "
if netstat -tln 2>/dev/null | grep -q ":$PROXY_PORT " || ss -tln 2>/dev/null | grep -q ":$PROXY_PORT "; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC}"
    echo "   Port $PROXY_PORT is not listening"
    exit 1
fi

echo ""

# Test 4: Check if web interface port is listening
echo -n "4. Checking if web interface port ($WEB_PORT) is listening... "
if netstat -tln 2>/dev/null | grep -q ":$WEB_PORT " || ss -tln 2>/dev/null | grep -q ":$WEB_PORT "; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC}"
    echo "   Port $WEB_PORT is not listening"
    exit 1
fi

echo ""

# Test 5: Test proxy connection
echo -n "5. Testing proxy connection... "
if curl -x "http://127.0.0.1:$PROXY_PORT" -s -o /dev/null -w "%{http_code}" --connect-timeout 5 "http://example.com" | grep -q "200"; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${YELLOW}⚠${NC}"
    echo "   Proxy connection test returned non-200 status"
    echo "   This may be normal depending on your setup"
fi

echo ""

# Test 6: Check web interface accessibility
echo -n "6. Testing web interface accessibility... "
if curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 "http://localhost:$WEB_PORT" | grep -q "200"; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${YELLOW}⚠${NC}"
    echo "   Web interface may not be accessible"
fi

echo ""

# Test 7: Check log directory
echo -n "7. Checking log directory... "
if [ -d "/var/log/mitmproxy" ]; then
    echo -e "${GREEN}✓${NC}"
    LOG_SIZE=$(du -sh /var/log/mitmproxy 2>/dev/null | cut -f1)
    echo "   Log size: $LOG_SIZE"
else
    echo -e "${YELLOW}⚠${NC}"
    echo "   Log directory not found at /var/log/mitmproxy"
fi

echo ""

# Test 8: Check if Muraena binary exists
echo -n "8. Checking if Muraena binary exists... "
if [ -f "./build/muraena" ] || [ -f "./muraena" ]; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${YELLOW}⚠${NC}"
    echo "   Muraena binary not found in current directory"
fi

echo ""

# Summary
echo "========================================"
echo -e "${GREEN}Test Summary${NC}"
echo "========================================"
echo ""
echo "mitmproxy Status:"
systemctl status mitmweb --no-pager -l | head -n 5
echo ""
echo "Access Points:"
echo "  - Proxy: http://127.0.0.1:$PROXY_PORT"
echo "  - Web Interface: http://localhost:$WEB_PORT"
echo ""
echo "To run Muraena with mitmproxy:"
echo "  ./muraena -config config/config.toml -proxy"
echo ""
echo "To access mitmweb remotely (SSH tunnel):"
echo "  ssh -L $WEB_PORT:localhost:$WEB_PORT user@$(hostname -I | awk '{print $1}')"
echo ""
echo "View mitmweb logs:"
echo "  sudo journalctl -u mitmweb -f"
echo ""
