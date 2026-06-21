# go-ralph

Autonomous development workflow orchestrator — runs `pi` coding sessions via `herdr`, sequentially or in parallel.

Ralph automates the final stage of autonomous software delivery: it reads issues from `issues/*.md`, assembles a prompt per issue (context files + skills + issue content + instruction), spawns a `pi --mode json` session in a dedicated `herdr` workspace pane, waits for it to finish, and records the result. By chaining AI agents through design refinement, PRD generation, issue breakdown, and implementation, go-ralph enables a complete autonomous development workflow from concept to shipped code.

## Inspiration: The RALP Loop

go-ralph is inspired by the **RALP loop** technique from [Matt Pocock's skills](https://github.com/mattpocock/skills), which combines sequential AI-powered workflows to implement software features end-to-end:

1. **`/grill-me`** — Stress-test a plan or design through relentless questioning until reaching shared understanding
2. **`/to-prd`** — Convert the refined plan into a formal PRD (Product Requirements Document)
3. **`/to-issue`** — Break the PRD into independently-grabbable implementation issues
4. **`go-ralph run`** — Execute all issues sequentially or in parallel, with each issue assigned to a dedicated `pi` coding session

This workflow bridges the gap between high-level design and hands-on implementation, ensuring clarity at each stage before handing off to the next. go-ralph automates the final orchestration step, running each issue through a full TDD (red-green-refactor) cycle with resumability and crash recovery.

## Requirements

- [pi](https://github.com/earendil-works/pi) — AI coding agent CLI
- [herdr](https://herdr.dev) — terminal agent multiplexer
- herdr-pi integration installed: `herdr integration install pi`

## Install

**From source (recommended during development):**

```bash
git clone https://github.com/johnyped/go-ralph
cd go-ralph
go install ./cmd/go-ralph/
```

`go install` places the binary in `$GOPATH/bin` (default `~/go/bin`). Make sure that directory is on your `PATH`:

```bash
export PATH="$HOME/go/bin:$PATH"
```

**Once published:**

```bash
go install github.com/johnyped/go-ralph/cmd/ralph@latest
```

## Usage

```bash
# Initialize ralph for a project (run once per project)
go-ralph init <project_name> [--dir <path>] [--parallel N]

# Run all pending issues
go-ralph run <project_name> [--dir <path>] [--from <N>] [--continue] [--dry-run] [--close-panes]

# Show issue status
go-ralph status <project_name> [--dir <path>]

# Reset an issue back to pending
go-ralph reset <project_name> <issue-number> [--dir <path>]

# Show log paths and pi session info for an issue
go-ralph logs <project_name> <issue-id> [--dir <path>]
```

`--dir` defaults to the current working directory.

### Example

```bash
cd ~/projects/my-app
go-ralph init my-app                  # sequential (prompts if omitted)
go-ralph init my-app --parallel 3    # parallel with 3 concurrent agents
go-ralph run my-app
go-ralph run my-app --from 004       # resume from issue 004
go-ralph run my-app --continue       # keep going past failures
go-ralph run my-app --close-panes    # close herdr panes after each issue
go-ralph reset my-app 003            # retry issue 003
go-ralph status my-app
go-ralph logs my-app 003             # show session info for issue 003
```

### Viewing a Session

```bash
go-ralph logs my-app 003
# Issue   : [003] 003-config-loading
# Status  : done
# Log     : .ralph/logs/003-config-loading.jsonl
# Session : abc123-uuid
# Resume  : pi --session abc123-uuid
```

Then run `pi --session abc123-uuid` to inspect the full session interactively.

## Project Layout

```
my-project/
  issues/
    001-module-scaffolding.md
    002-config-loading.md
    ...
  PRD.md
  .ralph/
    config.yaml         # ralph config (created by init)
    pipeline.yaml       # path definitions and depends_on (created by init)
    my-project.json     # issue state
    prompts/            # assembled prompts (generated per run)
    logs/               # JSONL event logs + session-id sidecars
```

## Configuration

`.ralph/config.yaml` is created by `go-ralph init`. Edit to customize:

```yaml
pi_path: pi
pi_session_prefix: ralph

# Files prepended to every issue prompt as <context> sections
context_files:
  - PRD.md

# Skill files embedded inline in every prompt as <skill> sections.
# Paths support ~ and relative (resolved against project dir).
# Directories are resolved to <dir>/SKILL.md automatically.
# Missing skill files cause a hard error.
skills:
  - ~/.pi/agent/skills/tdd

# Retry policy
max_retries: 3        # attempts per issue before marking failed
timeout_minutes: 30   # per-attempt timeout

# Stop at first failure (use --continue flag to override per run)
stop_on_failure: true

# Parallel execution (set by go-ralph init --parallel N)
max_parallel: 1       # 1 = sequential; ≥2 = parallel goroutines (requires pipeline.yaml)

# Default pi model for all issues (empty = pi default)
default_model: ""

instruction: |
  You are implementing a software issue.
  Complete all acceptance criteria using TDD (red-green-refactor).
  ...
```

## How It Works

1. `go-ralph init` scans `issues/*.md`, checks dependencies, prompts for sequential or parallel mode (or uses `--parallel N`), writes `pipeline.yaml` and `.ralph/<project>.json` with all issues as `pending`. Config is always overwritten.
2. `go-ralph run` selects one of three execution modes:
   - **No `pipeline.yaml` + `max_parallel=1`** — original sequential for-loop over pending issues.
   - **`pipeline.yaml` + `max_parallel=1`** — sequential, but in pipeline path order respecting `depends_on`.
   - **`pipeline.yaml` + `max_parallel>1`** — parallel goroutine dispatch, semaphore-bounded to `max_parallel`. Issues within a path run in order; paths run concurrently if their `depends_on` prerequisites are met.
3. For each issue, ralph retries up to `max_retries` times:
   - Assembles prompt fresh (reads updated issue file — picks up any `[x]` checkboxes from prior partial run)
   - Closes any stale pane with the same agent name (`ralph-<project>-<id>`)
   - Spawns `pi --mode json [--model <model>]` in a herdr pane
   - Waits up to `timeout_minutes` for `RALPH_DONE:` sentinel
   - On success: marks `done`; on timeout/failure: retries
4. When `stop_on_failure=true` in parallel mode, ralph stops dispatching new issues and drains already-running goroutines before exiting.

## Parallel Execution

`go-ralph init --parallel N` (or the interactive prompt) sets `max_parallel: N` and generates `pipeline.yaml` with round-robin path assignment:

```yaml
paths:
  A:
    - "001"
    - "003"
  B:
    - "002"
    - "004"
depends_on: {}
```

Each named path is a sequential lane. Issues in the same path run one after another; different paths run concurrently. Use `depends_on` to require a path to finish before another starts:

```yaml
depends_on:
  B:
    - A   # path B won't start until all issues in path A are done
```

## Prompt Structure

Each assembled prompt contains:

```
<context file="PRD.md">
...file content...
</context>

<skill path="~/.pi/agent/skills/tdd">
...skill content embedded inline...
</skill>

<issue>
...issue markdown...
</issue>

<instruction>
...instruction from config...
</instruction>
```

Skills are embedded as raw text — pi gets the knowledge without needing skill files installed.

## State

Issue statuses: `pending` | `running` | `done` | `failed`

State is written atomically to `.ralph/<project>.json`. Stale `running` issues are reset to `pending` on the next `run` (crash recovery).

## Resumability

When pi completes a checkbox, it updates the issue file (`- [ ]` → `- [x]`). On retry or rerun, the prompt is re-assembled from the updated file — pi sees which items are already done and continues from where it left off.

## Logs

Raw `pi --mode json` output is tee'd to `.ralph/logs/<slug>.jsonl`. On retries, each attempt gets its own file to avoid overwriting:

```
.ralph/logs/<slug>.jsonl              # attempt 1
.ralph/logs/<slug>-attempt-2.jsonl   # attempt 2
.ralph/logs/<slug>-attempt-3.jsonl   # attempt 3
```

The pi session ID sidecar follows the same naming: `<slug>.jsonl.session-id`, `<slug>-attempt-2.jsonl.session-id`, etc.

Use `go-ralph logs <project> <issue-id>` to get the session ID and `pi --session <id>` resume command.
