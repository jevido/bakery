package domain

import (
	"errors"
	"testing"
	"time"
)

func TestRunMoves(t *testing.T) {
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	a := Agent{ID: 4, GuildID: 1, Status: Idle}
	for _, s := range []Status{PendingApproval, Paused, Terminated} {
		var se *StatusError
		if _, err := StartRun(Agent{Status: s}, 0, 7, OnDemand, "", at); !errors.As(err, &se) {
			t.Errorf("a run of a %s agent started: %v", s, err)
		}
	}
	r, err := StartRun(a, 9, 7, OnDemand, "prompt", at)
	if err != nil || r.Status != RunQueued || r.NextSeq != 1 || r.IssueID != 9 {
		t.Fatalf("start: %+v %v", r, err)
	}
	if _, err := r.Append([]RunEvent{{Seq: 1, Kind: "assistant"}}, at); err == nil {
		t.Error("events appended to a queued run")
	}
	if err := r.Finish(RunSucceeded, Usage{}, nil, "", at); err == nil {
		t.Error("a queued run finished")
	}
	if err := r.Claim(3, at); err != nil || r.Status != RunRunning || r.DesktopID != 3 || !r.LeaseExpiresAt.Equal(at.Add(Lease)) {
		t.Fatalf("claim: %+v %v", r, err)
	}
	if err := r.Claim(3, at); err == nil {
		t.Error("a running run claimed again")
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
	if _, err := r.Append([]RunEvent{{Seq: 5, Kind: "result"}, {Seq: 5, Kind: "result"}}, at); err == nil {
		t.Error("a repeated seq in one report accepted")
	}
	if _, err := r.Append([]RunEvent{{Seq: 9, Kind: "nonsense"}}, at); err == nil {
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

func TestRunLostIsRequeued(t *testing.T) {
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	r, _ := StartRun(Agent{ID: 4, GuildID: 1, Status: Error}, 9, 7, OnDemand, "prompt", at)
	r.ID = 11
	if _, err := r.Lose(at); err == nil {
		t.Error("a queued run lost")
	}
	_ = r.Claim(3, at)
	retry, err := r.Lose(at.Add(time.Hour))
	if err != nil || r.Status != RunLost || retry.Status != RunQueued || retry.RetryOfRunID != 11 || retry.IssueID != 9 || retry.Prompt != "prompt" {
		t.Fatalf("lose: %+v %+v %v", r, retry, err)
	}
	q, _ := StartRun(Agent{ID: 4, Status: Idle}, 0, 7, OnDemand, "", at)
	if err := q.Cancel(at); err != nil || q.Status != RunCancelled || q.FinishedAt == nil {
		t.Fatalf("cancel queued: %+v %v", q, err)
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
	if err := a.Pause(at); err != nil || a.Status != Paused {
		t.Fatalf("pause a running agent: %v", err)
	}
	if a.RunEnded(RunSucceeded, at) || a.Status != Paused {
		t.Error("a paused agent moved when its run ended")
	}
}
