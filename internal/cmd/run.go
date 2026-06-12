package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/johnyped/go-ralph/internal/config"
	"github.com/johnyped/go-ralph/internal/herdr"
	"github.com/johnyped/go-ralph/internal/pi"
	"github.com/johnyped/go-ralph/internal/prompt"
	"github.com/johnyped/go-ralph/internal/state"
	"github.com/spf13/cobra"
)

var (
	runDir        string
	runFrom       string
	runContinue   bool
	runDryRun     bool
	closePanes    bool
	runWorkspace  string
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

			timeout := time.Duration(cfg.TimeoutMinutes) * time.Minute
			maxRetries := cfg.MaxRetries
			if maxRetries < 1 {
				maxRetries = 1
			}

			var lastErr error
			succeeded := false

			for attempt := 1; attempt <= maxRetries; attempt++ {
				if attempt > 1 {
					fmt.Printf("  retry %d/%d...\n", attempt, maxRetries)
				}

				// Assemble prompt fresh each attempt (picks up any checked boxes)
				promptFile, err := prompt.Assemble(dir, cfg, toIssue(issue))
				if err != nil {
					return fmt.Errorf("[%s] assemble prompt: %w", issue.ID, err)
				}

				logFile := pi.LogFile(dir, issue.Slug, attempt)
				if err := os.MkdirAll(fmt.Sprintf("%s/.ralph/logs", dir), 0o755); err != nil {
					return fmt.Errorf("create logs dir: %w", err)
				}

				agentName := fmt.Sprintf("%s-%s", cfg.PiSessionPrefix, issue.ID)
				argv := pi.PaneArgv(dir, cfg, issue.Slug, promptFile, logFile)

				herdr.AgentCloseByName(agentName)
				paneID, err := herdr.AgentStart(agentName, dir, workspaceID, argv)
				if err != nil {
					return fmt.Errorf("[%s] agent start: %w", issue.ID, err)
				}
				_ = herdr.PaneRename(paneID, agentName)

				now := time.Now().UTC()
				issue.Status = state.StatusRunning
				issue.StartedAt = &now
				issue.HerdrPaneID = paneID
				_ = state.Write(dir, project, s)
				fmt.Printf("  pane %s (%s)\n", agentName, paneID)
				fmt.Printf("  waiting (timeout %dm)...\n", cfg.TimeoutMinutes)

				matched, waitErr := herdr.WaitOutput(paneID, "RALPH_DONE:", timeout)

				// Read session ID sidecar (best-effort)
				sidecarPath := pi.SessionIDPath(logFile)
				if data, rerr := os.ReadFile(sidecarPath); rerr == nil {
					issue.PiSessionID = string(data)
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

				succeeded = true
				break
			}

			fin := time.Now().UTC()
			issue.FinishedAt = &fin

			if !succeeded {
				issue.Status = state.StatusFailed
				_ = state.Write(dir, project, s)
				fmt.Fprintf(os.Stderr, "[%s] failed after %d attempts: %v\n", issue.ID, maxRetries, lastErr)
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
	},
}

func init() {
	runCmd.Flags().StringVar(&runDir, "dir", mustCwd(), "project directory")
	runCmd.Flags().StringVar(&runFrom, "from", "", "start from issue number (e.g. 003)")
	runCmd.Flags().BoolVar(&runContinue, "continue", false, "continue past failures")
	runCmd.Flags().BoolVar(&runDryRun, "dry-run", false, "preview without running pi")
	runCmd.Flags().BoolVar(&closePanes, "close-panes", false, "close herdr panes after each issue completes")
	runCmd.Flags().StringVar(&runWorkspace, "workspace", "", "herdr workspace ID (set automatically on self-launch)")
}
