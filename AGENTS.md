# Agent Guidance

This repo is `go-ralph` — a Go CLI that orchestrates sequential `pi` coding sessions via `herdr`.

## Source of Truth

- `README.md` — user-facing usage and configuration
- `FLOW.md` — detailed run loop and data flow
- `go.mod` — module `github.com/johnyped/go-ralph`, Go 1.23+

## Architecture

```
cmd/go-ralph/main.go               entry point
internal/
  issues/     scan + parse issues/*.md (filename sort, NNN- prefix)
  config/     load/write .ralph/config.yaml with defaults
  state/      load/write .ralph/<project>.json atomically (tmp + rename)
  prompt/     assemble .ralph/prompts/<slug>.md from context_files + skills + issue + instruction
  herdr/      shell-out wrapper: WorkspaceCreate, AgentCloseByName, AgentStart, WaitOutput, PaneClose, PaneRename
  pi/         build pi CLI invocation: PaneArgv, LogFile, SessionIDPath
  cmd/        cobra commands: init, run, status, reset, logs
```

## Key Decisions

- **Sequential only** — one issue at a time, one herdr pane per issue.
- **Fresh workspace per run** — `herdr workspace create` at the start of each `run`; no configured workspace ID.
- **`--mode json`** — pi runs as `pi --mode json "$(cat promptFile)"`. Prompt injected via `$(cat)` to avoid shell arg length limits.
- **Skills embedded inline** — skill files are read and embedded as `<skill>` sections in the prompt. No `--skill` flags. Missing skill = hard error.
- **Directory skills** — if a skill path is a directory, `SKILL.md` inside it is used automatically.
- **Checkpoint instruction** — the default instruction tells pi to update issue checkboxes (`- [ ]` → `- [x]`) as it completes each item, enabling resumability across retries.
- **Retry policy** — each issue is retried up to `max_retries` times (default 3). Each retry closes the old pane, re-assembles the prompt (picking up any checked boxes), and spawns a new pane.
- **Configurable timeout** — `timeout_minutes` (default 10) controls per-attempt wait time.
- **Stale pane cleanup** — `herdr.AgentCloseByName` closes any existing pane with the same name before spawning, preventing `agent_name_taken` errors after a forceful kill.
- **Session ID tracking** — python3 renderer in pane parses `{"type":"session","id":"..."}` from JSON stream, writes to `.ralph/logs/<slug>.jsonl.session-id`. Stored in `IssueState.PiSessionID`.
- **RALPH_DONE sentinel** — success/failure detected by `RALPH_DONE:$?` appended to pi command, read via `herdr wait output --source recent-unwrapped`.
- **Crash recovery** — stale `running` issues reset to `pending` at the start of each `run`.
- **Atomic state writes** — write to `.json.tmp` then rename to avoid corruption.
- **Stop on failure by default** — `stop_on_failure: true` in config; `--continue` flag overrides per run.
- **Project-local only** — no global `~/.ralph/` config; everything under `.ralph/` in the project dir.

## Commands

```
go-ralph init   <project>  --dir <path>
go-ralph run    <project>  --dir <path>  --from <N>  --continue  --dry-run  --close-panes
go-ralph status <project>  --dir <path>
go-ralph reset  <project>  <issue-id>   --dir <path>
go-ralph logs   <project>  <issue-id>   --dir <path>
```

## herdr Integration

Ralph shells out to `herdr` directly (no SDK). Key calls:

- `herdr workspace create --cwd <dir> --label <project> --no-focus` → returns `workspace_id`
- `herdr agent list` → used by `AgentCloseByName` to find and close stale panes by name
- `herdr agent start <name> --cwd <dir> --workspace <id> --split down --no-focus -- bash -c "..."`
- `herdr pane rename <pane> <label>`
- `herdr wait output <pane> --match "RALPH_DONE:" --source recent-unwrapped --timeout <ms>`
- `herdr pane close <pane>`

## pi Invocation

```bash
pi --mode json --name "ralph: <slug>" "$(cat promptFile)" \
  | tee <slug>.jsonl \
  | python3 -u -c "<rendererScript>"
; _pi_rc=${PIPESTATUS[0]} _py_rc=${PIPESTATUS[2]}
; echo RALPH_DONE:$((_pi_rc > 0 ? _pi_rc : _py_rc))
; read -r
```

The renderer is defined as the `rendererScript` constant in `internal/pi/pi.go`.
It parses the JSONL stream, writes the session ID to the sidecar file, prints live
tool activity, and exits 1 if any tool returned an error.

Skills are **not** passed as `--skill` flags — they are embedded inline in the prompt file.

## Logs

Each issue's raw `pi --mode json` JSONL output is tee'd to:

```
<project_dir>/.ralph/logs/<slug>.jsonl              # attempt 1
<project_dir>/.ralph/logs/<slug>-attempt-2.jsonl   # attempt 2
<project_dir>/.ralph/logs/<slug>-attempt-3.jsonl   # attempt 3
```

The pi session ID sidecar follows the same naming: `<slug>.jsonl.session-id`, `<slug>-attempt-2.jsonl.session-id`, etc.

Use `go-ralph logs <project> <issue-id>` to retrieve the session ID and `pi --session` resume command.

## Testing

Wrap `herdr` and `pi` invocations behind interfaces to allow fakes. The `herdr` package's `run()` function is the single shell-out point — replace it in tests with a fake that returns fixture JSON.

Do not require a real herdr instance or pi installation for unit tests.
