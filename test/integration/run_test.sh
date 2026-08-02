#!/usr/bin/env bash
#
# Integration test runner for Muraena
#
# Prerequisites:
#   - Redis running on localhost:6379
#   - dnsmasq resolving *.muraena.anti to 127.0.0.1
#   - mkcert installed and initialized (mkcert -install)
#   - Network access to authenticationtest.com
#
# Usage:
#   bash test/integration/run_test.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
MURAENA_BIN="$PROJECT_DIR/build/muraena"
MURAENA_PID=""
TEST_CONFIG="$SCRIPT_DIR/config.toml"
CERTS_DIR="$SCRIPT_DIR/certs"
PHISHING_DOMAIN="evil-authtest.muraena.anti"

cleanup() {
    echo ""
    echo "=== Cleaning up ==="
    if [ -n "$MURAENA_PID" ] && kill -0 "$MURAENA_PID" 2>/dev/null; then
        echo "Stopping Muraena (PID $MURAENA_PID)..."
        kill "$MURAENA_PID" 2>/dev/null || true
        wait "$MURAENA_PID" 2>/dev/null || true
    fi
    echo "Done."
}
trap cleanup EXIT

echo "=== Muraena Integration Test Runner ==="
echo ""

# Check prerequisites
echo "Checking prerequisites..."

# 1. Redis
if ! redis-cli ping >/dev/null 2>&1; then
    echo "ERROR: Redis is not running. Start it with: redis-server &"
    exit 1
fi
echo "  [OK] Redis is running"

# 2. DNS resolution via dnsmasq
if getent hosts "$PHISHING_DOMAIN" >/dev/null 2>&1; then
    RESOLVED_IP=$(getent hosts "$PHISHING_DOMAIN" | awk '{print $1}')
    if [ "$RESOLVED_IP" != "127.0.0.1" ]; then
        echo "ERROR: $PHISHING_DOMAIN resolves to $RESOLVED_IP, expected 127.0.0.1"
        exit 1
    fi
    echo "  [OK] DNS resolves $PHISHING_DOMAIN -> 127.0.0.1"
else
    echo "ERROR: Cannot resolve $PHISHING_DOMAIN"
    echo ""
    echo "  dnsmasq setup required (one-time):"
    echo "    1. Install dnsmasq:"
    echo "       sudo apt install dnsmasq       # Debian/Ubuntu"
    echo "       sudo pacman -S dnsmasq         # Arch"
    echo ""
    echo "    2. Configure:"
    echo "       echo 'address=/muraena.anti/127.0.0.1' | sudo tee /etc/dnsmasq.d/muraena.conf"
    echo "       sudo systemctl restart dnsmasq"
    echo ""
    echo "    3. Verify:"
    echo "       dig $PHISHING_DOMAIN @127.0.0.1"
    echo ""
    exit 1
fi

# 3. mkcert certificates
if [ ! -f "$CERTS_DIR/_wildcard.muraena.anti+1.pem" ] || [ ! -f "$CERTS_DIR/_wildcard.muraena.anti+1-key.pem" ]; then
    echo "  Certificates not found in $CERTS_DIR, generating..."
    if ! command -v mkcert >/dev/null 2>&1; then
        echo "ERROR: mkcert is not installed."
        echo ""
        echo "  Install mkcert:"
        echo "    go install filippo.io/mkcert@latest"
        echo "    mkcert -install   # installs root CA into system trust store"
        echo ""
        exit 1
    fi
    mkdir -p "$CERTS_DIR"
    (cd "$CERTS_DIR" && mkcert "*.muraena.anti" "muraena.anti")
    echo "  [OK] Certificates generated"
else
    echo "  [OK] Certificates found"
fi

# 4. Check config
if [ ! -f "$TEST_CONFIG" ]; then
    echo "ERROR: Test config not found at $TEST_CONFIG"
    exit 1
fi
echo "  [OK] Test config found"

# 5. Build muraena
echo ""
echo "=== Building Muraena ==="
cd "$PROJECT_DIR"
make build
echo "  [OK] Build complete"

# 6. Start Muraena
echo ""
echo "=== Starting Muraena ==="
cd "$PROJECT_DIR"
"$MURAENA_BIN" -config "$TEST_CONFIG" -debug &
MURAENA_PID=$!
echo "  Muraena started (PID $MURAENA_PID)"

# Wait for Muraena to be ready
echo "  Waiting for Muraena to initialize..."
sleep 3

if ! kill -0 "$MURAENA_PID" 2>/dev/null; then
    echo "ERROR: Muraena process died. Check logs."
    exit 1
fi
echo "  [OK] Muraena is running"

# 7. Flush Redis before tests
echo ""
echo "=== Flushing Redis ==="
redis-cli FLUSHDB >/dev/null
echo "  [OK] Redis flushed"

# 8. Run integration tests
echo ""
echo "=== Running Integration Tests ==="
cd "$PROJECT_DIR"
MURAENA_INTEGRATION=1 go test -v -count=1 ./test/integration/
TEST_EXIT=$?

echo ""
if [ $TEST_EXIT -eq 0 ]; then
    echo "=== ALL TESTS PASSED ==="
else
    echo "=== SOME TESTS FAILED (exit code: $TEST_EXIT) ==="
fi

exit $TEST_EXIT
