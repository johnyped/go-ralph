#!/usr/bin/env bash
# reset.sh — restore ralph-test to its initial state for a fresh integration test run
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"

echo "→ Resetting issue checkboxes..."
# Replace [x] back to [ ] in all issue files
for f in "$DIR/issues/"*.md; do
  sed -i '' 's/- \[x\]/- [ ]/g' "$f"
done

echo "→ Removing generated source files..."
rm -f "$DIR/go.mod" "$DIR/go.sum" "$DIR/main.go"
rm -rf "$DIR/hello"

echo "→ Removing ralph state..."
rm -rf "$DIR/.ralph"

echo "✓ Reset complete. Run: go-ralph init ralph-test --dir $DIR"
