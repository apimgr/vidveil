#!/usr/bin/env bash
# @@License : WTFPL
# Vidveil Integration Tests - Incus Runtime (Full OS + systemd)
# Per AI.md PART 29: Testing & Development

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_NAME="vidveil"
PROJECT_ORG="apimgr"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Per PART 13: ALWAYS create org directory first, then use temp directories with org/project structure
mkdir -p "${TMPDIR:-/tmp}/${PROJECT_ORG}"
TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX")
INSTANCE_NAME="test-${PROJECT_NAME}-$$"
trap 'rm -rf "${TEMP_DIR}"; incus delete -f "${INSTANCE_NAME}" 2>/dev/null || true' EXIT

echo "Vidveil Incus Integration Tests (Debian + systemd)"
echo "===================================================="
echo "Temp dir: ${TEMP_DIR}"
echo "Instance: ${INSTANCE_NAME}"
echo

# Helper functions
pass() {
    echo -e "${GREEN}+${NC} $1"
    TESTS_PASSED=$((TESTS_PASSED + 1))
    TESTS_RUN=$((TESTS_RUN + 1))
}

fail() {
    echo -e "${RED}x${NC} $1"
    TESTS_FAILED=$((TESTS_FAILED + 1))
    TESTS_RUN=$((TESTS_RUN + 1))
}

info() {
    echo -e "${BLUE}>${NC} $1"
}

# Step 1: Build server binary
# Per AI.md PART 26: casjaysdev/go:latest entrypoint requires a shell script for multi-word commands
info "Building server binary with casjaysdev/go:latest..."
cat > "${TEMP_DIR}/build.sh" << 'EOF'
#!/bin/sh
set -e
cd /build
go mod tidy
go mod download
go build -buildvcs=false -ldflags '-s -w' -o /output/vidveil ./src
EOF
chmod +x "${TEMP_DIR}/build.sh"

docker run --rm \
    -v "${PROJECT_ROOT}":/build \
    -v "${TEMP_DIR}":/output \
    -e CGO_ENABLED=0 \
    -e GOFLAGS=-buildvcs=false \
    casjaysdev/go:latest \
    /output/build.sh 2>&1 | tail -5

if [ ! -f "${TEMP_DIR}/${PROJECT_NAME}" ]; then
    echo -e "${RED}x Build failed${NC}"
    exit 1
fi
pass "Server binary built successfully"

# Step 2: Build CLI binary (vidveil-cli — required per AI.md PART 8)
info "Building CLI binary (vidveil-cli) with casjaysdev/go:latest..."
cat > "${TEMP_DIR}/build-cli.sh" << 'EOF'
#!/bin/sh
set -e
cd /build
go build -buildvcs=false -ldflags '-s -w' -o /output/vidveil-cli ./src/client
EOF
chmod +x "${TEMP_DIR}/build-cli.sh"

docker run --rm \
    -v "${PROJECT_ROOT}":/build \
    -v "${TEMP_DIR}":/output \
    -e CGO_ENABLED=0 \
    -e GOFLAGS=-buildvcs=false \
    casjaysdev/go:latest \
    /output/build-cli.sh 2>&1 | tail -5

if [ ! -f "${TEMP_DIR}/vidveil-cli" ]; then
    echo -e "${RED}x CLI build failed${NC}"
    exit 1
fi
pass "CLI binary built successfully"

# Step 3: Launch Incus container (Debian trixie for full systemd support)
# Per AI.md PART 29: Incus tests MUST use images:debian/trixie (latest stable)
info "Launching Incus container (Debian trixie)..."
incus launch images:debian/trixie "${INSTANCE_NAME}"
sleep 5
pass "Incus container launched"

# Step 4: Install dependencies
info "Installing dependencies (curl)..."
incus exec "${INSTANCE_NAME}" -- apt-get update -qq 2>&1 | tail -1
incus exec "${INSTANCE_NAME}" -- apt-get install -y -qq curl 2>&1 | tail -1
pass "Dependencies installed"

# Step 5: Install server binary
info "Installing server binary..."
incus file push "${TEMP_DIR}/${PROJECT_NAME}" "${INSTANCE_NAME}/usr/local/bin/"
incus exec "${INSTANCE_NAME}" -- chmod +x "/usr/local/bin/${PROJECT_NAME}"
pass "Server binary installed"

# Step 6: Install CLI binary
info "Installing CLI binary..."
incus file push "${TEMP_DIR}/vidveil-cli" "${INSTANCE_NAME}/usr/local/bin/"
incus exec "${INSTANCE_NAME}" -- chmod +x "/usr/local/bin/vidveil-cli"
pass "CLI binary installed"

# Step 7: Test server version
info "Testing server binary..."
VERSION=$(incus exec "${INSTANCE_NAME}" -- ${PROJECT_NAME} --version | head -1)
if [ -n "$VERSION" ]; then
    pass "Server version check: $VERSION"
else
    fail "Server version check failed"
fi

# Step 8: Binary rename test (per AI.md PART 7: binary renames itself gracefully)
info "Testing binary rename behavior..."
incus exec "${INSTANCE_NAME}" -- cp "/usr/local/bin/${PROJECT_NAME}" "/usr/local/bin/myveil"
RENAMED_HELP=$(incus exec "${INSTANCE_NAME}" -- /usr/local/bin/myveil --help 2>&1 | head -5 || true)
if echo "$RENAMED_HELP" | grep -qi "myveil"; then
    pass "Binary rename: --help shows new name"
else
    fail "Binary rename: --help does not reflect new binary name"
fi
incus exec "${INSTANCE_NAME}" -- rm -f "/usr/local/bin/myveil"

# Step 9: Test CLI binary
info "Testing CLI binary..."
CLI_VERSION=$(incus exec "${INSTANCE_NAME}" -- vidveil-cli --version | head -1)
if [ -n "$CLI_VERSION" ]; then
    pass "CLI version check: $CLI_VERSION"
else
    fail "CLI version check failed"
fi

# Step 10: Install as systemd service (PART 13 requirement for Incus tests)
info "Installing systemd service..."
if incus exec "${INSTANCE_NAME}" -- ${PROJECT_NAME} --service --install; then
    pass "Systemd service installed"
else
    fail "Systemd service installation failed"
fi

# Step 11: Start service
info "Starting vidveil service..."
sleep 2
if incus exec "${INSTANCE_NAME}" -- systemctl start ${PROJECT_NAME}; then
    pass "Service started"
else
    fail "Service start failed"
fi

# Wait for service to be fully ready
sleep 8

# Step 12: Check service status
info "Checking service status..."
if incus exec "${INSTANCE_NAME}" -- systemctl is-active ${PROJECT_NAME} | grep -q "active"; then
    pass "Service is active"
else
    fail "Service is not active"
fi

# Detect port from service logs (random 64xxx per AI.md PART 5)
info "Detecting service port..."
# Try IPv6 format [::]:PORT first, then IPv4 0.0.0.0:PORT
SERVICE_PORT=$(incus exec "${INSTANCE_NAME}" -- journalctl -u ${PROJECT_NAME} --no-pager -n 50 2>/dev/null | grep -oP 'Listening on (\[::\]:|[0-9.]+:)\K[0-9]+' | tail -1 || echo "")
if [ -z "$SERVICE_PORT" ]; then
    SERVICE_PORT=$(incus exec "${INSTANCE_NAME}" -- ss -tlnp 2>/dev/null | grep vidveil | grep -oP ':\K[0-9]+' | head -1 || echo "")
fi
if [ -z "$SERVICE_PORT" ]; then
    # Fall back to reading config file
    SERVICE_PORT=$(incus exec "${INSTANCE_NAME}" -- grep -oP 'port:\s*["'"'"']?\K[0-9]+' /etc/apimgr/vidveil/server.yml 2>/dev/null || echo "64080")
fi
info "Service running on port: ${SERVICE_PORT}"

# Step 13: Test HTTP endpoints
# Per AI.md PART 13: /server/healthz is canonical; /healthz is opt-in via config
info "Testing API endpoints..."
if incus exec "${INSTANCE_NAME}" -- curl -s -o /dev/null -w "%{http_code}" "http://localhost:${SERVICE_PORT}/server/healthz" | grep -q "^200$"; then
    pass "Health endpoint responding"
else
    fail "Health endpoint not responding"
fi

# Per AI.md PART 14: API endpoints return JSON when Accept: application/json
ENGINES_OUT=$(incus exec "${INSTANCE_NAME}" -- curl -s -H "Accept: application/json" "http://localhost:${SERVICE_PORT}/api/v1/engines" 2>&1 || echo "curl failed")
if echo "$ENGINES_OUT" | grep -q '"ok"'; then
    pass "Engines API responding"
else
    fail "Engines API not responding (got: ${ENGINES_OUT:0:200})"
fi

BANGS_OUT=$(incus exec "${INSTANCE_NAME}" -- curl -s -H "Accept: application/json" "http://localhost:${SERVICE_PORT}/api/v1/bangs" 2>&1 || echo "curl failed")
if echo "$BANGS_OUT" | grep -q '"ok"'; then
    pass "Bangs API responding"
else
    fail "Bangs API not responding (got: ${BANGS_OUT:0:200})"
fi

# Test SSE streaming (informational - may fail without configured engines)
info "Testing SSE streaming..."
SSE_OUTPUT=$(incus exec "${INSTANCE_NAME}" -- timeout 5 curl -s -N -H "Accept: text/event-stream" "http://localhost:${SERVICE_PORT}/api/v1/search?q=test" 2>/dev/null || true)
if echo "$SSE_OUTPUT" | grep -q "data:\|event:"; then
    pass "SSE streaming responding"
else
    echo "  SSE streaming not returning data (engines may not be configured)"
fi

# Step 14: Test service logs
info "Checking service logs..."
if incus exec "${INSTANCE_NAME}" -- journalctl -u ${PROJECT_NAME} --no-pager -n 10 | grep -q "vidveil"; then
    pass "Service logging to journald"
else
    fail "No service logs found"
fi

# Step 15: Test service stop
info "Testing service stop..."
if incus exec "${INSTANCE_NAME}" -- systemctl stop ${PROJECT_NAME}; then
    pass "Service stopped cleanly"
else
    fail "Service stop failed"
fi

# Final Summary
echo
echo "===================================================="
echo -e "Tests run: ${TESTS_RUN}"
echo -e "${GREEN}Passed: ${TESTS_PASSED}${NC}"
if [ ${TESTS_FAILED} -gt 0 ]; then
    echo -e "${RED}Failed: ${TESTS_FAILED}${NC}"
    exit 1
else
    echo -e "${GREEN}All Incus tests passed!${NC}"
    exit 0
fi
