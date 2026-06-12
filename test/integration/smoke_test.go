//go:build integration

package integration_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFullLoop_RalphTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	if _, err := exec.LookPath("pi"); err != nil {
		t.Skip("pi not on PATH")
	}

	bin := buildBinary(t)
	root := repoRoot()
	exampleDir := filepath.Join(root, "example", "ralph-test")

	// 1. Reset
	out, err := exec.Command("bash", filepath.Join(exampleDir, "reset.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("reset.sh failed: %v\n%s", err, out)
	}

	// 2. Init
	stdout, stderr, code := runCmd(t, bin, nil, "init", "ralph-test", "--dir", exampleDir)
	if code != 0 {
		t.Fatalf("init failed (exit %d): %s %s", code, stdout, stderr)
	}

	// 3. Run
	stdout, stderr, code = runCmd(t, bin, nil, "run", "ralph-test", "--dir", exampleDir)
	if code != 0 {
		t.Fatalf("run failed (exit %d): %s %s", code, stdout, stderr)
	}

	// 4. Validate
	out, err = exec.Command("bash", filepath.Join(exampleDir, "test.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("test.sh failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "All checks passed") {
		t.Errorf("want 'All checks passed'; got: %s", out)
	}
}
