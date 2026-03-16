#!/usr/bin/env bash
# analyze-ts-structure.sh
#
# Summarize the public API and structural shape of a TypeScript or JavaScript
# file so an agent can quickly identify candidate extraction points before
# deciding how to split or refactor it.
#
# Usage:
#   ./scripts/analyze-ts-structure.sh src/api/userService.ts
#
# The output is intentionally terse so it fits in an agent's context window.
# Feed this output to the agent alongside the failing ls-lint message.

set -euo pipefail

FILE="${1:-}"

if [[ -z "$FILE" ]]; then
  echo "Usage: $0 <file.ts|.tsx|.js|.jsx>" >&2
  exit 1
fi

if [[ ! -f "$FILE" ]]; then
  echo "Error: file not found: $FILE" >&2
  exit 1
fi

LINE_COUNT=$(wc -l < "$FILE")

echo "=== Structure: $FILE ($LINE_COUNT lines) ==="
echo ""

echo "--- Public exports ---"
grep -En "^export (default |abstract )?(class|function|const|let|type|interface|enum|async function)" "$FILE" \
  || echo "  (none found)"
echo ""

echo "--- Named exports (re-exports / barrel) ---"
grep -En "^export \{" "$FILE" \
  || echo "  (none found)"
echo ""

echo "--- Class definitions ---"
grep -En "^(export )?(default )?(abstract )?class " "$FILE" \
  || echo "  (none found)"
echo ""

echo "--- Top-level functions ---"
grep -En "^(export )?(async )?function " "$FILE" \
  || echo "  (none found)"
echo ""

echo "--- Top-level constants (may be objects, closures, components) ---"
grep -En "^(export )?const [A-Za-z]" "$FILE" | head -20 \
  || echo "  (none found)"
echo ""

echo "--- Type and interface declarations ---"
grep -En "^(export )?(type|interface) [A-Z]" "$FILE" \
  || echo "  (none found)"
echo ""

echo "--- Approximate section boundaries (blank-line-separated blocks) ---"
awk 'NR==1 || /^$/{block_start=NR+1} /^(export|class|function|const|interface|type|enum)/{print NR": "$0}' "$FILE" \
  | head -30
