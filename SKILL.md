---
name: go-ralph
description: Orchestrate sequential pi coding sessions via herdr using go-ralph. Use when asked to run issues, set up a go-ralph project, configure retry policies, manage issue state, debug a failed run, or work with go-ralph's config, prompts, logs, or skills.
---

# go-ralph

Loop agent orchestrator — runs `pi --mode json` sessions sequentially in herdr panes.

## Quick start

```bash
# 1. Place issues in issues/NNN-slug.md
# 2. Init (once per project)
go-ralph init <project> [--dir <path>]

# 3. Run all pending issues
go-ralph run <project> [--dir <path>]

# 4. Check status
go-ralph status <project>

# 5. Inspect a session after completion
go-ralph logs <project> <issue-id>
# → Resume  : pi --session <uuid>
```

## Project layout

```
my-project/
  issues/           # NNN-slug.md files (sorted by prefix)
  PRD.md            # context file (prepended to every prompt)
  .ralph/
    config.yaml     # created by init, edit to customize
    <project>.json  # issue state (pending/running/done/failed)
    prompts/        # assembled prompt per issue
    logs/           # JSONL event streams + session-id sidecars
```

## Config (.ralph/config.yaml)

```yaml
pi_path: pi
pi_session_prefix: ralph
context_files:
  - PRD.md
skills:
  - ~/.pi/agent/skills/tdd   # embedded inline in prompt as <skill> section
max_retries: 3               # attempts per issue before marking failed
timeout_minutes: 10          # per-attempt timeout
stop_on_failure: true        # use --continue flag to override
instruction: |
  You are implementing a software issue.
  IMPORTANT — checkpoint after each acceptance criterion:
  - Mark done: change - [ ] to - [x] in the issue file immediately
  When done, verify EVERY checkbox is satisfied.
  Do not ask clarifying questions.
```

## Commands

| Command | Purpose |
|---------|---------|
| `init <project>` | Scan issues, write config + state |
| `run <project> [--from N] [--continue] [--dry-run] [--close-panes]` | Run pending issues |
| `status <project>` | Show ○◌✓✗ per issue |
| `reset <project> <id>` | Reset issue to pending |
| `logs <project> <id>` | Show log path + `pi --session` resume cmd |

## Prompt structure

Each assembled prompt:
```
<context file="PRD.md">...</context>
<skill path="~/.pi/agent/skills/tdd">...embedded inline...</skill>
<issue>...issue markdown with [x] checkboxes...</issue>
<instruction>...</instruction>
```

## Retry & resumability

- Each issue retries up to `max_retries` on timeout or non-zero exit
- Each retry: close old pane → re-assemble prompt (reads updated issue file) → spawn new pane
- Pi checkpoints progress by updating `- [ ]` → `- [x]` in the issue file
- On retry, pi sees checked items and continues from where it left off

## Crash recovery

Stale `running` issues are reset to `pending` automatically at the start of each `run`.

## Debugging failures

```bash
# Check what failed
go-ralph status my-app

# Get the pi session to replay
go-ralph logs my-app 003
pi --session <uuid>

# Reset and retry manually
go-ralph reset my-app 003
go-ralph run my-app --from 003
```

## See also

- `README.md` — full usage reference
- `FLOW.md` — detailed run loop internals
- `AGENTS.md` — architecture and key decisions
