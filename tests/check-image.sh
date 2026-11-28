#!/bin/sh
# @@License : WTFPL
# Validate a built container image against AI.md PART 26.

set -eu

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 IMAGE_REF" >&2
  exit 2
fi

if ! command -v docker >/dev/null 2>&1 || ! command -v curl >/dev/null 2>&1; then
  echo "ERROR: docker and curl are required to check the image" >&2
  exit 69
fi

CHECK_IMAGE_REF="$1"
CHECK_IMAGE_SUFFIX="$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n')"
CHECK_PACKAGE_CONTAINER="vidveil-${CHECK_IMAGE_SUFFIX}"
CHECK_HEALTH_CONTAINER="vidveil-$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n')"

docker pull "$CHECK_IMAGE_REF"
docker run --rm --name "$CHECK_PACKAGE_CONTAINER" --entrypoint sh "$CHECK_IMAGE_REF" -c \
  'for c in bash curl tor i2pd tini; do command -v "$c" >/dev/null || exit 1; done; test -s /etc/ssl/certs/ca-certificates.crt; test -f /usr/share/zoneinfo/America/New_York'

trap 'docker rm -f "$CHECK_HEALTH_CONTAINER" >/dev/null 2>&1 || true' EXIT
docker run -d --name "$CHECK_HEALTH_CONTAINER" -p '127.0.0.1::80' "$CHECK_IMAGE_REF" >/dev/null
CHECK_HEALTH_PORT="$(docker port "$CHECK_HEALTH_CONTAINER" 80/tcp | sed 's/.*://')"
trap 'exit 130' INT
trap 'exit 143' TERM

CHECK_OUTBOUND=true
if ! curl -q -LSsf --connect-timeout 3 --max-time 5 https://check.torproject.org/ -o /dev/null 2>/dev/null; then
  CHECK_OUTBOUND=false
fi
CHECK_LAST_STATUS=""
CHECK_ATTEMPT=0
while [ "$CHECK_ATTEMPT" -lt 180 ]; do
  CHECK_ATTEMPT=$((CHECK_ATTEMPT + 1))
  CHECK_HEALTH_JSON="$(curl -q -LSsf --connect-timeout 2 --max-time 5 -H 'Accept: application/json' \
    "http://127.0.0.1:${CHECK_HEALTH_PORT}/api/v1/server/healthz" 2>/dev/null || true)"
  CHECK_LAST_STATUS="$(printf '%s' "$CHECK_HEALTH_JSON" | sed -n 's/.*"tor"[[:space:]]*:[[:space:]]*{[^}]*"status"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
  if [ "$CHECK_LAST_STATUS" = healthy ]; then
    echo "Image Tor health check passed: $CHECK_IMAGE_REF"
    exit 0
  fi
  if [ "$CHECK_OUTBOUND" = false ] && [ "${REQUIRE_OVERLAY:-0}" != 1 ] && { [ "$CHECK_LAST_STATUS" = starting ] || [ "$CHECK_LAST_STATUS" = unavailable ]; }; then
    echo "SKIP: no outbound network; Tor status is $CHECK_LAST_STATUS" >&2
    exit 0
  fi
  sleep 1
done
echo "ERROR: Tor did not become healthy within 180 seconds (status: ${CHECK_LAST_STATUS:-missing})" >&2
exit 1
