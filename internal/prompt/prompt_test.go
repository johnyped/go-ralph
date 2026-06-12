package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/johnyped/go-ralph/internal/config"
	"github.com/johnyped/go-ralph/internal/issues"
)

func setup(t *testing.T) (dir string, cfg *config.Config, issue issues.Issue) {
	t.Helper()
	dir = t.TempDir()

	os.MkdirAll(filepath.Join(dir, "issues"), 0o755)
	os.WriteFile(filepath.Join(dir, "issues", "001-scaffold.md"), []byte("## Build the scaffold"), 0o644)
	os.WriteFile(filepath.Join(dir, "PRD.md"), []byte("# Product Requirements"), 0o644)

	cfg = &config.Config{
		ContextFiles: []string{"PRD.md"},
		Instruction:  "Do the work.",
	}
	issue = issues.Issue{ID: "001", Slug: "001-scaffold", File: "issues/001-scaffold.md"}
	return
}

func TestAssembleWritesFile(t *testing.T) {
	dir, cfg, issue := setup(t)

	dest, err := Assemble(dir, cfg, issue)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(dest); err != nil {
		t.Errorf("prompt file not created: %v", err)
	}
	if dest != PromptPath(dir, "001-scaffold") {
		t.Errorf("returned path = %q, want %q", dest, PromptPath(dir, "001-scaffold"))
	}
}

func TestAssembleContainsAllSections(t *testing.T) {
	dir, cfg, issue := setup(t)

	dest, err := Assemble(dir, cfg, issue)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(dest)
	content := string(data)

	for _, want := range []string{
		"<context",
		"Product Requirements",
		"</context>",
		"<issue file=\"issues/001-scaffold.md\">",
		"Build the scaffold",
		"</issue>",
		"<instruction>",
		"Do the work.",
		"</instruction>",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestAssembleErrorOnMissingContextFile(t *testing.T) {
	dir, cfg, issue := setup(t)
	cfg.ContextFiles = []string{"MISSING.md"}

	_, err := Assemble(dir, cfg, issue)
	if err == nil {
		t.Error("expected error for missing context file")
	}
}

func TestAssembleSkillsEmbedded(t *testing.T) {
	dir, cfg, issue := setup(t)

	// Create a skill file
	skillPath := filepath.Join(dir, "my-skill.md")
	os.WriteFile(skillPath, []byte("# TDD Skill\nAlways write tests first."), 0o644)
	cfg.Skills = []string{skillPath}

	dest, err := Assemble(dir, cfg, issue)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(dest)
	content := string(data)

	if !strings.Contains(content, "<skill") {
		t.Error("prompt missing <skill> tag")
	}
	if !strings.Contains(content, "Always write tests first.") {
		t.Error("prompt missing skill content")
	}
}

func TestAssembleSkillMissingFails(t *testing.T) {
	dir, cfg, issue := setup(t)
	cfg.Skills = []string{"/nonexistent/skill.md"}

	_, err := Assemble(dir, cfg, issue)
	if err == nil {
		t.Error("expected error for missing skill file")
	}
}

func TestPromptPathFormat(t *testing.T) {
	got := PromptPath("/proj", "001-scaffold")
	want := filepath.Join("/proj", ".ralph", "prompts", "001-scaffold.md")
	if got != want {
		t.Errorf("PromptPath = %q, want %q", got, want)
	}
}

func TestAssembleDirectorySkill(t *testing.T) {
	dir, cfg, issue := setup(t)

	// Create a skill as a directory containing SKILL.md
	skillDir := filepath.Join(dir, "my-skill-dir")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill\nAlways use TDD."), 0o644)
	cfg.Skills = []string{skillDir}

	dest, err := Assemble(dir, cfg, issue)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(dest)
	content := string(data)

	if !strings.Contains(content, "Always use TDD.") {
		t.Error("prompt missing content from directory SKILL.md")
	}
}

func TestResolvePathTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := resolvePath("/proj", "~/foo/bar")
	want := filepath.Join(home, "foo", "bar")
	if got != want {
		t.Errorf("resolvePath(~/foo/bar) = %q, want %q", got, want)
	}
}

func TestResolvePathAbsolute(t *testing.T) {
	got := resolvePath("/proj", "/abs/path/skill.md")
	if got != "/abs/path/skill.md" {
		t.Errorf("resolvePath absolute = %q, want /abs/path/skill.md", got)
	}
}

func TestResolvePathRelative(t *testing.T) {
	got := resolvePath("/proj", "skills/tdd")
	want := filepath.Join("/proj", "skills", "tdd")
	if got != want {
		t.Errorf("resolvePath relative = %q, want %q", got, want)
	}
}

func TestAssembleInstructionSubstitution(t *testing.T) {
	dir, cfg, issue := setup(t)
	cfg.Instruction = "Edit {{issue_file}} when done."

	dest, err := Assemble(dir, cfg, issue)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(dest)
	content := string(data)

	if strings.Contains(content, "{{issue_file}}") {
		t.Error("{{issue_file}} placeholder was not substituted")
	}
	if !strings.Contains(content, issue.File) {
		t.Errorf("instruction missing actual issue file path %q", issue.File)
	}
}