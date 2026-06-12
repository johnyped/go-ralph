package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnyped/go-ralph/internal/config"
	"github.com/johnyped/go-ralph/internal/issues"
)

// PromptDir returns the directory where assembled prompts are written.
func PromptDir(dir string) string {
	return filepath.Join(dir, ".ralph", "prompts")
}

// PromptPath returns the path for a single issue's prompt file.
func PromptPath(dir, slug string) string {
	return filepath.Join(PromptDir(dir), slug+".md")
}

// Assemble builds the prompt for an issue and writes it to .ralph/prompts/<slug>.md.
func Assemble(dir string, cfg *config.Config, issue issues.Issue) (string, error) {
	var sb strings.Builder

	// Context files
	for _, cf := range cfg.ContextFiles {
		path := resolvePath(dir, cf)
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("context file %s: %w", cf, err)
		}
		fmt.Fprintf(&sb, "<context file=\"%s\">\n%s\n</context>\n\n", cf, string(data))
	}

	// Skills — embedded inline as <skill> sections
	for _, sf := range cfg.Skills {
		path := resolvePath(dir, sf)
		// If path is a directory, read SKILL.md inside it
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			path = filepath.Join(path, "SKILL.md")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("skill file %s: %w", sf, err)
		}
		fmt.Fprintf(&sb, "<skill path=\"%s\">\n%s\n</skill>\n\n", sf, string(data))
	}

	// Issue content
	content, err := issues.ReadContent(dir, issue)
	if err != nil {
		return "", fmt.Errorf("issue %s: %w", issue.Slug, err)
	}
	fmt.Fprintf(&sb, "<issue file=\"%s\">\n%s\n</issue>\n\n", issue.File, content)

	// Instruction — substitute {{issue_file}} with the actual path
	instruction := strings.ReplaceAll(cfg.Instruction, "{{issue_file}}", issue.File)
	fmt.Fprintf(&sb, "<instruction>\n%s\n</instruction>\n", instruction)

	out := sb.String()
	dest := PromptPath(dir, issue.Slug)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, []byte(out), 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

// resolvePath expands ~ and resolves relative paths against dir.
func resolvePath(dir, p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, p[2:])
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}
