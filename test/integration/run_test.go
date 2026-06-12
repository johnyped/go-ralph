package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func initProject(t *testing.T, bin, fix string) {
	t.Helper()
	stdout, stderr, code := runCmd(t, bin, nil, "init", "test-project", "--dir", fix)
	if code != 0 {
		t.Fatalf("init failed (exit %d): stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestRunDryRun_PrintsIssues(t *testing.T) {
	bin := buildBinary(t)
	binDir := makeBinDir(t, map[string]string{
		"herdr": buildFakeHerdr(t),
		"pi":    buildFakePi(t),
	})
	prependPath(t, binDir)

	fix := makeFixtureN(t, 2)
	initProject(t, bin, fix)

	stdout, _, code := runCmd(t, bin, []string{"HERDR_ENV=1"}, "run", "test-project", "--dir", fix, "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d; stdout: %s", code, stdout)
	}
	if cnt := strings.Count(stdout, "dry-run: would run pi for"); cnt != 2 {
		t.Errorf("want 2 dry-run lines, got %d; stdout: %s", cnt, stdout)
	}
}

func TestRunDryRun_SkipsDone(t *testing.T) {
	bin := buildBinary(t)
	binDir := makeBinDir(t, map[string]string{
		"herdr": buildFakeHerdr(t),
		"pi":    buildFakePi(t),
	})
	prependPath(t, binDir)

	fix := makeFixtureN(t, 2)
	initProject(t, bin, fix)

	// Mark 001 as done in state
	statePath := filepath.Join(fix, ".ralph", "test-project.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		UpdatedAt string       `json:"updated_at"`
		Issues    []stateIssue `json:"issues"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	for i := range s.Issues {
		if s.Issues[i].ID == "001" {
			s.Issues[i].Status = "done"
		}
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(statePath, b, 0o644)

	stdout, _, code := runCmd(t, bin, []string{"HERDR_ENV=1"}, "run", "test-project", "--dir", fix, "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d; stdout: %s", code, stdout)
	}
	if strings.Contains(stdout, "001-first") {
		t.Errorf("001 should be skipped; stdout: %s", stdout)
	}
	if !strings.Contains(stdout, "002-second") {
		t.Errorf("want 002-second in stdout; got: %s", stdout)
	}
}

func TestRunDryRun_FromFlag(t *testing.T) {
	bin := buildBinary(t)
	binDir := makeBinDir(t, map[string]string{
		"herdr": buildFakeHerdr(t),
		"pi":    buildFakePi(t),
	})
	prependPath(t, binDir)

	fix := makeFixtureN(t, 2)
	initProject(t, bin, fix)

	stdout, _, code := runCmd(t, bin, []string{"HERDR_ENV=1"}, "run", "test-project", "--dir", fix, "--dry-run", "--from", "002")
	if code != 0 {
		t.Fatalf("exit %d; stdout: %s", code, stdout)
	}
	if strings.Contains(stdout, "001") {
		t.Errorf("001 should be skipped; stdout: %s", stdout)
	}
	if !strings.Contains(stdout, "002") {
		t.Errorf("want 002 in stdout; got: %s", stdout)
	}
}

func TestRunFakeHerdr_AllDone(t *testing.T) {
	bin := buildBinary(t)
	binDir := makeBinDir(t, map[string]string{
		"herdr": buildFakeHerdr(t),
		"pi":    buildFakePi(t),
	})
	prependPath(t, binDir)

	fix := makeFixtureN(t, 2)
	initProject(t, bin, fix)

	stdout, stderr, code := runCmd(t, bin,
		[]string{"HERDR_ENV=1"},
		"run", "test-project", "--dir", fix, "--workspace", "t1")
	if code != 0 {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if cnt := strings.Count(stdout, "done ✓"); cnt != 2 {
		t.Errorf("want 2 'done ✓', got %d; stdout: %s", cnt, stdout)
	}

	// Check state file
	data, _ := os.ReadFile(filepath.Join(fix, ".ralph", "test-project.json"))
	if strings.Count(string(data), `"status": "done"`) != 2 {
		t.Errorf("want 2 done issues in state; got: %s", data)
	}
}

func TestRunFakeHerdr_FailureSetsStateFailed(t *testing.T) {
	bin := buildBinary(t)
	binDir := makeBinDir(t, map[string]string{
		"herdr": buildFakeHerdr(t),
		"pi":    buildFakePi(t),
	})
	prependPath(t, binDir)

	fix := makeFixtureN(t, 1)
	initProject(t, bin, fix)

	_, _, code := runCmd(t, bin,
		[]string{"HERDR_ENV=1", `FAKE_HERDR_WAIT_RESULT={"result":{"matched_line":"RALPH_DONE:1"}}`},
		"run", "test-project", "--dir", fix, "--workspace", "t1")
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}

	data, _ := os.ReadFile(filepath.Join(fix, ".ralph", "test-project.json"))
	if !strings.Contains(string(data), `"status": "failed"`) {
		t.Errorf("want failed status in state; got: %s", data)
	}
}

func TestRunFakeHerdr_ContinueFlag(t *testing.T) {
	bin := buildBinary(t)
	binDir := makeBinDir(t, map[string]string{
		"herdr": buildFakeHerdr(t),
		"pi":    buildFakePi(t),
	})
	prependPath(t, binDir)

	fix := makeFixtureN(t, 2)
	initProject(t, bin, fix)

	stdout, stderr, _ := runCmd(t, bin,
		[]string{"HERDR_ENV=1", `FAKE_HERDR_WAIT_RESULT={"result":{"matched_line":"RALPH_DONE:1"}}`},
		"run", "test-project", "--dir", fix, "--workspace", "t1", "--continue")

	// Both issues should have been attempted (appear in stderr as failed)
	failedCount := strings.Count(stderr, "failed")
	if failedCount < 2 {
		t.Errorf("want both issues failed in stderr, got %d; stdout=%s stderr=%s", failedCount, stdout, stderr)
	}
}
