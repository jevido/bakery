package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunMoves(t *testing.T) {
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	a := Agent{ID: 4, GuildID: 1, Status: Idle, Heartbeat: DefaultHeartbeatPolicy()}
	for _, s := range []Status{PendingApproval, Paused, Terminated} {
		var se *StatusError
		if _, err := StartRun(Agent{Status: s}, 0, 7, OnDemand, Manual, WakeContext{}, at); !errors.As(err, &se) {
			t.Errorf("a run of a %s agent started: %v", s, err)
		}
	}
	r, err := StartRun(a, 9, 7, OnDemand, Manual, WakeContext{}, at)
	if err != nil || r.Status != RunQueued || r.NextSeq != 1 || r.IssueID != 9 || r.WakeCount != 1 || r.WakeReason != Manual {
		t.Fatalf("start: %+v %v", r, err)
	}
	if _, err := r.Append([]RunEvent{{Seq: 1, Kind: "assistant"}}, at); err == nil {
		t.Error("events appended to a queued run")
	}
	if err := r.Finish(RunSucceeded, Usage{}, nil, "", at); err == nil {
		t.Error("a queued run finished")
	}
	if err := r.Claim(3, 5, "abc", at); err != nil || r.Status != RunRunning || r.DesktopID != 3 || r.ProjectID != 5 || r.KeyHash != "abc" || !r.LeaseExpiresAt.Equal(at.Add(Lease)) {
		t.Fatalf("claim: %+v %v", r, err)
	}
	// The Project is set once: a second claim is refused and keeps it.
	if err := r.Claim(3, 6, "abc", at); err == nil || r.ProjectID != 5 {
		t.Errorf("a running run claimed again: %v, project %d", err, r.ProjectID)
	}
	keep, err := r.Append([]RunEvent{{Seq: 1, Kind: "init"}, {Seq: 2, Kind: "assistant"}}, at)
	if err != nil || len(keep) != 2 || r.NextSeq != 3 {
		t.Fatalf("append: %v %v next %d", keep, err, r.NextSeq)
	}
	// The same seqs again are a no-op; a new one after them is kept.
	keep, err = r.Append([]RunEvent{{Seq: 2, Kind: "assistant"}, {Seq: 3, Kind: "result"}}, at)
	if err != nil || len(keep) != 1 || keep[0].Seq != 3 || r.NextSeq != 4 {
		t.Fatalf("append again: %v %v", keep, err)
	}
	if _, err := r.Append([]RunEvent{{Seq: 4, Kind: "result"}, {Seq: 4, Kind: "result"}}, at); err == nil {
		t.Error("a repeated seq in one report accepted")
	}
	var se *SeqError
	if _, err := r.Append([]RunEvent{{Seq: 5, Kind: "result"}}, at); !errors.As(err, &se) || se.Expected != 4 {
		t.Errorf("a gap accepted: %v", err)
	}
	if r.NextSeq != 4 {
		t.Errorf("a refused report moved NextSeq to %d", r.NextSeq)
	}
	if _, err := r.Append([]RunEvent{{Seq: 4, Kind: "nonsense"}}, at); err == nil {
		t.Error("an unknown kind accepted")
	}
	if err := r.Finish(RunQueued, Usage{}, nil, "", at); err == nil {
		t.Error("finished as queued")
	}
	code := 0
	if err := r.Finish(RunSucceeded, Usage{OutputTokens: 5}, &code, "", at); err != nil || r.Status != RunSucceeded || r.Usage.OutputTokens != 5 || r.LeaseExpiresAt != nil {
		t.Fatalf("finish: %+v %v", r, err)
	}
	if err := r.Cancel(at); err == nil {
		t.Error("a succeeded run cancelled")
	}
}

func TestJoinAndWakeable(t *testing.T) {
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	a := Agent{ID: 4, GuildID: 1, Status: Idle, Heartbeat: DefaultHeartbeatPolicy()}
	r, err := StartRun(a, 9, 7, Assignment, IssueAssigned, WakeContext{}, at)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Join(WakeContext{CommentIDs: []uint64{5}}, at); err != nil {
		t.Fatal(err)
	}
	if err := r.Join(WakeContext{CommentIDs: []uint64{5, 6}}, at); err != nil {
		t.Fatal(err)
	}
	if r.WakeCount != 3 || len(r.WakeContext.CommentIDs) != 2 || r.InvocationSource != Assignment || r.WakeReason != IssueAssigned {
		t.Errorf("joined: %+v", r)
	}
	_ = r.Claim(3, 0, "abc", at)
	var rse *RunStatusError
	if err := r.Join(WakeContext{}, at); !errors.As(err, &rse) || r.WakeCount != 3 {
		t.Errorf("a running run joined: %v", err)
	}
	a.Heartbeat.WakeOnDemand = false
	for _, src := range []InvocationSource{OnDemand, Assignment, Automation} {
		if err := a.Wakeable(src); !errors.As(err, &WakeRefused{}) {
			t.Errorf("%s woke an agent without wake on demand: %v", src, err)
		}
	}
	if err := a.Wakeable(Timer); err != nil {
		t.Errorf("the timer: %v", err)
	}
	a.Status = Paused
	var se *StatusError
	if err := a.Wakeable(Timer); !errors.As(err, &se) {
		t.Errorf("a paused agent woke: %v", err)
	}
}

func TestRunLostIsRequeued(t *testing.T) {
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	r, _ := StartRun(Agent{ID: 4, GuildID: 1, Status: Error, Heartbeat: DefaultHeartbeatPolicy()}, 9, 7, Automation, IssueCommented, WakeContext{CommentIDs: []uint64{5}}, at)
	r.ID = 11
	if _, err := r.Lose(at); err == nil {
		t.Error("a queued run lost")
	}
	_ = r.Claim(3, 0, "abc", at)
	retry, err := r.Lose(at.Add(time.Hour))
	if err != nil || r.Status != RunLost || retry.Status != RunQueued || retry.RetryOfRunID != 11 || retry.IssueID != 9 ||
		retry.InvocationSource != Automation || retry.WakeReason != IssueCommented || retry.WakeContext.CommentIDs[0] != 5 {
		t.Fatalf("lose: %+v %+v %v", r, retry, err)
	}
	q, _ := StartRun(Agent{ID: 4, Status: Idle, Heartbeat: DefaultHeartbeatPolicy()}, 0, 7, OnDemand, HeartbeatInvoked, WakeContext{}, at)
	if err := q.Cancel(at); err != nil || q.Status != RunCancelled || q.FinishedAt == nil {
		t.Fatalf("cancel queued: %+v %v", q, err)
	}
}

func TestRunLimit(t *testing.T) {
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	r := Run{ID: 5, GuildID: 1, AgentID: 4, IssueID: 9, InvocationSource: Assignment, WakeReason: IssueAssigned, WakeCount: 2,
		WakeContext: WakeContext{CommentIDs: []uint64{3}}, Status: RunQueued, RequestedByID: 7, NextSeq: 1}
	if _, err := r.Limit(Usage{}, nil, "", at.Add(time.Hour), at); err == nil {
		t.Error("a queued run was limited")
	}
	_ = r.Claim(2, 0, "h", at)
	var ve *FieldError
	for _, resets := range []time.Time{at, at.Add(-time.Minute), at.Add(MaxLimitWait + time.Second)} {
		if _, err := r.Limit(Usage{}, nil, "", resets, at); !errors.As(err, &ve) || ve.Field != "limit_resets_at" {
			t.Errorf("a reset at %s: %v", resets, err)
		}
	}
	if r.Status != RunRunning {
		t.Fatalf("a refused limit moved the run to %s", r.Status)
	}
	resets := at.Add(2 * time.Hour)
	next, err := r.Limit(Usage{OutputTokens: 40}, nil, " ", resets, at.Add(time.Minute))
	if err != nil || r.Status != RunLimited || !r.Status.Final() || r.Usage.OutputTokens != 40 || r.LimitResetsAt == nil || !r.LimitResetsAt.Equal(resets) {
		t.Fatalf("limit: %+v %v", r, err)
	}
	if r.Error != "the subscription limit was reached" || r.LeaseExpiresAt != nil || r.FinishedAt == nil {
		t.Errorf("limited run: %q %v %v", r.Error, r.LeaseExpiresAt, r.FinishedAt)
	}
	if next.Status != RunQueued || next.RetryOfRunID != 5 || next.IssueID != 9 || next.AgentID != 4 || next.WakeReason != IssueAssigned ||
		next.InvocationSource != Assignment || next.WakeCount != 1 || len(next.WakeContext.CommentIDs) != 1 || next.RequestedByID != 7 {
		t.Errorf("replacement: %+v", next)
	}
	l := DesktopLimit{DesktopID: 2, ResetsAt: resets}
	if !l.Active(at) || l.Active(resets) || (DesktopLimit{}).Active(at) {
		t.Error("desktop limit active")
	}
}

func TestAgentFollowsItsRuns(t *testing.T) {
	at := time.Now()
	a := Agent{Status: Idle}
	if err := a.StartRunning(at); err != nil || a.Status != Running {
		t.Fatalf("running: %v", err)
	}
	if err := a.StartRunning(at); err == nil {
		t.Error("a running agent started running again")
	}
	if _, err := a.Edit(Patch{}, at); err == nil {
		t.Error("a running agent was edited")
	}
	if !a.RunEnded(RunFailed, at) || a.Status != Error {
		t.Fatalf("after a failed run: %s", a.Status)
	}
	if a.RunEnded(RunSucceeded, at) {
		t.Error("an error agent moved without running")
	}
	if a.Rolable() != nil {
		t.Error("an error agent cannot be given roles")
	}
	if a.Resume(at) == nil {
		t.Error("an error agent was resumed")
	}
	_ = a.StartRunning(at)
	if !a.RunEnded(RunCancelled, at) || a.Status != Idle {
		t.Fatalf("after a cancelled run: %s", a.Status)
	}
	_ = a.StartRunning(at)
	if !a.RunEnded(RunLimited, at) || a.Status != Idle {
		t.Fatalf("after a limited run: %s", a.Status)
	}
	_ = a.StartRunning(at)
	if err := a.Pause(at); err != nil || a.Status != Paused {
		t.Fatalf("pause a running agent: %v", err)
	}
	if a.RunEnded(RunSucceeded, at) || a.Status != Paused {
		t.Error("a paused agent moved when its run ended")
	}
}

func TestValidRunKey(t *testing.T) {
	good := RunKeyPrefix + strings.Repeat("ab", 24)
	if !ValidRunKey(good) {
		t.Errorf("%s refused", good)
	}
	for _, bad := range []string{"", strings.Repeat("ab", 24), RunKeyPrefix + strings.Repeat("ab", 23), RunKeyPrefix + strings.Repeat("AB", 24), "bky_desk_" + strings.Repeat("ab", 24)} {
		if ValidRunKey(bad) {
			t.Errorf("%q let through", bad)
		}
	}
}
