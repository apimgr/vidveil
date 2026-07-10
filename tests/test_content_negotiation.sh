#!/usr/bin/env bash
# @@License : WTFPL
# Vidveil Content Negotiation Tests
# Per AI.md PART 28: Every route tested with ALL applicable Accept headers
# Usage: BASE_URL=http://localhost:8080 EXEC_PREFIX="docker exec container" ./tests/test_content_negotiation.sh

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
# EXEC_PREFIX: prefix before curl for remote exec (e.g. "docker exec vidveil-test" or "incus exec instance --")
EXEC_PREFIX="${EXEC_PREFIX:-}"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

pass() {
    echo -e "  ${GREEN}+${NC} $1"
    TESTS_PASSED=$((TESTS_PASSED + 1))
    TESTS_RUN=$((TESTS_RUN + 1))
}

fail() {
    echo -e "  ${RED}x${NC} $1"
    TESTS_FAILED=$((TESTS_FAILED + 1))
    TESTS_RUN=$((TESTS_RUN + 1))
}

info() {
    echo -e "${BLUE}>${NC} $1"
}

# Run curl with optional exec prefix
run_curl() {
    if [ -n "$EXEC_PREFIX" ]; then
        $EXEC_PREFIX curl "$@" 2>/dev/null
    else
        curl "$@" 2>/dev/null
    fi
}

# Test an endpoint returns 2xx with a given Accept header
test_accept() {
    local description="$1"
    local accept="$2"
    local url="${BASE_URL}$3"
    local extra_args="${4:-}"

    local status
    # shellcheck disable=SC2086
    status=$(run_curl -s -o /dev/null -w "%{http_code}" -H "Accept: ${accept}" ${extra_args} "${url}" || echo "000")
    case "$status" in
        2[0-9][0-9]|3[0-9][0-9])
            pass "${description} [Accept: ${accept}] → ${status}"
            ;;
        *)
            fail "${description} [Accept: ${accept}] → ${status} (expected 2xx)"
            ;;
    esac
}

# Test a .txt URL returns 2xx
test_txt() {
    local description="$1"
    local path="$2"

    local status
    status=$(run_curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}${path}" || echo "000")
    case "$status" in
        2[0-9][0-9]|3[0-9][0-9])
            pass "${description} → ${status}"
            ;;
        *)
            fail "${description} → ${status} (expected 2xx)"
            ;;
    esac
}

# Test that response body contains a pattern
test_body() {
    local description="$1"
    local accept="$2"
    local path="$3"
    local pattern="$4"
    local extra_args="${5:-}"

    local body
    # shellcheck disable=SC2086
    body=$(run_curl -s -H "Accept: ${accept}" ${extra_args} "${BASE_URL}${path}" || echo "")
    if echo "$body" | grep -qi -- "$pattern"; then
        pass "${description} [Accept: ${accept}] body matches '${pattern}'"
    else
        fail "${description} [Accept: ${accept}] body does not match '${pattern}' (got: ${body:0:100})"
    fi
}

echo "Vidveil Content Negotiation Tests"
echo "=================================="
echo "Base URL: ${BASE_URL}"
echo

# ---------------------------------------------------------------------------
# SECTION 1: Frontend routes — text/html + text/plain required per PART 28
# ---------------------------------------------------------------------------
info "Frontend routes — text/html and text/plain"

BROWSER_UA="-H 'User-Agent: Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36'"
BROWSER_FLAGS="-L -H 'User-Agent: Mozilla/5.0'"

test_body   "/"           "text/html"  "/" "<!DOCTYPE html\|<html" "-L -H 'User-Agent: Mozilla/5.0'"
test_accept "/"           "text/plain" "/"
test_body   "/search"     "text/html"  "/search?q=test" "<!DOCTYPE html\|<html" "-L -H 'User-Agent: Mozilla/5.0'"
test_accept "/search"     "text/plain" "/search?q=test"
test_accept "/preferences"   "text/html"  "/preferences" "-L -H 'User-Agent: Mozilla/5.0'"
test_accept "/preferences"   "text/plain" "/preferences"
test_accept "/favorites"     "text/html"  "/favorites" "-L -H 'User-Agent: Mozilla/5.0'"
test_accept "/favorites"     "text/plain" "/favorites"
test_accept "/age-verify"    "text/html"  "/age-verify" "-L"
test_accept "/age-verify"    "text/plain" "/age-verify"
test_accept "/content-restricted" "text/html"  "/content-restricted" "-L"
test_accept "/content-restricted" "text/plain" "/content-restricted"
test_accept "/server/about"  "text/html"  "/server/about" "-L -H 'User-Agent: Mozilla/5.0'"
test_accept "/server/about"  "text/plain" "/server/about"
test_accept "/server/privacy" "text/html"  "/server/privacy" "-L -H 'User-Agent: Mozilla/5.0'"
test_accept "/server/privacy" "text/plain" "/server/privacy"
test_accept "/server/contact" "text/html"  "/server/contact" "-L -H 'User-Agent: Mozilla/5.0'"
test_accept "/server/contact" "text/plain" "/server/contact"
test_accept "/server/help"   "text/html"  "/server/help" "-L -H 'User-Agent: Mozilla/5.0'"
test_accept "/server/help"   "text/plain" "/server/help"
test_accept "/offline.html"  "text/html"  "/offline.html" "-L"
test_accept "/offline.html"  "text/plain" "/offline.html"

echo

# ---------------------------------------------------------------------------
# SECTION 2: API routes — application/json + text/plain required per PART 28
# ---------------------------------------------------------------------------
info "API routes — application/json and text/plain"

test_body   "/api/v1/engines"  "application/json" "/api/v1/engines" '"ok"'
test_accept "/api/v1/engines"  "text/plain"        "/api/v1/engines"
test_body   "/api/v1/bangs"    "application/json" "/api/v1/bangs"   '"ok"'
test_accept "/api/v1/bangs"    "text/plain"        "/api/v1/bangs"
test_body   "/api/v1/bangs/autocomplete" "application/json" "/api/v1/bangs/autocomplete?q=!p" '"ok"'
test_accept "/api/v1/bangs/autocomplete" "text/plain"        "/api/v1/bangs/autocomplete?q=!p"
test_body   "/api/v1/stats"    "application/json" "/api/v1/stats"   '"ok"'
test_accept "/api/v1/stats"    "text/plain"        "/api/v1/stats"
test_body   "/api/v1/version"  "application/json" "/api/v1/version" '"version"\|"ok"'
test_accept "/api/v1/version"  "text/plain"        "/api/v1/version"
test_body   "/api/v1/server/healthz" "application/json" "/api/v1/server/healthz" '"status"\|"ok"'
test_accept "/api/v1/server/healthz" "text/plain"        "/api/v1/server/healthz"
test_body   "/api/v1/server/about"   "application/json" "/api/v1/server/about"   '"ok"'
test_accept "/api/v1/server/about"   "text/plain"        "/api/v1/server/about"
test_body   "/api/v1/server/privacy" "application/json" "/api/v1/server/privacy" '"ok"'
test_accept "/api/v1/server/privacy" "text/plain"        "/api/v1/server/privacy"
test_body   "/api/v1/server/help"    "application/json" "/api/v1/server/help"    '"ok"'
test_accept "/api/v1/server/help"    "text/plain"        "/api/v1/server/help"
test_body   "/api/healthz"   "application/json" "/api/healthz"   '"status"\|"ok"'
test_accept "/api/healthz"   "text/plain"        "/api/healthz"
test_body   "/server/healthz" "application/json" "/server/healthz" '"status"\|"ok"'
test_accept "/server/healthz" "text/plain"        "/server/healthz"
test_body   "/api/autodiscover" "application/json" "/api/autodiscover" '"ok"\|"api"'
test_accept "/api/autodiscover" "text/plain"         "/api/autodiscover"

echo

# ---------------------------------------------------------------------------
# SECTION 3: .txt endpoints — plain text required per PART 28
# ---------------------------------------------------------------------------
info ".txt endpoints and well-known files"

test_txt "robots.txt"                   "/robots.txt"
test_txt "humans.txt"                   "/humans.txt"
test_txt "security.txt"                 "/.well-known/security.txt"
test_txt ".well-known/pgp-key.asc"      "/.well-known/pgp-key.asc"
test_txt ".well-known/change-password"  "/.well-known/change-password"
test_txt ".well-known/vidveil.json"     "/.well-known/vidveil.json"
test_txt "/server/healthz.txt"          "/server/healthz.txt"
test_txt "/server/healthz.json"         "/server/healthz.json"
test_txt "/api/v1/engines.txt"          "/api/v1/engines.txt"
test_txt "/api/v1/bangs.txt"            "/api/v1/bangs.txt"
test_txt "/api/v1/stats.txt"            "/api/v1/stats.txt"
test_txt "/api/v1/search.txt"           "/api/v1/search.txt?q=test"
test_txt "/manifest.json"               "/manifest.json"
test_txt "/sw.js"                       "/sw.js"
test_txt "/sitemap.xml"                 "/sitemap.xml"

echo

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo "=================================="
echo "Content Negotiation Results"
echo -e "Tests run:    ${TESTS_RUN}"
echo -e "${GREEN}Passed:       ${TESTS_PASSED}${NC}"
if [ "${TESTS_FAILED}" -gt 0 ]; then
    echo -e "${RED}Failed:       ${TESTS_FAILED}${NC}"
    exit 1
else
    echo -e "${GREEN}All content negotiation tests passed!${NC}"
    exit 0
fi
