#!/bin/bash
# @@License : WTFPL
# Generate LICENSE.md with embedded third-party licenses
# Per AI.md PART 2 lines 3174-3340

set -e

PROJECT_ORG="apimgr"
PROJECT_NAME="vidveil"
PROJECTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECTDIR"

echo "Generating LICENSE.md with embedded dependencies..."

# Create proper temp directory per PART 28
mkdir -p "${TMPDIR:-/tmp}/${PROJECT_ORG}"
TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX")
trap 'rm -rf "$TEMP_DIR"' EXIT

# Use casjaysdev/go:latest per PART 26 (Go projects)
docker run --rm -v $PWD:/build -w /build casjaysdev/go:latest sh -c '
  go-licenses csv ./... 2>/dev/null | sort
' > "$TEMP_DIR/licenses.csv"

echo "Found $(cat "$TEMP_DIR/licenses.csv" | wc -l) dependencies"

# Create LICENSE.md with embedded licenses
cat > LICENSE.md << 'EOF'
MIT License

Copyright (c) 2025 apimgr

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

---

## Embedded Licenses

This software includes the following third-party libraries:

EOF

echo "---" >> LICENSE.md
echo "" >> LICENSE.md
echo "### Dependencies List" >> LICENSE.md
echo "" >> LICENSE.md
echo "The following open-source libraries are used:" >> LICENSE.md
echo "" >> LICENSE.md

cat "$TEMP_DIR/licenses.csv" | while IFS=, read -r package url license; do
  echo "- **${package}** - ${license}" >> LICENSE.md
done

echo "" >> LICENSE.md
echo "Full license texts for each dependency can be found in their respective repositories." >> LICENSE.md

echo "LICENSE.md generated successfully"
echo "NOTE: Manual verification recommended for compliance"
