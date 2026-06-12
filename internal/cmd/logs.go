package cmd

import (
	"fmt"
	"os"

	"github.com/johnyped/go-ralph/internal/pi"
	"github.com/johnyped/go-ralph/internal/state"
	"github.com/spf13/cobra"
)

var logsDir string

var logsCmd = &cobra.Command{
	Use:   "logs <project_name> <issue-id>",
	Short: "Show log paths and pi session info for an issue",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]
		id := args[1]

		s, err := state.Load(logsDir, project)
		if err != nil {
			return fmt.Errorf("load state: %w", err)
		}

		issue, err := s.Find(id)
		if err != nil {
			return err
		}

		logFile := pi.LogFile(logsDir, issue.Slug, 1)
		sidecarPath := pi.SessionIDPath(logFile)

		fmt.Printf("Issue   : [%s] %s\n", issue.ID, issue.Slug)
		fmt.Printf("Status  : %s\n", issue.Status)
		fmt.Printf("Log     : %s\n", logFile)

		// Session ID: prefer state, fall back to sidecar on disk
		sessionID := issue.PiSessionID
		if sessionID == "" {
			if data, rerr := os.ReadFile(sidecarPath); rerr == nil {
				sessionID = string(data)
			}
		}

		if sessionID != "" {
			fmt.Printf("Session : %s\n", sessionID)
			fmt.Printf("Resume  : pi --session %s\n", sessionID)
		} else {
			fmt.Println("Session : (not recorded)")
		}

		return nil
	},
}

func init() {
	logsCmd.Flags().StringVar(&logsDir, "dir", mustCwd(), "project directory")
}
