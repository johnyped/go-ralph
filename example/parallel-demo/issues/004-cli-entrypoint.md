# 004 — CLI entrypoint

Add a `main.go` that accepts an operation and two numbers as arguments.

## Acceptance Criteria

- [ ] `main.go` exists at the project root with `package main`
- [ ] Running `go run main.go add 3 4` prints `7.00`
- [ ] Running `go run main.go div 10 0` prints an error message and exits non-zero
- [ ] `go build .` succeeds with no errors
