package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit_Success(t *testing.T) {
	bin := buildBinary(t)
	binDir := makeBinDir(t, map[string]string{
		"herdr": buildFakeHerdr(t),
		"pi":    buildFakePi(t),
	})
	prependPath(t, binDir)

	fix := makeFixture(t)
	stdout, _, code := runCmd(t, bin, nil, "init", "test-project", "--dir", fix)
	if code != 0 {
		t.Fatalf("exit %d; stdout: %s", code, stdout)
	}
	if !strings.Contains(stdout, "1 issues found") {
		t.Errorf("want '1 issues found' in stdout; got: %s", stdout)
	}
	if _, err := os.Stat(filepath.Join(fix, ".ralph", "test-project.json")); err != nil {
		t.Errorf("state file missing: %v", err)
	}
}

func TestInit_MissingIssues(t *testing.T) {
	bin := buildBinary(t)
	binDir := makeBinDir(t, map[string]string{
		"herdr": buildFakeHerdr(t),
		"pi":    buildFakePi(t),
	})
	prependPath(t, binDir)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "PRD.md"), []byte("# PRD\n"), 0o644)

	stdout, _, code := runCmd(t, bin, nil, "init", "test-project", "--dir", dir)
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(stdout, "no issues found") {
		t.Errorf("want 'no issues found'; got: %s", stdout)
	}
}

func TestInit_MissingBinaries(t *testing.T) {
	bin := buildBinary(t)
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	fix := makeFixture(t)
	stdout, _, code := runCmd(t, bin, nil, "init", "test-project", "--dir", fix)
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(stdout, "not found on PATH") {
		t.Errorf("want 'not found on PATH'; got: %s", stdout)
	}
}

func TestInit_IdempotentConfig(t *testing.T) {
	bin := buildBinary(t)
	binDir := makeBinDir(t, map[string]string{
		"herdr": buildFakeHerdr(t),
		"pi":    buildFakePi(t),
	})
	prependPath(t, binDir)

	fix := makeFixture(t)
	runCmd(t, bin, nil, "init", "test-project", "--dir", fix)
	stdout, _, code := runCmd(t, bin, nil, "init", "test-project", "--dir", fix)
	if code != 0 {
		t.Fatalf("second init exit %d; stdout: %s", code, stdout)
	}
	if !strings.Contains(stdout, "Config exists:") {
		t.Errorf("want 'Config exists:'; got: %s", stdout)
	}
}
