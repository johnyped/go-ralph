# ralph-test

A minimal Go project used as an integration test fixture for go-ralph.

## Goal

Three sequential issues that a pi agent can complete end-to-end:
1. Scaffold a Go module with a `hello` package
2. Add a `Greet(name string) string` function with a unit test
3. Add a CLI `main.go` that prints the greeting

Each issue has checkbox acceptance criteria so ralph can checkpoint progress.
