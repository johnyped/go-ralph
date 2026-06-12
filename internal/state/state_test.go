package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := &State{
		Issues: []IssueState{
			{ID: "001", Slug: "001-scaffold", File: "issues/001-scaffold.md", Status: StatusPending},
			{ID: "002", Slug: "002-config", File: "issues/002-config.md", Status: StatusDone},
		},
	}

	if err := Write(dir, "myproject", s); err != nil {
		t.Fatal(err)
	}

	got, err := Load(dir, "myproject")
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Issues) != 2 {
		t.Fatalf("want 2 issues, got %d", len(got.Issues))
	}
	if got.Issues[0].ID != "001" || got.Issues[0].Status != StatusPending {
		t.Errorf("issue[0] = %+v", got.Issues[0])
	}
	if got.Issues[1].Status != StatusDone {
		t.Errorf("issue[1] status = %q, want done", got.Issues[1].Status)
	}
}

func TestWriteIsAtomic(t *testing.T) {
	dir := t.TempDir()
	s := &State{Issues: []IssueState{{ID: "001", Slug: "s", File: "f", Status: StatusPending}}}

	if err := Write(dir, "proj", s); err != nil {
		t.Fatal(err)
	}

	// tmp file must not exist after write
	tmp := StatePath(dir, "proj") + ".tmp"
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Error("tmp file still exists after Write")
	}

	// final file must exist
	if _, err := os.Stat(StatePath(dir, "proj")); err != nil {
		t.Errorf("state file missing: %v", err)
	}
}

func TestStatePathFormat(t *testing.T) {
	got := StatePath("/some/dir", "myproject")
	want := filepath.Join("/some/dir", ".ralph", "myproject.json")
	if got != want {
		t.Errorf("StatePath = %q, want %q", got, want)
	}
}

func TestFind(t *testing.T) {
	s := &State{
		Issues: []IssueState{
			{ID: "001", Slug: "001-scaffold", Status: StatusPending},
			{ID: "002", Slug: "002-config", Status: StatusDone},
		},
	}

	issue, err := s.Find("002")
	if err != nil {
		t.Fatal(err)
	}
	if issue.Status != StatusDone {
		t.Errorf("Find(002).Status = %q, want done", issue.Status)
	}

	_, err = s.Find("999")
	if err == nil {
		t.Error("Find(999) should return error")
	}
}

func TestFindMutatesInPlace(t *testing.T) {
	s := &State{
		Issues: []IssueState{{ID: "001", Status: StatusPending}},
	}
	issue, _ := s.Find("001")
	issue.Status = StatusDone

	if s.Issues[0].Status != StatusDone {
		t.Error("Find should return pointer to original, mutation should reflect in State")
	}
}

func TestPiSessionIDRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := &State{
		Issues: []IssueState{
			{ID: "001", Slug: "001-scaffold", File: "issues/001-scaffold.md", Status: StatusDone, PiSessionID: "abc-123-uuid"},
		},
	}
	if err := Write(dir, "proj", s); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir, "proj")
	if err != nil {
		t.Fatal(err)
	}
	if got.Issues[0].PiSessionID != "abc-123-uuid" {
		t.Errorf("PiSessionID = %q, want abc-123-uuid", got.Issues[0].PiSessionID)
	}
}
