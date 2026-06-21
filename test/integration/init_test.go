package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

func TestInit_ParallelFlag_GeneratesPipeline(t *testing.T) {
	bin := buildBinary(t)
	binDir := makeBinDir(t, map[string]string{
		"herdr": buildFakeHerdr(t),
		"pi":    buildFakePi(t),
	})
	prependPath(t, binDir)

	fix := makeFixtureN(t, 4)
	stdout, _, code := runCmd(t, bin, nil, "init", "test-project", "--dir", fix, "--parallel", "2")
	if code != 0 {
		t.Fatalf("exit %d; stdout: %s", code, stdout)
	}

	data, err := os.ReadFile(filepath.Join(fix, ".ralph", "pipeline.yaml"))
	if err != nil {
		t.Fatalf("pipeline.yaml missing: %v", err)
	}

	var p struct {
		Paths map[string][]string `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &p); err != nil {
		t.Fatalf("parse pipeline.yaml: %v", err)
	}
	if len(p.Paths) != 2 {
		t.Errorf("want 2 paths, got %d; yaml: %s", len(p.Paths), data)
	}
	total := 0
	for _, ids := range p.Paths {
		total += len(ids)
	}
	if total != 4 {
		t.Errorf("want 4 total IDs, got %d; yaml: %s", total, data)
	}
}

func TestInit_Sequential_GeneratesSinglePathPipeline(t *testing.T) {
	bin := buildBinary(t)
	binDir := makeBinDir(t, map[string]string{
		"herdr": buildFakeHerdr(t),
		"pi":    buildFakePi(t),
	})
	prependPath(t, binDir)

	fix := makeFixtureN(t, 2)
	stdout, _, code := runCmd(t, bin, nil, "init", "test-project", "--dir", fix, "--parallel", "1")
	if code != 0 {
		t.Fatalf("exit %d; stdout: %s", code, stdout)
	}

	data, err := os.ReadFile(filepath.Join(fix, ".ralph", "pipeline.yaml"))
	if err != nil {
		t.Fatalf("pipeline.yaml missing: %v", err)
	}

	var p struct {
		Paths map[string][]string `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &p); err != nil {
		t.Fatalf("parse pipeline.yaml: %v", err)
	}
	ids, ok := p.Paths["main"]
	if !ok {
		t.Fatalf("want 'main' path; yaml: %s", data)
	}
	if len(ids) != 2 {
		t.Errorf("want 2 IDs in main, got %d; yaml: %s", len(ids), data)
	}
}

func TestInit_ModelSeededInState(t *testing.T) {
	bin := buildBinary(t)
	binDir := makeBinDir(t, map[string]string{
		"herdr": buildFakeHerdr(t),
		"pi":    buildFakePi(t),
	})
	prependPath(t, binDir)

	fix := makeFixtureN(t, 2)
	// Write config with default_model
	os.MkdirAll(filepath.Join(fix, ".ralph"), 0o755)
	os.WriteFile(filepath.Join(fix, ".ralph", "config.yaml"),
		[]byte("skills: []\ncontext_files:\n  - PRD.md\ndefault_model: gpt-4o\n"), 0o644)

	stdout, _, code := runCmd(t, bin, nil, "init", "test-project", "--dir", fix, "--parallel", "1")
	if code != 0 {
		t.Fatalf("exit %d; stdout: %s", code, stdout)
	}

	data, err := os.ReadFile(filepath.Join(fix, ".ralph", "test-project.json"))
	if err != nil {
		t.Fatalf("state file missing: %v", err)
	}

	var s struct {
		Issues []stateIssue `json:"issues"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	for _, iss := range s.Issues {
		if iss.Model != "gpt-4o" {
			t.Errorf("issue %s: want model=gpt-4o, got %q", iss.ID, iss.Model)
		}
	}
}
