package domain

import (
	"errors"
	"slices"
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
	i, err := NewIssue(1, ByMember(7), "  Fix the login  ", "")
	if err != nil {
		t.Fatal(err)
	}
	if i.Title != "Fix the login" || i.Status != Backlog || i.Priority != Medium || i.CreatedBy != ByMember(7) {
		t.Errorf("NewIssue = %+v", i)
	}
	if _, err := NewIssue(1, ByMember(7), " ", ""); err == nil {
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

func TestIssueHasOneAssignee(t *testing.T) {
	i := Issue{AssigneeID: 3}
	i.AssignAgent(5)
	if i.AssigneeID != 0 || i.AssigneeAgentID != 5 {
		t.Fatalf("AssignAgent(5) = member %d, agent %d", i.AssigneeID, i.AssigneeAgentID)
	}
	i.Assign(0)
	if i.AssigneeAgentID != 5 {
		t.Errorf("Assign(0) took the issue from its agent")
	}
	i.Assign(3)
	if i.AssigneeID != 3 || i.AssigneeAgentID != 0 {
		t.Errorf("Assign(3) = member %d, agent %d", i.AssigneeID, i.AssigneeAgentID)
	}
	i.AssignAgent(0)
	if i.AssigneeID != 3 {
		t.Errorf("AssignAgent(0) took the issue from its member")
	}
}

func TestIssueCheckout(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	live := func(ids ...uint64) func(uint64) bool {
		return func(id uint64) bool { return slices.Contains(ids, id) }
	}
	assigned := func(st IssueStatus, holder uint64) Issue {
		return Issue{ID: 1, Status: st, AssigneeAgentID: 7, CheckoutRunID: holder}
	}

	i := assigned(Todo, 0)
	if changed, err := i.Checkout(7, 30, nil, live(), now); err != nil || !changed {
		t.Fatalf("free: %v %v", changed, err)
	}
	if i.CheckoutRunID != 30 || i.CheckedOutAt == nil || i.Status != InProgress || i.StartedAt == nil {
		t.Errorf("free: %+v", i)
	}
	if changed, err := i.Checkout(7, 30, nil, live(30), now.Add(time.Hour)); err != nil || changed {
		t.Errorf("same run: %v %v", changed, err)
	}
	if !i.CheckedOutAt.Equal(now) {
		t.Errorf("same run moved the time: %v", i.CheckedOutAt)
	}

	held := assigned(InProgress, 30)
	var he *HeldError
	if _, err := held.Checkout(7, 31, nil, live(30), now); !errors.As(err, &he) || he.RunID != 30 {
		t.Errorf("held: %v", err)
	}
	if !held.HeldByOther(31, live(30)) || held.HeldByOther(30, live(30)) || held.HeldByOther(31, live()) {
		t.Error("HeldByOther")
	}

	stale := assigned(InProgress, 30)
	if changed, err := stale.Checkout(7, 31, nil, live(), now); err != nil || !changed || stale.CheckoutRunID != 31 {
		t.Errorf("stale: %v %v %+v", changed, err, stale)
	}

	other := Issue{Status: Todo, AssigneeAgentID: 8}
	if _, err := other.Checkout(7, 30, nil, live(), now); !errors.Is(err, ErrNotAssignee) {
		t.Errorf("other agent: %v", err)
	}
	member := Issue{Status: Todo, AssigneeID: 3}
	if _, err := member.Checkout(7, 30, nil, live(), now); !errors.Is(err, ErrNotAssignee) {
		t.Errorf("member: %v", err)
	}

	done := assigned(Done, 0)
	var se *StatusError
	if _, err := done.Checkout(7, 30, nil, live(), now); !errors.As(err, &se) || se.Status != Done {
		t.Errorf("done: %v", err)
	}
	review := assigned(InReview, 0)
	if _, err := review.Checkout(7, 30, nil, live(), now); !errors.As(err, &se) || se.Status != InReview {
		t.Errorf("in review by default: %v", err)
	}
	if changed, err := review.Checkout(7, 30, []IssueStatus{InReview}, live(), now); err != nil || !changed {
		t.Errorf("in review expected: %v %v", changed, err)
	}
	otherInProgress := Issue{Status: InProgress}
	if _, err := otherInProgress.Checkout(7, 30, nil, live(), now); !errors.As(err, &se) {
		t.Errorf("unassigned in progress: %v", err)
	}

	free := Issue{Status: Backlog}
	if changed, err := free.Checkout(7, 30, nil, live(), now); err != nil || !changed || free.AssigneeAgentID != 7 || free.CheckoutRunID != 30 {
		t.Errorf("unassigned: %v %v %+v", changed, err, free)
	}
}

func TestIssueRelease(t *testing.T) {
	at := time.Now()
	i := Issue{Status: InProgress, AssigneeAgentID: 7, CheckoutRunID: 30, CheckedOutAt: &at}
	if err := i.Release(31, at); !errors.Is(err, ErrNotHolder) {
		t.Errorf("other run: %v", err)
	}
	if err := i.Release(30, at); err != nil || i.CheckoutRunID != 0 || i.CheckedOutAt != nil || i.Status != Todo || i.AssigneeAgentID != 0 {
		t.Errorf("release: %v %+v", err, i)
	}
	if err := i.Release(30, at); !errors.Is(err, ErrNotHolder) {
		t.Errorf("released twice: %v", err)
	}
	done := Issue{Status: Done, AssigneeAgentID: 7, CheckoutRunID: 30, CheckedOutAt: &at}
	if err := done.Release(30, at); err != nil || done.Status != Done || done.AssigneeAgentID != 7 {
		t.Errorf("release done: %v %+v", err, done)
	}
}

func TestIssueAssigneeChangeEndsCheckout(t *testing.T) {
	at := time.Now()
	i := Issue{Status: InProgress, AssigneeAgentID: 7, CheckoutRunID: 30, CheckedOutAt: &at}
	i.AssignAgent(7)
	if i.CheckoutRunID != 30 {
		t.Error("same assignee ended the checkout")
	}
	i.AssignAgent(8)
	if i.CheckoutRunID != 0 || i.CheckedOutAt != nil {
		t.Error("new agent kept the checkout")
	}
	i = Issue{AssigneeAgentID: 7, CheckoutRunID: 30, CheckedOutAt: &at}
	i.Assign(3)
	if i.CheckoutRunID != 0 {
		t.Error("a member kept the checkout")
	}
}
