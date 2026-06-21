# Requirements Document

## Introduction

This feature upgrades go-ralph in four related areas: (1) fix a cross-project agent name collision caused by the current agent naming scheme, (2) introduce DAG-based parallel execution of pi sessions so multiple issues can run concurrently, (3) allow per-issue model selection so different issues can use different AI models, and (4) make `go-ralph init` interactive so it can generate the pipeline configuration automatically.

## Glossary

- **Agent_Name**: The unique string passed to `herdr agent start` and used by `AgentCloseByName` to identify a pane across all herdr workspaces. Currently `<pi_session_prefix>-<issue.ID>`.
- **Config**: The YAML file at `.ralph/config.yaml` loaded by `internal/config`.
- **DAG**: Directed Acyclic Graph of issue execution order derived from `pipeline.yaml`.
- **IssueState**: A single entry in the state JSON, tracking status, timing, and session metadata for one issue.
- **Path**: A named, ordered list of issue IDs in `pipeline.yaml` that execute sequentially within that path.
- **Pipeline**: The artifact `.ralph/pipeline.yaml` that declares named paths and `depends_on` relationships between them.
- **Slot**: One unit of concurrency; up to `max_parallel` slots may run simultaneously.
- **State**: The JSON file at `.ralph/<project>.json` managed by `internal/state`.
- **Run_Engine**: The logic inside `internal/cmd/run.go` that dispatches issues to herdr panes.

---

## Requirements

### Requirement 1: Fix Cross-Project Agent Name Collision

**User Story:** As a user running go-ralph across multiple projects simultaneously, I want agent names to be unique per project so that `AgentCloseByName` in one project does not accidentally kill panes belonging to another project.

#### Acceptance Criteria

1. THE Run_Engine SHALL construct each agent name using the format `<pi_session_prefix>-<project>-<issue.ID>` instead of `<pi_session_prefix>-<issue.ID>`.
2. WHEN `AgentCloseByName` is called before spawning a new pane, THE Run_Engine SHALL use the project-scoped name format so only panes from the same project are affected.
3. THE Run_Engine SHALL pass the project-scoped agent name to `herdr pane rename` after the pane is started.

---

### Requirement 2: Config Extensions for Parallel Execution and Model Selection

**User Story:** As a user, I want to configure maximum concurrency and a default AI model in `config.yaml` so that the run engine and init command respect those settings.

#### Acceptance Criteria

1. THE Config SHALL include a `max_parallel` integer field with a default value of `1`.
2. THE Config SHALL include a `default_model` string field with a default value of `""` (empty string).
3. WHEN `max_parallel` is `1`, THE Run_Engine SHALL execute issues sequentially, treating it as a special case that disables concurrency entirely.
4. WHEN `max_parallel` is greater than `1`, THE Run_Engine SHALL execute up to `max_parallel` issues concurrently.
5. WHEN `go-ralph run` starts, THE Run_Engine SHALL validate that `max_parallel` is a positive integer and return an error if it is not, preventing conflicting execution modes.
6. IF `default_model` is empty, THEN THE Run_Engine SHALL invoke `pi` without a `--model` flag.

---

### Requirement 3: Per-Issue Model Field in State

**User Story:** As a user, I want to override the AI model for individual issues by editing the state file before running, so that different issues can use different models.

#### Acceptance Criteria

1. THE State SHALL include a `model` string field on each `IssueState`, serialised as `"model"` in JSON, omitted when empty.
2. WHEN `go-ralph init` writes the initial state, THE Init_Command SHALL seed each `IssueState.Model` from `Config.DefaultModel`.
3. WHEN `IssueState.Model` is non-empty, THE Pi_Invoker SHALL append `--model <IssueState.Model>` to the `pi` invocation argv for that issue.
4. IF `IssueState.Model` is empty, THEN THE Pi_Invoker SHALL invoke `pi` without a `--model` flag for that issue, regardless of the value of `Config.DefaultModel`.
5. IF `IssueState.Model` contains a value that causes the `pi` invocation to fail before producing any output, THEN THE Pi_Invoker SHALL treat it as an empty model and retry without the `--model` flag.

---

### Requirement 4: Pipeline Artifact

**User Story:** As a user, I want a `pipeline.yaml` file that describes which issues belong to which execution path and which paths depend on others, so that the run engine can schedule work in parallel correctly.

#### Acceptance Criteria

1. THE Pipeline SHALL be stored at `.ralph/pipeline.yaml` within the project directory.
2. THE Pipeline SHALL declare a `paths` map where each key is a path name (string) and each value is an ordered list of issue IDs (strings).
3. THE Pipeline SHALL declare a `depends_on` map where each key is a path name and each value is a list of issue IDs that must be in `StatusDone` before the first issue of that path may be dispatched.
4. WHEN `depends_on` is absent or empty for a path, THE Run_Engine SHALL treat that path as having no prerequisites.
5. THE Pipeline SHALL be a valid YAML file that can be loaded without error when all referenced issue IDs exist in the state.

---

### Requirement 5: DAG-Based Parallel Run Engine

**User Story:** As a user, I want `go-ralph run` to read `pipeline.yaml`, build a DAG, and dispatch issues greedily into parallel slots, so that independent work streams run concurrently.

#### Acceptance Criteria

1. WHEN `go-ralph run` starts inside herdr, THE Run_Engine SHALL load `pipeline.yaml` to determine execution order.
2. THE Run_Engine SHALL resolve the DAG by treating each `depends_on` entry as an ordered dependency: a path's first issue may not start until all listed prerequisite issue IDs are `StatusDone`.
3. WHEN a slot becomes free and one or more issues are ready (all `depends_on` satisfied and status is `StatusPending`), THE Run_Engine SHALL dispatch the next ready issue into that slot.
4. THE Run_Engine SHALL dispatch no more than `max_parallel` issues concurrently at any time.
5. WHEN all slots are occupied, THE Run_Engine SHALL wait for any running issue to finish before dispatching the next ready issue.
6. WHEN an issue completes successfully, THE Run_Engine SHALL mark it `StatusDone` in memory immediately and attempt to write state atomically; a state write failure SHALL NOT prevent the issue from being treated as done for dependency resolution in the current run.
7. IF `stop_on_failure` is `true` AND an issue reaches `StatusFailed` after exhausting retries, THEN THE Run_Engine SHALL drain all currently-running slots to completion without starting any new issues, then return an error.
8. WHEN `--from <ID>` is specified, THE Run_Engine SHALL skip all issues whose ID precedes `<ID>` in their path order and treat them as already done for dependency resolution purposes.
9. WHEN `pipeline.yaml` does not exist, THE Run_Engine SHALL fall back to sequential execution of all pending issues in filename-sort order, preserving existing behaviour.

---

### Requirement 6: pi Model Flag Integration

**User Story:** As a user, I want `go-ralph` to pass `--model <value>` to `pi` when an issue has a model configured, so that pi uses the specified model for that issue's session.

#### Acceptance Criteria

1. WHEN `IssueState.Model` is non-empty, THE Pi_Invoker SHALL insert `--model <IssueState.Model>` into the `pi` argv after `--mode json` and before `--name`.
2. WHEN `IssueState.Model` is empty, THE Pi_Invoker SHALL produce a `pi` argv identical to the current behaviour with no `--model` flag present.
3. WHEN assembling the `pi` argv, THE Pi_Invoker SHALL validate that no `--model` flag is included when `IssueState.Model` is empty, and return an error immediately if one is found.

---

### Requirement 7: Interactive Init with Pipeline Generation

**User Story:** As a user running `go-ralph init`, I want to be prompted for execution mode and concurrency so that the command generates `pipeline.yaml` and `config.yaml` correctly without manual editing.

#### Acceptance Criteria

1. WHEN `go-ralph init` is invoked without `--parallel`, THE Init_Command SHALL prompt the user to choose between sequential and parallel execution modes.
2. WHEN the user selects parallel mode interactively, THE Init_Command SHALL prompt for a maximum concurrency value (integer ≥ 2).
3. WHEN `--parallel N` is provided (where N ≥ 2), THE Init_Command SHALL skip interactive prompts, set `max_parallel` to N, and proceed directly to pipeline generation.
4. WHEN `--parallel 1` is provided, THE Init_Command SHALL treat the project as sequential and generate a minimal `pipeline.yaml` with a single path containing all issue IDs.
5. WHEN sequential mode is chosen (interactively or via `--parallel 1`), THE Init_Command SHALL generate a `pipeline.yaml` with a single path named `"main"` containing all issue IDs in filename-sort order.
6. WHEN parallel mode is chosen with concurrency N, THE Init_Command SHALL distribute issue IDs evenly across N paths named `"A"`, `"B"`, … and write `pipeline.yaml` with no `depends_on` entries unless the user specifies dependencies.
7. THE Init_Command SHALL write `config.yaml` with `max_parallel` set to the chosen concurrency value and `default_model` set to `""`.
8. THE Init_Command SHALL seed each `IssueState.Model` from `default_model` when writing the initial state file.
9. WHEN `pipeline.yaml` already exists, THE Init_Command SHALL overwrite it only if the user confirms, or if `--force` is specified.
