package app

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

func (m *memIssues) SaveCheckout(_ context.Context, i, before domain.Issue) (bool, error) {
	cur := m.byID[i.ID]
	if cur.Status != before.Status || cur.CheckoutRunID != before.CheckoutRunID ||
		cur.AssigneeID != before.AssigneeID || cur.AssigneeAgentID != before.AssigneeAgentID {
		return false, nil
	}
	m.byID[i.ID] = i
	return true, nil
}

func TestCheckoutAndRelease(t *testing.T) {
	ctx := context.Background()
	s, _ := wakeService(t)
	s.Assigned = nil
	all := func([]uint64) ([]uint64, error) { return nil, nil }
	running := []uint64{30, 31}
	s.RunsLive = func(_ context.Context, ids []uint64) (map[uint64]bool, error) {
		out := map[uint64]bool{}
		for _, id := range ids {
			out[id] = slices.Contains(running, id)
		}
		return out, nil
	}
	if _, err := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: "Fix it", Status: "todo", AssigneeAgentID: 3}, all); err != nil {
		t.Fatal(err)
	}
	runA := domain.Actor{AgentID: 3, RunID: 30}
	got, err := s.CheckoutIssue(ctx, 1, runA, "1", nil, all)
	if err != nil || got.CheckoutRunID != 30 || got.Status != domain.InProgress {
		t.Fatalf("checkout: %v %+v", err, got)
	}
	if _, err := s.CheckoutIssue(ctx, 1, domain.ByMember(7), "1", nil, all); !errors.Is(err, ErrAgentsOnly) {
		t.Errorf("a person's checkout: %v", err)
	}
	// Another Agent's live Run may not change it; a person may.
	runB := domain.Actor{AgentID: 4, RunID: 31}
	var held *domain.HeldError
	if _, err := s.WriteComment(ctx, 1, runB, "1", "Mine now.", all); !errors.As(err, &held) || held.RunID != 30 {
		t.Errorf("comment by another run: %v", err)
	}
	if _, err := s.ChangeIssue(ctx, 1, runB, "1", IssuePatch{Title: ptr("Taken")}, all); !errors.As(err, &held) {
		t.Errorf("patch by another run: %v", err)
	}
	if _, err := s.WriteComment(ctx, 1, runA, "1", "On it.", all); err != nil {
		t.Errorf("comment by the holder: %v", err)
	}
	if _, err := s.ChangeIssue(ctx, 1, domain.ByMember(7), "1", IssuePatch{Title: ptr("Fix it now")}, all); err != nil {
		t.Errorf("a person's patch: %v", err)
	}
	// Handing it to Agent 4 ends the Checkout.
	got, err = s.ChangeIssue(ctx, 1, domain.ByMember(7), "1", IssuePatch{AssigneeAgentID: ptr(uint64(4))}, all)
	if err != nil || got.CheckoutRunID != 0 {
		t.Fatalf("reassigned: %v %+v", err, got)
	}
	if got, err = s.CheckoutIssue(ctx, 1, runB, "1", nil, all); err != nil || got.CheckoutRunID != 31 {
		t.Fatalf("checkout by B: %v %+v", err, got)
	}
	if _, err := s.ReleaseIssue(ctx, 1, runA, "1", all); !errors.Is(err, domain.ErrNotHolder) {
		t.Errorf("release by A: %v", err)
	}
	// B's Run ends: its next Run takes the Stale checkout over.
	running = nil
	if got, err = s.CheckoutIssue(ctx, 1, domain.Actor{AgentID: 4, RunID: 32}, "1", nil, all); err != nil || got.CheckoutRunID != 32 {
		t.Fatalf("stale taken over: %v %+v", err, got)
	}
	if got, err = s.ReleaseIssue(ctx, 1, domain.Actor{AgentID: 4, RunID: 32}, "1", all); err != nil || got.CheckoutRunID != 0 || got.Status != domain.Todo || got.AssigneeAgentID != 0 {
		t.Fatalf("release: %v %+v", err, got)
	}
	var st *domain.StatusError
	if _, err := s.CheckoutIssue(ctx, 1, runB, "1", []string{"blocked"}, all); !errors.As(err, &st) {
		t.Errorf("unexpected status: %v", err)
	}
	if _, err := s.CheckoutIssue(ctx, 1, runB, "1", []string{"open"}, all); err == nil {
		t.Error("an unknown expected status was accepted")
	}
}
