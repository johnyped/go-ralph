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
└─ for each pending issue:
     ├─ for attempt 1..max_retries:
     │    ├─ prompt.Assemble()       → .ralph/prompts/<slug>.md  (reads updated issue file)
     │    ├─ herdr AgentCloseByName  → close stale pane if exists
     │    ├─ herdr agent start       → paneID
     │    ├─ state: running
     │    ├─ herdr wait output       → RALPH_DONE:0 or RALPH_DONE:N or timeout
     │    ├─ read session-id sidecar → IssueState.PiSessionID
     │    └─ success → break; failure → retry
     └─ state: done | failed
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

### 3. Per-Issue Loop (with retries)

For each `pending` issue (in NNN order), up to `max_retries` attempts:

#### 3a. Prompt Assembly

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

#### 3b. Stale Pane Cleanup

```
herdr agent list
  → find any agent with name == "ralph-<id>"
  → herdr pane close <pane_id>
```

Prevents `agent_name_taken` errors when re-running after a forceful kill.

#### 3c. Pane Spawn

```
herdr agent start <name> \
  --cwd <dir> \
  --workspace <workspaceID> \
  --split down --no-focus \
  -- bash -c "<pane command>"
```

The pane command (built by `pi.PaneArgv`):

```bash
pi --mode json --name "ralph: <slug>" "$(cat .ralph/prompts/<slug>.md)" \
  | tee .ralph/logs/<slug>.jsonl \
  | python3 -u -c "
import sys, json
had_error = False
for line in sys.stdin:
    e = json.loads(line)
    t = e.get('type', '')
    if t == 'session':
        open('.ralph/logs/<slug>.jsonl.session-id', 'w').write(e.get('id', ''))
    elif t == 'tool_execution_start':
        print('  ⚙', e.get('toolName', ''), flush=True)
        args = e.get('args', {})
        if args: print('    └─', json.dumps(args)[:200], flush=True)  # tool args (truncated)
    elif t == 'tool_execution_end':
        if e.get('isError'): had_error = True
        text = ''.join(p.get('text','') for p in e.get('result',{}).get('content',[]) if p.get('type')=='text').strip()
        if text: print('    →', text[:200], flush=True)  # tool result (truncated)
    elif t == 'agent_end':
        print('  ✓ done', flush=True)
    elif t == 'message_update':
        ae = e.get('assistantMessageEvent', {})
        atype = ae.get('type', '')
        if atype == 'thinking_start': print('💭 ', end='', flush=True)  # one emoji per thinking block
        elif atype in ('text_delta', 'thinking_delta'): print(ae.get('delta',''), end='', flush=True)
        elif atype in ('text_end', 'thinking_end'): print(flush=True)
sys.exit(1 if had_error else 0)
"
_pi_rc=${PIPESTATUS[0]} _py_rc=${PIPESTATUS[2]}
echo RALPH_DONE:$((_pi_rc > 0 ? _pi_rc : _py_rc))
read -r
```

Key points:
- `$(cat promptFile)` — shell expands file at run time, avoids arg length limits
- **No `--skill` flags** — skills are already embedded in the prompt file
- `tee` — raw JSONL → log file AND python3 renderer simultaneously
- **Session line** — first JSON line `{"type":"session","id":"<uuid>"}` → written to sidecar
- **`RALPH_DONE:$?`** — uses `PIPESTATUS` to correctly capture pi's exit code through the pipe
- **`read -r`** — keeps pane alive for inspection after completion

#### 3d. State → Running

```
issue.Status      = "running"
issue.StartedAt   = now
issue.HerdrPaneID = paneID
state.Write()  ← atomic
```

#### 3e. Wait for Completion

```
herdr wait output <paneID> \
  --match "RALPH_DONE:" \
  --source recent-unwrapped \
  --timeout <timeout_minutes * 60000>ms
```

- `RALPH_DONE:0` → success, break retry loop
- `RALPH_DONE:N` → failure, retry if attempts remain
- timeout → failure, retry if attempts remain

#### 3f. Read Session ID

```
os.ReadFile(.ralph/logs/<slug>.jsonl.session-id)
  → issue.PiSessionID = "<uuid>"
```

#### 3g. State → Done | Failed

After retry loop:
```
succeeded  → issue.Status = "done"
exhausted  → issue.Status = "failed"
state.Write()  ← atomic
```

If `stop_on_failure: true` and not `--continue`, run exits after first failed issue.

---

## File Layout After a Run

```
.ralph/
  config.yaml
  <project>.json                   ← issue state (id, slug, status, pi_session_id, timestamps)
  prompts/
    <slug>.md                      ← assembled prompt per issue (re-written each attempt)
  logs/
    <slug>.jsonl                   ← attempt 1 raw pi --mode json event stream
    <slug>.jsonl.session-id        ← attempt 1 pi session UUID
    <slug>.jsonl.attempt-2         ← attempt 2 (if retried)
    <slug>.jsonl.attempt-2.session-id
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
