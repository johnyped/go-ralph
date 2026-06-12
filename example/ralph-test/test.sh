#!/usr/bin/env bash
# test.sh — verify ralph-test project is correctly implemented
# Run this after go-ralph finishes all issues, or use it as the agent's validation step.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
PASS=0
FAIL=0

check() {
  local desc="$1"; shift
  if "$@" &>/dev/null; then
    echo "  ✓ $desc"
    PASS=$((PASS + 1))
  else
    echo "  ✗ $desc"
    FAIL=$((FAIL + 1))
  fi
}

cd "$DIR"

echo "--- 001: scaffold module ---"
check "go.mod exists"               test -f go.mod
check "module path is ralph-test"   grep -q "^module ralph-test" go.mod
check "hello/ directory exists"     test -d hello
check "hello/hello.go exists"       test -f hello/hello.go
check "hello.go has package hello"  grep -q "^package hello" hello/hello.go

echo "--- 002: greet function ---"
check "Greet function exported"     grep -q "func Greet" hello/hello.go
check "hello_test.go exists"        test -f hello/hello_test.go
check "go test ./hello/... passes"  go test ./hello/...

echo "--- 003: cli entrypoint ---"
check "main.go exists"              test -f main.go
check "main.go has package main"    grep -q "^package main" main.go
check "go build succeeds"           go build .
check "prints Hello, World!"        bash -c 'go run main.go World | grep -q "Hello, World!"'

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] && echo "✓ All checks passed" && exit 0 || exit 1
