#!/usr/bin/env bash
#
# Integration test runner for Muraena
#
# Prerequisites:
#   - Redis running on localhost:6379
#   - Network access to authenticationtest.com
#
# Usage:
#   bash test/integration/run_test.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
MURAENA_BIN="$PROJECT_DIR/muraena"
MURAENA_PID=""
TEST_CONFIG="$SCRIPT_DIR/config.toml"

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

# Redis
if ! redis-cli ping >/dev/null 2>&1; then
    echo "ERROR: Redis is not running. Start it with: redis-server &"
    exit 1
fi
echo "  [OK] Redis is running"

# Check config
if [ ! -f "$TEST_CONFIG" ]; then
    echo "ERROR: Test config not found at $TEST_CONFIG"
    exit 1
fi
echo "  [OK] Test config found"

# Build muraena if needed
if [ ! -f "$MURAENA_BIN" ] || [ "$PROJECT_DIR/main.go" -nt "$MURAENA_BIN" ]; then
    echo ""
    echo "=== Building Muraena ==="
    cd "$PROJECT_DIR"
    make build
    echo "  [OK] Build complete"
fi

# Check /etc/hosts entry
if ! grep -q "evil-authtest.local" /etc/hosts 2>/dev/null; then
    echo ""
    echo "WARNING: 'evil-authtest.local' not found in /etc/hosts"
    echo "  Add this line to /etc/hosts:"
    echo "  127.0.0.1 evil-authtest.local"
    echo ""
    echo "  Without this, the integration tests will fail to connect."
    echo ""
fi

# Start Muraena
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

# Flush Redis before tests
echo ""
echo "=== Flushing Redis ==="
redis-cli FLUSHDB >/dev/null
echo "  [OK] Redis flushed"

# Run integration tests
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
