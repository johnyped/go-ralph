package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigPath(t *testing.T) {
	got := ConfigPath("/some/dir")
	want := filepath.Join("/some/dir", ".ralph", "config.yaml")
	if got != want {
		t.Errorf("ConfigPath = %q, want %q", got, want)
	}
}

func TestLoadReturnsDefaultsWhenMissing(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PiPath != defaults.PiPath {
		t.Errorf("PiPath = %q, want %q", cfg.PiPath, defaults.PiPath)
	}
	if cfg.PiSessionPrefix != defaults.PiSessionPrefix {
		t.Errorf("PiSessionPrefix = %q, want %q", cfg.PiSessionPrefix, defaults.PiSessionPrefix)
	}
	if cfg.MaxRetries != defaults.MaxRetries {
		t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, defaults.MaxRetries)
	}
	if cfg.TimeoutMinutes != defaults.TimeoutMinutes {
		t.Errorf("TimeoutMinutes = %d, want %d", cfg.TimeoutMinutes, defaults.TimeoutMinutes)
	}
	if !cfg.StopOnFailure {
		t.Error("StopOnFailure default should be true")
	}
}

func TestWriteLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := &Config{
		PiPath:          "my-pi",
		PiSessionPrefix: "test",
		ContextFiles:    []string{"SPEC.md"},
		Skills:          []string{"/some/skill"},
		StopOnFailure:   false,
		MaxRetries:      5,
		TimeoutMinutes:  15,
		Instruction:     "Do it.",
	}

	if err := Write(dir, c); err != nil {
		t.Fatal(err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	if got.PiPath != c.PiPath {
		t.Errorf("PiPath = %q, want %q", got.PiPath, c.PiPath)
	}
	if got.MaxRetries != c.MaxRetries {
		t.Errorf("MaxRetries = %d, want %d", got.MaxRetries, c.MaxRetries)
	}
	if got.TimeoutMinutes != c.TimeoutMinutes {
		t.Errorf("TimeoutMinutes = %d, want %d", got.TimeoutMinutes, c.TimeoutMinutes)
	}
	if got.StopOnFailure != c.StopOnFailure {
		t.Errorf("StopOnFailure = %v, want %v", got.StopOnFailure, c.StopOnFailure)
	}
	if got.Instruction != c.Instruction {
		t.Errorf("Instruction = %q, want %q", got.Instruction, c.Instruction)
	}
	if len(got.ContextFiles) != 1 || got.ContextFiles[0] != "SPEC.md" {
		t.Errorf("ContextFiles = %v, want [SPEC.md]", got.ContextFiles)
	}
}

func TestWriteCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	c := &Config{PiPath: "pi", MaxRetries: 3}

	if err := Write(dir, c); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(ConfigPath(dir)); err != nil {
		t.Errorf("config file not created: %v", err)
	}
}

func TestLoadInvalidYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := ConfigPath(dir)
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(": invalid: yaml: ]["), 0o644)

	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadPartialOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := ConfigPath(dir)
	os.MkdirAll(filepath.Dir(path), 0o755)
	// Only override pi_path; other fields should retain defaults
	os.WriteFile(path, []byte("pi_path: custom-pi\n"), 0o644)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PiPath != "custom-pi" {
		t.Errorf("PiPath = %q, want custom-pi", cfg.PiPath)
	}
	// Default fields should be preserved
	if cfg.MaxRetries != defaults.MaxRetries {
		t.Errorf("MaxRetries = %d, want default %d", cfg.MaxRetries, defaults.MaxRetries)
	}
}
