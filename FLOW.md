# FLOW.md — go-ralph Run Loop

Detailed walkthrough of what happens when `go-ralph run <project>` executes.

## Overview

```
go-ralph run <project>
│
├─ Load config (.ralph/config.yaml)
├─ Load state (.ralph/<project>.json)
├─ Reset stale running → pending
├─ herdr workspace create  → workspaceID
│
├─ Try load .ralph/pipeline.yaml
│
├─ [No pipeline.yaml, max_parallel=1] sequential for-loop:
│    └─ for each pending issue (filename order): run + retry
│
├─ [pipeline.yaml, max_parallel=1] sequential pipeline:
│    └─ loop ReadyIssues → run one at a time in path order
│
└─ [pipeline.yaml, max_parallel>1] parallel dispatch:
     └─ loop ReadyIssues → goroutine per issue, semaphore-bounded
```

---

## Step-by-Step

### 1. Load & Reset

```
state.Load()
  for each issue with status=running:
    → reset to pending, clear started_at, herdr_pane_id
state.Write()  ← atomic
```

Any issue stuck in `running` from a prior crash is recovered automatically.

### 2. Workspace Creation

```
herdr workspace create --cwd <dir> --label <project> --no-focus
  → {"result":{"workspace":{"workspace_id":"2"},...}}
```

A fresh herdr workspace is created per run. All issue panes live inside it.

### 3. Execution Mode Selection

```
pipeline.Load(".ralph/pipeline.yaml")
  → not found + max_parallel=1 → runSequential()
  → found     + max_parallel=1 → runPipelineSequential()
  → found     + max_parallel>1 → runPipelineParallel()
```

### 4. Per-Issue Loop (with retries)

For each dispatched issue, up to `max_retries` attempts:

#### 4a. Prompt Assembly

Re-assembled fresh each attempt — reads the current issue file, picking up any `[x]` checkboxes from prior partial work:

```
.ralph/prompts/<slug>.md:

<context file="PRD.md">
...file content...
</context>

<skill path="~/.pi/agent/skills/tdd">
...skill content embedded inline...
</skill>

<issue>
...issue content (with any [x] checkboxes from prior partial run)...
</issue>

<instruction>
...cfg.Instruction...
</instruction>
```

Skills are resolved at assembly time:
- `~/path` → expanded to absolute
- `relative/path` → resolved against project dir
- `directory/` → reads `directory/SKILL.md`
- missing file → hard error, run stops

#### 4b. Stale Pane Cleanup

```
herdr agent list
  → find any agent with name == "ralph-<project>-<id>"
  → herdr pane close <pane_id>
```

Prevents `agent_name_taken` errors when re-running after a forceful kill.

#### 4c. Pane Spawn

```
herdr agent start <name> \
  --cwd <dir> \
  --workspace <workspaceID> \
  --split down --no-focus \
  -- bash -c "<pane command>"
```

The pane command (built by `pi.PaneArgv`):

```bash
pi --mode json [--model <model>] --name "ralph: <slug>" "$(cat .ralph/prompts/<slug>.md)" \
  | tee .ralph/logs/<slug>.jsonl \
  | python3 -u -c "..."
_pi_rc=${PIPESTATUS[0]} _py_rc=${PIPESTATUS[2]}
echo RALPH_DONE:$((_pi_rc > 0 ? _pi_rc : _py_rc))
read -r
```

`--model <model>` is included only when `IssueState.Model` is non-empty.

Key points:
- `$(cat promptFile)` — shell expands file at run time, avoids arg length limits
- **No `--skill` flags** — skills are already embedded in the prompt file
- `tee` — raw JSONL → log file AND python3 renderer simultaneously
- **Session line** — first JSON line `{"type":"session","id":"<uuid>"}` → written to sidecar
- **`RALPH_DONE:$?`** — uses `PIPESTATUS` to correctly capture pi's exit code through the pipe
- **`read -r`** — keeps pane alive for inspection after completion

#### 4d. State → Running

```
issue.Status      = "running"
issue.StartedAt   = now
issue.HerdrPaneID = paneID
state.Write()  ← atomic
```

#### 4e. Wait for Completion

```
herdr wait output <paneID> \
  --match "RALPH_DONE:" \
  --source recent-unwrapped \
  --timeout <timeout_minutes * 60000>ms
```

- `RALPH_DONE:0` → success, break retry loop
- `RALPH_DONE:N` → failure, retry if attempts remain
- timeout → failure, retry if attempts remain

#### 4f. Read Session ID

```
os.ReadFile(.ralph/logs/<slug>.jsonl.session-id)
  → issue.PiSessionID = "<uuid>"
```

#### 4g. State → Done | Failed

After retry loop:
```
succeeded  → issue.Status = "done"
exhausted  → issue.Status = "failed"
state.Write()  ← atomic
```

If `stop_on_failure: true` and not `--continue`:
- Sequential: run exits immediately after the failed issue.
- Parallel: stops dispatching new issues; already-running goroutines are drained before exit.

---

## Parallel Dispatch Detail

```
sem := make(chan struct{}, max_parallel)   // concurrency limiter

loop:
  ready = pipeline.ReadyIssues(p, done, inFlight)
  if ready is empty and inFlight is empty → done
  if ready is empty → wait for a result, update done/inFlight, loop

  for each id in ready:
    acquire sem slot (or wait for a result first if full)
    inFlight[id] = true
    go runIssueRetry(id)   // releases sem slot on return

  drain non-blocking results
```

`pipeline.ReadyIssues` returns issue IDs whose path's `depends_on` prerequisites are all in `done` and which are not yet `done` or `inFlight`.

---

## File Layout After a Run

```
.ralph/
  config.yaml
  pipeline.yaml                        ← path definitions and depends_on
  <project>.json                       ← issue state (id, slug, status, model, pi_session_id, timestamps)
  prompts/
    <slug>.md                          ← assembled prompt per issue (re-written each attempt)
  logs/
    <slug>.jsonl                       ← attempt 1 raw pi --mode json event stream
    <slug>.jsonl.session-id            ← attempt 1 pi session UUID
    <slug>-attempt-2.jsonl             ← attempt 2 (if retried)
    <slug>-attempt-2.jsonl.session-id
```

---

## Crash Recovery

If `go-ralph run` is killed mid-issue:
- The issue remains `running` in state
- On the next `go-ralph run`, it is reset to `pending` and re-queued
- `AgentCloseByName` cleans up any stale herdr pane before the new attempt

---

## Resumability

Pi is instructed to update issue file checkboxes as it completes each item:
```
- [ ] item A  →  - [x] item A
```

On retry, `prompt.Assemble` re-reads the issue file — pi sees checked items and skips them, continuing from where it left off.

---

## go-ralph logs

```
go-ralph logs <project> <issue-id>
```

Prints:
```
Issue   : [003] 003-config-loading
Status  : done
Log     : .ralph/logs/003-config-loading.jsonl
Session : abc123-uuid
Resume  : pi --session abc123-uuid
```

Run `pi --session abc123-uuid` to replay the session interactively.
