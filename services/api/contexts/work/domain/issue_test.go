package domain

import (
	"errors"
	"testing"
	"time"
)

func TestParseIssueStatusAndPriority(t *testing.T) {
	for _, s := range []string{"backlog", "todo", "in_progress", "in_review", "blocked", "done", "cancelled"} {
		if _, err := ParseIssueStatus(s); err != nil {
			t.Errorf("ParseIssueStatus(%q) = %v", s, err)
		}
	}
	for _, s := range []string{"", "open", "Done", "achieved"} {
		if _, err := ParseIssueStatus(s); err == nil {
			t.Errorf("ParseIssueStatus(%q) accepted", s)
		}
	}
	for _, s := range []string{"critical", "high", "medium", "low"} {
		if _, err := ParsePriority(s); err != nil {
			t.Errorf("ParsePriority(%q) = %v", s, err)
		}
	}
	for _, s := range []string{"", "urgent", "none"} {
		if _, err := ParsePriority(s); err == nil {
			t.Errorf("ParsePriority(%q) accepted", s)
		}
	}
}

func TestNewIssue(t *testing.T) {
	i, err := NewIssue(1, 7, "  Fix the login  ", "")
	if err != nil {
		t.Fatal(err)
	}
	if i.Title != "Fix the login" || i.Status != Backlog || i.Priority != Medium || i.CreatedByID != 7 {
		t.Errorf("NewIssue = %+v", i)
	}
	if _, err := NewIssue(1, 7, " ", ""); err == nil {
		t.Error("an empty title was accepted")
	}
}

func TestIssueSetStatus(t *testing.T) {
	t1 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	set := func(at time.Time) *time.Time { return &at }
	for _, tc := range []struct {
		name                          string
		from                          Issue
		to                            IssueStatus
		started, completed, cancelled *time.Time
	}{
		{"backlog to todo sets nothing", Issue{Status: Backlog}, Todo, nil, nil, nil},
		{"to in_progress starts", Issue{Status: Todo}, InProgress, set(t2), nil, nil},
		{"in_progress again keeps the start", Issue{Status: InReview, StartedAt: set(t1)}, InProgress, set(t1), nil, nil},
		{"to done completes", Issue{Status: InProgress, StartedAt: set(t1)}, Done, set(t1), set(t2), nil},
		{"out of done clears completed", Issue{Status: Done, StartedAt: set(t1), CompletedAt: set(t1)}, Todo, set(t1), nil, nil},
		{"to cancelled cancels", Issue{Status: Backlog}, IssueCancelled, nil, nil, set(t2)},
		{"out of cancelled clears cancelled", Issue{Status: IssueCancelled, CancelledAt: set(t1)}, Backlog, nil, nil, nil},
		{"done to cancelled swaps the times", Issue{Status: Done, CompletedAt: set(t1)}, IssueCancelled, nil, nil, set(t2)},
		{"same status changes nothing", Issue{Status: Done, CompletedAt: set(t1)}, Done, nil, set(t1), nil},
		{"blocked and in_review set nothing", Issue{Status: InProgress, StartedAt: set(t1)}, Blocked, set(t1), nil, nil},
	} {
		i := tc.from
		i.SetStatus(tc.to, t2)
		if i.Status != tc.to || !sameTime(i.StartedAt, tc.started) || !sameTime(i.CompletedAt, tc.completed) || !sameTime(i.CancelledAt, tc.cancelled) {
			t.Errorf("%s: got status %s started %v completed %v cancelled %v", tc.name, i.Status, i.StartedAt, i.CompletedAt, i.CancelledAt)
		}
	}
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func TestIssueMoveUnder(t *testing.T) {
	i := Issue{ID: 1, GuildID: 1}
	if err := i.MoveUnder(&Issue{ID: 2, GuildID: 1}, []uint64{3}); err != nil || i.ParentID != 2 {
		t.Errorf("MoveUnder = %v, parent %d", err, i.ParentID)
	}
	if err := i.MoveUnder(&Issue{ID: 1, GuildID: 1}, nil); !errors.Is(err, ErrIssueCycle) {
		t.Errorf("under itself = %v", err)
	}
	if err := i.MoveUnder(&Issue{ID: 4, GuildID: 1}, []uint64{5, 1}); !errors.Is(err, ErrIssueCycle) {
		t.Errorf("under a sub-issue = %v", err)
	}
	if err := i.MoveUnder(&Issue{ID: 6, GuildID: 2}, nil); err == nil {
		t.Error("another Guild's parent was accepted")
	}
	if err := i.MoveUnder(nil, nil); err != nil || i.ParentID != 0 {
		t.Errorf("MoveUnder(nil) = %v, parent %d", err, i.ParentID)
	}
}

func TestIssueServeGoal(t *testing.T) {
	i := Issue{GuildID: 1}
	if err := i.ServeGoal(&Goal{ID: 3, GuildID: 2}); err == nil {
		t.Error("another Guild's Goal was accepted")
	}
	if err := i.ServeGoal(&Goal{ID: 3, GuildID: 1}); err != nil || i.GoalID != 3 {
		t.Errorf("ServeGoal = %v, goal %d", err, i.GoalID)
	}
}

func TestIdentifier(t *testing.T) {
	if got := Identifier("DEF", 12); got != "DEF-12" {
		t.Errorf("Identifier = %q", got)
	}
	for _, tc := range []struct {
		in     string
		prefix string
		number int
		ok     bool
	}{
		{"DEF-12", "DEF", 12, true},
		{"def-3", "DEF", 3, true},
		{"A-B-7", "A-B", 7, true},
		{"DEF-0", "", 0, false},
		{"DEF-", "", 0, false},
		{"-4", "", 0, false},
		{"12", "", 0, false},
		{"DEF-x", "", 0, false},
	} {
		p, n, ok := ParseIdentifier(tc.in)
		if p != tc.prefix || n != tc.number || ok != tc.ok {
			t.Errorf("ParseIdentifier(%q) = %q, %d, %v", tc.in, p, n, ok)
		}
	}
}

func TestBlockWith(t *testing.T) {
	i := Issue{ID: 1, GuildID: 1}
	a, b := Issue{ID: 2, GuildID: 1}, Issue{ID: 3, GuildID: 1}
	ids, err := i.BlockWith([]Issue{a, b, a}, []uint64{4})
	if err != nil || len(ids) != 2 || ids[0] != 2 || ids[1] != 3 {
		t.Errorf("BlockWith(a, b, a) = %v, %v", ids, err)
	}
	if ids, err := i.BlockWith(nil, nil); err != nil || len(ids) != 0 {
		t.Errorf("BlockWith(none) = %v, %v", ids, err)
	}
	if _, err := i.BlockWith([]Issue{i}, nil); !errors.Is(err, ErrSelfBlock) {
		t.Errorf("BlockWith(itself) = %v", err)
	}
	var fe *FieldError
	if _, err := i.BlockWith([]Issue{{ID: 9, GuildID: 2}}, nil); !errors.As(err, &fe) || fe.Field != "blocked_by_ids" {
		t.Errorf("BlockWith(another guild) = %v", err)
	}
	// 1 blocks 2, 2 blocks 3: 3 may not block 1.
	if _, err := i.BlockWith([]Issue{b}, []uint64{2, 3}); !errors.Is(err, ErrBlockerCycle) {
		t.Errorf("BlockWith(two-step cycle) = %v", err)
	}
}
