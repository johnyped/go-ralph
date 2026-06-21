package pi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/johnyped/go-ralph/internal/config"
)

func TestPaneArgv_Shape(t *testing.T) {
	cfg := &config.Config{PiPath: "pi", PiSessionPrefix: "ralph", Skills: nil}
	argv := PaneArgv("/proj", cfg, "001-scaffold", "/proj/.ralph/prompts/001-scaffold.md", "/proj/.ralph/logs/001-scaffold.jsonl", "")
	if len(argv) != 3 || argv[0] != "bash" || argv[1] != "-c" {
		t.Fatalf("expected [bash -c <cmd>], got %v", argv)
	}
	cmd := argv[2]
	if !strings.Contains(cmd, "$(cat") {
		t.Errorf("cmd missing $(cat ...) prompt injection: %s", cmd)
	}
	if !strings.Contains(cmd, "tee") {
		t.Errorf("cmd missing tee: %s", cmd)
	}
	if !strings.Contains(cmd, "RALPH_DONE:") {
		t.Errorf("cmd missing sentinel: %s", cmd)
	}
	if !strings.Contains(cmd, "read -r") {
		t.Errorf("cmd missing read -r: %s", cmd)
	}
}

func TestPaneArgv_JsonMode(t *testing.T) {
	cfg := &config.Config{PiPath: "pi", PiSessionPrefix: "ralph", Skills: nil}
	argv := PaneArgv("/proj", cfg, "001-scaffold", "/proj/.ralph/prompts/001-scaffold.md", "/proj/.ralph/logs/001-scaffold.jsonl", "")
	cmd := argv[2]
	if !strings.Contains(cmd, "--mode json") {
		t.Errorf("cmd missing --mode json: %s", cmd)
	}
}

func TestPaneArgv_SessionSidecar(t *testing.T) {
	cfg := &config.Config{PiPath: "pi", PiSessionPrefix: "ralph", Skills: nil}
	logFile := "/proj/.ralph/logs/001-scaffold.jsonl"
	argv := PaneArgv("/proj", cfg, "001-scaffold", "/proj/.ralph/prompts/001-scaffold.md", logFile, "")
	cmd := argv[2]
	sidecar := SessionIDPath(logFile)
	if !strings.Contains(cmd, filepath.Base(sidecar)) {
		t.Errorf("cmd missing sidecar file reference: %s", cmd)
	}
}

func TestPaneArgv_SkillsResolved(t *testing.T) {
	// Skills are now embedded in the prompt, not passed as --skill flags
	cfg := &config.Config{
		PiPath:          "pi",
		PiSessionPrefix: "ralph",
		Skills:          []string{"/abs/tdd", ".kiro/skills/herdr"},
	}
	argv := PaneArgv("/myproj", cfg, "001", "/myproj/.ralph/prompts/001.md", "/myproj/.ralph/logs/001.jsonl", "")
	cmd := argv[2]
	// --skill flags must NOT appear in the pane command
	if strings.Contains(cmd, "--skill") {
		t.Errorf("cmd must not contain --skill flags (skills are now inline in prompt): %s", cmd)
	}
}

func TestBuildArgs_JsonMode(t *testing.T) {
	cfg := &config.Config{PiPath: "pi", PiSessionPrefix: "ralph"}
	args := buildArgs("", cfg, "001-scaffold", "")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--mode json") {
		t.Errorf("args missing --mode json: %v", args)
	}
}

func TestBuildArgs_Name(t *testing.T) {
	cfg := &config.Config{PiPath: "pi", PiSessionPrefix: "ralph"}
	args := buildArgs("", cfg, "001-scaffold", "")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--name") || !strings.Contains(joined, "ralph: 001-scaffold") {
		t.Errorf("args missing --name: %v", args)
	}
}

func TestBuildArgs_NoSkillFlags(t *testing.T) {
	cfg := &config.Config{PiPath: "pi", PiSessionPrefix: "ralph", Skills: []string{"/abs/tdd"}}
	args := buildArgs("", cfg, "001", "")
	for _, a := range args {
		if a == "--skill" {
			t.Errorf("buildArgs must not emit --skill flags: %v", args)
		}
	}
}

func TestSessionIDPath(t *testing.T) {
	logFile := "/proj/.ralph/logs/001-scaffold.jsonl"
	got := SessionIDPath(logFile)
	want := logFile + ".session-id"
	if got != want {
		t.Errorf("SessionIDPath = %q, want %q", got, want)
	}
}

func TestLogFile(t *testing.T) {
	got := LogFile("/proj", "001-scaffold", 1)
	want := filepath.Join("/proj", ".ralph", "logs", "001-scaffold.jsonl")
	if got != want {
		t.Errorf("LogFile attempt 1 = %q, want %q", got, want)
	}
}

func TestLogFileAttempt(t *testing.T) {
	got := LogFile("/proj", "001-scaffold", 2)
	want := filepath.Join("/proj", ".ralph", "logs", "001-scaffold-attempt-2.jsonl")
	if got != want {
		t.Errorf("LogFile attempt 2 = %q, want %q", got, want)
	}
}

func TestPaneArgv_NoRPCMode(t *testing.T) {
	cfg := &config.Config{PiPath: "pi", PiSessionPrefix: "ralph", Skills: nil}
	argv := PaneArgv("/proj", cfg, "001", "/proj/.ralph/prompts/001.md", "/proj/.ralph/logs/001.jsonl", "")
	cmd := argv[2]
	if strings.Contains(cmd, "--mode rpc") {
		t.Errorf("cmd must not contain --mode rpc: %s", cmd)
	}
}

func TestPaneArgv_PromptFileReferenced(t *testing.T) {
	cfg := &config.Config{PiPath: "pi", PiSessionPrefix: "ralph", Skills: nil}
	promptFile := "/proj/.ralph/prompts/001-scaffold.md"
	argv := PaneArgv("/proj", cfg, "001-scaffold", promptFile, "/proj/.ralph/logs/001.jsonl", "")
	cmd := argv[2]
	if !strings.Contains(cmd, "001-scaffold.md") {
		t.Errorf("cmd missing prompt file reference: %s", cmd)
	}
}

// TestPythonRendererWritesSidecar verifies the sidecar path is embedded in the pane command.
func TestPythonRendererWritesSidecar(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "001.jsonl")
	sidecar := SessionIDPath(logFile)

	cfg := &config.Config{PiPath: "pi", PiSessionPrefix: "ralph", Skills: nil}
	argv := PaneArgv(dir, cfg, "001", filepath.Join(dir, "001.md"), logFile, "")
	cmd := argv[2]

	// sidecar path must appear in the shell command so python3 can write to it
	if !strings.Contains(cmd, sidecar) {
		// also accept basename match since shellQuote may alter the representation
		if !strings.Contains(cmd, filepath.Base(sidecar)) {
			t.Errorf("sidecar path not in cmd.\ncmd: %s\nwant: %s", cmd, sidecar)
		}
	}
	// Ensure no leftover sidecar exists before a real run
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Error("sidecar should not exist before pi runs")
	}
}
