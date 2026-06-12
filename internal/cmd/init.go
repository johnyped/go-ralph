package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/johnyped/go-ralph/internal/config"
	"github.com/johnyped/go-ralph/internal/issues"
	"github.com/johnyped/go-ralph/internal/state"
	"github.com/spf13/cobra"
)

var initDir string

var initCmd = &cobra.Command{
	Use:   "init <project_name>",
	Short: "Initialize ralph for a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]
		dir := initDir

		fmt.Printf("Initializing ralph for project %q in %s\n\n", project, dir)

		// --- Checks ---
		ok := true

		// pi binary
		piPath, err := exec.LookPath("pi")
		if err != nil {
			fmt.Println("  pi:    ✗ not found on PATH")
			ok = false
		} else {
			fmt.Printf("  pi:    ✓ %s\n", piPath)
		}

		// herdr binary
		herdrPath, err := exec.LookPath("herdr")
		if err != nil {
			fmt.Println("  herdr: ✗ not found on PATH")
			ok = false
		} else {
			fmt.Printf("  herdr: ✓ %s\n", herdrPath)
		}

		// herdr-pi integration (warning only — screen heuristics work without it)
		piIntegration := herdrPiExtensionPath()
		if _, err := os.Stat(piIntegration); err != nil {
			fmt.Printf("  herdr-pi integration: ⚠ not found at %s (optional)\n", piIntegration)
			fmt.Println("    run: herdr integration install pi  (for better state reporting)")
		} else {
			fmt.Printf("  herdr-pi integration: ✓ %s\n", piIntegration)
		}

		// issues dir
		issueList, err := issues.Scan(dir)
		if err != nil || len(issueList) == 0 {
			fmt.Printf("  issues/: ✗ no issues found in %s\n", filepath.Join(dir, "issues"))
			ok = false
		} else {
			fmt.Printf("  issues/: ✓ %d issues found\n", len(issueList))
		}

		// config
		cfg, _ := config.Load(dir)

		// context files
		for _, cf := range cfg.ContextFiles {
			p := resolvePathInit(dir, cf)
			if _, err := os.Stat(p); err != nil {
				fmt.Printf("  context %s: ✗ missing\n", cf)
			} else {
				fmt.Printf("  context %s: ✓\n", cf)
			}
		}

		fmt.Println()

		if !ok {
			return fmt.Errorf("pre-flight checks failed — fix the issues above and re-run")
		}

		// --- Write config if not exists ---
		cfgPath := config.ConfigPath(dir)
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			if err := config.Write(dir, cfg); err != nil {
				return fmt.Errorf("write config: %w", err)
			}
			fmt.Printf("Created %s\n", cfgPath)
		} else {
			fmt.Printf("Config exists: %s\n", cfgPath)
		}

		// --- Write state ---
		s := &state.State{}
		for _, issue := range issueList {
			s.Issues = append(s.Issues, state.IssueState{
				ID:     issue.ID,
				Slug:   issue.Slug,
				File:   issue.File,
				Status: state.StatusPending,
			})
		}
		if err := state.Write(dir, project, s); err != nil {
			return fmt.Errorf("write state: %w", err)
		}
		fmt.Printf("Created %s\n", state.StatePath(dir, project))

		// --- Print execution plan ---
		fmt.Println("\nExecution order:")
		for _, issue := range issueList {
			fmt.Printf("  %s  %s\n", issue.ID, issue.Slug)
		}

		return nil
	},
}

func init() {
	initCmd.Flags().StringVar(&initDir, "dir", mustCwd(), "project directory")
}

func herdrPiExtensionPath() string {
	piAgentDir := os.Getenv("PI_CODING_AGENT_DIR")
	if piAgentDir == "" {
		home, _ := os.UserHomeDir()
		piAgentDir = filepath.Join(home, ".pi", "agent")
	}
	return filepath.Join(piAgentDir, "extensions", "herdr-agent-state.ts")
}

func resolvePathInit(dir, p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, p[2:])
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}

func mustCwd() string {
	cwd, _ := os.Getwd()
	return cwd
}
