# Architecture

## Overview

Ralph is a sequential loop agent orchestrator. It reads issues from a project, assembles a prompt per issue, spawns a `pi` coding agent session inside a `herdr` pane, waits for completion, and records the result.

## CLI Workflow

```
go-ralph init <project>
    │
    ├── scan issues/*.md  (filename order, NNN- prefix)
    ├── check: pi on PATH
    ├── check: herdr on PATH
    ├── check: herdr-pi integration (warning only)
    ├── check: context_files exist
    ├── write .ralph/config.yaml  (if not exists)
    └── write .ralph/<project>.json  (all issues → pending)

go-ralph run <project>
    │
    ├── if HERDR_ENV != "1" (not inside herdr):
    │       ├── herdr workspace create → workspaceID, rootPaneID
    │       ├── herdr pane rename <rootPaneID> "ralph: <project>"
    │       ├── herdr pane run <rootPaneID> "<same command> --workspace <id>"
    │       └── exit (execution continues inside herdr pane)
    │
    ├── load .ralph/config.yaml
    ├── load .ralph/<project>.json
    ├── reset stale running → pending  (crash recovery)
    │
    └── for each issue (status=pending, filename order):
            │
            ├── assemble .ralph/prompts/<slug>.md  (fresh each attempt)
            │
            ├── for attempt 1..max_retries:
            │       ├── herdr.AgentCloseByName(ralph-<id>)  (stale pane cleanup)
            │       ├── herdr agent start ralph-<id>
            │       │       --cwd <dir> --workspace <id> --split down --no-focus
            │       │       -- bash -c "pi --mode json ... "$(cat prompt)" | tee log.jsonl
            │       │                   | python3 renderer
            │       │                   ; echo RALPH_DONE:$?; read -r"
            │       ├── mark issue → running, write state
            │       ├── herdr wait output <pane> --match "RALPH_DONE:" --timeout <ms>
            │       ├── read session ID sidecar → issue.PiSessionID
            │       ├── RALPH_DONE:0  → succeeded = true, break
            │       └── RALPH_DONE:≠0 / timeout → retry
            │
            ├── herdr pane close <pane>  (if --close-panes or on failure)
            ├── succeeded → mark done,   write state
            └── failed    → mark failed, write state
                    ├── stop_on_failure=true (default) → exit unless --continue
                    └── stop_on_failure=false or --continue → next issue

go-ralph status <project>
    └── read .ralph/<project>.json, print ○/◌/✓/✗ per issue

go-ralph reset <project> <id>
    └── set issue status → pending, clear timestamps + pane_id + session_id

go-ralph logs <project> <id>
    └── print log path, session ID, and "pi --session <id>" resume command
```

## Key Design Decisions

### Self-launch into herdr

When `HERDR_ENV` is not set, `run` creates a workspace, renames the root pane, and re-runs itself inside that pane with `--workspace <id>`. This means the user only needs to run `go-ralph run` once — the herdr workspace appears automatically.

### Skills embedded inline (not `--skill` flags)

Skill files are read at prompt-assembly time and embedded as `<skill>` sections in the prompt file. pi receives the full knowledge without needing skill files installed on the agent machine, and without `--skill` flags polluting the CLI invocation.

### `pi --mode json "$(cat promptFile)"` (not `pi -p @file`)

`--mode json` emits structured JSONL, enabling reliable session-ID extraction, tool-call rendering, and exit-code detection via the inline Python renderer. `$(cat ...)` avoids shell argument length limits. `@file` in `-p` mode requires project trust in `~/.pi/agent/trust.json` and is bypassed silently in non-interactive mode.

### `RALPH_DONE:$?` sentinel

The pane command appends `echo RALPH_DONE:$((...))` after pi exits. Ralph reads this deterministic string via `herdr wait output --match "RALPH_DONE:" --source recent-unwrapped`. Exit code 0 = success; any non-zero = failure. This is independent of the herdr-pi integration.

### `read -r` keeps the pane alive

`herdr agent start` closes the pane immediately when the process exits. `read -r` holds the shell open so Ralph can read the sentinel line. Ralph closes the pane explicitly with `herdr pane close` after processing.

### Session ID sidecar

The inline Python renderer writes the `{"type":"session","id":"..."}` event's `id` field to `<logFile>.session-id`. Ralph reads this after each attempt and stores it in `IssueState.PiSessionID`. `go-ralph logs` falls back to the sidecar on disk if the state field is empty.

### Retry with re-assembled prompt

On retry, the prompt is re-assembled from the current issue file. Since pi marks completed checkboxes (`- [ ]` → `- [x]`) as it works, re-assembly lets pi see its prior progress and continue from where it left off.

### Stop on failure by default

Issues typically have sequential dependencies (001 must compile before 002 can import it). `--continue` overrides when failures are known to be acceptable.

## Package Responsibilities

```
internal/issues    scan + parse issues/*.md
internal/config    load/write .ralph/config.yaml with defaults
internal/state     load/write .ralph/<project>.json (atomic tmp→rename)
internal/prompt    assemble .ralph/prompts/<slug>.md
                   (context + skills inline + issue + instruction)
internal/herdr     shell-out: WorkspaceCreate, AgentCloseByName, AgentStart,
                   WaitOutput, PaneClose, PaneRename, PaneRun
internal/pi        PaneArgv, LogFile, SessionIDPath
internal/cmd       cobra commands: init, run, status, reset, logs
```

## State Machine (per issue)

```
pending → running → done
                 ↘ failed → (pending via go-ralph reset)
```

State is written atomically after each transition using `os.Rename` from a `.tmp` file.

## Prompt Structure

```xml
<context file="PRD.md">
...file contents...
</context>

<skill path="~/.pi/agent/skills/tdd">
...skill file contents embedded inline...
</skill>

<issue file="issues/001-scaffold.md">
...issue markdown contents...
</issue>

<instruction>
...config.instruction (with {{issue_file}} substituted)...
</instruction>
```

## Log Files

```
.ralph/logs/<slug>.jsonl              # attempt 1 raw JSONL
.ralph/logs/<slug>-attempt-2.jsonl   # attempt 2
.ralph/logs/<slug>.jsonl.session-id  # pi session ID sidecar
```
