#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Vidveil Integration Tests - Incus Runtime (Full OS + systemd)
# Per TEMPLATE.md PART 13: Testing & Development

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

echo "🧪 Vidveil Incus Integration Tests (Debian + systemd)"
echo "======================================================" 
echo "Temp dir: ${TEMP_DIR}"
echo "Instance: ${INSTANCE_NAME}"
echo

# Helper functions
pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((TESTS_PASSED++))
    ((TESTS_RUN++))
}

fail() {
    echo -e "${RED}✗${NC} $1"
    ((TESTS_FAILED++))
    ((TESTS_RUN++))
}

info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

# Step 1: Build binary
info "Building binary with Docker (golang:alpine)..."
docker run --rm \
    -v "${PROJECT_ROOT}":/build \
    -v "${TEMP_DIR}":/output \
    -w /build \
    -e CGO_ENABLED=0 \
    golang:alpine \
    sh -c "go mod download && go build -ldflags '-s -w' -o /output/${PROJECT_NAME} ./src" 2>&1 | tail -5

if [ ! -f "${TEMP_DIR}/${PROJECT_NAME}" ]; then
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi
pass "Binary built successfully"

# Step 2: Launch Incus container (Debian for full systemd support)
info "Launching Incus container (Debian)..."
incus launch images:debian/12 "${INSTANCE_NAME}"
sleep 5
pass "Incus container launched"

# Step 3: Install binary
info "Installing binary..."
incus file push "${TEMP_DIR}/${PROJECT_NAME}" "${INSTANCE_NAME}/usr/local/bin/"
incus exec "${INSTANCE_NAME}" -- chmod +x "/usr/local/bin/${PROJECT_NAME}"
pass "Binary installed"

# Step 4: Test version
info "Testing binary..."
VERSION=$(incus exec "${INSTANCE_NAME}" -- ${PROJECT_NAME} --version | head -1)
if [ -n "$VERSION" ]; then
    pass "Version check: $VERSION"
else
    fail "Version check failed"
fi

# Step 5: Install as systemd service (PART 13 requirement for Incus tests)
info "Installing systemd service..."
if incus exec "${INSTANCE_NAME}" -- ${PROJECT_NAME} --service --install; then
    pass "Systemd service installed"
else
    fail "Systemd service installation failed"
fi

# Step 6: Start service
info "Starting vidveil service..."
sleep 2
if incus exec "${INSTANCE_NAME}" -- systemctl start ${PROJECT_NAME}; then
    pass "Service started"
else
    fail "Service start failed"
fi

sleep 5

# Step 7: Check service status
info "Checking service status..."
if incus exec "${INSTANCE_NAME}" -- systemctl is-active ${PROJECT_NAME} | grep -q "active"; then
    pass "Service is active"
else
    fail "Service is not active"
fi

# Step 8: Test HTTP endpoints
info "Testing API endpoints..."
if incus exec "${INSTANCE_NAME}" -- curl -s http://localhost:8080/healthz | grep -q "ok"; then
    pass "Health endpoint responding"
else
    fail "Health endpoint not responding"
fi

# Test vidveil-specific endpoints
if incus exec "${INSTANCE_NAME}" -- curl -s "http://localhost:8080/api/v1/engines" | grep -q "success"; then
    pass "Engines API responding"
else
    fail "Engines API not responding"
fi

if incus exec "${INSTANCE_NAME}" -- curl -s "http://localhost:8080/api/v1/bangs" | grep -q "success"; then
    pass "Bangs API responding"
else
    fail "Bangs API not responding"
fi

# Test SSE streaming
info "Testing SSE streaming..."
SSE_OUTPUT=$(incus exec "${INSTANCE_NAME}" -- timeout 5 curl -s -N "http://localhost:8080/api/v1/search/stream?q=test" 2>/dev/null || true)
if echo "$SSE_OUTPUT" | grep -q "data:"; then
    pass "SSE streaming responding"
else
    fail "SSE streaming not working"
fi

# Step 9: Test service logs
info "Checking service logs..."
if incus exec "${INSTANCE_NAME}" -- journalctl -u ${PROJECT_NAME} --no-pager -n 10 | grep -q "vidveil"; then
    pass "Service logging to journald"
else
    fail "No service logs found"
fi

# Step 10: Test service stop
info "Testing service stop..."
if incus exec "${INSTANCE_NAME}" -- systemctl stop ${PROJECT_NAME}; then
    pass "Service stopped cleanly"
else
    fail "Service stop failed"
fi

# Final Summary
echo
echo "======================================================" 
echo -e "Tests run: ${TESTS_RUN}"
echo -e "${GREEN}Passed: ${TESTS_PASSED}${NC}"
if [ ${TESTS_FAILED} -gt 0 ]; then
    echo -e "${RED}Failed: ${TESTS_FAILED}${NC}"
    exit 1
else
    echo -e "${GREEN}All Incus tests passed!${NC}"
    exit 0
fi
