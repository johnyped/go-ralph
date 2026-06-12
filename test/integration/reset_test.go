package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReset_MarksIssuePending(t *testing.T) {
	bin := buildBinary(t)
	fix := makeFixture(t)
	started := "2024-01-01T00:00:00Z"
	writeState(t, fix, "test-project", []stateIssue{
		{ID: "001", Slug: "001-init-check", File: "issues/001-init-check.md", Status: "done", StartedAt: &started},
	})

	stdout, _, code := runCmd(t, bin, nil, "reset", "test-project", "001", "--dir", fix)
	if code != 0 {
		t.Fatalf("exit %d; stdout: %s", code, stdout)
	}
	if !strings.Contains(stdout, "reset to pending") {
		t.Errorf("want 'reset to pending'; got: %s", stdout)
	}

	data, err := os.ReadFile(filepath.Join(fix, ".ralph", "test-project.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Issues []struct {
			ID        string     `json:"id"`
			Status    string     `json:"status"`
			StartedAt *time.Time `json:"started_at"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	if s.Issues[0].Status != "pending" {
		t.Errorf("want status=pending, got %q", s.Issues[0].Status)
	}
	if s.Issues[0].StartedAt != nil {
		t.Errorf("want started_at=null, got %v", s.Issues[0].StartedAt)
	}
}

func TestReset_UnknownIssue(t *testing.T) {
	bin := buildBinary(t)
	fix := makeFixture(t)
	writeState(t, fix, "test-project", []stateIssue{
		{ID: "001", Slug: "001-init-check", File: "issues/001-init-check.md", Status: "pending"},
	})

	stdout, stderr, code := runCmd(t, bin, nil, "reset", "test-project", "999", "--dir", fix)
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(stdout+stderr, "not found in state") {
		t.Errorf("want 'not found in state'; got stdout=%s stderr=%s", stdout, stderr)
	}
}
