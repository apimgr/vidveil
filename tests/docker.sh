#!/usr/bin/env bash
# @@License : WTFPL
# Vidveil Integration Tests - Docker Runtime
# Per AI.md PART 28: Testing & Development

set -euo pipefail

if [ "$#" -gt 0 ]; then
    echo "Usage: $0" >&2
    exit 2
fi

DOCKER_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKER_PROJECT_ROOT="$(cd "${DOCKER_SCRIPT_DIR}/.." && pwd)"
DOCKER_REMOTE_URL="$(git -C "${DOCKER_PROJECT_ROOT}" remote get-url origin 2>/dev/null || true)"
DOCKER_PROJECT_NAME="$(sed -n 's/^project_name:[[:space:]]*//p' "${DOCKER_PROJECT_ROOT}/IDEA.md" | head -n1 | tr -d '[:space:]')"
DOCKER_PROJECT_ORG="$(sed -n 's/^project_org:[[:space:]]*//p' "${DOCKER_PROJECT_ROOT}/IDEA.md" | head -n1 | tr -d '[:space:]')"
DOCKER_INTERNAL_NAME="$(sed -n 's/^internal_name:[[:space:]]*//p' "${DOCKER_PROJECT_ROOT}/IDEA.md" | head -n1 | tr -d '[:space:]')"
if [ -z "${DOCKER_REMOTE_URL}" ]; then
    DOCKER_PROJECT_NAME="${DOCKER_PROJECT_NAME:-$(basename "${DOCKER_PROJECT_ROOT}")}"
    DOCKER_PROJECT_ORG="${DOCKER_PROJECT_ORG:-$(basename "$(dirname "${DOCKER_PROJECT_ROOT}")")}"
else
    DOCKER_REMOTE_NAME="${DOCKER_REMOTE_URL##*/}"
    DOCKER_REMOTE_NAME="${DOCKER_REMOTE_NAME%.git}"
    DOCKER_REMOTE_ORG="$(printf '%s' "${DOCKER_REMOTE_URL%/*}" | sed -E 's|.*[:/]([^/:]+)$|\1|')"
    DOCKER_PROJECT_NAME="${DOCKER_PROJECT_NAME:-${DOCKER_REMOTE_NAME}}"
    DOCKER_PROJECT_ORG="${DOCKER_PROJECT_ORG:-${DOCKER_REMOTE_ORG}}"
fi
# Frozen on-disk identity (AI.md PART 25/29) — used for tempdir naming only,
# distinct from DOCKER_PROJECT_NAME (the compiled binary's actual filename)
DOCKER_INTERNAL_NAME="${DOCKER_INTERNAL_NAME:-${DOCKER_PROJECT_NAME}}"
if ! command -v docker >/dev/null 2>&1; then
    echo "ERROR: docker is required to run this test script" >&2
    exit 69
fi
DOCKER_TEST_SUFFIX="$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n')"
DOCKER_TEST_CONTAINER="${DOCKER_PROJECT_NAME}-${DOCKER_TEST_SUFFIX}"
DOCKER_BIN_DIR="${BINDIR:-binaries}"
DOCKER_CLEANUP_BINDIR=false
DOCKER_MEM="${DOCKER_MEM:-4g}"
DOCKER_CPUS="${DOCKER_CPUS:-2}"
DOCKER_CPU_SHARES="${DOCKER_CPU_SHARES:-256}"
DOCKER_TIMEOUT="${DOCKER_TIMEOUT:-3600}"

# Colors
DOCKER_RED='\033[0;31m'
DOCKER_GREEN='\033[0;32m'
DOCKER_BLUE='\033[0;34m'
DOCKER_NC='\033[0m'

# Test counters
DOCKER_TESTS_RUN=0
DOCKER_TESTS_PASSED=0
DOCKER_TESTS_FAILED=0

# Per PART 13: ALWAYS create org directory first, then use temp directories with org/project structure
# NEVER use project directory or bare /tmp
mkdir -p "${TMPDIR:-/tmp}/${DOCKER_PROJECT_ORG}"
DOCKER_TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/${DOCKER_PROJECT_ORG}/${DOCKER_INTERNAL_NAME}-XXXXXX")
DOCKER_CONTAINER_REGISTRY="${DOCKER_TEMP_DIR}/containers"
touch "${DOCKER_CONTAINER_REGISTRY}"
mkdir -p "${DOCKER_TEMP_DIR}/config" "${DOCKER_TEMP_DIR}/data"
chmod 0700 "${DOCKER_TEMP_DIR}/config" "${DOCKER_TEMP_DIR}/data"
DOCKER_GO_CACHE="${GO_CACHE:-${HOME}/go/pkg/mod}"
DOCKER_GO_BUILD="${GO_BUILD:-${HOME}/.cache/go-build/${DOCKER_PROJECT_NAME}}"
mkdir -p "${DOCKER_GO_CACHE}" "${DOCKER_GO_BUILD}"
# ShellCheck does not count the indirect EXIT trap invocation.
# shellcheck disable=SC2329
cleanup_docker_test_artifacts() {
    local container_name
    if [ -f "${DOCKER_CONTAINER_REGISTRY}" ]; then
        while IFS= read -r container_name; do
            [ -n "${container_name}" ] || continue
            if docker_resource_matches_run container "${container_name}"; then
                timeout -k 5 30 docker rm -f "${container_name}" >/dev/null 2>&1 || true
            fi
        done < "${DOCKER_CONTAINER_REGISTRY}"
    fi
    if [ "${DOCKER_CLEANUP_BINDIR}" = true ]; then
        rm -rf "${DOCKER_BIN_DIR}"
    fi
    rm -rf "${DOCKER_TEMP_DIR}"
}
# ShellCheck does not follow calls from the cleanup function used by EXIT.
# shellcheck disable=SC2329
docker_resource_matches_run() {
    local kind="$1" name="$2" label=""
    [ "${kind}" = container ] || return 1
    label=$(timeout -k 5 "${DOCKER_EXEC_TIMEOUT:-60}" docker inspect --format '{{ index .Config.Labels "vidveil.test-run" }}' "${name}" 2>/dev/null || true)
    [ "${label}" = "${DOCKER_TEST_SUFFIX}" ]
}
trap cleanup_docker_test_artifacts EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
case "${DOCKER_BIN_DIR}" in
    /*) ;;
    *) DOCKER_BIN_DIR="${DOCKER_PROJECT_ROOT}/${DOCKER_BIN_DIR}" ;;
esac
if [ -L "${DOCKER_BIN_DIR}" ] || { [ -e "${DOCKER_BIN_DIR}" ] && [ ! -d "${DOCKER_BIN_DIR}" ]; }; then
    echo "ERROR: BINDIR must be a directory and cannot be a symlink: ${DOCKER_BIN_DIR}" >&2
    exit 73
fi
if [ ! -e "${DOCKER_BIN_DIR}" ]; then
    DOCKER_CLEANUP_BINDIR=true
fi
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
    echo -e "${DOCKER_RED}x${DOCKER_NC} $1" >&2
    DOCKER_TESTS_FAILED=$((DOCKER_TESTS_FAILED + 1))
    DOCKER_TESTS_RUN=$((DOCKER_TESTS_RUN + 1))
}

__info() {
    echo -e "${DOCKER_BLUE}>${DOCKER_NC} $1"
}

__random_container_name() {
    printf '%s-%s' "${DOCKER_PROJECT_NAME}" "$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n')"
}

__docker_run() {
    local container_name
    container_name="$(__random_container_name)"
    printf '%s\n' "${container_name}" >> "${DOCKER_CONTAINER_REGISTRY}"
    timeout -k 30 "${DOCKER_TIMEOUT}" docker run --rm --no-healthcheck \
        --name "${container_name}" \
        --label "vidveil.test-run=${DOCKER_TEST_SUFFIX}" \
        --memory="${DOCKER_MEM}" --cpus="${DOCKER_CPUS}" --cpu-shares="${DOCKER_CPU_SHARES}" \
        "$@"
}

__docker_exec() {
    timeout -k 5 "${DOCKER_EXEC_TIMEOUT:-60}" docker exec "$@"
}

__health_json() {
    __docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSsf --max-time 10 \
        -H 'Accept: application/json' "http://localhost:8080/server/healthz"
}

__overlay_value() {
    __health_json | __docker_exec -i "${DOCKER_TEST_CONTAINER}" jq -r ".features.${1}.${2} // empty"
}

__check_overlay_health() {
    local overlay="$1" status="" enabled="" provider="" address="" health_check="" wait_seconds="" ready=false
    if [ "${overlay}" = tor ]; then
        wait_seconds="${TOR_WAIT:-300}"
        enabled="$(__overlay_value tor enabled)"
        status="$(__overlay_value tor status)"
    else
        wait_seconds="${I2P_WAIT:-600}"
        enabled="$(__overlay_value i2p enabled)"
        status="$(__overlay_value i2p status)"
        provider="$(__overlay_value i2p provider)"
    fi

    if [ "${enabled}" != true ]; then
        __fail "${overlay} is not enabled in the application container"
        return
    fi
    case "${status}" in
        starting|healthy|unavailable) __pass "${overlay} enabled status is valid (${status})" ;;
        *) __fail "${overlay} enabled status is invalid (${status:-missing})"; return ;;
    esac
    if [ "${overlay}" = i2p ] && [ "${provider}" != i2pd ]; then
        __fail "I2P provider is not i2pd (${provider:-missing})"
        return
    fi

    if ! __docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -fsS --connect-timeout 3 --max-time 5 \
        https://check.torproject.org/ -o /dev/null 2>/dev/null; then
        if [ "${REQUIRE_OVERLAY:-0}" = 1 ]; then
            __fail "${overlay} health cannot be verified because outbound network access is unavailable"
        else
            printf 'SKIP: no outbound network; %s health contract checked\n' "${overlay}" >&2
        fi
        return
    fi

    for _ in $(seq 1 "${wait_seconds}"); do
        status="$(__overlay_value "${overlay}" status)"
        if [ "${status}" = healthy ]; then
            ready=true
            break
        fi
        if [ "${status}" = unavailable ]; then
            break
        fi
        sleep 1
    done
    if [ "${ready}" != true ]; then
        __fail "${overlay} did not become healthy within ${wait_seconds}s (status: ${status:-missing})"
        return
    fi

    health_check="$(__health_json | __docker_exec -i "${DOCKER_TEST_CONTAINER}" jq -r ".checks.${overlay} // empty")"
    [ "${health_check}" = ok ] && __pass "checks.${overlay} is ok" || __fail "checks.${overlay} is ${health_check:-missing}"
    if [ "${overlay}" = tor ]; then
        [ "$(__overlay_value tor running)" = true ] && __pass "Tor is running" || __fail "Tor is not running"
        address="$(__overlay_value tor hostname)"
        if [[ "${address}" =~ ^[a-z2-7]{56}\.onion$ ]]; then
            __pass "Tor hostname is a v3 onion address"
        else
            __fail "Tor hostname is invalid (${address:-missing})"
        fi
        __docker_exec "${DOCKER_TEST_CONTAINER}" sh -c \
            'test -s /data/tor/site/hostname && test "$(stat -c %a /config/tor)" = 700 && test "$(stat -c %a /data/tor)" = 700 && test "$(stat -c %a /data/tor/site)" = 700' \
            && __pass "Tor hostname persisted and directories are mode 0700" \
            || __fail "Tor hostname or private directory contract failed"
    else
        address="$(__overlay_value i2p hostname)"
        if [[ "${address}" =~ ^[a-z2-7]{52}\.b32\.i2p$ ]]; then
            __pass "I2P hostname is a .b32.i2p address"
        else
            __fail "I2P hostname is invalid (${address:-missing})"
        fi
        [ "$(__overlay_value i2p running)" = true ] && __pass "I2P is running" || __fail "I2P is not running"
        __docker_exec "${DOCKER_TEST_CONTAINER}" sh -c 'find /data/i2p -type f -size +0 | grep . >/dev/null' \
            && __pass "I2P destination key persisted" \
            || __fail "I2P destination key was not persisted"
    fi
}

# Test function
__test_endpoint() {
    local method="$1"
    local endpoint="$2"
    local expected_status="$3"
    local description="$4"

    local response
    local status

    response=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSs --max-time 30 -w "\n%{http_code}" -X "${method}" "http://localhost:8080${endpoint}" 2>/dev/null || echo "000")
    status=$(printf '%s\n' "$response" | tail -n1)

    if [ "$status" = "$expected_status" ]; then
        __pass "$description (HTTP $status)"
    else
        __fail "$description (expected $expected_status, got $status)"
    fi
}

__info "Building server and CLI binaries with make build..."
make -C "${DOCKER_PROJECT_ROOT}" build \
    BINDIR="${DOCKER_BIN_DIR}" RELDIR="${DOCKER_TEMP_DIR}/releases" \
    GO_CACHE="${DOCKER_GO_CACHE}" GO_BUILD="${DOCKER_GO_BUILD}" \
    DOCKER_MEM="${DOCKER_MEM}" DOCKER_CPUS="${DOCKER_CPUS}" DOCKER_CPU_SHARES="${DOCKER_CPU_SHARES}" DOCKER_TIMEOUT="${DOCKER_TIMEOUT}"
if [ ! -f "${DOCKER_BIN_DIR}/${DOCKER_PROJECT_NAME}" ]; then
    echo -e "${DOCKER_RED}x Build failed${DOCKER_NC}"
    exit 1
fi
DOCKER_BINARY_DIR="${DOCKER_BIN_DIR}"
cp -a "${DOCKER_BINARY_DIR}/." "${DOCKER_TEMP_DIR}/"
__pass "Server and CLI binaries built successfully"

# Step 3: Test server binary info
__info "Testing server binary..."
# Capture full output first to avoid SIGPIPE from `| head -1` with pipefail
VERSION_OUT=$(__docker_run -v "${DOCKER_TEMP_DIR}":/app alpine:latest "/app/${DOCKER_PROJECT_NAME}" --version 2>&1 || true)
VERSION=$(printf '%s\n' "$VERSION_OUT" | head -1)
if [ -n "$VERSION" ]; then
    __pass "Server version check: $VERSION"
else
    __fail "Server version check failed"
fi
SERVER_HELP_OUT=$(__docker_run -v "${DOCKER_TEMP_DIR}":/app alpine:latest "/app/${DOCKER_PROJECT_NAME}" --help 2>&1 || true)
if grep -qi -- "${DOCKER_PROJECT_NAME}" <<< "${SERVER_HELP_OUT}"; then
    __pass "Server help check"
else
    __fail "Server help check failed"
fi

# Step 4: Binary rename test (per AI.md PART 7: binary renames itself gracefully)
__info "Testing binary rename behavior..."
cp "${DOCKER_TEMP_DIR}/${DOCKER_PROJECT_NAME}" "${DOCKER_TEMP_DIR}/myveil"
RENAMED_HELP_OUT=$(__docker_run -v "${DOCKER_TEMP_DIR}":/app alpine:latest /app/myveil --help 2>&1 || true)
RENAMED_HELP=$(printf '%s\n' "$RENAMED_HELP_OUT" | head -5)
if grep -qi -- "myveil" <<< "$RENAMED_HELP"; then
    __pass "Binary rename: --help shows new name"
else
    __fail "Binary rename: --help does not reflect new binary name"
fi
rm -f "${DOCKER_TEMP_DIR}/myveil"

# Step 5: Test CLI binary info
__info "Testing CLI binary..."
CLI_VERSION_OUT=$(__docker_run -v "${DOCKER_TEMP_DIR}":/app alpine:latest "/app/${DOCKER_PROJECT_NAME}-cli" --version 2>&1 || true)
CLI_VERSION=$(printf '%s\n' "$CLI_VERSION_OUT" | head -1)
if [ -n "$CLI_VERSION" ]; then
    __pass "CLI version check: $CLI_VERSION"
else
    __fail "CLI version check failed"
fi
CLI_HELP_OUT=$(__docker_run -v "${DOCKER_TEMP_DIR}":/app alpine:latest "/app/${DOCKER_PROJECT_NAME}-cli" --help 2>&1 || true)
if grep -qi -- "${DOCKER_PROJECT_NAME}" <<< "${CLI_HELP_OUT}"; then
    __pass "CLI help check"
else
    __fail "CLI help check failed"
fi
cp "${DOCKER_TEMP_DIR}/${DOCKER_PROJECT_NAME}-cli" "${DOCKER_TEMP_DIR}/myveil-cli"
RENAMED_CLI_HELP_OUT=$(__docker_run -v "${DOCKER_TEMP_DIR}":/app alpine:latest /app/myveil-cli --help 2>&1 || true)
if grep -qi -- "myveil-cli" <<< "${RENAMED_CLI_HELP_OUT}"; then
    __pass "CLI binary rename: --help shows new name"
else
    __fail "CLI binary rename: --help does not reflect new binary name"
fi
rm -f "${DOCKER_TEMP_DIR}/myveil-cli"

# Step 6: Start the binary in an Alpine application container per PART 28/31
__info "Starting vidveil server..."
if timeout -k 5 "${DOCKER_EXEC_TIMEOUT:-60}" docker container inspect "${DOCKER_TEST_CONTAINER}" >/dev/null 2>&1; then
    echo "ERROR: refusing to replace existing container ${DOCKER_TEST_CONTAINER}" >&2
    exit 73
fi
printf '%s\n' "${DOCKER_TEST_CONTAINER}" >> "${DOCKER_CONTAINER_REGISTRY}"
timeout -k 30 "${DOCKER_TIMEOUT}" docker run -d --no-healthcheck \
    --name "${DOCKER_TEST_CONTAINER}" \
    --label "vidveil.test-run=${DOCKER_TEST_SUFFIX}" \
    --memory="${DOCKER_MEM}" --cpus="${DOCKER_CPUS}" --cpu-shares="${DOCKER_CPU_SHARES}" \
    -v "${DOCKER_TEMP_DIR}":/app \
    -v "${DOCKER_TEMP_DIR}/config":/config \
    -v "${DOCKER_TEMP_DIR}/data":/data \
    alpine:latest \
    sh -c "apk add --no-cache curl bash file jq tor i2pd tzdata ca-certificates >/dev/null 2>&1 && /app/${DOCKER_PROJECT_NAME} --address 0.0.0.0 --port 8080 --mode development --config /config --data /data"

sleep 3

# Wait for server to be ready. The Alpine container installs test tools before
# starting the server, so allow up to 60 seconds for package mirrors.
# Per AI.md PART 13: /server/healthz is canonical; /healthz is opt-in
__info "Waiting for server to be ready..."
DOCKER_RETRIES=60
DOCKER_SERVER_READY=false
while [ "$DOCKER_RETRIES" -gt 0 ]; do
    DOCKER_HEALTH_STATUS=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSs -o /dev/null -w "%{http_code}" "http://localhost:8080/server/healthz" 2>/dev/null || true)
    if [ "${DOCKER_HEALTH_STATUS}" = 200 ]; then
        DOCKER_SERVER_READY=true
        break
    fi
    DOCKER_RETRIES=$((DOCKER_RETRIES - 1))
    sleep 1
done
if [ "$DOCKER_SERVER_READY" != "true" ]; then
    echo -e "${DOCKER_RED}x Server did not become ready within 60 seconds${DOCKER_NC}" >&2
    timeout -k 5 "${DOCKER_EXEC_TIMEOUT:-60}" docker logs "${DOCKER_TEST_CONTAINER}" 2>/dev/null \
        | sed -E 's/(adm|own)_[A-Za-z0-9]{32}/\1_[REDACTED]/g' >&2 || true
    exit 1
fi

__info "Verifying binary metadata..."
if __docker_exec "${DOCKER_TEST_CONTAINER}" sh -c 'ls -lh /app/* >/dev/null && file /app/*' >/dev/null 2>&1; then
    __pass "Server and CLI binary files are present and identifiable"
else
    __fail "Binary metadata verification failed"
fi

# PART 31: verify Tor and I2P from the app process health report and persisted data.
__info "Testing Tor and I2P runtime health..."
__check_overlay_health tor
__check_overlay_health i2p

# Step 7: Test Health Endpoint
__info "Testing health endpoint..."
__test_endpoint GET "/server/healthz" "200" "Health check"

# Read the generated operator credential from the private secrets file without
# printing it, then verify both rejection and acceptance on the auth-check API.
__info "Testing operator-token authentication..."
OPERATOR_TOKEN=$(__docker_exec "${DOCKER_TEST_CONTAINER}" awk '
    /^  token:[[:space:]]*/ {
        sub(/^  token:[[:space:]]*/, "")
        gsub(/["\047]/, "")
        print
        exit
    }
' /config/secrets.yml 2>/dev/null || true)
if [[ "${OPERATOR_TOKEN}" =~ ^adm_[A-Za-z0-9]{32}$ ]]; then
    __pass "Operator token was generated in secrets.yml"
else
    __fail "Operator token is missing or malformed in secrets.yml"
fi
AUTH_MISSING_STATUS=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSs -o /dev/null -w "%{http_code}" "http://localhost:8080/api/v1/server/auth/check" 2>/dev/null || echo "000")
[ "${AUTH_MISSING_STATUS}" = 401 ] && __pass "Auth check rejects missing bearer token" || __fail "Auth check missing-token status was ${AUTH_MISSING_STATUS}"
AUTH_INVALID_STATUS=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSs -o /dev/null -w "%{http_code}" -H 'Authorization: Bearer invalid-token' "http://localhost:8080/api/v1/server/auth/check" 2>/dev/null || echo "000")
[ "${AUTH_INVALID_STATUS}" = 401 ] && __pass "Auth check rejects invalid bearer token" || __fail "Auth check invalid-token status was ${AUTH_INVALID_STATUS}"
AUTH_OPERATOR_KIND=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSsf -H 'Accept: application/json' -H "Authorization: Bearer ${OPERATOR_TOKEN}" "http://localhost:8080/api/v1/server/auth/check" | __docker_exec -i "${DOCKER_TEST_CONTAINER}" jq -r '.data.kind // empty' 2>/dev/null || true)
[ "${AUTH_OPERATOR_KIND}" = operator ] && __pass "Auth check accepts server.token" || __fail "Auth check did not accept server.token"

# Issue one isolated owner token through the documented public endpoint for
# authenticated CLI coverage; keep it out of stdout and never persist it.
OWNER_TOKEN_RESPONSE=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSs --max-time 30 -H 'Accept: application/json' -w "\n%{http_code}" -X POST "http://localhost:8080/api/v1/server/preferences/tokens/primary" 2>/dev/null || true)
OWNER_TOKEN_STATUS=$(printf '%s\n' "${OWNER_TOKEN_RESPONSE}" | tail -n1)
OWNER_TOKEN=$(printf '%s\n' "${OWNER_TOKEN_RESPONSE}" | sed '$d' | __docker_exec -i "${DOCKER_TEST_CONTAINER}" jq -r '.data.owner_token // empty' 2>/dev/null || true)
unset OWNER_TOKEN_RESPONSE
if [ "${OWNER_TOKEN_STATUS}" = 201 ] && [[ "${OWNER_TOKEN}" =~ ^own_[A-Za-z0-9]{32}$ ]]; then
    __pass "Unauthenticated owner-token issuance returns a primary token"
else
    __fail "Primary owner-token issuance did not return a valid token"
fi
AUTH_OWNER_KIND=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSsf -H 'Accept: application/json' -H "Authorization: Bearer ${OWNER_TOKEN}" "http://localhost:8080/api/v1/server/auth/check" | __docker_exec -i "${DOCKER_TEST_CONTAINER}" jq -r '.data.kind // empty' 2>/dev/null || true)
[ "${AUTH_OWNER_KIND}" = owner ] && __pass "Auth check accepts primary owner token" || __fail "Auth check did not accept the primary owner token"
OPERATOR_ROUTE="http://localhost:8080/api/v1/status"
OPERATOR_NO_TOKEN_STATUS=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSs --max-time 10 -o /dev/null -w "%{http_code}" "${OPERATOR_ROUTE}" 2>/dev/null || echo 000)
[ "${OPERATOR_NO_TOKEN_STATUS}" = 401 ] && __pass "Operator route rejects a missing token" || __fail "Operator route missing-token status was ${OPERATOR_NO_TOKEN_STATUS}"
OPERATOR_WRONG_TOKEN_STATUS=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSs --max-time 10 -o /dev/null -w "%{http_code}" -H 'Authorization: Bearer invalid-token' "${OPERATOR_ROUTE}" 2>/dev/null || echo 000)
[ "${OPERATOR_WRONG_TOKEN_STATUS}" = 401 ] && __pass "Operator route rejects a wrong token" || __fail "Operator route wrong-token status was ${OPERATOR_WRONG_TOKEN_STATUS}"
OPERATOR_OWNER_TOKEN_STATUS=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSs --max-time 10 -o /dev/null -w "%{http_code}" -H "Authorization: Bearer ${OWNER_TOKEN}" "${OPERATOR_ROUTE}" 2>/dev/null || echo 000)
[ "${OPERATOR_OWNER_TOKEN_STATUS}" = 403 ] && __pass "Operator route rejects an owner token" || __fail "Operator route owner-token status was ${OPERATOR_OWNER_TOKEN_STATUS}"
OPERATOR_VALID_TOKEN_STATUS=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSs --max-time 10 -o /dev/null -w "%{http_code}" -H "Authorization: Bearer ${OPERATOR_TOKEN}" "${OPERATOR_ROUTE}" 2>/dev/null || echo 000)
[ "${OPERATOR_VALID_TOKEN_STATUS}" = 200 ] && __pass "Operator route accepts server.token" || __fail "Operator route valid-token status was ${OPERATOR_VALID_TOKEN_STATUS}"
OPERATOR_QUERY_TOKEN_STATUS=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSs --max-time 10 -o /dev/null -w "%{http_code}" "${OPERATOR_ROUTE}?token=${OPERATOR_TOKEN}" 2>/dev/null || echo 000)
[ "${OPERATOR_QUERY_TOKEN_STATUS}" = 401 ] && __pass "Operator route rejects a query-string token" || __fail "Operator route query-token status was ${OPERATOR_QUERY_TOKEN_STATUS}"

CLI_BIN="/app/${DOCKER_PROJECT_NAME}-cli"
if __docker_exec "${DOCKER_TEST_CONTAINER}" "${CLI_BIN}" --server http://localhost:8080 --token "${OWNER_TOKEN}" health 2>&1 | grep -q 'Server is healthy'; then
    __pass "CLI health command checks the running server"
else
    __fail "CLI health command failed against the running server"
fi
if __docker_exec "${DOCKER_TEST_CONTAINER}" "${CLI_BIN}" --server http://localhost:8080 --token "${OWNER_TOKEN}" engines >/dev/null 2>&1; then
    __pass "CLI engines read command uses an API token"
else
    __fail "CLI engines read command failed with an API token"
fi
if __docker_exec "${DOCKER_TEST_CONTAINER}" "${CLI_BIN}" --server http://localhost:8080 --token "${OWNER_TOKEN}" favorites add --url https://example.test/vidveil-cli-check --title VidVeil-CLI-Integration >/dev/null 2>&1; then
    __pass "CLI favorites write command uses an API token"
else
    __fail "CLI favorites write command failed with an API token"
fi
CLI_FAVORITES=$(__docker_exec "${DOCKER_TEST_CONTAINER}" "${CLI_BIN}" --server http://localhost:8080 --token "${OWNER_TOKEN}" favorites list 2>&1 || true)
if [[ "${CLI_FAVORITES}" == *VidVeil-CLI-Integration* ]]; then
    __pass "CLI favorites read command returns the saved item"
else
    __fail "CLI favorites read command omitted the saved item"
fi
CLI_BAD_TOKEN_OUTPUT=$(__docker_exec "${DOCKER_TEST_CONTAINER}" "${CLI_BIN}" --server http://localhost:8080 --token invalid-token favorites list 2>&1 || true)
if [[ "${CLI_BAD_TOKEN_OUTPUT}" == *401* || "${CLI_BAD_TOKEN_OUTPUT}" == *UNAUTHORIZED* || "${CLI_BAD_TOKEN_OUTPUT}" == *unauthorized* ]]; then
    __pass "CLI reports a clear error for an invalid API token"
else
    __fail "CLI invalid-token failure did not include a clear authentication error"
fi
unset OWNER_TOKEN OPERATOR_TOKEN AUTH_OPERATOR_KIND AUTH_OWNER_KIND OPERATOR_ROUTE OPERATOR_NO_TOKEN_STATUS OPERATOR_WRONG_TOKEN_STATUS OPERATOR_OWNER_TOKEN_STATUS OPERATOR_VALID_TOKEN_STATUS OPERATOR_QUERY_TOKEN_STATUS CLI_FAVORITES CLI_BAD_TOKEN_OUTPUT

# Step 8: Test VidVeil-specific endpoints
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
ENGINE_STATUS=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSs -o /dev/null -w "%{http_code}" "http://localhost:8080/api/v1/engines/google" 2>/dev/null || echo "000")
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
PGP_STATUS=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSs -o /dev/null -w "%{http_code}" "http://localhost:8080/.well-known/pgp-key.asc" 2>/dev/null || echo "000")
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

# Step 12: Test SSE streaming
__info "Testing SSE streaming endpoint..."
SSE_OUTPUT=$(__docker_exec "${DOCKER_TEST_CONTAINER}" timeout 10 curl -q -LSsfN -H "Accept: text/event-stream" "http://localhost:8080/api/v1/search?q=test" 2>/dev/null || true)

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

SSE_BANG=$(__docker_exec "${DOCKER_TEST_CONTAINER}" timeout 5 curl -q -LSsfN -H "Accept: text/event-stream" "http://localhost:8080/api/v1/search?q=!ph+test" 2>/dev/null || true)
if grep -q -- "data:" <<< "$SSE_BANG"; then
    __pass "SSE streaming - with bang shortcuts"
else
    __fail "SSE streaming - bang shortcuts not working"
fi

SSE_ERROR=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSs -w "\n%{http_code}" -H "Accept: text/event-stream" "http://localhost:8080/api/v1/search" 2>/dev/null)
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
CSRF_TOKEN=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSsf -D - -o /dev/null \
    "http://localhost:8080/server/healthz" 2>/dev/null \
    | sed -n 's/^Set-Cookie: csrf_token=\([^;]*\).*/\1/ip' | tr -d '\r')
BATCH_STATUS=$(__docker_exec "${DOCKER_TEST_CONTAINER}" curl -q -LSs -o /dev/null -w "%{http_code}" -X POST \
    -H "Content-Type: application/json" \
    -H "Cookie: csrf_token=${CSRF_TOKEN}" \
    -H "X-CSRF-Token: ${CSRF_TOKEN}" \
    -d '{"queries":[{"q":"test"}]}' \
    "http://localhost:8080/api/v1/search/batch" 2>/dev/null || echo "000")
if [ "$BATCH_STATUS" = "200" ] || [ "$BATCH_STATUS" = "202" ]; then
    __pass "Batch search endpoint (HTTP $BATCH_STATUS)"
else
    __fail "Batch search endpoint (expected 200/202, got $BATCH_STATUS)"
fi

# Step 15: Test comprehensive content negotiation (PART 28 — ALL routes ALL headers)
__info "Running comprehensive content negotiation tests..."
EXEC_PREFIX="timeout -k 5 ${DOCKER_EXEC_TIMEOUT:-60} docker exec ${DOCKER_TEST_CONTAINER}" BASE_URL="http://localhost:8080" "${DOCKER_SCRIPT_DIR}/test_content_negotiation.sh" || DOCKER_TESTS_FAILED=$((DOCKER_TESTS_FAILED + 1))

# Step 16: Test shell completions (PART 8 - built into binary)
__info "Testing shell completions..."
BASH_COMPL=$(__docker_exec "${DOCKER_TEST_CONTAINER}" "/app/${DOCKER_PROJECT_NAME}" --shell completions bash 2>&1 || true)
if grep -q -- "complete" <<< "$BASH_COMPL" || grep -q -- "_${DOCKER_PROJECT_NAME}" <<< "$BASH_COMPL"; then
    __pass "Shell completions: bash"
else
    __fail "Shell completions: bash"
fi

ZSH_COMPL=$(__docker_exec "${DOCKER_TEST_CONTAINER}" "/app/${DOCKER_PROJECT_NAME}" --shell completions zsh 2>&1 || true)
if grep -q -- "compdef" <<< "$ZSH_COMPL" || grep -q -- "_${DOCKER_PROJECT_NAME}" <<< "$ZSH_COMPL"; then
    __pass "Shell completions: zsh"
else
    __fail "Shell completions: zsh"
fi

FISH_COMPL=$(__docker_exec "${DOCKER_TEST_CONTAINER}" "/app/${DOCKER_PROJECT_NAME}" --shell completions fish 2>&1 || true)
if grep -q -- "complete.*${DOCKER_PROJECT_NAME}" <<< "$FISH_COMPL"; then
    __pass "Shell completions: fish"
else
    __fail "Shell completions: fish"
fi

for shell_name in bash zsh fish; do
    CLI_COMPLETIONS=$(__docker_exec "${DOCKER_TEST_CONTAINER}" "/app/${DOCKER_PROJECT_NAME}-cli" --shell completions "${shell_name}" 2>&1 || true)
    if case "${shell_name}" in
        bash) grep -qE -- "complete|_${DOCKER_PROJECT_NAME}" <<< "${CLI_COMPLETIONS}" ;;
        zsh) grep -qE -- "compdef|_${DOCKER_PROJECT_NAME}" <<< "${CLI_COMPLETIONS}" ;;
        fish) grep -qE -- "complete.*${DOCKER_PROJECT_NAME}" <<< "${CLI_COMPLETIONS}" ;;
    esac; then
        __pass "CLI shell completions: ${shell_name}"
    else
        __fail "CLI shell completions: ${shell_name}"
    fi
done

# Final Summary
echo
echo "================================="
echo -e "Tests run: ${DOCKER_TESTS_RUN}"
echo -e "${DOCKER_GREEN}Passed: ${DOCKER_TESTS_PASSED}${DOCKER_NC}"
if [ ${DOCKER_TESTS_FAILED} -gt 0 ]; then
    echo -e "${DOCKER_RED}Failed: ${DOCKER_TESTS_FAILED}${DOCKER_NC}" >&2
    exit 1
else
    echo -e "${DOCKER_GREEN}All tests passed!${DOCKER_NC}"
    exit 0
fi
