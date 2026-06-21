package pipeline

import (
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

type Pipeline struct {
	Paths     map[string][]string `yaml:"paths"`
	DependsOn map[string][]string `yaml:"depends_on"`
}

func PipelinePath(dir string) string {
	return filepath.Join(dir, ".ralph", "pipeline.yaml")
}

func Load(dir string) (*Pipeline, error) {
	data, err := os.ReadFile(PipelinePath(dir))
	if err != nil {
		return nil, err
	}
	var p Pipeline
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.DependsOn == nil {
		p.DependsOn = map[string][]string{}
	}
	return &p, nil
}

func Write(dir string, p *Pipeline) error {
	path := PipelinePath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadyIssues returns issue IDs ready to dispatch (sorted path keys for determinism).
func ReadyIssues(p *Pipeline, done map[string]bool, inFlight map[string]bool) []string {
	keys := make([]string, 0, len(p.Paths))
	for k := range p.Paths {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var ready []string
	for _, path := range keys {
		// Check depends_on
		blocked := false
		for _, prereq := range p.DependsOn[path] {
			if !done[prereq] {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		// Emit first non-done, non-in-flight ID
		for _, id := range p.Paths[path] {
			if done[id] {
				continue
			}
			if !inFlight[id] {
				ready = append(ready, id)
			}
			break // in-flight or emitted: path is blocked here
		}
	}
	return ready
}
