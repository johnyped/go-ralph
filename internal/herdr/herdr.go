package herdr

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// WorkspaceCreateResult holds the result of a workspace create call.
type WorkspaceCreateResult struct {
	WorkspaceID string
	RootPaneID  string
}

// WorkspaceCreate creates a new herdr workspace and returns its IDs.
func WorkspaceCreate(cwd, label string) (WorkspaceCreateResult, error) {
	out, err := run("workspace", "create", "--cwd", cwd, "--label", label, "--no-focus")
	if err != nil {
		return WorkspaceCreateResult{}, err
	}
	var resp struct {
		Result struct {
			Workspace struct{ WorkspaceID string `json:"workspace_id"` } `json:"workspace"`
			RootPane  struct{ PaneID string `json:"pane_id"` } `json:"root_pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return WorkspaceCreateResult{}, fmt.Errorf("workspace create parse: %w\nraw: %s", err, out)
	}
	return WorkspaceCreateResult{
		WorkspaceID: resp.Result.Workspace.WorkspaceID,
		RootPaneID:  resp.Result.RootPane.PaneID,
	}, nil
}

// AgentCloseByName closes any existing agent pane with the given name.
// This prevents agent_name_taken errors when re-running after a forceful kill.
func AgentCloseByName(name string) {
	out, err := run("agent", "list")
	if err != nil {
		return
	}
	var resp struct {
		Result struct {
			Agents []struct {
				Name   string `json:"name"`
				PaneID string `json:"pane_id"`
			} `json:"agents"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return
	}
	for _, a := range resp.Result.Agents {
		if a.Name == name {
			_, _ = run("pane", "close", a.PaneID)
		}
	}
}

// AgentStart spawns a named agent in a new pane and returns the pane ID.
func AgentStart(name, cwd, workspaceID string, argv []string) (string, error) {
	args := []string{
		"agent", "start", name,
		"--cwd", cwd,
		"--workspace", workspaceID,
		"--split", "down",
		"--no-focus",
		"--",
	}
	args = append(args, argv...)
	out, err := run(args...)
	if err != nil {
		return "", err
	}
	var resp struct {
		Result struct {
			Agent struct{ PaneID string `json:"pane_id"` } `json:"agent"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return "", fmt.Errorf("agent start parse: %w\nraw: %s", err, out)
	}
	return resp.Result.Agent.PaneID, nil
}

// WaitOutput waits for match to appear in pane output, returns the matched line.
func WaitOutput(paneID, match string, timeout time.Duration) (string, error) {
	ms := fmt.Sprintf("%d", int(timeout.Milliseconds()))
	out, err := run("wait", "output", paneID,
		"--match", match,
		"--source", "recent-unwrapped",
		"--timeout", ms,
	)
	if err != nil {
		return "", err
	}
	var resp struct {
		Result struct{ MatchedLine string `json:"matched_line"` } `json:"result"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return "", fmt.Errorf("wait output parse: %w\nraw: %s", err, out)
	}
	return resp.Result.MatchedLine, nil
}

// PaneClose closes a pane.
func PaneClose(paneID string) error {
	_, err := run("pane", "close", paneID)
	return err
}

// PaneRename sets the display label of a pane.
func PaneRename(paneID, label string) error {
	_, err := run("pane", "rename", paneID, label)
	return err
}

// PaneRun sends a command string followed by Enter to a pane.
func PaneRun(paneID, command string) error {
	_, err := run("pane", "run", paneID, command)
	return err
}

func run(args ...string) (string, error) {
	cmd := exec.Command("herdr", args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("herdr %s: %w\nstderr: %s", strings.Join(args, " "), err, ee.Stderr)
		}
		return "", fmt.Errorf("herdr %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}
