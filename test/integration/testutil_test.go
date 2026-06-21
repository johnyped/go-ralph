package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	testBinDir    string
	onceBin       sync.Once
	binPath       string
	onceFakeHerdr sync.Once
	fakeHerdrBin  string
	onceFakePi    sync.Once
	fakePiBin     string
)

func TestMain(m *testing.M) {
	var err error
	testBinDir, err = os.MkdirTemp("", "go-ralph-test-bin-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "MkdirTemp: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(testBinDir)
	os.Exit(m.Run())
}

func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("go.mod not found")
		}
		dir = parent
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	onceBin.Do(func() {
		out := filepath.Join(testBinDir, "go-ralph")
		cmd := exec.Command("go", "build", "-o", out, "./cmd/go-ralph/")
		cmd.Dir = repoRoot()
		if b, err := cmd.CombinedOutput(); err != nil {
			panic(fmt.Sprintf("build go-ralph: %v\n%s", err, b))
		}
		binPath = out
	})
	return binPath
}

func buildFakeHerdr(t *testing.T) string {
	t.Helper()
	onceFakeHerdr.Do(func() {
		out := filepath.Join(testBinDir, "fake-herdr")
		cmd := exec.Command("go", "build", "-o", out, "./test/integration/testdata/fake-herdr/")
		cmd.Dir = repoRoot()
		if b, err := cmd.CombinedOutput(); err != nil {
			panic(fmt.Sprintf("build fake-herdr: %v\n%s", err, b))
		}
		fakeHerdrBin = out
	})
	return fakeHerdrBin
}

func buildFakePi(t *testing.T) string {
	t.Helper()
	onceFakePi.Do(func() {
		out := filepath.Join(testBinDir, "fake-pi")
		cmd := exec.Command("go", "build", "-o", out, "./test/integration/testdata/fake-pi/")
		cmd.Dir = repoRoot()
		if b, err := cmd.CombinedOutput(); err != nil {
			panic(fmt.Sprintf("build fake-pi: %v\n%s", err, b))
		}
		fakePiBin = out
	})
	return fakePiBin
}

// makeBinDir creates a temp dir and copies each binary under the given name.
func makeBinDir(t *testing.T, binaries map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range binaries {
		dst := filepath.Join(dir, name)
		copyFile(t, src, dst)
	}
	return dir
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
}

func prependPath(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}

func makeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "issues"), 0o755)
	os.WriteFile(filepath.Join(dir, "issues", "001-init-check.md"), []byte("# Issue 001\n\n- [ ] check something\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "PRD.md"), []byte("# PRD\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, ".ralph"), 0o755)
	os.WriteFile(filepath.Join(dir, ".ralph", "config.yaml"), []byte("skills: []\ncontext_files:\n  - PRD.md\n"), 0o644)
	return dir
}

func makeFixtureN(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "issues"), 0o755)
	names := []string{"first", "second", "third", "fourth", "fifth"}
	for i := 1; i <= n; i++ {
		name := "issue"
		if i <= len(names) {
			name = names[i-1]
		}
		filename := fmt.Sprintf("%03d-%s.md", i, name)
		os.WriteFile(filepath.Join(dir, "issues", filename), []byte(fmt.Sprintf("# Issue %03d\n\n- [ ] step\n", i)), 0o644)
	}
	os.WriteFile(filepath.Join(dir, "PRD.md"), []byte("# PRD\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, ".ralph"), 0o755)
	os.WriteFile(filepath.Join(dir, ".ralph", "config.yaml"), []byte("skills: []\ncontext_files:\n  - PRD.md\n"), 0o644)
	return dir
}

func runCmd(t *testing.T, bin string, env []string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), env...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// writeState writes a state JSON directly to fixture/.ralph/<project>.json
func writeState(t *testing.T, dir, project string, issues []stateIssue) {
	t.Helper()
	os.MkdirAll(filepath.Join(dir, ".ralph"), 0o755)
	type stateFile struct {
		UpdatedAt string       `json:"updated_at"`
		Issues    []stateIssue `json:"issues"`
	}
	s := stateFile{
		UpdatedAt: "2024-01-01T00:00:00Z",
		Issues:    issues,
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".ralph", project+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

type stateIssue struct {
	ID          string  `json:"id"`
	Slug        string  `json:"slug"`
	File        string  `json:"file"`
	Status      string  `json:"status"`
	StartedAt   *string `json:"started_at"`
	FinishedAt  *string `json:"finished_at"`
	PiSessionID string  `json:"pi_session_id,omitempty"`
	Model       string  `json:"model,omitempty"`
}
