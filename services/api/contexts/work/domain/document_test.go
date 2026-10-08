package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseDocumentKey(t *testing.T) {
	for _, k := range []string{"plan", "a", "9-notes", "spec_v2", strings.Repeat("a", 64)} {
		if _, err := ParseDocumentKey(k); err != nil {
			t.Errorf("ParseDocumentKey(%q) = %v", k, err)
		}
	}
	for _, k := range []string{"", "Plan", "Bad Key", "-plan", "_plan", "plan.md", "plän", strings.Repeat("a", 65)} {
		if _, err := ParseDocumentKey(k); err == nil {
			t.Errorf("ParseDocumentKey(%q) accepted", k)
		}
	}
}

func TestNewIssueDocument(t *testing.T) {
	now := time.Now()
	d, r, err := NewIssueDocument(1, 2, "plan", "  Plan ", "# v1", "first", 7, now)
	if err != nil {
		t.Fatal(err)
	}
	if d.Latest != 1 || r.Number != 1 || d.Title != "Plan" || r.Title != "Plan" || r.Body != "# v1" || r.Summary != "first" || d.CreatedByID != 7 || d.UpdatedByID != 7 {
		t.Errorf("got %+v and %+v", d, r)
	}
	if _, _, err := NewIssueDocument(1, 2, "Bad Key", "", "", "", 7, now); err == nil {
		t.Error("bad key accepted")
	}
	if _, _, err := NewIssueDocument(1, 2, "plan", strings.Repeat("t", 201), "", "", 7, now); err == nil {
		t.Error("long title accepted")
	}
	if _, _, err := NewIssueDocument(1, 2, "plan", "", strings.Repeat("b", MaxDocumentBody+1), "", 7, now); err == nil {
		t.Error("long body accepted")
	}
	if _, _, err := NewIssueDocument(1, 2, "plan", "", "", strings.Repeat("s", 501), 7, now); err == nil {
		t.Error("long summary accepted")
	}
}

func TestSaveNeedsTheNewestBase(t *testing.T) {
	now := time.Now()
	d, _, _ := NewIssueDocument(1, 2, "plan", "Plan", "# v1", "", 7, now)
	r, err := d.Save("Plan", "# v2", "", 1, 8, now)
	if err != nil {
		t.Fatal(err)
	}
	if r.Number != 2 || d.Latest != 2 || d.Body != "# v2" || d.UpdatedByID != 8 || d.CreatedByID != 7 {
		t.Errorf("got %+v and %+v", d, r)
	}
	if _, err := d.Save("Plan", "# v3", "", 1, 8, now); !errors.Is(err, ErrStaleRevision) {
		t.Errorf("stale base = %v", err)
	}
	if _, err := d.Save("Plan", "# v3", "", 0, 8, now); !errors.Is(err, ErrStaleRevision) {
		t.Errorf("no base = %v", err)
	}
	if d.Latest != 2 {
		t.Errorf("a refused save changed the document: %+v", d)
	}
}

func TestRestore(t *testing.T) {
	now := time.Now()
	d, r1, _ := NewIssueDocument(1, 2, "plan", "Plan", "# v1", "", 7, now)
	r2, _ := d.Save("Plan 2", "# v2", "", 1, 7, now)
	r3, err := d.Restore(r1, 8, now)
	if err != nil {
		t.Fatal(err)
	}
	if r3.Number != 3 || d.Body != "# v1" || d.Title != "Plan" || r3.Summary != "Restored from revision 1" {
		t.Errorf("got %+v and %+v", d, r3)
	}
	if _, err := d.Restore(r3, 8, now); !errors.Is(err, ErrRestoreNewest) {
		t.Errorf("restoring the newest = %v", err)
	}
	if _, err := d.Restore(r2, 8, now); err != nil {
		t.Errorf("restoring r2 = %v", err)
	}
}
