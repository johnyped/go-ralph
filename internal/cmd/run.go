package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/johnyped/go-ralph/internal/config"
	"github.com/johnyped/go-ralph/internal/herdr"
	"github.com/johnyped/go-ralph/internal/pi"
	"github.com/johnyped/go-ralph/internal/pipeline"
	"github.com/johnyped/go-ralph/internal/prompt"
	"github.com/johnyped/go-ralph/internal/state"
	"github.com/spf13/cobra"
)

var (
	runDir       string
	runFrom      string
	runContinue  bool
	runDryRun    bool
	closePanes   bool
	runWorkspace string
)

var runCmd = &cobra.Command{
	Use:   "run <project_name>",
	Short: "Run pending issues sequentially via pi in herdr panes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]
		dir := runDir

		// Self-launch: if not already inside herdr, create a workspace and
		// re-run this command inside its first pane, then exit.
		if os.Getenv("HERDR_ENV") != "1" {
			ws, err := herdr.WorkspaceCreate(dir, project)
			if err != nil {
				return fmt.Errorf("workspace create: %w", err)
			}
			if err := herdr.PaneRename(ws.RootPaneID, "ralph: "+project); err != nil {
				return fmt.Errorf("rename pane: %w", err)
			}
			// Re-exec this command inside the herdr pane, passing the workspace ID explicitly
			reCmd := strings.Join(os.Args, " ") + " --workspace " + ws.WorkspaceID
			if err := herdr.PaneRun(ws.RootPaneID, reCmd); err != nil {
				return fmt.Errorf("pane run: %w", err)
			}
			fmt.Printf("workspace %s — running in herdr pane %s\n", ws.WorkspaceID, ws.RootPaneID)
			return nil
		}

		cfg, err := config.Load(dir)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		s, err := state.Load(dir, project)
		if err != nil {
			return err
		}

		// Reset any stale running issues back to pending (crash recovery)
		for i := range s.Issues {
			if s.Issues[i].Status == state.StatusRunning {
				s.Issues[i].Status = state.StatusPending
				s.Issues[i].StartedAt = nil
				s.Issues[i].FinishedAt = nil
				s.Issues[i].HerdrPaneID = ""
			}
		}
		if err := state.Write(dir, project, s); err != nil {
			return fmt.Errorf("reset stale state: %w", err)
		}

		if runDryRun {
			drySkipping := runFrom != ""
			for _, issue := range s.Issues {
				if drySkipping {
					if issue.ID == runFrom {
						drySkipping = false
					} else {
						continue
					}
				}
				if issue.Status != state.StatusPending {
					continue
				}
				promptFile, err := prompt.Assemble(dir, cfg, toIssue(&issue))
				if err != nil {
					return fmt.Errorf("[%s] assemble prompt: %w", issue.ID, err)
				}
				fmt.Printf("  dry-run: would run pi for [%s] %s (prompt: %s)\n", issue.ID, issue.Slug, promptFile)
			}
			return nil
		}

		// Already inside herdr — workspace ID passed via --workspace flag
		workspaceID := runWorkspace

		// Try to load pipeline
		p, pipelineErr := pipeline.Load(dir)

		if pipelineErr != nil {
			// No pipeline.yaml
			if !os.IsNotExist(pipelineErr) {
				return fmt.Errorf("load pipeline: %w", pipelineErr)
			}
			if cfg.MaxParallel > 1 {
				return fmt.Errorf("pipeline.yaml required when max_parallel > 1")
			}
			// Fall through to sequential for-loop
			return runSequential(dir, project, cfg, s, workspaceID)
		}

		if cfg.MaxParallel <= 1 {
			return runPipelineSequential(dir, project, cfg, s, p, workspaceID)
		}
		return runPipelineParallel(dir, project, cfg, s, p, workspaceID)
	},
}

// runSequential is the original sequential for-loop (no pipeline).
func runSequential(dir, project string, cfg *config.Config, s *state.State, workspaceID string) error {
	skipping := runFrom != ""
	for i := range s.Issues {
		issue := &s.Issues[i]
		if skipping {
			if issue.ID == runFrom {
				skipping = false
			} else {
				continue
			}
		}
		if issue.Status == state.StatusDone {
			fmt.Printf("[%s] already done, skipping\n", issue.ID)
			continue
		}
		fmt.Printf("\n[%s] %s\n", issue.ID, issue.Slug)
		succeeded, lastErr := runIssueRetry(dir, project, cfg, s, issue, workspaceID, nil)
		fin := time.Now().UTC()
		issue.FinishedAt = &fin
		if !succeeded {
			issue.Status = state.StatusFailed
			_ = state.Write(dir, project, s)
			fmt.Fprintf(os.Stderr, "[%s] failed after %d attempts: %v\n", issue.ID, cfg.MaxRetries, lastErr)
			if cfg.StopOnFailure && !runContinue {
				return fmt.Errorf("stopping on failure (use --continue to proceed)")
			}
			continue
		}
		issue.Status = state.StatusDone
		_ = state.Write(dir, project, s)
		fmt.Printf("  [%s] done ✓\n", issue.ID)
	}
	return nil
}

// runPipelineSequential runs issues in pipeline path order, sequentially.
func runPipelineSequential(dir, project string, cfg *config.Config, s *state.State, p *pipeline.Pipeline, workspaceID string) error {
	// Build done set from state
	done := map[string]bool{}
	for _, iss := range s.Issues {
		if iss.Status == state.StatusDone {
			done[iss.ID] = true
		}
	}
	// Handle --from: mark earlier issues as done for dep resolution
	if runFrom != "" {
		marking := true
		for _, iss := range s.Issues {
			if iss.ID == runFrom {
				marking = false
				break
			}
			if marking {
				done[iss.ID] = true
			}
		}
	}

	inFlight := map[string]bool{}
	for {
		ready := pipeline.ReadyIssues(p, done, inFlight)
		if len(ready) == 0 {
			break
		}
		for _, id := range ready {
			issue, err := s.Find(id)
			if err != nil {
				continue
			}
			if issue.Status == state.StatusDone {
				done[id] = true
				continue
			}
			fmt.Printf("\n[%s] %s\n", issue.ID, issue.Slug)
			succeeded, lastErr := runIssueRetry(dir, project, cfg, s, issue, workspaceID, nil)
			fin := time.Now().UTC()
			issue.FinishedAt = &fin
			if !succeeded {
				issue.Status = state.StatusFailed
				_ = state.Write(dir, project, s)
				fmt.Fprintf(os.Stderr, "[%s] failed after %d attempts: %v\n", issue.ID, cfg.MaxRetries, lastErr)
				if cfg.StopOnFailure && !runContinue {
					return fmt.Errorf("stopping on failure (use --continue to proceed)")
				}
			} else {
				issue.Status = state.StatusDone
				_ = state.Write(dir, project, s)
				fmt.Printf("  [%s] done ✓\n", issue.ID)
			}
			done[id] = true // mark as resolved (done or failed) to unblock deps
		}
	}
	return nil
}

type issueResult struct {
	id      string
	success bool
	err     error
}

// runPipelineParallel runs issues in parallel using the pipeline and semaphore.
func runPipelineParallel(dir, project string, cfg *config.Config, s *state.State, p *pipeline.Pipeline, workspaceID string) error {
	// Count total issues across all paths
	totalIssues := 0
	for _, ids := range p.Paths {
		totalIssues += len(ids)
	}

	sem := make(chan struct{}, cfg.MaxParallel)
	results := make(chan issueResult, totalIssues)
	var wg sync.WaitGroup
	var mu sync.Mutex

	done := map[string]bool{}
	inFlight := map[string]bool{}
	stopping := false
	anyFailed := false

	// Pre-populate done from state
	for _, iss := range s.Issues {
		if iss.Status == state.StatusDone {
			done[iss.ID] = true
		}
	}
	// Handle --from: mark earlier issues as skipped/done for dep resolution
	if runFrom != "" {
		marking := true
		for _, iss := range s.Issues {
			if iss.ID == runFrom {
				marking = false
				break
			}
			if marking {
				done[iss.ID] = true
			}
		}
	}

	processResult := func(r issueResult) {
		mu.Lock()
		defer mu.Unlock()
		delete(inFlight, r.id)
		issue, err := s.Find(r.id)
		if err != nil {
			return
		}
		fin := time.Now().UTC()
		issue.FinishedAt = &fin
		if r.success {
			issue.Status = state.StatusDone
			done[r.id] = true
			_ = state.Write(dir, project, s)
			fmt.Printf("  [%s] done ✓\n", r.id)
		} else {
			issue.Status = state.StatusFailed
			anyFailed = true
			_ = state.Write(dir, project, s)
			fmt.Fprintf(os.Stderr, "[%s] failed: %v\n", r.id, r.err)
			if cfg.StopOnFailure && !runContinue {
				stopping = true
			}
		}
	}

	// Sort path keys for determinism
	// (ReadyIssues sorts internally)

	for {
		mu.Lock()
		isStopping := stopping
		mu.Unlock()

		if isStopping {
			wg.Wait()
			// drain remaining results
			for {
				select {
				case r := <-results:
					processResult(r)
				default:
					goto done
				}
			}
		}

		mu.Lock()
		ready := pipeline.ReadyIssues(p, done, inFlight)
		mu.Unlock()

		if len(ready) == 0 {
			mu.Lock()
			numInFlight := len(inFlight)
			mu.Unlock()
			if numInFlight == 0 {
				break // all done or deadlock
			}
			// wait for a result
			r := <-results
			processResult(r)
			continue
		}

		for _, id := range ready {
			issueID := id
			// acquire semaphore slot
			select {
			case sem <- struct{}{}:
				// got slot
			default:
				// no slot available, wait for a result first
				r := <-results
				processResult(r)
				sem <- struct{}{} // now acquire
			}

			mu.Lock()
			inFlight[issueID] = true
			mu.Unlock()

			issue, err := s.Find(issueID)
			if err != nil {
				<-sem
				mu.Lock()
				delete(inFlight, issueID)
				mu.Unlock()
				continue
			}

			wg.Add(1)
			go func(iss *state.IssueState) {
				defer wg.Done()
				defer func() { <-sem }()

				fmt.Printf("\n[%s] %s\n", iss.ID, iss.Slug)
				succeeded, lastErr := runIssueRetry(dir, project, cfg, s, iss, workspaceID, &mu)
				results <- issueResult{id: iss.ID, success: succeeded, err: lastErr}
			}(issue)
		}

		// non-blocking drain of results
		for {
			select {
			case r := <-results:
				processResult(r)
			default:
				goto nextIteration
			}
		}
	nextIteration:
	}

done:
	wg.Wait()
	// drain remaining results
	for {
		select {
		case r := <-results:
			processResult(r)
		default:
			if anyFailed && cfg.StopOnFailure && !runContinue {
				return fmt.Errorf("stopping on failure (use --continue to proceed)")
			}
			return nil
		}
	}
}

// runIssueRetry runs one issue with retries. mu may be nil (sequential path).
func runIssueRetry(dir, project string, cfg *config.Config, s *state.State, issue *state.IssueState, workspaceID string, mu *sync.Mutex) (bool, error) {
	timeout := time.Duration(cfg.TimeoutMinutes) * time.Minute
	maxRetries := cfg.MaxRetries
	if maxRetries < 1 {
		maxRetries = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			fmt.Printf("  retry %d/%d...\n", attempt, maxRetries)
		}

		promptFile, err := prompt.Assemble(dir, cfg, toIssue(issue))
		if err != nil {
			return false, fmt.Errorf("[%s] assemble prompt: %w", issue.ID, err)
		}

		logFile := pi.LogFile(dir, issue.Slug, attempt)
		if err := os.MkdirAll(fmt.Sprintf("%s/.ralph/logs", dir), 0o755); err != nil {
			return false, fmt.Errorf("create logs dir: %w", err)
		}

		agentName := fmt.Sprintf("%s-%s-%s", cfg.PiSessionPrefix, project, issue.ID)
		argv := pi.PaneArgv(dir, cfg, issue.Slug, promptFile, logFile, issue.Model)

		herdr.AgentCloseByName(agentName)
		paneID, err := herdr.AgentStart(agentName, dir, workspaceID, argv)
		if err != nil {
			return false, fmt.Errorf("[%s] agent start: %w", issue.ID, err)
		}
		_ = herdr.PaneRename(paneID, agentName)

		now := time.Now().UTC()
		if mu != nil {
			mu.Lock()
		}
		issue.Status = state.StatusRunning
		issue.StartedAt = &now
		issue.HerdrPaneID = paneID
		_ = state.Write(dir, project, s)
		if mu != nil {
			mu.Unlock()
		}
		fmt.Printf("  pane %s (%s)\n", agentName, paneID)
		fmt.Printf("  waiting (timeout %dm)...\n", cfg.TimeoutMinutes)

		matched, waitErr := herdr.WaitOutput(paneID, "RALPH_DONE:", timeout)

		// Read session ID sidecar (best-effort)
		sidecarPath := pi.SessionIDPath(logFile)
		if data, rerr := os.ReadFile(sidecarPath); rerr == nil {
			if mu != nil {
				mu.Lock()
			}
			issue.PiSessionID = string(data)
			if mu != nil {
				mu.Unlock()
			}
		}

		if closePanes || waitErr != nil || matched != "RALPH_DONE:0" {
			_ = herdr.PaneClose(paneID)
		}

		if waitErr != nil {
			lastErr = fmt.Errorf("timeout: %w", waitErr)
			fmt.Fprintf(os.Stderr, "  [%s] attempt %d timeout\n", issue.ID, attempt)
			continue
		}
		if matched != "RALPH_DONE:0" {
			lastErr = fmt.Errorf("exit: %s", matched)
			fmt.Fprintf(os.Stderr, "  [%s] attempt %d failed (%s)\n", issue.ID, attempt, matched)
			continue
		}

		return true, nil
	}
	return false, lastErr
}

func init() {
	runCmd.Flags().StringVar(&runDir, "dir", mustCwd(), "project directory")
	runCmd.Flags().StringVar(&runFrom, "from", "", "start from issue number (e.g. 003)")
	runCmd.Flags().BoolVar(&runContinue, "continue", false, "continue past failures")
	runCmd.Flags().BoolVar(&runDryRun, "dry-run", false, "preview without running pi")
	runCmd.Flags().BoolVar(&closePanes, "close-panes", false, "close herdr panes after each issue completes")
	runCmd.Flags().StringVar(&runWorkspace, "workspace", "", "herdr workspace ID (set automatically on self-launch)")
}
