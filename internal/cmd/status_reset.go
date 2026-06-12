package cmd

import (
	"fmt"

	"github.com/johnyped/go-ralph/internal/issues"
	"github.com/johnyped/go-ralph/internal/state"
	"github.com/spf13/cobra"
)

var statusDir string

var statusCmd = &cobra.Command{
	Use:   "status <project_name>",
	Short: "Show issue states for a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]
		s, err := state.Load(statusDir, project)
		if err != nil {
			return fmt.Errorf("load state: %w", err)
		}

		icons := map[state.Status]string{
			state.StatusPending: "○",
			state.StatusRunning: "◌",
			state.StatusDone:    "✓",
			state.StatusFailed:  "✗",
		}

		fmt.Printf("Project: %s  (updated %s)\n\n", project, s.UpdatedAt.Format("2006-01-02 15:04:05"))
		for _, issue := range s.Issues {
			icon := icons[issue.Status]
			fmt.Printf("  %s  [%s] %s\n", icon, issue.ID, issue.Slug)
		}
		return nil
	},
}

var (
	resetDir string
)

var resetCmd = &cobra.Command{
	Use:   "reset <project_name> <issue-number>",
	Short: "Mark an issue as pending so it will re-run",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]
		id := args[1]

		s, err := state.Load(resetDir, project)
		if err != nil {
			return fmt.Errorf("load state: %w", err)
		}

		issue, err := s.Find(id)
		if err != nil {
			return err
		}

		issue.Status = state.StatusPending
		issue.StartedAt = nil
		issue.FinishedAt = nil
		issue.HerdrPaneID = ""

		if err := state.Write(resetDir, project, s); err != nil {
			return fmt.Errorf("write state: %w", err)
		}
		fmt.Printf("Issue %s (%s) reset to pending\n", id, issue.Slug)
		return nil
	},
}

func init() {
	statusCmd.Flags().StringVar(&statusDir, "dir", mustCwd(), "project directory")
	resetCmd.Flags().StringVar(&resetDir, "dir", mustCwd(), "project directory")
}

// toIssue converts an IssueState back to an issues.Issue for prompt assembly.
func toIssue(is *state.IssueState) issues.Issue {
	return issues.Issue{ID: is.ID, Slug: is.Slug, File: is.File}
}
