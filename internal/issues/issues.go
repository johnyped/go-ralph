package issues

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Issue struct {
	ID   string // "001"
	Slug string // "001-module-scaffolding"
	File string // "issues/001-module-scaffolding.md"
}

var idRe = regexp.MustCompile(`^(\d+)-`)

// Scan returns issues found in dir/issues/*.md sorted by filename.
func Scan(dir string) ([]Issue, error) {
	pattern := filepath.Join(dir, "issues", "*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)

	var out []Issue
	for _, m := range matches {
		base := filepath.Base(m)
		ext := filepath.Ext(base)
		slug := strings.TrimSuffix(base, ext)
		sub := idRe.FindStringSubmatch(slug)
		if sub == nil {
			continue
		}
		rel, _ := filepath.Rel(dir, m)
		out = append(out, Issue{ID: sub[1], Slug: slug, File: rel})
	}
	return out, nil
}

// ReadContent returns the raw markdown content of an issue file.
func ReadContent(dir string, issue Issue) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, issue.File))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
