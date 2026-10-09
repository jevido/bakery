package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/app/secret"
	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// queuedRun hires Ada by Member 7 with an Issue assigned to her and
// queues a Run of her on it.
func queuedRun(t *testing.T, s *Service, w *fakeWork) (domain.Agent, domain.Run) {
	t.Helper()
	ada := hired(t, s, "Ada", 0)
	w.issues = map[uint64]IssueBrief{30: {ID: 30, Identifier: "BAK-3", Title: "Fix it", AgentAssigneeID: ada.ID}}
	r, err := startRun(s, context.Background(), 1, Actor{ID: 7, Permissions: hirer}, ada.ID, 30, nil)
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
	second, err := startRun(s, ctx, 1, Actor{ID: 7, Permissions: hirer}, ada.ID, 30, nil)
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
	second, _ := startRun(s, ctx, 1, Actor{ID: 7, Permissions: hirer}, ada.ID, 30, nil)
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

func TestPromptIsWrittenAtClaim(t *testing.T) {
	ctx := context.Background()
	s, _, _, w := newTest()
	runs := s.runs.(*fakeRuns)
	ada, r := queuedRun(t, s, w)
	w.comments = map[uint64]RunComment{51: {ID: 51, AuthorName: "Grace", Body: "Also the link."}, 52: {ID: 52, AuthorName: "Linus", Body: "And the icon."}}
	for _, c := range []uint64{51, 52} {
		if _, _, err := s.Wake(ctx, 1, ada.ID, WakeInput{Source: domain.Automation, Reason: domain.IssueCommented, IssueID: 30, CommentID: c}); err != nil {
			t.Fatal(err)
		}
	}
	if qs, _ := s.DesktopRuns(ctx, Desktop{ID: 3, MemberID: 7}); len(qs) != 1 || qs[0].Run.Prompt != "" || qs[0].Run.WakeCount != 3 {
		t.Fatalf("queue %+v", qs)
	}
	q, err := s.ClaimRun(ctx, Desktop{ID: 3, MemberID: 7}, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := "BAK-3: Fix it\n\nYou are Ada, the guild's General.\n\nNew comments:\n\nGrace wrote:\nAlso the link.\n\nLinus wrote:\nAnd the icon."
	if q.Run.Prompt != want || runs.rows[r.ID].Prompt != want {
		t.Errorf("prompt %q", q.Run.Prompt)
	}
	// A lost Run's replacement keeps its Wake reason and comments, and
	// joins a twin queued meanwhile.
	twin, _, _ := s.Wake(ctx, 1, ada.ID, WakeInput{Source: domain.Automation, Reason: domain.IssueCommented, IssueID: 30, CommentID: 53})
	at := time.Now().Add(domain.Lease + time.Second)
	s.now = func() time.Time { return at }
	if err := s.SweepLostRuns(ctx); err != nil {
		t.Fatal(err)
	}
	got := runs.rows[twin.ID]
	if runs.rows[r.ID].Status != domain.RunLost || got.Status != domain.RunQueued || got.WakeCount != 2 ||
		!slices.Equal(got.WakeContext.CommentIDs, []uint64{53, 51, 52}) || runs.next != twin.ID {
		t.Errorf("twin after the sweep %+v", got)
	}
}

func TestRunKeyLivesWithItsRun(t *testing.T) {
	ctx := context.Background()
	s, _, _, w := newTest()
	ada, r := queuedRun(t, s, w)
	laptop := Desktop{ID: 3, MemberID: 7}
	if qs, _ := s.DesktopRuns(ctx, laptop); len(qs) != 1 || qs[0].RunKey != "" {
		t.Fatalf("the queue shows a run key: %+v", qs)
	}
	q, err := s.ClaimRun(ctx, laptop, r.ID)
	if err != nil || !domain.ValidRunKey(q.RunKey) || q.Run.KeyHash != secret.Hash(q.RunKey) {
		t.Fatalf("claim: %q %+v %v", q.RunKey, q.Run, err)
	}
	if qs, _ := s.DesktopRuns(ctx, laptop); len(qs) != 1 || qs[0].RunKey != "" {
		t.Errorf("the running run's listing shows its key: %+v", qs)
	}
	h, ok, err := s.RunKeyHolder(ctx, secret.Hash(q.RunKey))
	if err != nil || !ok || h != (RunKeyHolder{AgentID: ada.ID, GuildID: 1, RunID: r.ID, HirerMemberID: 7}) {
		t.Fatalf("holder: %+v %v %v", h, ok, err)
	}
	if _, ok, _ := s.RunKeyHolder(ctx, secret.Hash(domain.RunKeyPrefix+"00")); ok {
		t.Error("a made-up key has a holder")
	}
	if _, err := s.FinishRun(ctx, laptop, r.ID, Finish{Status: "succeeded"}); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.RunKeyHolder(ctx, secret.Hash(q.RunKey)); ok {
		t.Error("the key outlived its run")
	}
}

func TestClaimCarriesTheWorkspace(t *testing.T) {
	ctx := context.Background()
	laptop := Desktop{ID: 3, MemberID: 7}
	claim := func(applicationID uint64, repos map[uint64]GitRepository) QueuedRun {
		t.Helper()
		s, _, _, w := newTest()
		w.repos = repos
		ada := hired(t, s, "Ada", 0)
		w.issues = map[uint64]IssueBrief{30: {ID: 30, Identifier: "DEF-12", Title: "Fix it", AgentAssigneeID: ada.ID, ApplicationID: applicationID, AgentBranch: "bakery/def-12"}}
		r, err := startRun(s, ctx, 1, Actor{ID: 7, Permissions: hirer}, ada.ID, 30, nil)
		if err != nil {
			t.Fatal(err)
		}
		if qs, _ := s.DesktopRuns(ctx, laptop); len(qs) != 1 || qs[0].Workspace != nil {
			t.Errorf("the queue carries a workspace: %+v", qs)
		}
		q, err := s.ClaimRun(ctx, laptop, r.ID)
		if err != nil {
			t.Fatal(err)
		}
		return q
	}
	shop := map[uint64]GitRepository{
		7: {Name: "shop", URL: "http://127.0.0.1:4950/e2e/shop.git", Branch: "main"},
	}
	q := claim(7, shop)
	want := Workspace{ApplicationID: 7, ApplicationName: "shop", Repository: "http://127.0.0.1:4950/e2e/shop.git", BaseBranch: "main", Branch: "bakery/def-12"}
	if q.Workspace == nil || *q.Workspace != want {
		t.Fatalf("workspace %+v", q.Workspace)
	}
	if !strings.Contains(q.Run.Prompt, "on branch bakery/def-12, based on main") {
		t.Errorf("prompt %q", q.Run.Prompt)
	}
	for name, id := range map[string]uint64{"no application": 0, "a deleted one or an image": 9, "a failing lookup": 99} {
		if q := claim(id, shop); q.Workspace != nil || strings.Contains(q.Run.Prompt, "worktree") {
			t.Errorf("%s: %+v %q", name, q.Workspace, q.Run.Prompt)
		}
	}
}

func TestLimitedRunsWaitForTheReset(t *testing.T) {
	ctx := context.Background()
	s, _, _, w := newTest()
	runs := s.runs.(*fakeRuns)
	ada, r := queuedRun(t, s, w)
	laptop, desk := Desktop{ID: 3, MemberID: 7}, Desktop{ID: 4, MemberID: 7}
	signedIn := []uint64{laptop.ID}
	s.SignedInDesktops = func(_ context.Context, ids []uint64) (map[uint64][]uint64, error) {
		return map[uint64][]uint64{7: signedIn}, nil
	}
	at := time.Now().UTC().Truncate(time.Second)
	s.now = func() time.Time { return at }
	if _, err := s.ClaimRun(ctx, laptop, r.ID); err != nil {
		t.Fatal(err)
	}

	// A limited finish needs a reset in the future.
	var fe *domain.FieldError
	past := at.Add(-time.Minute)
	for _, f := range []Finish{{Status: "limited"}, {Status: "limited", LimitResetsAt: &past}} {
		if _, err := s.FinishRun(ctx, laptop, r.ID, f); !errors.As(err, &fe) || fe.Field != "limit_resets_at" {
			t.Errorf("finish %+v: %v", f, err)
		}
	}

	resets := at.Add(2 * time.Minute)
	done, err := s.FinishRun(ctx, laptop, r.ID, Finish{Status: "limited", LimitResetsAt: &resets, Usage: domain.Usage{OutputTokens: 40, DurationMS: 900}})
	if err != nil || done.Status != domain.RunLimited || done.LimitResetsAt == nil || !done.LimitResetsAt.Equal(resets) {
		t.Fatalf("limited finish: %+v %v", done, err)
	}
	if a, _ := s.Agent(ctx, 1, ada.ID); a.Status != domain.Idle {
		t.Errorf("agent %s after a limited run", a.Status)
	}
	if w.last.Action != "run.finished" || w.last.Details["status"] != "limited" {
		t.Errorf("activity %+v", w.last)
	}
	next := runs.rows[runs.next]
	if next.Status != domain.RunQueued || next.RetryOfRunID != r.ID || next.IssueID != 30 {
		t.Fatalf("replacement %+v", next)
	}
	if l := runs.limits[laptop.ID]; l.MemberID != 7 || !l.ResetsAt.Equal(resets) {
		t.Errorf("desktop limit %+v", l)
	}

	// Its usage counts in Costs.
	if c, err := s.Costs(ctx, CostRange{GuildID: 1}, func(ids []uint64) ([]uint64, error) { return ids, nil }); err != nil ||
		c.Total.OutputTokens != 40 || c.Total.Runs != 1 {
		t.Errorf("costs %+v %v", c.Total, err)
	}

	// The waiting Run shows the Hirer's limit while every signed-in Desktop
	// is at it.
	agents := map[uint64]domain.Agent{ada.ID: ada}
	if got, _ := s.SubscriptionLimits(ctx, []domain.Run{next, done}, agents); len(got) != 1 || !got[next.ID].Equal(resets) {
		t.Errorf("waits %v", got)
	}
	signedIn = []uint64{laptop.ID, desk.ID}
	if got, _ := s.SubscriptionLimits(ctx, []domain.Run{next}, agents); len(got) != 0 {
		t.Errorf("waits with a free desktop: %v", got)
	}
	signedIn = []uint64{laptop.ID}

	// The limited Desktop is refused until the reset; the Run stays queued.
	var le *LimitError
	if _, err := s.ClaimRun(ctx, laptop, next.ID); !errors.As(err, &le) || !le.ResetsAt.Equal(resets) {
		t.Fatalf("claim at the limit: %v", err)
	}
	if runs.rows[next.ID].Status != domain.RunQueued {
		t.Errorf("refused run %s", runs.rows[next.ID].Status)
	}

	// Limited Runs never use up the lost Runs' retries.
	current := next.ID
	for i := range domain.MaxChainRetries + 2 {
		at = at.Add(3 * time.Minute)
		if got, _ := s.SubscriptionLimits(ctx, []domain.Run{runs.rows[current]}, agents); len(got) != 0 {
			t.Errorf("waits after the reset: %v", got)
		}
		if _, err := s.ClaimRun(ctx, laptop, current); err != nil {
			t.Fatalf("claim %d after the reset: %v", i, err)
		}
		resets = at.Add(2 * time.Minute)
		before := runs.next
		if _, err := s.FinishRun(ctx, laptop, current, Finish{Status: "limited", LimitResetsAt: &resets}); err != nil {
			t.Fatal(err)
		}
		if runs.next != before+1 || runs.rows[runs.next].RetryOfRunID != current {
			t.Fatalf("limited run %d not queued again", i)
		}
		current = runs.next
	}
	if n, _ := s.retries(ctx, runs.rows[current]); n != 0 {
		t.Errorf("retries %d along a limited chain", n)
	}
}

func TestCompletionReply(t *testing.T) {
	ctx := context.Background()
	s, _, _, w := newTest()
	runs := s.runs.(*fakeRuns)
	ada, r := queuedRun(t, s, w)
	chat := w.issues[30]
	chat.Conversation = &ConversationBrief{MemberID: 7}
	w.issues[30] = chat
	w.history = map[uint64][]RunComment{30: {{ID: 1, AuthorName: "Grace", Body: "hello"}}}
	laptop := Desktop{ID: 3, MemberID: 7}
	finish := func(r domain.Run, status string, events ...domain.RunEvent) {
		t.Helper()
		if _, err := s.ClaimRun(ctx, laptop, r.ID); err != nil {
			t.Fatal(err)
		}
		if len(events) > 0 {
			if _, err := s.AppendRunEvents(ctx, laptop, r.ID, events); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.FinishRun(ctx, laptop, r.ID, Finish{Status: status}); err != nil {
			t.Fatal(err)
		}
	}
	result := func(seq int64, text string) domain.RunEvent {
		return domain.RunEvent{Seq: seq, Kind: "result", Payload: []byte(`{"subtype":"success","result":"` + text + `"}`)}
	}
	finish(r, "succeeded", domain.RunEvent{Seq: 1, Kind: "result", Payload: []byte(`{"result":"first"}`)}, result(2, "Hi Grace."))
	if w.replies[r.ID] != "Hi Grace." || len(w.replies) != 1 {
		t.Errorf("replies %+v", w.replies)
	}
	if !strings.Contains(runs.rows[r.ID].Prompt, "Conversation so far:\n\nGrace wrote:\nhello") {
		t.Errorf("prompt %q", runs.rows[r.ID].Prompt)
	}
	// A retried finish answers the finished Run's status error and posts
	// nothing more.
	if _, err := s.FinishRun(ctx, laptop, r.ID, Finish{Status: "succeeded"}); err == nil || len(w.replies) != 1 {
		t.Errorf("finish again: %v, %d replies", err, len(w.replies))
	}
	// A failed Run, and one without a result text, post none.
	failed, _ := startRun(s, ctx, 1, Actor{ID: 7, Permissions: hirer}, ada.ID, 30, nil)
	finish(failed, "failed", domain.RunEvent{Seq: 1, Kind: "result", Payload: []byte(`{"result":"oops"}`)})
	silent, _ := startRun(s, ctx, 1, Actor{ID: 7, Permissions: hirer}, ada.ID, 30, nil)
	finish(silent, "succeeded", domain.RunEvent{Seq: 1, Kind: "result", Payload: []byte(`{"result":"  "}`)})
	if len(w.replies) != 1 {
		t.Errorf("replies after failed and silent runs %+v", w.replies)
	}
	// A Run on an Issue that is no Conversation posts none.
	chat.Conversation = nil
	w.issues[30] = chat
	plain, _ := startRun(s, ctx, 1, Actor{ID: 7, Permissions: hirer}, ada.ID, 30, nil)
	finish(plain, "succeeded", result(1, "Done."))
	if len(w.replies) != 1 {
		t.Errorf("replies after a plain issue's run %+v", w.replies)
	}
}
