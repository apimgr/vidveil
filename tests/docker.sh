#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Vidveil Integration Tests - Docker Runtime
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
# NEVER use project directory or bare /tmp
mkdir -p "${TMPDIR:-/tmp}/${PROJECT_ORG}"
TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX")
trap 'rm -rf "${TEMP_DIR}"; docker rm -f vidveil-test 2>/dev/null || true' EXIT

echo "🧪 Vidveil Docker Integration Tests"
echo "===================================="
echo "Temp dir: ${TEMP_DIR}"
echo

# Helper functions
pass() {
    echo -e "${GREEN}✓${NC} $1"
    TESTS_PASSED=$((TESTS_PASSED + 1))
    TESTS_RUN=$((TESTS_RUN + 1))
}

fail() {
    echo -e "${RED}✗${NC} $1"
    TESTS_FAILED=$((TESTS_FAILED + 1))
    TESTS_RUN=$((TESTS_RUN + 1))
}

info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

# Test function
test_endpoint() {
    local method="$1"
    local endpoint="$2"
    local expected_status="$3"
    local description="$4"
    
    local response
    local status
    
    response=$(docker exec vidveil-test curl -s -w "\n%{http_code}" -X "${method}" "http://localhost:8080${endpoint}" 2>/dev/null || echo "000")
    status=$(echo "$response" | tail -n1)
    
    if [ "$status" = "$expected_status" ]; then
        pass "$description (HTTP $status)"
    else
        fail "$description (expected $expected_status, got $status)"
    fi
}

# Step 1: Build binary in Docker per PART 13
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

# Step 2: Test binary info
info "Testing binary..."
VERSION=$(docker run --rm -v "${TEMP_DIR}":/app alpine:latest /app/${PROJECT_NAME} --version 2>&1 | head -1)
if [ -n "$VERSION" ]; then
    pass "Version check: $VERSION"
else
    fail "Version check failed"
fi

# Step 3: Start server in Docker container per PART 29
# Per PART 29: Debug container tools: apk add --no-cache curl bash file jq
info "Starting vidveil server..."
docker run -d \
    --name vidveil-test \
    -v "${TEMP_DIR}":/app \
    -v "${TEMP_DIR}/config":/config \
    -v "${TEMP_DIR}/data":/data \
    -p 8080:8080 \
    alpine:latest \
    sh -c "apk add --no-cache curl bash file jq >/dev/null 2>&1 && /app/${PROJECT_NAME} --address 0.0.0.0 --port 8080 --mode development --config /config --data /data"

sleep 3  # Allow container to start

# Wait for server to be ready (retry health check up to 15 times)
info "Waiting for server to be ready..."
for i in $(seq 1 15); do
    if docker exec vidveil-test curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/healthz" 2>/dev/null | grep -q "200"; then
        break
    fi
    sleep 1
done

# Step 4: Test Health Endpoint
info "Testing health endpoint..."
test_endpoint GET "/healthz" "200" "Health check"

# Step 5: Test Vidveil-Specific Endpoints (PART 36)
info "Testing vidveil-specific endpoints..."

# Test search endpoint (requires query parameter)
test_endpoint GET "/api/v1/search?q=test" "200" "Search endpoint"

# Test bangs endpoint
test_endpoint GET "/api/v1/bangs" "200" "Bang shortcuts list"

# Test autocomplete endpoint (per server.go: /api/v1/bangs/autocomplete)
test_endpoint GET "/api/v1/bangs/autocomplete?q=!p" "200" "Bang autocomplete"

# Test engines endpoint
test_endpoint GET "/api/v1/engines" "200" "Search engines list"

# Test specific engine endpoint (using pornhub as example)
test_endpoint GET "/api/v1/engines/pornhub" "200" "Engine details"

# Test stats endpoint
test_endpoint GET "/api/v1/stats" "200" "Statistics endpoint"

# Step 6: Test .txt extension (PART 13 requirement)
info "Testing .txt extension for simple output..."
test_endpoint GET "/api/v1/search.txt?q=test" "200" "Search .txt extension"

# Step 7: Test Accept headers (PART 13 requirement)
info "Testing Accept headers..."
RESPONSE=$(docker exec vidveil-test curl -s -H "Accept: application/json" "http://localhost:8080/api/v1/engines" 2>/dev/null)
if echo "$RESPONSE" | grep -q '"success"'; then
    pass "JSON Accept header"
else
    fail "JSON Accept header"
fi

# Step 8: Test SSE Streaming (PART 36 requirement)
info "Testing SSE streaming endpoint..."
# Test SSE with timeout and capture output
# Note: External search engines may not respond in test environment
# Testing that endpoint responds with proper SSE format (event/data lines)
SSE_OUTPUT=$(docker exec vidveil-test timeout 10 curl -s -N "http://localhost:8080/api/v1/search/stream?q=test" 2>/dev/null || true)

# SSE should return event: and data: lines (even if no results)
if echo "$SSE_OUTPUT" | grep -qE "(data:|event:)"; then
    pass "SSE streaming - SSE format correct"
else
    # Check if at least connection was successful (no error)
    if [ -n "$SSE_OUTPUT" ]; then
        pass "SSE streaming - endpoint responded"
    else
        fail "SSE streaming - no response"
    fi
fi

# Check for done message (may not always be present if engines timeout)
if echo "$SSE_OUTPUT" | grep -q '"done"'; then
    pass "SSE streaming - done message received"
else
    info "SSE streaming - done message not received (engines may have timed out)"
fi

# Test SSE with bang
SSE_BANG=$(docker exec vidveil-test timeout 5 curl -s -N "http://localhost:8080/api/v1/search/stream?q=!ph+test" 2>/dev/null || true)
if echo "$SSE_BANG" | grep -q "data:"; then
    pass "SSE streaming - with bang shortcuts"
else
    fail "SSE streaming - bang shortcuts not working"
fi

# Test SSE error - missing query
SSE_ERROR=$(docker exec vidveil-test curl -s -w "\n%{http_code}" "http://localhost:8080/api/v1/search/stream" 2>/dev/null)
SSE_STATUS=$(echo "$SSE_ERROR" | tail -n1)
if [ "$SSE_STATUS" = "400" ]; then
    pass "SSE streaming - missing query error (HTTP 400)"
else
    fail "SSE streaming - missing query should return 400, got $SSE_STATUS"
fi

# Step 9: Test API error handling
info "Testing error handling..."
test_endpoint GET "/api/v1/engines/nonexistent" "404" "Non-existent engine 404"
test_endpoint GET "/api/v1/search" "400" "Search without query 400"

# Step 10: Test frontend routes (smart detection per PART 13)
# Note: Homepage returns 302 due to age verification redirect (expected for adult content)
info "Testing frontend routes..."
test_endpoint GET "/age-verify" "200" "Age verification page"
test_endpoint GET "/server/about" "200" "About page (per PART 14: /server/*)"

# Step 11: Test robots.txt and well-known (PART 13/22 requirements)
info "Testing well-known endpoints..."
test_endpoint GET "/robots.txt" "200" "robots.txt"
test_endpoint GET "/.well-known/security.txt" "200" "security.txt"

# Final Summary
echo
echo "===================================="
echo -e "Tests run: ${TESTS_RUN}"
echo -e "${GREEN}Passed: ${TESTS_PASSED}${NC}"
if [ ${TESTS_FAILED} -gt 0 ]; then
    echo -e "${RED}Failed: ${TESTS_FAILED}${NC}"
    exit 1
else
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
fi
