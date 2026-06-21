# Implementation Plan: parallel-run

## Overview

Deliver four related changes in dependency order: agent name fix → config/state fields → pi model flag → pipeline package → init updates → parallel run engine → integration test support.

## Tasks

- [ ] 1. Fix agent name collision in run.go
  - In `internal/cmd/run.go`, change the `agentName` assignment from `fmt.Sprintf("%s-%s", cfg.PiSessionPrefix, issue.ID)` to `fmt.Sprintf("%s-%s-%s", cfg.PiSessionPrefix, project, issue.ID)`
  - `project` is already in scope as `args[0]`
  - _Requirements: 1.1, 1.2, 1.3_

- [ ] 2. Add MaxParallel and DefaultModel to Config
  - In `internal/config/config.go`, add `MaxParallel int \`yaml:"max_parallel"\`` and `DefaultModel string \`yaml:"default_model"\`` to the `Config` struct
  - In the `defaults` var, set `MaxParallel: 1` and `DefaultModel: ""`
  - _Requirements: 2.1, 2.2_

- [ ] 3. Add Model field to IssueState
  - In `internal/state/state.go`, add `Model string \`json:"model,omitempty"\`` to the `IssueState` struct
  - Also add `Model string` to the `stateIssue` test helper struct in `test/integration/testutil_test.go`
  - _Requirements: 3.1_

- [ ] 4. Add --model flag to pi invocation
  - In `internal/pi/pi.go`, add `model string` parameter to `PaneArgv` and update `buildArgs` to accept and use it: append `"--model", model` after `"--mode", "json"` when `model != ""`
  - In `internal/cmd/run.go`, update the `pi.PaneArgv(...)` call to pass `issue.Model` as the new argument
  - _Requirements: 3.3, 3.4, 6.1, 6.2_

- [ ] 5. Create internal/pipeline package
  - Create `internal/pipeline/pipeline.go` with:
    - `Pipeline` struct: `Paths map[string][]string \`yaml:"paths"\`` and `DependsOn map[string][]string \`yaml:"depends_on"\``
    - `PipelinePath(dir string) string` returning `filepath.Join(dir, ".ralph", "pipeline.yaml")`
    - `Load(dir string) (*Pipeline, error)` — read file and yaml.Unmarshal
    - `Write(dir string, p *Pipeline) error` — yaml.Marshal and write atomically (tmp+rename)
    - `ReadyIssues(p *Pipeline, done map[string]bool, inFlight map[string]bool) []string` implementing the per-path logic: skip path if any depends_on ID is not done; emit first non-done non-in-flight ID per path (break on in-flight to preserve ordering)
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 5.2, 5.3_

  - [ ]* 5.1 Write property test for ReadyIssues
    - **Property 3: ReadyIssues never returns done or in-flight IDs**
    - **Property 4: ReadyIssues respects depends_on**
    - **Property 6: Pipeline serialisation round-trip**
    - **Validates: Requirements 4.1, 4.5, 5.2, 5.3**

- [ ] 6. Update init command
  - In `internal/cmd/init.go`:
    - Add `var initParallel int` flag: `initCmd.Flags().IntVar(&initParallel, "parallel", -1, "concurrency (1=sequential, ≥2=parallel, -1=ask)")`
    - After the pre-flight checks pass and before writing state, determine concurrency N:
      - If `initParallel >= 1`: use it directly
      - Else: prompt `"Run sequentially or in parallel? [sequential/parallel]: "`, if `"parallel"` prompt `"Max concurrency (≥2): "` and read N (error if N < 2), else N=1
    - Generate a `Pipeline` and call `pipeline.Write`:
      - N=1: single path `"main"` with all issue IDs in filename-sort order
      - N>1: round-robin distribute IDs across paths `"A"`, `"B"`, … (up to N paths)
    - Set `cfg.MaxParallel = N` and call `config.Write` (overwriting, not guarding with IsNotExist)
    - Seed `IssueState.Model = cfg.DefaultModel` when building state issues
  - _Requirements: 2.1, 3.2, 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7, 7.8_

- [ ] 7. Replace sequential loop with parallel dispatch engine in run.go
  - In `internal/cmd/run.go`, after loading state and resetting stale issues:
    - Attempt `pipeline.Load(dir)`. If the file does not exist and `cfg.MaxParallel > 1`, return an error `"pipeline.yaml required when max_parallel > 1"`. If the file does not exist and `cfg.MaxParallel == 1`, fall through to the existing sequential for-loop (unchanged)
    - When pipeline loaded and `cfg.MaxParallel == 1`: iterate paths in sorted key order, running issues sequentially using the existing per-issue retry block (no goroutines)
    - When pipeline loaded and `cfg.MaxParallel > 1`: implement goroutine-based dispatch:
      - Declare `type issueResult struct { id string; success bool; err error }`
      - Initialise: `sem := make(chan struct{}, cfg.MaxParallel)`, `results := make(chan issueResult, totalIssues)`, `var wg sync.WaitGroup`, `var mu sync.Mutex`, `done map[string]bool`, `inFlight map[string]bool`, `stopping bool`
      - Main loop: while any issue is not done and not stopping, call `pipeline.ReadyIssues`; if empty, block on `<-results` and process; for each ready ID acquire sem, mark inFlight, `wg.Add(1)`, launch goroutine; drain results non-blocking after dispatching
      - Goroutine: run the existing retry block for one issue, send `issueResult`, `defer wg.Done(); defer func() { <-sem }()`
      - `process(r)`: under `mu`, delete inFlight, mark state done/failed; if failed and `cfg.StopOnFailure && !runContinue` set `stopping=true`
      - After loop: `wg.Wait()`, drain remaining results, return error if any failure and stop_on_failure
    - All `state.Write` calls inside goroutines must be protected by `mu`
  - _Requirements: 2.3, 2.4, 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 5.9_

- [ ] 8. Update fake-herdr to log agent names
  - In `test/integration/testdata/fake-herdr/main.go`, in the `case "agent": / case "start":` branch, after printing the JSON response, check `os.Getenv("FAKE_HERDR_LOG_DIR")`: if non-empty, open `$FAKE_HERDR_LOG_DIR/agents.txt` with `O_CREATE|O_APPEND|O_WRONLY` and `fmt.Fprintln` the agent name (`os.Args[3]` when `len(os.Args) > 3`)
  - _Requirements: integration test support_

- [ ] 9. Add integration tests
  - In `test/integration/init_test.go`, add:
    - `TestInit_ParallelFlag_GeneratesPipeline`: call `init --parallel 2` on a 4-issue fixture; read and unmarshal `.ralph/pipeline.yaml`; assert exactly 2 paths each containing 2 IDs
    - `TestInit_Sequential_GeneratesSinglePathPipeline`: call `init --parallel 1` on a 2-issue fixture; assert single `"main"` path with 2 IDs in order
    - `TestInit_ModelSeededInState`: write `config.yaml` with `default_model: gpt-4o`; run `init --parallel 1`; parse state JSON; assert every `IssueState` entry has `"model":"gpt-4o"`
  - In `test/integration/run_test.go`, add:
    - `TestRun_AgentNameIncludesProject`: 1-issue fixture with `FAKE_HERDR_LOG_DIR` set; run `run test-project`; read `agents.txt`; assert it contains a line with `ralph-test-project-001`
    - `TestRunParallel_TwoIssuesRunConcurrently`: 2-issue fixture; write `pipeline.yaml` with paths `A:[001]` `B:[002]`; write `config.yaml` with `max_parallel: 2`; run; assert both issues are `"done"` in state and stdout has 2 `"done ✓"`
    - `TestRunParallel_DependsOn_Ordering`: 3-issue fixture; write pipeline paths `A:[001]` `B:[002]` `C:[003]` with `depends_on: {C: [001, 002]}`; `max_parallel: 2`; run with `FAKE_HERDR_LOG_DIR` set; assert all 3 done and line for `003` in `agents.txt` appears after lines for `001` and `002`
    - `TestRunParallel_StopOnFailure_Drains`: 2-issue fixture; pipeline `A:[001]` `B:[002]`; `max_parallel: 2`; run with `FAKE_HERDR_WAIT_RESULT` failing for issue `001`; assert exit non-zero, issue `001` is `"failed"` in state, issue `002` is `"done"` (drain completed before exit)
  - _Requirements: 1.1, 2.4, 5.3, 5.4, 5.7, 7.3, 7.5_

- [ ] 10. Final checkpoint
  - Ensure all tests pass: `go build ./...` and `go test ./...`
  - Ask the user if any questions arise before closing.

## Notes

- Tasks 1–4 are independent of the pipeline and each leaves the code compilable and tests passing
- Task 5 (pipeline package) must precede Tasks 6 and 7
- Task 8 must precede Task 9
- Tasks marked `*` are optional and can be skipped for a faster MVP
- Each task references specific requirements for traceability
- Property tests use `pgregory.net/rapid` or `testing/quick` — confirm with team before Task 5.1
