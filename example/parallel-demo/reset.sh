#!/usr/bin/env bash
set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"

echo "→ Resetting issue checkboxes..."
for f in "$DIR/issues/"*.md; do
  sed -i '' 's/- \[x\]/- [ ]/g' "$f"
done

echo "→ Removing generated source files..."
rm -f "$DIR/go.mod" "$DIR/go.sum" "$DIR/main.go"
rm -rf "$DIR/calc"

echo "→ Removing ralph state..."
rm -rf "$DIR/.ralph"

echo "✓ Reset complete."
echo "  Sequential: go-ralph init parallel-demo --dir $DIR --parallel 1"
echo "  Parallel:   go-ralph init parallel-demo --dir $DIR --parallel 2"
