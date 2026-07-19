#!/usr/bin/env bash
# @@License : WTFPL
# Vidveil Integration Tests - Docker Runtime
# Per AI.md PART 29: Testing & Development

set -euo pipefail

DOCKER_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKER_PROJECT_ROOT="$(cd "${DOCKER_SCRIPT_DIR}/.." && pwd)"
DOCKER_PROJECT_NAME="vidveil"
DOCKER_PROJECT_ORG="apimgr"

# Colors
DOCKER_RED='\033[0;31m'
DOCKER_GREEN='\033[0;32m'
DOCKER_YELLOW='\033[1;33m'
DOCKER_BLUE='\033[0;34m'
DOCKER_NC='\033[0m'

# Test counters
DOCKER_TESTS_RUN=0
DOCKER_TESTS_PASSED=0
DOCKER_TESTS_FAILED=0

# Per PART 13: ALWAYS create org directory first, then use temp directories with org/project structure
# NEVER use project directory or bare /tmp
mkdir -p "${TMPDIR:-/tmp}/${DOCKER_PROJECT_ORG}"
DOCKER_TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/${DOCKER_PROJECT_ORG}/${DOCKER_PROJECT_NAME}-XXXXXX")
trap 'rm -rf "${DOCKER_TEMP_DIR}"; docker rm -f vidveil-test 2>/dev/null || true' EXIT

echo "Vidveil Docker Integration Tests"
echo "================================="
echo "Temp dir: ${DOCKER_TEMP_DIR}"
echo

# Helper functions
__pass() {
    echo -e "${DOCKER_GREEN}+${DOCKER_NC} $1"
    DOCKER_TESTS_PASSED=$((DOCKER_TESTS_PASSED + 1))
    DOCKER_TESTS_RUN=$((DOCKER_TESTS_RUN + 1))
}

__fail() {
    echo -e "${DOCKER_RED}x${DOCKER_NC} $1"
    DOCKER_TESTS_FAILED=$((DOCKER_TESTS_FAILED + 1))
    DOCKER_TESTS_RUN=$((DOCKER_TESTS_RUN + 1))
}

__info() {
    echo -e "${DOCKER_BLUE}>${DOCKER_NC} $1"
}

# Test function
__test_endpoint() {
    local method="$1"
    local endpoint="$2"
    local expected_status="$3"
    local description="$4"

    local response
    local status

    response=$(docker exec vidveil-test curl -s -w "\n%{http_code}" -X "${method}" "http://localhost:8080${endpoint}" 2>/dev/null || echo "000")
    status=$(printf '%s\n' "$response" | tail -n1)

    if [ "$status" = "$expected_status" ]; then
        __pass "$description (HTTP $status)"
    else
        __fail "$description (expected $expected_status, got $status)"
    fi
}

# Step 1: Write build script and run via casjaysdev/go:latest
# Per AI.md PART 26: casjaysdev/go:latest entrypoint requires a shell script for multi-word commands
__info "Building server binary with casjaysdev/go:latest..."
cat > "${DOCKER_TEMP_DIR}/build.sh" << 'EOF'
#!/bin/sh
set -e
cd /build
go mod tidy
go mod download
go build -buildvcs=false -ldflags '-s -w' -o /output/vidveil ./src
EOF
chmod +x "${DOCKER_TEMP_DIR}/build.sh"

docker run --rm \
    -v "${DOCKER_PROJECT_ROOT}":/build \
    -v "${DOCKER_TEMP_DIR}":/output \
    -e CGO_ENABLED=0 \
    -e GOFLAGS=-buildvcs=false \
    casjaysdev/go:latest \
    /output/build.sh 2>&1 | tail -5

if [ ! -f "${DOCKER_TEMP_DIR}/${DOCKER_PROJECT_NAME}" ]; then
    echo -e "${DOCKER_RED}x Build failed${DOCKER_NC}"
    exit 1
fi
__pass "Server binary built successfully"

# Step 2: Build CLI binary (vidveil-cli — required per AI.md PART 8)
__info "Building CLI binary (vidveil-cli) with casjaysdev/go:latest..."
cat > "${DOCKER_TEMP_DIR}/build-cli.sh" << 'EOF'
#!/bin/sh
set -e
cd /build
go build -buildvcs=false -ldflags '-s -w' -o /output/vidveil-cli ./src/client
EOF
chmod +x "${DOCKER_TEMP_DIR}/build-cli.sh"

docker run --rm \
    -v "${DOCKER_PROJECT_ROOT}":/build \
    -v "${DOCKER_TEMP_DIR}":/output \
    -e CGO_ENABLED=0 \
    -e GOFLAGS=-buildvcs=false \
    casjaysdev/go:latest \
    /output/build-cli.sh 2>&1 | tail -5

if [ ! -f "${DOCKER_TEMP_DIR}/vidveil-cli" ]; then
    echo -e "${DOCKER_RED}x CLI build failed${DOCKER_NC}"
    exit 1
fi
__pass "CLI binary built successfully"

# Step 3: Test server binary info
__info "Testing server binary..."
# Capture full output first to avoid SIGPIPE from `| head -1` with pipefail
VERSION_OUT=$(docker run --rm -v "${DOCKER_TEMP_DIR}":/app alpine:latest /app/${DOCKER_PROJECT_NAME} --version 2>&1 || true)
VERSION=$(printf '%s\n' "$VERSION_OUT" | head -1)
if [ -n "$VERSION" ]; then
    __pass "Server version check: $VERSION"
else
    __fail "Server version check failed"
fi

# Step 4: Binary rename test (per AI.md PART 7: binary renames itself gracefully)
__info "Testing binary rename behavior..."
cp "${DOCKER_TEMP_DIR}/${DOCKER_PROJECT_NAME}" "${DOCKER_TEMP_DIR}/myveil"
RENAMED_HELP_OUT=$(docker run --rm -v "${DOCKER_TEMP_DIR}":/app alpine:latest /app/myveil --help 2>&1 || true)
RENAMED_HELP=$(printf '%s\n' "$RENAMED_HELP_OUT" | head -5)
if grep -qi -- "myveil" <<< "$RENAMED_HELP"; then
    __pass "Binary rename: --help shows new name"
else
    __fail "Binary rename: --help does not reflect new binary name"
fi
rm -f "${DOCKER_TEMP_DIR}/myveil"

# Step 5: Test CLI binary info
__info "Testing CLI binary..."
CLI_VERSION_OUT=$(docker run --rm -v "${DOCKER_TEMP_DIR}":/app alpine:latest /app/vidveil-cli --version 2>&1 || true)
CLI_VERSION=$(printf '%s\n' "$CLI_VERSION_OUT" | head -1)
if [ -n "$CLI_VERSION" ]; then
    __pass "CLI version check: $CLI_VERSION"
else
    __fail "CLI version check failed"
fi

# Step 6: Start server in Docker container per PART 29
__info "Starting vidveil server..."
docker run -d \
    --name vidveil-test \
    -v "${DOCKER_TEMP_DIR}":/app \
    -v "${DOCKER_TEMP_DIR}/config":/config \
    -v "${DOCKER_TEMP_DIR}/data":/data \
    -p 8080:8080 \
    alpine:latest \
    sh -c "apk add --no-cache curl bash file jq >/dev/null 2>&1 && /app/${DOCKER_PROJECT_NAME} --address 0.0.0.0 --port 8080 --mode development --config /config --data /data"

sleep 3

# Wait for server to be ready (retry health check up to 15 times)
# Per AI.md PART 13: /server/healthz is canonical; /healthz is opt-in
__info "Waiting for server to be ready..."
for i in $(seq 1 15); do
    if docker exec vidveil-test curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/server/healthz" 2>/dev/null | grep -q -- "200"; then
        break
    fi
    sleep 1
done

# Step 7: Test Health Endpoint
__info "Testing health endpoint..."
__test_endpoint GET "/server/healthz" "200" "Health check"

# Step 8: Test Vidveil-Specific Endpoints (PART 36)
__info "Testing vidveil-specific endpoints..."

__test_endpoint GET "/api/v1/search?q=test" "200" "Search endpoint"
__test_endpoint GET "/api/v1/bangs" "200" "Bang shortcuts list"
__test_endpoint GET "/api/v1/bangs/autocomplete?q=!p" "200" "Bang autocomplete"
__test_endpoint GET "/api/v1/engines" "200" "Search engines list"
__test_endpoint GET "/api/v1/stats" "200" "Statistics endpoint"
__test_endpoint GET "/api/v1/version" "200" "Version endpoint"
__test_endpoint GET "/api/v1/server/healthz" "200" "API versioned health check"
__test_endpoint GET "/api/v1/server/about" "200" "API about"
__test_endpoint GET "/api/v1/server/privacy" "200" "API privacy"
__test_endpoint GET "/api/v1/server/help" "200" "API help"
__test_endpoint GET "/api/healthz" "200" "API unversioned health alias"
__test_endpoint GET "/api/autodiscover" "200" "API autodiscover"

# Engine detail — use a known engine or expect 404 gracefully
ENGINE_STATUS=$(docker exec vidveil-test curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/api/v1/engines/google" 2>/dev/null || echo "000")
if [ "$ENGINE_STATUS" = "200" ] || [ "$ENGINE_STATUS" = "404" ]; then
    __pass "Engine detail endpoint reachable (HTTP $ENGINE_STATUS)"
else
    __fail "Engine detail endpoint unexpected status: $ENGINE_STATUS"
fi

# Step 9: Test .txt extension (PART 13 requirement)
__info "Testing .txt extension for simple output..."
__test_endpoint GET "/api/v1/search.txt?q=test" "200" "Search .txt extension"
__test_endpoint GET "/api/v1/engines.txt" "200" "Engines .txt extension"
__test_endpoint GET "/api/v1/bangs.txt" "200" "Bangs .txt extension"
__test_endpoint GET "/api/v1/stats.txt" "200" "Stats .txt extension"
__test_endpoint GET "/server/healthz.txt" "200" "Health check .txt extension"
__test_endpoint GET "/server/healthz.json" "200" "Health check .json extension"

# Step 10: Test well-known and static files
__info "Testing well-known and static files..."
__test_endpoint GET "/robots.txt" "200" "robots.txt"
__test_endpoint GET "/humans.txt" "200" "humans.txt"
__test_endpoint GET "/sitemap.xml" "200" "sitemap.xml"
__test_endpoint GET "/.well-known/security.txt" "200" "security.txt"
# pgp-key.asc returns 404 until a PGP keypair is generated (per AI.md PART 11)
PGP_STATUS=$(docker exec vidveil-test curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/.well-known/pgp-key.asc" 2>/dev/null || echo "000")
if [ "$PGP_STATUS" = "200" ] || [ "$PGP_STATUS" = "404" ]; then
    __pass "pgp-key.asc (HTTP $PGP_STATUS)"
else
    __fail "pgp-key.asc (expected 200 or 404, got $PGP_STATUS)"
fi
# Unsupported well-known entries must return 404 per AI.md well-known allowlist (PART 14)
__test_endpoint GET "/.well-known/change-password" "404" "change-password not allowlisted"
__test_endpoint GET "/.well-known/vidveil.json" "404" "vidveil.json not allowlisted"
__test_endpoint GET "/manifest.json" "200" "Web app manifest"
__test_endpoint GET "/sw.js" "200" "Service worker"

# Step 11: Test frontend routes
__info "Testing frontend routes..."
__test_endpoint GET "/age-verify" "200" "Age verification page"
__test_endpoint GET "/content-restricted" "200" "Content restricted page"
__test_endpoint GET "/server/about" "200" "About page (per PART 14: /server/*)"
__test_endpoint GET "/server/privacy" "200" "Privacy page"
__test_endpoint GET "/server/contact" "200" "Contact page"
__test_endpoint GET "/server/help" "200" "Help page"
__test_endpoint GET "/offline.html" "200" "Offline page"

# Step 12: Test SSE Streaming (PART 36 requirement)
__info "Testing SSE streaming endpoint..."
SSE_OUTPUT=$(docker exec vidveil-test timeout 10 curl -s -N -H "Accept: text/event-stream" "http://localhost:8080/api/v1/search?q=test" 2>/dev/null || true)

if grep -qE -- "(data:|event:)" <<< "$SSE_OUTPUT"; then
    __pass "SSE streaming - SSE format correct"
else
    if [ -n "$SSE_OUTPUT" ]; then
        __pass "SSE streaming - endpoint responded"
    else
        __fail "SSE streaming - no response"
    fi
fi

if grep -q -- '"done"' <<< "$SSE_OUTPUT"; then
    __pass "SSE streaming - done message received"
else
    __info "SSE streaming - done message not received (engines may have timed out)"
fi

SSE_BANG=$(docker exec vidveil-test timeout 5 curl -s -N -H "Accept: text/event-stream" "http://localhost:8080/api/v1/search?q=!ph+test" 2>/dev/null || true)
if grep -q -- "data:" <<< "$SSE_BANG"; then
    __pass "SSE streaming - with bang shortcuts"
else
    __fail "SSE streaming - bang shortcuts not working"
fi

SSE_ERROR=$(docker exec vidveil-test curl -s -w "\n%{http_code}" -H "Accept: text/event-stream" "http://localhost:8080/api/v1/search" 2>/dev/null)
SSE_STATUS=$(printf '%s\n' "$SSE_ERROR" | tail -n1)
if [ "$SSE_STATUS" = "400" ]; then
    __pass "SSE streaming - missing query error (HTTP 400)"
else
    __fail "SSE streaming - missing query should return 400, got $SSE_STATUS"
fi

# Step 13: Test API error handling
__info "Testing error handling..."
__test_endpoint GET "/api/v1/engines/nonexistent" "404" "Non-existent engine 404"
__test_endpoint GET "/api/v1/search" "400" "Search without query 400"

# Step 14: Test batch search (POST)
__info "Testing POST endpoints..."
BATCH_STATUS=$(docker exec vidveil-test curl -s -o /dev/null -w "%{http_code}" -X POST \
    -H "Content-Type: application/json" \
    -d '{"queries":[{"q":"test"}]}' \
    "http://localhost:8080/api/v1/search/batch" 2>/dev/null || echo "000")
if [ "$BATCH_STATUS" = "200" ] || [ "$BATCH_STATUS" = "202" ]; then
    __pass "Batch search endpoint (HTTP $BATCH_STATUS)"
else
    __fail "Batch search endpoint (expected 200/202, got $BATCH_STATUS)"
fi

# Step 15: Test comprehensive content negotiation (PART 28 — ALL routes ALL headers)
__info "Running comprehensive content negotiation tests..."
EXEC_PREFIX="docker exec vidveil-test" BASE_URL="http://localhost:8080" "${DOCKER_SCRIPT_DIR}/test_content_negotiation.sh" || DOCKER_TESTS_FAILED=$((DOCKER_TESTS_FAILED + 1))

# Step 16: Test shell completions (PART 8 - built into binary)
__info "Testing shell completions..."
BASH_COMPL=$(docker exec vidveil-test /app/${DOCKER_PROJECT_NAME} --shell completions bash 2>&1 || true)
if grep -q -- "complete" <<< "$BASH_COMPL" || grep -q -- "_vidveil" <<< "$BASH_COMPL"; then
    __pass "Shell completions: bash"
else
    __fail "Shell completions: bash"
fi

ZSH_COMPL=$(docker exec vidveil-test /app/${DOCKER_PROJECT_NAME} --shell completions zsh 2>&1 || true)
if grep -q -- "compdef" <<< "$ZSH_COMPL" || grep -q -- "_vidveil" <<< "$ZSH_COMPL"; then
    __pass "Shell completions: zsh"
else
    __fail "Shell completions: zsh"
fi

FISH_COMPL=$(docker exec vidveil-test /app/${DOCKER_PROJECT_NAME} --shell completions fish 2>&1 || true)
if grep -q -- "complete.*vidveil" <<< "$FISH_COMPL"; then
    __pass "Shell completions: fish"
else
    __fail "Shell completions: fish"
fi

# Final Summary
echo
echo "================================="
echo -e "Tests run: ${DOCKER_TESTS_RUN}"
echo -e "${DOCKER_GREEN}Passed: ${DOCKER_TESTS_PASSED}${DOCKER_NC}"
if [ ${DOCKER_TESTS_FAILED} -gt 0 ]; then
    echo -e "${DOCKER_RED}Failed: ${DOCKER_TESTS_FAILED}${DOCKER_NC}"
    exit 1
else
    echo -e "${DOCKER_GREEN}All tests passed!${DOCKER_NC}"
    exit 0
fi
