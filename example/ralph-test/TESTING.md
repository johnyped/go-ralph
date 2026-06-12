# ralph-test — Agent Testing Guide

This project is the integration test fixture for go-ralph. Use it to verify that go-ralph correctly orchestrates pi sessions end-to-end.

## Project Layout

```
example/ralph-test/
  PRD.md                        project context (fed to every issue prompt)
  reset.sh                      restore to initial state
  test.sh                       validate all acceptance criteria
  issues/
    001-scaffold-module.md      init go.mod + hello/ package
    002-greet-function.md       TDD Greet() function
    003-cli-entrypoint.md       main.go CLI + full test suite
```

## How to Run a Full Integration Test

```bash
# 1. Reset to clean state
bash example/ralph-test/reset.sh

# 2. Init ralph state
go-ralph init ralph-test --dir example/ralph-test

# 3. Run all issues
go-ralph run ralph-test --dir example/ralph-test

# 4. Verify results
bash example/ralph-test/test.sh
```

## Reset Script

`reset.sh` does three things:
- Unchecks all `[x]` back to `[ ]` in every issue file
- Deletes generated source files: `go.mod`, `go.sum`, `main.go`, `hello/`
- Removes `.ralph/` (state, prompts, logs)

Run it before every test cycle to guarantee a clean baseline.

## Test Script

`test.sh` validates each issue's acceptance criteria in order:

| Section | Checks |
|---------|--------|
| 001 scaffold | `go.mod` exists, module path, `hello/` dir, `hello/hello.go`, package declaration |
| 002 greet | `Greet` exported, `hello_test.go` exists, `go test ./hello/...` passes |
| 003 CLI | `main.go` exists, package main, `go build` succeeds, `go run main.go World` prints `Hello, World!` |

Exit code 0 = all checks passed. Non-zero = at least one failure.

The agent implementing issue 003 must run `bash test.sh` and confirm exit 0 before marking the final checkbox.

## Checkpointing

After completing each checkbox, the agent must write `- [x]` back to the issue file immediately. This enables ralph to resume from the correct checkpoint on retry.

At the end of each issue the agent appends a `## Summary` section listing what was implemented and which files were modified.

## What a Passing Run Looks Like

```
[001] 001-scaffold-module    done ✓
[002] 002-greet-function     done ✓
[003] 003-cli-entrypoint     done ✓
```

`test.sh` output:
```
--- 001: scaffold module ---
  ✓ go.mod exists
  ✓ module path is ralph-test
  ✓ hello/ directory exists
  ✓ hello/hello.go exists
  ✓ hello.go has package hello
--- 002: greet function ---
  ✓ Greet function exported
  ✓ hello_test.go exists
  ✓ go test ./hello/... passes
--- 003: cli entrypoint ---
  ✓ main.go exists
  ✓ main.go has package main
  ✓ go build succeeds
  ✓ prints Hello, World!

Results: 12 passed, 0 failed
✓ All checks passed
```
