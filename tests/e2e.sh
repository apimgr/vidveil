#!/usr/bin/env bash
# @@License : WTFPL
# Vidveil Browser E2E Suite - Headless Chromium (chromedp)
# Per AI.md PART 28: Browser E2E Testing (on-demand, NEVER the commit gate)

set -eo pipefail

E2E_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_PROJECT_ROOT="$(cd "${E2E_SCRIPT_DIR}/.." && pwd)"
E2E_PROJECT_NAME="vidveil"
E2E_PROJECT_ORG="apimgr"
E2E_INTERNAL_NAME="vidveil"

E2E_GO_IMAGE="casjaysdev/go:latest"
E2E_CHROME_IMAGE="chromedp/headless-shell:latest"
E2E_NETWORK="${E2E_PROJECT_NAME}-e2e"
E2E_CHROME_CONTAINER="${E2E_PROJECT_NAME}-e2e-chrome"
E2E_GO_CONTAINER="${E2E_PROJECT_NAME}-e2e-runner"

# Failure artifacts and caches live outside the project tree (AI.md PART 29)
mkdir -p "${TMPDIR:-/tmp}/${E2E_PROJECT_ORG}"
E2E_TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/${E2E_PROJECT_ORG}/${E2E_INTERNAL_NAME}-XXXXXX")"

E2E_GO_CACHE="${HOME}/go/pkg/mod"
E2E_GO_BUILD="${HOME}/.cache/go-build/${E2E_PROJECT_NAME}"
mkdir -p "${E2E_GO_CACHE}" "${E2E_GO_BUILD}" "${E2E_TEMP_DIR}/artifacts"

__cleanup() {
  docker rm -f "${E2E_GO_CONTAINER}" >/dev/null 2>&1 || true
  docker rm -f "${E2E_CHROME_CONTAINER}" >/dev/null 2>&1 || true
  docker network rm "${E2E_NETWORK}" >/dev/null 2>&1 || true
  if [ "${E2E_KEEP_ARTIFACTS:-0}" = "1" ]; then
    printf 'Artifacts kept in %s\n' "${E2E_TEMP_DIR}"
  else
    rm -rf "${E2E_TEMP_DIR}"
  fi
}
trap __cleanup EXIT INT TERM

printf '🧪 Vidveil Browser E2E Suite\n'
printf '============================\n\n'

if ! command -v docker >/dev/null 2>&1; then
  printf '✗ docker is required for the E2E suite\n' >&2
  exit 1
fi

printf '→ Pulling images\n'
docker pull -q "${E2E_GO_IMAGE}" >/dev/null
docker pull -q "${E2E_CHROME_IMAGE}" >/dev/null

printf '→ Creating isolated network %s\n' "${E2E_NETWORK}"
docker network rm "${E2E_NETWORK}" >/dev/null 2>&1 || true
docker network create "${E2E_NETWORK}" >/dev/null

printf '→ Starting headless Chromium sidecar\n'
docker run -d --rm \
  --name "${E2E_CHROME_CONTAINER}" \
  --network "${E2E_NETWORK}" \
  --shm-size=1g \
  "${E2E_CHROME_IMAGE}" \
  --no-sandbox \
  --disable-gpu \
  --remote-debugging-address=0.0.0.0 \
  --remote-debugging-port=9222 >/dev/null

printf '→ Running go test -tags e2e ./tests/e2e/...\n\n'
set +e
docker run --rm \
  --name "${E2E_GO_CONTAINER}" \
  --hostname "${E2E_GO_CONTAINER}" \
  --network "${E2E_NETWORK}" \
  -v "${E2E_PROJECT_ROOT}:/app" \
  -v "${E2E_GO_CACHE}:/usr/local/share/go/pkg/mod" \
  -v "${E2E_GO_BUILD}:/usr/local/share/go/cache" \
  -v "${E2E_TEMP_DIR}:/e2e" \
  -w /app \
  -e CGO_ENABLED=0 \
  -e GOFLAGS=-buildvcs=false \
  -e GOPATH=/usr/local/share/go \
  -e GOCACHE=/usr/local/share/go/cache \
  -e E2E_CHROME_URL="http://${E2E_CHROME_CONTAINER}:9222" \
  -e E2E_SERVER_HOST="${E2E_GO_CONTAINER}" \
  -e E2E_ARTIFACT_DIR=/e2e/artifacts \
  -e E2E_WORK_DIR=/e2e \
  "${E2E_GO_IMAGE}" \
  go test -tags e2e -count=1 -timeout 20m -v ./tests/e2e/...
E2E_STATUS=$?
set -e

printf '\n'
if [ "${E2E_STATUS}" -eq 0 ]; then
  printf '✅ Browser E2E suite passed\n'
else
  printf '❌ Browser E2E suite failed (exit %s)\n' "${E2E_STATUS}"
  printf 'Re-run with E2E_KEEP_ARTIFACTS=1 to preserve screenshots and page HTML\n'
fi

exit "${E2E_STATUS}"
