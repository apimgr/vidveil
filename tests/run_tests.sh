#!/usr/bin/env bash
# @@License : WTFPL
# Vidveil Integration Tests - Docker basic tier
# Per AI.md PART 28: Testing & Development

set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if command -v docker >/dev/null 2>&1; then
    echo "Docker detected - running the basic tests..."
    exec "${SCRIPT_DIR}/docker.sh"
fi

echo "ERROR: docker not found: every test script builds with Docker" >&2
echo "Install Docker: https://docker.com/" >&2
exit 69
