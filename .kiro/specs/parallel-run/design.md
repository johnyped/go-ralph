# Design Document: parallel-run

## Overview

This feature delivers four related changes to go-ralph:

1. **Agent name fix** — include `<project>` in the agent name to prevent cross-project pane collisions.
2. **Model selection** — add `default_model` to Config and `Model` to IssueState; pass `--model` to `pi` when set.
3. **Pipeline artifact** — new `internal/pipeline` package that reads/writes `.ralph/pipeline.yaml` and resolves which issues are ready to dispatch.
4. **Parallel run engine** — replace the sequential for-loop in `run.go` with a goroutine-based dispatcher bounded by `max_parallel` slots.

Existing sequential behaviour (no `pipeline.yaml`, `max_parallel=1`) is fully preserved.

---

## Architecture

Files that change and files that are new:

```
internal/
  config/config.go          +MaxParallel int, +DefaultModel string
  state/state.go            +Model string on IssueState
  pi/pi.go                  PaneArgv adds model string param; buildArgs appends --model
  pipeline/pipeline.go      NEW — Load, Write, PipelinePath, ReadyIssues
  cmd/init.go               +--parallel flag, interactive prompt, write pipeline.yaml, seed Model
  cmd/run.go                agent name fix; parallel dispatch loop

test/integration/
  testdata/fake-herdr/main.go   log agent names to FAKE_HERDR_LOG_DIR/agents.txt
  init_test.go                  +3 tests
  run_test.go                   +4 tests
```

---

## Data Model Changes

### Config (`internal/config/config.go`)

```go
type Config struct {
    // ... existing fields unchanged ...
    MaxParallel  int    `yaml:"max_parallel"`   // default: 1
    DefaultModel string `yaml:"default_model"`  // default: ""
}

var defaults = Config{
    // ... existing defaults unchanged ...
    MaxParallel:  1,
    DefaultModel: "",
}
```

### IssueState (`internal/state/state.go`)

```go
type IssueState struct {
    // ... existing fields unchanged ...
    Model string `json:"model,omitempty"`
}
```

### Pipeline (`internal/pipeline/pipeline.go`)

```go
type Pipeline struct {
    Paths     map[string][]string `yaml:"paths"`      // path name → ordered issue IDs
    DependsOn map[string][]string `yaml:"depends_on"` // path name → prerequisite issue IDs
}
```

---

## Pipeline Package (`internal/pipeline/pipeline.go`)

```go
package pipeline

import (
    "os"
    "path/filepath"
    "gopkg.in/yaml.v3"
)

func PipelinePath(dir string) string {
    return filepath.Join(dir, ".ralph", "pipeline.yaml")
}

func Load(dir string) (*Pipeline, error) { /* read + unmarshal */ }
func Write(dir string, p *Pipeline) error { /* marshal + write */ }

// ReadyIssues returns issue IDs that are ready to dispatch:
//   - all depends_on IDs are in done
//   - not already done
//   - not in-flight
// Preserves within-path order: only the next pending ID in each path is eligible.
func ReadyIssues(p *Pipeline, done map[string]bool, inFlight map[string]bool) []string
```

**ReadyIssues logic:**

```pascal
FOR each path in p.Paths:
    // Check depends_on
    FOR each prereqID in p.DependsOn[path]:
        IF NOT done[prereqID] THEN skip this path
    END FOR

    // Emit the first non-done, non-in-flight ID in the path
    FOR each id in p.Paths[path]:
        IF done[id] THEN continue
        IF NOT inFlight[id] THEN emit id; break
        ELSE break  // this path is blocked by in-flight issue
    END FOR
END FOR
```

The "else break" on an in-flight issue preserves sequential within-path ordering: path B's second issue cannot start until path B's first in-flight issue finishes.

---

## pi.go Changes

`PaneArgv` gains a `model string` parameter. `buildArgs` appends `--model <value>` when non-empty, inserted after `--mode json` and before `--name`:

```go
func PaneArgv(dir string, cfg *config.Config, slug, promptFile, logFile, model string) []string

func buildArgs(dir string, cfg *config.Config, slug, model string) []string {
    args := []string{"--mode", "json"}
    if model != "" {
        args = append(args, "--model", model)
    }
    args = append(args, "--name", fmt.Sprintf("%s: %s", cfg.PiSessionPrefix, slug))
    return args
}
```

All existing callers pass `issue.Model` (or `""` for the dry-run path).

---

## Init Changes (`internal/cmd/init.go`)

New flag: `--parallel N` (int, default 0 = ask interactively).

**Interactive prompt flow (when `--parallel` not given):**

```pascal
print "Run sequentially or in parallel? [sequential/parallel]: "
read mode from stdin

IF mode == "parallel":
    print "Max concurrency (≥2): "
    read N from stdin
    IF N < 2: return error
ELSE:
    N = 1
END IF
```

**Pipeline generation:**

```pascal
IF N == 1:
    pipeline.Paths = {"main": [all issue IDs in filename-sort order]}
    pipeline.DependsOn = {}
ELSE:
    distribute issue IDs round-robin across paths "A", "B", ..., up to N paths
    // e.g. 5 issues, N=2 → A:[001,003,005], B:[002,004]
    pipeline.DependsOn = {}
END IF
```

**Pipeline overwrite:** always overwrite `pipeline.yaml` (issue list may have changed); config.yaml only written if absent (existing behaviour unchanged).

**State seeding:** set `IssueState.Model = cfg.DefaultModel` for every issue during state write.

**Requirement 7.9 note:** the requirements spec says "overwrite only if user confirms or `--force`". The agreed design decision says "overwrite always". The design follows the agreed decision (init is idempotent by design).

---

## Run Engine Changes (`internal/cmd/run.go`)

### Agent Name Fix

```go
agentName := fmt.Sprintf("%s-%s-%s", cfg.PiSessionPrefix, project, issue.ID)
```

### Sequential Path (max_parallel == 1, no pipeline.yaml)

Identical to today's for-loop. No goroutines. Fallback triggered when `pipeline.yaml` is absent.

### Parallel Dispatch Loop

When `pipeline.yaml` exists and `max_parallel > 1`:

```go
type issueResult struct {
    id      string
    success bool
    err     error
}

sem     := make(chan struct{}, cfg.MaxParallel)  // semaphore
results := make(chan issueResult, len(allIssues))
var wg sync.WaitGroup

done     := map[string]bool{}    // StatusDone IDs
inFlight := map[string]bool{}    // currently dispatched
stopping := false
```

```pascal
WHILE issues remain:
    IF stopping:
        wg.Wait()
        break
    END IF

    ready = pipeline.ReadyIssues(p, done, inFlight)

    IF len(ready) == 0:
        // wait for any result
        r = <-results
        process(r)
        continue
    END IF

    FOR EACH id IN ready:
        sem <- struct{}{}          // acquire slot (blocks if full)
        inFlight[id] = true
        wg.Add(1)
        GO runIssue(id, sem, results, &wg)
    END FOR

    // drain any pending results without blocking
    DRAIN results (non-blocking select)
END WHILE

wg.Wait()
drain remaining results
```

`runIssue` goroutine:
```pascal
FUNCTION runIssue(id):
    DEFER: <-sem; wg.Done()
    run issue with retry logic (same as sequential path)
    results <- issueResult{id, succeeded, lastErr}
END FUNCTION
```

`process(r)`:
```pascal
FUNCTION process(r):
    delete inFlight[r.id]
    IF r.success:
        mark state StatusDone
        done[r.id] = true
    ELSE:
        mark state StatusFailed
        IF cfg.StopOnFailure AND NOT runContinue:
            stopping = true
        END IF
    END IF
END FUNCTION
```

**State writes from goroutines:** each goroutine calls `state.Write` under a `sync.Mutex` to prevent concurrent writes. The existing atomic write (tmp+rename) alone is not safe for concurrent callers in the same process.

**Concurrency primitives used:** `sync.WaitGroup`, `sync.Mutex`, `chan struct{}` (semaphore), `chan issueResult`. No external libraries.

---

## Sequence Diagram: Parallel Dispatch (2 slots, 3 issues, A→[001,002], B→[003])

```mermaid
sequenceDiagram
    participant Main as run.go main loop
    participant G1 as goroutine 001
    participant G2 as goroutine 002
    participant G3 as goroutine 003

    Main->>G1: dispatch 001 (sem slot 1)
    Main->>G2: dispatch 003 (sem slot 2, B has no depends_on)
    Main->>Main: wait for result (sem full)
    G1-->>Main: result{001, success}
    Main->>Main: done[001]=true; release slot
    Main->>G3: dispatch 002 (sem slot 1, A path unblocked)
    G2-->>Main: result{003, success}
    G3-->>Main: result{002, success}
    Main->>Main: all done
```

---

## Integration Test Plan

### fake-herdr additions

Add `FAKE_HERDR_LOG_DIR` support to `fake-herdr/main.go`. When set, the `agent start` handler appends the agent name argument to `$FAKE_HERDR_LOG_DIR/agents.txt`:

```go
case "agent":
    if sub2 == "start" && len(os.Args) > 3 {
        if logDir := os.Getenv("FAKE_HERDR_LOG_DIR"); logDir != "" {
            agentName := os.Args[3]
            f, _ := os.OpenFile(filepath.Join(logDir, "agents.txt"),
                os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
            fmt.Fprintln(f, agentName)
            f.Close()
        }
    }
```

### New tests in `init_test.go`

| Test | What it does |
|------|-------------|
| `TestInit_ParallelFlag_GeneratesPipeline` | Run `init --parallel 2` on 4-issue fixture; read `.ralph/pipeline.yaml`; assert 2 paths, each with 2 IDs |
| `TestInit_Sequential_GeneratesSinglePathPipeline` | Run `init --parallel 1` on 2-issue fixture; assert single `"main"` path with 2 IDs |
| `TestInit_ModelSeededInState` | Write `config.yaml` with `default_model: gpt-4o`; run `init`; parse state JSON; assert all IssueState entries have `"model":"gpt-4o"` |

### New tests in `run_test.go`

| Test | Setup | Assertions |
|------|-------|-----------|
| `TestRun_AgentNameIncludesProject` | 1-issue fixture, `FAKE_HERDR_LOG_DIR` set; run `test-project` | `agents.txt` contains `ralph-test-project-001` |
| `TestRunParallel_TwoIssuesRunConcurrently` | 2-issue fixture, `pipeline.yaml` with paths A:[001] B:[002], `max_parallel:2` in config | Both issues done in state; stdout has 2 `"done ✓"` |
| `TestRunParallel_DependsOn_Ordering` | 3-issue fixture, pipeline paths A:[001] B:[002] C:[003], C depends_on [001,002], `max_parallel:2` | All 3 done; fake-herdr `agents.txt` shows 003 started after 001 and 002 (via line ordering) |
| `TestRunParallel_StopOnFailure_Drains` | 2-issue fixture, `max_parallel:2`, `FAKE_HERDR_WAIT_RESULT` fails for issue 001 | Run exits non-zero; state has 001 failed; 002 is done (drain completed) |

---

## Migration / Backward Compatibility

| Scenario | Behaviour |
|----------|-----------|
| No `pipeline.yaml`, `max_parallel=1` (default) | Identical to today: sequential for-loop, no goroutines |
| `pipeline.yaml` absent, `max_parallel>1` | Error: "pipeline.yaml required when max_parallel > 1" |
| `pipeline.yaml` present, `max_parallel=1` | Read pipeline, run sequentially in path order (no goroutines) |
| Existing state file without `"model"` field | `omitempty` means zero value; `IssueState.Model` is `""` — no change in behaviour |
| Existing `config.yaml` without new fields | `yaml.Unmarshal` into defaults-prefilled struct: `max_parallel=1`, `default_model=""` — no change |
| `PaneArgv` callers | Signature gains `model string`; all call sites are in `run.go` — single change point |

---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do.*

### Property 1: Agent name uniqueness across projects

*For any* two concurrent runs with different project names, every agent name observed by herdr must contain the project name, so no two projects ever produce the same agent name for the same issue ID.

**Validates: Requirement 1.1, 1.2, 1.3**

### Property 2: Model flag round-trip

*For any* non-empty model string stored in `IssueState.Model`, the `pi` argv produced by `PaneArgv` must contain `--model <value>` immediately after `--mode json` and before `--name`; for empty model, no `--model` flag must appear anywhere in the argv.

**Validates: Requirements 6.1, 6.2, 6.3**

### Property 3: ReadyIssues never returns done or in-flight IDs

*For any* pipeline, done set, and in-flight set, `ReadyIssues` must return only IDs that are absent from both the done set and the in-flight set.

**Validates: Requirement 5.2, 5.3**

### Property 4: ReadyIssues respects depends_on

*For any* pipeline where path P has `depends_on` listing issue IDs D, `ReadyIssues` must not include any issue from path P until every ID in D is present in the done set.

**Validates: Requirement 5.2**

### Property 5: Parallel dispatch slot bound

*For any* run with `max_parallel=N`, the number of concurrently active goroutines (issues in `inFlight`) must never exceed N at any point during dispatch.

**Validates: Requirement 5.4**

### Property 6: Pipeline serialisation round-trip

*For any* valid `Pipeline` struct, `Write` then `Load` must produce an equivalent struct (same paths and depends_on entries).

**Validates: Requirement 4.1, 4.5**
