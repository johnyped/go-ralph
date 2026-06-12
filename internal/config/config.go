package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	PiPath          string   `yaml:"pi_path"`
	PiSessionPrefix string   `yaml:"pi_session_prefix"`
	ContextFiles    []string `yaml:"context_files"`
	Skills          []string `yaml:"skills"`
	StopOnFailure   bool     `yaml:"stop_on_failure"`
	MaxRetries      int      `yaml:"max_retries"`
	TimeoutMinutes  int      `yaml:"timeout_minutes"`
	Instruction     string   `yaml:"instruction"`
}

var defaults = Config{
	PiPath:          "pi",
	PiSessionPrefix: "ralph",
	ContextFiles:    []string{"PRD.md"},
	Skills: []string{
		"~/.pi/agent/skills/tdd",
	},
	StopOnFailure:  true,
	MaxRetries:     3,
	TimeoutMinutes: 30,
	Instruction: `You are implementing a software issue.
Complete all acceptance criteria using TDD (red-green-refactor).

IMPORTANT — checkpoint after each acceptance criterion:
- The issue file is at: {{issue_file}}
- When you complete a checkbox item, immediately write - [x] back to {{issue_file}}: change - [ ] to - [x]
- This lets ralph resume from where you left off if the session is interrupted

When done, verify EVERY acceptance criteria checkbox in the issue is satisfied:
- Read each checkbox item
- Run a concrete check (bash command, file read, test) to confirm it passes
- Only exit after ALL checkboxes are verified
Before exiting, append a summary to {{issue_file}}:
- Add a ` + "`## Summary`" + ` section at the bottom of the file
- List what was implemented as bullet points
- List files created or modified as bullet points
Do not ask clarifying questions — make reasonable decisions and proceed.`,
}

func ConfigPath(dir string) string {
	return filepath.Join(dir, ".ralph", "config.yaml")
}

// Load reads config from dir/.ralph/config.yaml, returning defaults if not found.
func Load(dir string) (*Config, error) {
	path := ConfigPath(dir)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		c := defaults
		return &c, nil
	}
	if err != nil {
		return nil, err
	}
	c := defaults
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Write saves the config to dir/.ralph/config.yaml.
func Write(dir string, c *Config) error {
	path := ConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
