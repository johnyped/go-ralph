package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogs_NoSession(t *testing.T) {
	bin := buildBinary(t)
	fix := makeFixture(t)
	writeState(t, fix, "test-project", []stateIssue{
		{ID: "001", Slug: "001-init-check", File: "issues/001-init-check.md", Status: "done"},
	})

	stdout, _, code := runCmd(t, bin, nil, "logs", "test-project", "001", "--dir", fix)
	if code != 0 {
		t.Fatalf("exit %d; stdout: %s", code, stdout)
	}
	if !strings.Contains(stdout, "(not recorded)") {
		t.Errorf("want '(not recorded)'; got: %s", stdout)
	}
}

func TestLogs_WithSessionSidecar(t *testing.T) {
	bin := buildBinary(t)
	fix := makeFixture(t)
	writeState(t, fix, "test-project", []stateIssue{
		{ID: "001", Slug: "001-init-check", File: "issues/001-init-check.md", Status: "done"},
	})

	logsDir := filepath.Join(fix, ".ralph", "logs")
	os.MkdirAll(logsDir, 0o755)
	os.WriteFile(filepath.Join(logsDir, "001-init-check.jsonl.session-id"), []byte("abc-123"), 0o644)

	stdout, _, code := runCmd(t, bin, nil, "logs", "test-project", "001", "--dir", fix)
	if code != 0 {
		t.Fatalf("exit %d; stdout: %s", code, stdout)
	}
	if !strings.Contains(stdout, "abc-123") {
		t.Errorf("want 'abc-123' in stdout; got: %s", stdout)
	}
	if !strings.Contains(stdout, "pi --session abc-123") {
		t.Errorf("want 'pi --session abc-123'; got: %s", stdout)
	}
}

func TestLogs_UnknownIssue(t *testing.T) {
	bin := buildBinary(t)
	fix := makeFixture(t)
	writeState(t, fix, "test-project", []stateIssue{
		{ID: "001", Slug: "001-init-check", File: "issues/001-init-check.md", Status: "done"},
	})

	_, _, code := runCmd(t, bin, nil, "logs", "test-project", "999", "--dir", fix)
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
}
