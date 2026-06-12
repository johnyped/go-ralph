package issues

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScan(t *testing.T) {
	dir := t.TempDir()
	issuesDir := filepath.Join(dir, "issues")
	os.MkdirAll(issuesDir, 0o755)

	// Write files out of order, include one non-matching file
	for _, f := range []string{
		"003-state-tracking.md",
		"001-module-scaffolding.md",
		"002-config-loading.md",
		"README.md", // no numeric prefix — should be skipped
	} {
		os.WriteFile(filepath.Join(issuesDir, f), []byte("# "+f), 0o644)
	}

	got, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 3 {
		t.Fatalf("want 3 issues, got %d", len(got))
	}

	want := []struct{ id, slug string }{
		{"001", "001-module-scaffolding"},
		{"002", "002-config-loading"},
		{"003", "003-state-tracking"},
	}
	for i, w := range want {
		if got[i].ID != w.id {
			t.Errorf("[%d] ID = %q, want %q", i, got[i].ID, w.id)
		}
		if got[i].Slug != w.slug {
			t.Errorf("[%d] Slug = %q, want %q", i, got[i].Slug, w.slug)
		}
		if got[i].File != filepath.Join("issues", w.slug+".md") {
			t.Errorf("[%d] File = %q, want %q", i, got[i].File, filepath.Join("issues", w.slug+".md"))
		}
	}
}

func TestScanEmptyDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "issues"), 0o755)

	got, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("want 0 issues, got %d", len(got))
	}
}

func TestReadContent(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "issues"), 0o755)
	os.WriteFile(filepath.Join(dir, "issues", "001-scaffold.md"), []byte("# Scaffold\n- [ ] create module"), 0o644)

	issue := Issue{ID: "001", Slug: "001-scaffold", File: filepath.Join("issues", "001-scaffold.md")}
	got, err := ReadContent(dir, issue)
	if err != nil {
		t.Fatal(err)
	}
	if got != "# Scaffold\n- [ ] create module" {
		t.Errorf("ReadContent = %q", got)
	}
}

func TestReadContentMissingReturnsError(t *testing.T) {
	dir := t.TempDir()
	issue := Issue{ID: "001", Slug: "001-missing", File: filepath.Join("issues", "001-missing.md")}
	_, err := ReadContent(dir, issue)
	if err == nil {
		t.Error("expected error for missing issue file")
	}
}