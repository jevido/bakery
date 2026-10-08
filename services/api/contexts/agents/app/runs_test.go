package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

type fakeRuns struct {
	rows   map[uint64]domain.Run
	next   uint64
	events map[uint64][]domain.RunEvent
	agents *fakeAgents
}

func (f *fakeRuns) DesktopRuns(_ context.Context, memberID, desktopID uint64) ([]domain.Run, error) {
	var out []domain.Run
	for id := uint64(1); id <= f.next; id++ {
		r, ok := f.rows[id]
		a := f.agents.rows[r.AgentID]
		if !ok || a.HirerID != memberID {
			continue
		}
		queued := r.Status == domain.RunQueued && a.Status != domain.Paused && a.Status != domain.Terminated
		if queued || (r.Status == domain.RunRunning && r.DesktopID == desktopID) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRuns) AppendRunEvents(_ context.Context, r domain.Run, es []domain.RunEvent, from int64) (bool, error) {
	if now := f.rows[r.ID]; now.Status != domain.RunRunning || now.NextSeq != from {
		return false, nil
	}
	f.rows[r.ID] = r
	f.events[r.ID] = append(f.events[r.ID], es...)
	return true, nil
}

func (f *fakeRuns) KeepRunLease(_ context.Context, r domain.Run) (bool, error) {
	now := f.rows[r.ID]
	if now.Status != domain.RunRunning {
		return false, nil
	}
	now.LeaseExpiresAt = r.LeaseExpiresAt
	f.rows[r.ID] = now
	return true, nil
}

func (f *fakeRuns) ExpiredRuns(_ context.Context, at time.Time) ([]domain.Run, error) {
	var out []domain.Run
	for id := uint64(1); id <= f.next; id++ {
		if r, ok := f.rows[id]; ok && r.Status == domain.RunRunning && r.LeaseExpiresAt.Before(at) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRuns) CreateRun(_ context.Context, r domain.Run) (domain.Run, error) {
	f.next++
	r.ID = f.next
	f.rows[r.ID] = r
	return r, nil
}

func (f *fakeRuns) Run(_ context.Context, id uint64) (domain.Run, bool, error) {
	r, ok := f.rows[id]
	return r, ok, nil
}

func (f *fakeRuns) Runs(_ context.Context, q RunQuery) ([]domain.Run, error) {
	var out []domain.Run
	for id := f.next; id > 0; id-- {
		r, ok := f.rows[id]
		if !ok || r.GuildID != q.GuildID || (q.AgentID != 0 && r.AgentID != q.AgentID) || (q.IssueID != 0 && r.IssueID != q.IssueID) {
			continue
		}
		if len(q.Statuses) > 0 && !slices.Contains(q.Statuses, r.Status) {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// SaveRun refuses, as the conditional UPDATE does, a Run moved meanwhile
// and a second running Run of one Agent.
func (f *fakeRuns) SaveRun(_ context.Context, r domain.Run, from domain.RunStatus) (bool, error) {
	if f.rows[r.ID].Status != from {
		return false, nil
	}
	if r.Status == domain.RunRunning {
		for _, o := range f.rows {
			if o.ID != r.ID && o.AgentID == r.AgentID && o.Status == domain.RunRunning {
				return false, errors.New("runs_agent_id_running_unique")
			}
		}
	}
	f.rows[r.ID] = r
	return true, nil
}

func (f *fakeRuns) RunningRuns(_ context.Context, ids []uint64) (map[uint64]uint64, error) {
	out := map[uint64]uint64{}
	for _, r := range f.rows {
		if r.Status == domain.RunRunning && slices.Contains(ids, r.AgentID) {
			out[r.AgentID] = r.ID
		}
	}
	return out, nil
}

func (f *fakeRuns) RunEvents(context.Context, uint64, int64, int) ([]domain.RunEvent, error) {
	return nil, nil
}

func TestStartAndCancelRun(t *testing.T) {
	ctx := context.Background()
	s, _, g, w := newTest()
	runs := s.runs.(*fakeRuns)
	ada := hired(t, s, "Ada", 0)
	w.issues = map[uint64]IssueBrief{
		30: {ID: 30, Identifier: "BAK-3", Title: "Fix it", Description: "The button.", AgentAssigneeID: ada.ID},
		31: {ID: 31, Identifier: "BAK-4", Title: "Not hers", AgentAssigneeID: 99},
		32: {ID: 32, ProjectID: 5, Identifier: "BAK-5", Title: "Hidden", AgentAssigneeID: ada.ID},
	}
	hirerActor := Actor{ID: 7, Permissions: hirer}
	r, err := s.StartRun(ctx, 1, hirerActor, ada.ID, 30, nil)
	if err != nil || r.Status != domain.RunQueued || r.IssueID != 30 || r.RequestedByID != 7 {
		t.Fatalf("start: %+v %v", r, err)
	}
	if !strings.HasPrefix(r.Prompt, "BAK-3: Fix it\n\nThe button.\n\nYou are Ada") {
		t.Errorf("prompt %q", r.Prompt)
	}
	if w.last.Action != "run.started" || w.last.Details["run_id"] != r.ID {
		t.Errorf("activity %+v", w.last)
	}
	var fe *domain.FieldError
	for _, issue := range []uint64{31, 77, 0} {
		if _, err := s.StartRun(ctx, 1, hirerActor, ada.ID, issue, nil); !errors.As(err, &fe) {
			t.Errorf("issue %d: %v", issue, err)
		}
	}
	hidden := func([]uint64) ([]uint64, error) { return nil, nil }
	if _, err := s.StartRun(ctx, 1, hirerActor, ada.ID, 32, hidden); !errors.As(err, &fe) {
		t.Errorf("a hidden issue: %v", err)
	}
	g.rank = map[uint64]int{7: 5, 9: 1}
	if _, err := s.StartRun(ctx, 1, Actor{ID: 9, Permissions: hirer}, ada.ID, 30, nil); !errors.Is(err, ErrMayNotManage) {
		t.Errorf("one below the hirer: %v", err)
	}
	if _, err := s.CancelRun(ctx, 1, Actor{ID: 9, Permissions: hirer}, r.ID); !errors.Is(err, ErrMayNotManage) {
		t.Errorf("cancel by one below the hirer: %v", err)
	}
	if _, err := s.CancelRun(ctx, 2, hirerActor, r.ID); !errors.Is(err, ErrRunNotFound) {
		t.Errorf("cancel in another guild: %v", err)
	}
	c, err := s.CancelRun(ctx, 1, hirerActor, r.ID)
	if err != nil || c.Status != domain.RunCancelled || runs.rows[r.ID].Status != domain.RunCancelled {
		t.Fatalf("cancel: %+v %v", c, err)
	}
	if w.last.Action != "run.finished" || w.last.Details["status"] != "cancelled" {
		t.Errorf("activity %+v", w.last)
	}
	var rse *domain.RunStatusError
	if _, err := s.CancelRun(ctx, 1, hirerActor, r.ID); !errors.As(err, &rse) {
		t.Errorf("a second cancel: %v", err)
	}
	if _, err := s.Pause(ctx, 1, hirerActor, ada.ID); err != nil {
		t.Fatal(err)
	}
	var se *domain.StatusError
	if _, err := s.StartRun(ctx, 1, hirerActor, ada.ID, 30, nil); !errors.As(err, &se) {
		t.Errorf("a run of a paused agent: %v", err)
	}
}

func TestPauseAndTerminateCancelRuns(t *testing.T) {
	ctx := context.Background()
	s, _, _, w := newTest()
	runs := s.runs.(*fakeRuns)
	ada := hired(t, s, "Ada", 0)
	w.issues = map[uint64]IssueBrief{30: {ID: 30, Identifier: "BAK-3", Title: "Fix it", AgentAssigneeID: ada.ID}}
	actor := Actor{ID: 7, Permissions: hirer}
	first, _ := s.StartRun(ctx, 1, actor, ada.ID, 30, nil)
	second, _ := s.StartRun(ctx, 1, actor, ada.ID, 30, nil)
	// The first is claimed: the Agent is running and its Run too.
	if _, err := s.agentRunning(ctx, ada.ID); err != nil {
		t.Fatal(err)
	}
	claimed := runs.rows[first.ID]
	_ = claimed.Claim(3, time.Now())
	runs.rows[first.ID] = claimed
	if got, _ := s.RunningRuns(ctx, []uint64{ada.ID}); got[ada.ID] != first.ID {
		t.Errorf("running runs %v", got)
	}
	paused, err := s.Pause(ctx, 1, actor, ada.ID)
	if err != nil || paused.Status != domain.Paused {
		t.Fatalf("pause a running agent: %+v %v", paused, err)
	}
	for _, id := range []uint64{first.ID, second.ID} {
		if runs.rows[id].Status != domain.RunCancelled {
			t.Errorf("run %d is %s after pause", id, runs.rows[id].Status)
		}
	}
	if a, _ := s.Agent(ctx, 1, ada.ID); a.Status != domain.Paused {
		t.Errorf("agent %s after its run was cancelled by pause", a.Status)
	}
	if _, err := s.Resume(ctx, 1, actor, ada.ID); err != nil {
		t.Fatal(err)
	}
	third, _ := s.StartRun(ctx, 1, actor, ada.ID, 30, nil)
	if _, err := s.Terminate(ctx, 1, actor, ada.ID); err != nil {
		t.Fatal(err)
	}
	if runs.rows[third.ID].Status != domain.RunCancelled {
		t.Errorf("run %s after terminate", runs.rows[third.ID].Status)
	}
}

func TestRunEndedMovesTheAgent(t *testing.T) {
	ctx := context.Background()
	s, _, _, _ := newTest()
	ada := hired(t, s, "Ada", 0)
	if a, err := s.agentRunning(ctx, ada.ID); err != nil || a.Status != domain.Running {
		t.Fatalf("running: %+v %v", a, err)
	}
	if _, err := s.agentRunning(ctx, ada.ID); err == nil {
		t.Error("a running agent made running again")
	}
	if a, _ := s.runEnded(ctx, ada.ID, domain.RunLost); a.Status != domain.Error {
		t.Errorf("after a lost run: %s", a.Status)
	}
	title := "Fixer"
	if a, _ := s.Edit(ctx, 1, Actor{ID: 7, Permissions: hirer}, ada.ID, domain.Patch{Title: &title}); a.Title != "Fixer" {
		t.Errorf("an error agent was not edited: %+v", a)
	}
}
