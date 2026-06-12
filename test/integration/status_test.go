package integration_test

import (
	"strings"
	"testing"
)

func TestStatus_ShowsAllIssues(t *testing.T) {
	bin := buildBinary(t)
	fix := makeFixture(t)
	writeState(t, fix, "test-project", []stateIssue{
		{ID: "001", Slug: "001-init-check", File: "issues/001-init-check.md", Status: "pending"},
		{ID: "002", Slug: "002-running", File: "issues/002-running.md", Status: "running"},
		{ID: "003", Slug: "003-done", File: "issues/003-done.md", Status: "done"},
		{ID: "004", Slug: "004-failed", File: "issues/004-failed.md", Status: "failed"},
	})

	stdout, _, code := runCmd(t, bin, nil, "status", "test-project", "--dir", fix)
	if code != 0 {
		t.Fatalf("exit %d; stdout: %s", code, stdout)
	}
	for _, want := range []string{"○", "◌", "✓", "✗"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("want %q in stdout; got: %s", want, stdout)
		}
	}
}

func TestStatus_Uninitialised(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()

	stdout, stderr, code := runCmd(t, bin, nil, "status", "test-project", "--dir", dir)
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	combined := stdout + stderr
	if !strings.Contains(combined, "not initialised") {
		t.Errorf("want 'not initialised'; got: %s", combined)
	}
}
