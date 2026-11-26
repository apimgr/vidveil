#!/usr/bin/env bash
# @@License : WTFPL
# Vidveil Browser E2E Suite - Headless Chromium (chromedp)
# Per AI.md PART 28: Browser E2E Testing (on-demand, NEVER the commit gate)

set -eo pipefail

E2E_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_PROJECT_ROOT="$(cd "${E2E_SCRIPT_DIR}/.." && pwd)"
E2E_REMOTE_URL="$(git -C "${E2E_PROJECT_ROOT}" remote get-url origin 2>/dev/null || true)"
E2E_PROJECT_NAME="${E2E_REMOTE_URL##*/}"
E2E_PROJECT_NAME="${E2E_PROJECT_NAME%.git}"
E2E_PROJECT_ORG="$(printf '%s' "${E2E_REMOTE_URL%/*}" | sed -E 's|.*[:/]([^/:]+)$|\1|')"
if [ -z "${E2E_REMOTE_URL}" ]; then
  E2E_PROJECT_NAME="$(basename "${E2E_PROJECT_ROOT}")"
  E2E_PROJECT_ORG="$(basename "$(dirname "${E2E_PROJECT_ROOT}")")"
fi
E2E_INTERNAL_NAME="$(awk '$1 == "internal_name:" {print $2; exit}' "${E2E_PROJECT_ROOT}/IDEA.md")"
E2E_INTERNAL_NAME="${E2E_INTERNAL_NAME:-${E2E_PROJECT_NAME}}"

E2E_GO_IMAGE="casjaysdev/go:latest"
E2E_CHROME_IMAGE="chromedp/headless-shell:latest"
E2E_RUN_ID="$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n')"
E2E_NETWORK="${E2E_PROJECT_NAME}-${E2E_RUN_ID}-e2e"
E2E_CHROME_CONTAINER="${E2E_PROJECT_NAME}-${E2E_RUN_ID}-e2e-chrome"
E2E_GO_CONTAINER="${E2E_PROJECT_NAME}-${E2E_RUN_ID}-e2e-runner"
E2E_TEST_FILTER="${E2E_TEST_FILTER:-}"

# Failure artifacts live in the project-scoped temporary directory (AI.md PART 29).
# Go module and build caches are persistent host caches mounted into the runner.
mkdir -p "${TMPDIR:-/tmp}/${E2E_PROJECT_ORG}"
E2E_TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/${E2E_PROJECT_ORG}/${E2E_INTERNAL_NAME}-XXXXXX")"

E2E_GO_CACHE="${GO_CACHE:-${HOME}/go/pkg/mod}"
E2E_GO_BUILD="${GO_BUILD:-${HOME}/.cache/go-build/${E2E_PROJECT_NAME}}"
E2E_TIMEOUT="${E2E_TIMEOUT:-1500}"
E2E_GO_MEM="${E2E_GO_MEM:-4g}"
E2E_GO_CPUS="${E2E_GO_CPUS:-2}"
E2E_CHROME_MEM="${E2E_CHROME_MEM:-2g}"
E2E_CHROME_CPUS="${E2E_CHROME_CPUS:-2}"
mkdir -p "${E2E_GO_CACHE}" "${E2E_GO_BUILD}" "${E2E_TEMP_DIR}/artifacts"

# ShellCheck does not count the indirect EXIT/INT/TERM trap invocation.
# shellcheck disable=SC2329
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
timeout -k 30 600 docker pull -q "${E2E_GO_IMAGE}" >/dev/null
timeout -k 30 600 docker pull -q "${E2E_CHROME_IMAGE}" >/dev/null

printf '→ Creating isolated network %s\n' "${E2E_NETWORK}"
timeout -k 30 60 docker network create "${E2E_NETWORK}" >/dev/null

printf '→ Starting headless Chromium sidecar\n'
timeout -k 30 60 docker run -d --rm \
  --name "${E2E_CHROME_CONTAINER}" \
  --network "${E2E_NETWORK}" \
  --memory="${E2E_CHROME_MEM}" \
  --cpus="${E2E_CHROME_CPUS}" \
  --shm-size=1g \
  "${E2E_CHROME_IMAGE}" >/dev/null

printf '→ Starting E2E Go runner\n'
timeout -k 30 60 docker run -d --rm \
  --name "${E2E_GO_CONTAINER}" \
  --hostname "${E2E_GO_CONTAINER}" \
  --network "${E2E_NETWORK}" \
  --memory="${E2E_GO_MEM}" \
  --cpus="${E2E_GO_CPUS}" \
  -v "${E2E_PROJECT_ROOT}:/app" \
  -v "${E2E_GO_CACHE}:/usr/local/share/go/pkg/mod" \
  -v "${E2E_GO_BUILD}:/usr/local/share/go/cache" \
  -v "${E2E_TEMP_DIR}:/e2e" \
  -w /app \
  -e CGO_ENABLED=0 \
  -e GOFLAGS=-buildvcs=false \
  -e GOPATH=/usr/local/share/go \
  -e GOCACHE=/usr/local/share/go/cache \
  -e E2E_ARTIFACT_DIR=/e2e/artifacts \
  -e E2E_WORK_DIR=/e2e \
  "${E2E_GO_IMAGE}" \
  sh -c 'while :; do sleep 3600; done' >/dev/null

# The server drops to its dedicated uid. Give it path traversal through the
# private temp root without allowing directory listing; its runtime subdirs
# retain their owner-only permissions.
timeout -k 30 60 docker exec "${E2E_GO_CONTAINER}" chmod 0711 /e2e

E2E_CHROME_IP="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${E2E_CHROME_CONTAINER}")"
E2E_GO_IP="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${E2E_GO_CONTAINER}")"
printf '→ Running go test -tags e2e ./tests/e2e/...\n\n'
set +e
if [ -n "${E2E_TEST_FILTER}" ]; then
  timeout -k 30 "${E2E_TIMEOUT}" docker exec \
    -e "E2E_CHROME_URL=http://${E2E_CHROME_IP}:9222" \
    -e "E2E_SERVER_HOST=${E2E_GO_IP}" \
    "${E2E_GO_CONTAINER}" \
    go test -tags e2e -count=1 -timeout 20m -v -run "${E2E_TEST_FILTER}" ./tests/e2e/...
else
  timeout -k 30 "${E2E_TIMEOUT}" docker exec \
    -e "E2E_CHROME_URL=http://${E2E_CHROME_IP}:9222" \
    -e "E2E_SERVER_HOST=${E2E_GO_IP}" \
    "${E2E_GO_CONTAINER}" \
    go test -tags e2e -count=1 -timeout 20m -v ./tests/e2e/...
fi
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
