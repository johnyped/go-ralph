package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

type IssueState struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	File        string     `json:"file"`
	Status      Status     `json:"status"`
	StartedAt   *time.Time `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	HerdrPaneID string     `json:"herdr_pane_id,omitempty"`
	PiSessionID string     `json:"pi_session_id,omitempty"`
}

type State struct {
	UpdatedAt time.Time    `json:"updated_at"`
	Issues    []IssueState `json:"issues"`
}

func StatePath(dir, project string) string {
	return filepath.Join(dir, ".ralph", project+".json")
}

func Load(dir, project string) (*State, error) {
	data, err := os.ReadFile(StatePath(dir, project))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("project %q not initialised — run: go-ralph init %s", project, project)
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Write saves state atomically via a temp file + rename.
func Write(dir, project string, s *State) error {
	s.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := StatePath(dir, project)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *State) Find(id string) (*IssueState, error) {
	for i := range s.Issues {
		if s.Issues[i].ID == id {
			return &s.Issues[i], nil
		}
	}
	return nil, fmt.Errorf("issue %s not found in state", id)
}
