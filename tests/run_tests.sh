#!/usr/bin/env bash
# @@License : WTFPL
# Vidveil Integration Tests - Auto-detect Runtime
# Per AI.md PART 29: Testing & Development

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "🧪 Vidveil Integration Test Suite"
echo "================================="
echo

# Auto-detect runtime
if command -v incus &>/dev/null; then
    echo "✓ Incus detected - running full systemd tests"
    exec "${SCRIPT_DIR}/incus.sh"
elif command -v docker &>/dev/null; then
    echo "✓ Docker detected - running container tests"
    exec "${SCRIPT_DIR}/docker.sh"
else
    echo "${RED}✗ Neither Incus nor Docker found${NC}"
    echo "Please install Docker or Incus to run integration tests"
    exit 1
fi
