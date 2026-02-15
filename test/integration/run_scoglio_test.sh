#!/usr/bin/env bash
#
# Integration test runner for Muraena + Scoglio (self-hosted target)
#
# Prerequisites:
#   - Redis running on localhost:6379
#   - dnsmasq resolving *.muraena.anti to 127.0.0.1
#   - mkcert installed and initialized (mkcert -install)
#
# Usage:
#   bash test/integration/run_scoglio_test.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
MURAENA_BIN="$PROJECT_DIR/build/muraena"
SCOGLIO_DIR="$SCRIPT_DIR/scoglio"
SCOGLIO_BIN="$SCOGLIO_DIR/scoglio"
MURAENA_PID=""
SCOGLIO_PID=""
TEST_CONFIG="$SCRIPT_DIR/config-scoglio.toml"
CERTS_DIR="$SCRIPT_DIR/certs"
PHISHING_DOMAIN="evil-scoglio.muraena.anti"

cleanup() {
    echo ""
    echo "=== Cleaning up ==="
    if [ -n "$MURAENA_PID" ] && kill -0 "$MURAENA_PID" 2>/dev/null; then
        echo "Stopping Muraena (PID $MURAENA_PID)..."
        kill "$MURAENA_PID" 2>/dev/null || true
        wait "$MURAENA_PID" 2>/dev/null || true
    fi
    if [ -n "$SCOGLIO_PID" ] && kill -0 "$SCOGLIO_PID" 2>/dev/null; then
        echo "Stopping Scoglio (PID $SCOGLIO_PID)..."
        kill "$SCOGLIO_PID" 2>/dev/null || true
        wait "$SCOGLIO_PID" 2>/dev/null || true
    fi
    # Clean up test database.
    rm -f /tmp/scoglio-test.db
    echo "Done."
}
trap cleanup EXIT

echo "=== Muraena + Scoglio Integration Test Runner ==="
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
    echo "    echo 'address=/muraena.anti/127.0.0.1' | sudo tee /etc/dnsmasq.d/muraena.conf"
    echo "    sudo systemctl restart dnsmasq"
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
        echo "    mkcert -install"
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

# 5. Build Scoglio
echo ""
echo "=== Building Scoglio ==="
cd "$SCOGLIO_DIR"
go build -o scoglio .
echo "  [OK] Scoglio build complete"

# 6. Start Scoglio
echo ""
echo "=== Starting Scoglio ==="
"$SCOGLIO_BIN" \
    --addr :9443 \
    --cert "$CERTS_DIR/_wildcard.muraena.anti+1.pem" \
    --key "$CERTS_DIR/_wildcard.muraena.anti+1-key.pem" \
    --db /tmp/scoglio-test.db &
SCOGLIO_PID=$!
echo "  Scoglio started (PID $SCOGLIO_PID)"

# Wait for Scoglio to be ready.
echo "  Waiting for Scoglio to initialize..."
sleep 2

if ! kill -0 "$SCOGLIO_PID" 2>/dev/null; then
    echo "ERROR: Scoglio process died. Check output above."
    exit 1
fi
echo "  [OK] Scoglio is running on :9443"

# 7. Build Muraena
echo ""
echo "=== Building Muraena ==="
cd "$PROJECT_DIR"
make build
echo "  [OK] Muraena build complete"

# 8. Start Muraena
echo ""
echo "=== Starting Muraena ==="
cd "$PROJECT_DIR"
"$MURAENA_BIN" -config "$TEST_CONFIG" -debug &
MURAENA_PID=$!
echo "  Muraena started (PID $MURAENA_PID)"

# Wait for Muraena to be ready.
echo "  Waiting for Muraena to initialize..."
sleep 3

if ! kill -0 "$MURAENA_PID" 2>/dev/null; then
    echo "ERROR: Muraena process died. Check logs."
    exit 1
fi
echo "  [OK] Muraena is running on :8443"

# 9. Flush Redis before tests
echo ""
echo "=== Flushing Redis ==="
redis-cli FLUSHDB >/dev/null
echo "  [OK] Redis flushed"

# 10. Run integration tests
echo ""
echo "=== Running Scoglio Integration Tests ==="
cd "$PROJECT_DIR"
MURAENA_INTEGRATION=1 SCOGLIO_INTEGRATION=1 go test -v -count=1 -run TestScoglio ./test/integration/
TEST_EXIT=$?

echo ""
if [ $TEST_EXIT -eq 0 ]; then
    echo "=== ALL SCOGLIO TESTS PASSED ==="
else
    echo "=== SOME SCOGLIO TESTS FAILED (exit code: $TEST_EXIT) ==="
fi

exit $TEST_EXIT
