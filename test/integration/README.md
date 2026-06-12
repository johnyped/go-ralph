# Integration Tests

End-to-end tests that exercise all 5 `go-ralph` CLI commands by spawning the real binary as a subprocess. No mocks inside the binary itself — external dependencies (`herdr`, `pi`) are replaced with fake binaries on `PATH`.

## How to Run

```bash
# Standard — no real herdr/pi needed (runs 17 tests)
go test ./test/integration/ -v

# or via Makefile
make test-integration

# Full smoke test — requires real herdr + pi installed
go test -tags integration ./test/integration/ -v
make test-integration-live
```

## File Layout

```
test/integration/
  testutil_test.go              shared helpers (build, fixture, runCmd)
  init_test.go                  TestInit_*        (4 tests)
  run_test.go                   TestRunDryRun_*   (3 tests)
                                TestRunFakeHerdr_* (3 tests)
  status_test.go                TestStatus_*      (2 tests)
  reset_test.go                 TestReset_*       (2 tests)
  logs_test.go                  TestLogs_*        (3 tests)
  smoke_test.go                 TestFullLoop_*    (1 test, integration build tag)
  testdata/
    fake-herdr/main.go          fake herdr binary
    fake-pi/main.go             fake pi binary
```

## How It Works

### Three Layers of herdr/pi Coverage

**Layer 1 — `--dry-run` (no herdr/pi needed)**

`run --dry-run` prints what it *would* do without touching herdr or pi at all. These tests need only the `go-ralph` binary itself.

**Layer 2 — fake binaries on PATH**

Go programs compiled from `testdata/` are copied into a temp dir which is prepended to `PATH` before each test. `go-ralph` shells out to `herdr` and `pi` by name, so it picks up the fakes transparently.

- `fake-herdr` returns fixture JSON for every herdr subcommand:
  - `workspace create` → workspace ID `t1`, root pane `t1-1`
  - `agent list` → empty agent list
  - `agent start` → pane ID `t1-2`
  - `wait output` → `RALPH_DONE:0` (controllable via `FAKE_HERDR_WAIT_RESULT` env)
  - `pane rename/close/run` → `{}`
- `fake-pi` prints `{"type":"agent_end"}` and exits 0, satisfying the Python renderer in `pi.go`

**Layer 3 — smoke test with real binaries (build tag `integration`)**

`smoke_test.go` runs the full `reset.sh` → `init` → `run` → `test.sh` loop against `example/ralph-test/`. Skipped automatically if `herdr` or `pi` are not on PATH.

### Test Execution Flow

```
go test ./test/integration/
        │
        ▼
  TestMain
  ├── os.MkdirTemp  → testBinDir (shared across all tests)
  └── m.Run()
       │
       ├── [first test that calls buildBinary]
       │     └── sync.Once: go build ./cmd/go-ralph/ → testBinDir/go-ralph
       │
       ├── [first test that calls buildFakeHerdr]
       │     └── sync.Once: go build ./testdata/fake-herdr/ → testBinDir/fake-herdr
       │
       ├── [first test that calls buildFakePi]
       │     └── sync.Once: go build ./testdata/fake-pi/ → testBinDir/fake-pi
       │
       └── each test:
             ├── makeFixture / makeFixtureN  → t.TempDir() with issues/ + PRD.md + config.yaml
             ├── makeBinDir  → t.TempDir() with copies of binaries named herdr/pi
             ├── prependPath → t.Setenv("PATH", binDir + ":" + existing PATH)
             ├── runCmd      → exec.Command(binary, args...), captures stdout/stderr/exitCode
             └── assertions  → exitCode, stdout/stderr substrings, state JSON content
  │
  └── TestMain defers os.RemoveAll(testBinDir)
```

### Self-Launch Bypass

The `run` command has a self-launch mechanism: when `HERDR_ENV != 1`, it creates a herdr workspace and re-execs itself inside a herdr pane. Tests bypass this by:
- Setting `HERDR_ENV=1` in the subprocess environment
- Passing `--workspace t1` explicitly

This makes `run` behave as if it's already inside herdr, skipping the workspace-creation branch entirely.

### Fixture Config

Every fixture writes `.ralph/config.yaml` with:
```yaml
skills: []
context_files:
  - PRD.md
```
This prevents skill-not-found errors (the default config points to `~/.pi/agent/skills/tdd` which doesn't exist in CI).

### State Pre-seeding

Tests that need specific issue states (e.g. an issue already `done`) use `writeState()` to write a `.ralph/<project>.json` directly before running a command, bypassing the need to run `init` first.

## Test Coverage by Command

| Command | Tests | Strategy |
|---------|-------|----------|
| `init` | 4 | fake herdr+pi on PATH |
| `run --dry-run` | 3 | no external deps |
| `run` (live loop) | 3 | fake herdr+pi on PATH, `HERDR_ENV=1` |
| `status` | 2 | pre-seeded state JSON, no external deps |
| `reset` | 2 | pre-seeded state JSON, no external deps |
| `logs` | 3 | pre-seeded state + sidecar file, no external deps |
| full smoke | 1 | real herdr+pi, `//go:build integration` |

## Controlling Fake Herdr Behaviour

Set `FAKE_HERDR_WAIT_RESULT` in the subprocess env to override what `wait output` returns:

```go
// Simulate pi failure
runCmd(t, bin,
    []string{"HERDR_ENV=1", `FAKE_HERDR_WAIT_RESULT={"result":{"matched_line":"RALPH_DONE:1"}}`},
    "run", "test-project", "--dir", fix, "--workspace", "t1")
```

Default (no env var set): returns `{"result":{"matched_line":"RALPH_DONE:0"}}` (success).

## Limitations

**Binary build time** — `go build` runs once per `go test` invocation (cached via `sync.Once`). First run is slow (~1–2s per binary). Subsequent tests reuse the same binary.

**`sync.Once` is not reset between test runs in the same process** — if a binary build panics, the `Once` is permanently triggered and all subsequent calls return the empty path. Re-running `go test` starts a fresh process, resetting the state.

**fake-pi is minimal** — it outputs `{"type":"agent_end"}` and exits. The Python renderer inside `pi.go` runs in the herdr pane subprocess, not in the test process, so the renderer's sidecar-writing and error-detection logic is exercised by fake-herdr only indirectly. The renderer is covered separately in `internal/pi/pi_test.go`.

**`run` retry loop with fake herdr** — `FAKE_HERDR_WAIT_RESULT` applies to *every* `wait output` call. There is no per-attempt override, so testing partial retry success (fail attempt 1, succeed attempt 2) is not directly supported without a more stateful fake.

**No parallel test execution** — tests call `t.Setenv("PATH", ...)` which modifies process-level environment. Running tests with `t.Parallel()` would cause PATH races. Tests are intentionally serial.

**Smoke test is manual gate** — `TestFullLoop_RalphTest` requires real `herdr` and `pi` installed and a working network/AI session. It is excluded from standard CI (`go test ./...`) and must be triggered explicitly with `-tags integration`.

**macOS `sed -i ''` in reset.sh** — `reset.sh` uses BSD `sed` syntax. On Linux the smoke test would require `sed -i` (no empty string). The smoke test is not expected to run on Linux without adjusting `reset.sh`.
