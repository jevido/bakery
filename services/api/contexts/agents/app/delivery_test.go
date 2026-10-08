package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// queuedRun hires Ada by Member 7 with an Issue assigned to her and
// queues a Run of her on it.
func queuedRun(t *testing.T, s *Service, w *fakeWork) (domain.Agent, domain.Run) {
	t.Helper()
	ada := hired(t, s, "Ada", 0)
	w.issues = map[uint64]IssueBrief{30: {ID: 30, Identifier: "BAK-3", Title: "Fix it", AgentAssigneeID: ada.ID}}
	r, err := s.StartRun(context.Background(), 1, Actor{ID: 7, Permissions: hirer}, ada.ID, 30, nil)
	if err != nil {
		t.Fatal(err)
	}
	return ada, r
}

func TestDesktopClaimsOnce(t *testing.T) {
	ctx := context.Background()
	s, _, _, w := newTest()
	ada, r := queuedRun(t, s, w)
	laptop, desk := Desktop{ID: 3, MemberID: 7}, Desktop{ID: 4, MemberID: 7}
	stranger := Desktop{ID: 5, MemberID: 8}

	qs, err := s.DesktopRuns(ctx, laptop)
	if err != nil || len(qs) != 1 || qs[0].Run.ID != r.ID || qs[0].GuildName != "Guild 1" || qs[0].Issue.Identifier != "BAK-3" {
		t.Fatalf("queue: %+v %v", qs, err)
	}
	if qs, _ := s.DesktopRuns(ctx, stranger); len(qs) != 0 {
		t.Errorf("another person's desktop sees %+v", qs)
	}
	if _, err := s.ClaimRun(ctx, stranger, r.ID); !errors.Is(err, ErrRunNotFound) {
		t.Errorf("another person's claim: %v", err)
	}
	q, err := s.ClaimRun(ctx, laptop, r.ID)
	if err != nil || q.Run.Status != domain.RunRunning || q.Run.DesktopID != 3 || q.Agent.Status != domain.Running {
		t.Fatalf("claim: %+v %v", q, err)
	}
	var rse *domain.RunStatusError
	if _, err := s.ClaimRun(ctx, desk, r.ID); !errors.As(err, &rse) {
		t.Errorf("a second claim: %v", err)
	}
	// The running Run shows only to the Desktop that holds it.
	if qs, _ := s.DesktopRuns(ctx, laptop); len(qs) != 1 || qs[0].Run.Status != domain.RunRunning {
		t.Errorf("holder's queue %+v", qs)
	}
	if qs, _ := s.DesktopRuns(ctx, desk); len(qs) != 0 {
		t.Errorf("other desktop's queue %+v", qs)
	}
	// A second queued Run of the busy Agent waits.
	second, err := s.StartRun(ctx, 1, Actor{ID: 7, Permissions: hirer}, ada.ID, 30, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimRun(ctx, desk, second.ID); !errors.Is(err, ErrAgentBusy) {
		t.Errorf("a claim on a busy agent: %v", err)
	}
}

func TestDesktopReportsAndFinishes(t *testing.T) {
	ctx := context.Background()
	s, _, _, w := newTest()
	runs := s.runs.(*fakeRuns)
	ada, r := queuedRun(t, s, w)
	laptop := Desktop{ID: 3, MemberID: 7}
	if _, err := s.ClaimRun(ctx, laptop, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendRunEvents(ctx, Desktop{ID: 4, MemberID: 7}, r.ID, []domain.RunEvent{{Seq: 1, Kind: "init"}}); !errors.Is(err, ErrOtherDesktop) {
		t.Errorf("another desktop's report: %v", err)
	}
	init := domain.RunEvent{Seq: 1, Kind: "init", Payload: []byte(`{"session_id":"abc"}`)}
	got, err := s.AppendRunEvents(ctx, laptop, r.ID, []domain.RunEvent{init, {Seq: 2, Kind: "assistant"}})
	if err != nil || got.NextSeq != 3 || got.SessionID != "abc" || len(runs.events[r.ID]) != 2 {
		t.Fatalf("report: %+v %v", got, err)
	}
	if got, err = s.AppendRunEvents(ctx, laptop, r.ID, []domain.RunEvent{{Seq: 1, Kind: "init"}}); err != nil || len(runs.events[r.ID]) != 2 {
		t.Errorf("a repeated seq: %v, %d events", err, len(runs.events[r.ID]))
	}
	var qe *domain.SeqError
	if _, err := s.AppendRunEvents(ctx, laptop, r.ID, []domain.RunEvent{{Seq: 5, Kind: "assistant"}}); !errors.As(err, &qe) || qe.Expected != 3 {
		t.Errorf("a gap: %v", err)
	}
	if _, err := s.KeepRunLease(ctx, laptop, r.ID); err != nil {
		t.Errorf("lease: %v", err)
	}
	code := 0
	done, err := s.FinishRun(ctx, laptop, r.ID, Finish{Status: "succeeded", ExitCode: &code, Usage: domain.Usage{OutputTokens: 12, Turns: 2}})
	if err != nil || done.Status != domain.RunSucceeded || done.Usage.OutputTokens != 12 {
		t.Fatalf("finish: %+v %v", done, err)
	}
	if a, _ := s.Agent(ctx, 1, ada.ID); a.Status != domain.Idle {
		t.Errorf("agent %s after success", a.Status)
	}
	if w.last.Action != "run.finished" || w.last.ActorID != 7 || w.last.Details["status"] != "succeeded" {
		t.Errorf("activity %+v", w.last)
	}
	var rse *domain.RunStatusError
	if _, err := s.AppendRunEvents(ctx, laptop, r.ID, []domain.RunEvent{{Seq: 3, Kind: "result"}}); !errors.As(err, &rse) {
		t.Errorf("a report on a finished run: %v", err)
	}

	// Finishing a Run cancelled meanwhile answers it as it is.
	second, _ := s.StartRun(ctx, 1, Actor{ID: 7, Permissions: hirer}, ada.ID, 30, nil)
	if _, err := s.ClaimRun(ctx, laptop, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CancelRun(ctx, 1, Actor{ID: 7, Permissions: hirer}, second.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := s.FinishRun(ctx, laptop, second.ID, Finish{Status: "failed"}); err != nil || got.Status != domain.RunCancelled {
		t.Errorf("finish after cancel: %+v %v", got, err)
	}
}

func TestSweepLosesAndRequeues(t *testing.T) {
	ctx := context.Background()
	s, _, _, w := newTest()
	runs := s.runs.(*fakeRuns)
	ada, r := queuedRun(t, s, w)
	laptop := Desktop{ID: 3, MemberID: 7}
	at := time.Now()
	s.now = func() time.Time { return at }
	current := r.ID
	for retry := 0; retry <= domain.MaxChainRetries; retry++ {
		if _, err := s.ClaimRun(ctx, laptop, current); err != nil {
			t.Fatalf("claim %d: %v", retry, err)
		}
		at = at.Add(domain.Lease + time.Second)
		before := runs.next
		if err := s.SweepLostRuns(ctx); err != nil {
			t.Fatal(err)
		}
		// A second sweep (another API process) changes nothing more.
		if err := s.SweepLostRuns(ctx); err != nil {
			t.Fatal(err)
		}
		if runs.rows[current].Status != domain.RunLost {
			t.Fatalf("retry %d: %s", retry, runs.rows[current].Status)
		}
		a, _ := s.Agent(ctx, 1, ada.ID)
		if retry < domain.MaxChainRetries {
			if runs.next != before+1 {
				t.Fatalf("retry %d: %d runs made", retry, runs.next-before)
			}
			next := runs.rows[runs.next]
			if next.Status != domain.RunQueued || next.RetryOfRunID != current || next.Prompt != r.Prompt {
				t.Fatalf("requeued %+v", next)
			}
			if a.Status != domain.Error {
				t.Errorf("agent %s while its run waits again", a.Status)
			}
			current = next.ID
		} else {
			if runs.next != before {
				t.Errorf("queued again past %d retries", domain.MaxChainRetries)
			}
			if a.Status != domain.Error {
				t.Errorf("agent %s after the last lost run", a.Status)
			}
		}
	}
}
