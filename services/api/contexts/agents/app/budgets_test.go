package app

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// fakeBudgets keeps Budgets and Budget incidents in memory, upserting
// and opening as the unique keys do.
type fakeBudgets struct {
	rows      []domain.Budget
	incidents []domain.BudgetIncident
}

func (f *fakeBudgets) BudgetIncidents(_ context.Context, guildID, budgetID uint64, statuses []domain.IncidentStatus) ([]domain.BudgetIncident, error) {
	var out []domain.BudgetIncident
	for _, i := range f.incidents {
		if i.GuildID == guildID && (budgetID == 0 || i.BudgetID == budgetID) && (len(statuses) == 0 || slices.Contains(statuses, i.Status)) {
			out = append(out, i)
		}
	}
	return out, nil
}

func (f *fakeBudgets) OpenIncident(_ context.Context, i domain.BudgetIncident) (domain.BudgetIncident, bool, error) {
	for _, o := range f.incidents {
		if o.BudgetID == i.BudgetID && o.WindowStart.Equal(i.WindowStart) && o.Threshold == i.Threshold && o.Amount == i.Amount && o.Status != domain.IncidentDismissed {
			return domain.BudgetIncident{}, false, nil
		}
	}
	i.ID = uint64(len(f.incidents) + 1)
	f.incidents = append(f.incidents, i)
	return i, true, nil
}

func (f *fakeBudgets) OpenIncidentsOf(_ context.Context, scope domain.BudgetScope, scopeID uint64) ([]domain.BudgetIncident, error) {
	var out []domain.BudgetIncident
	for _, i := range f.incidents {
		if i.Scope == scope && i.ScopeID == scopeID && i.Status == domain.IncidentOpen {
			out = append(out, i)
		}
	}
	return out, nil
}

func (f *fakeBudgets) SaveIncident(_ context.Context, i domain.BudgetIncident) (bool, error) {
	for n, o := range f.incidents {
		if o.ID == i.ID && o.Status == domain.IncidentOpen {
			f.incidents[n] = i
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeBudgets) Budgets(_ context.Context, guildID uint64) ([]domain.Budget, error) {
	var out []domain.Budget
	for _, b := range f.rows {
		if b.GuildID == guildID {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeBudgets) SaveBudget(_ context.Context, b domain.Budget) (domain.Budget, error) {
	for i, o := range f.rows {
		if o.GuildID == b.GuildID && o.Scope == b.Scope && o.ScopeID == b.ScopeID && o.Metric == b.Metric && o.Window == b.Window {
			b.ID, b.CreatedBy, b.CreatedAt = o.ID, o.CreatedBy, o.CreatedAt
			f.rows[i] = b
			return b, nil
		}
	}
	b.ID = uint64(len(f.rows) + 1)
	f.rows = append(f.rows, b)
	return b, nil
}

func (f *fakeBudgets) DeleteBudgetsOf(_ context.Context, scope domain.BudgetScope, scopeID uint64) error {
	f.rows = slices.DeleteFunc(f.rows, func(b domain.Budget) bool { return b.Scope == scope && b.ScopeID == scopeID })
	f.incidents = slices.DeleteFunc(f.incidents, func(i domain.BudgetIncident) bool { return i.Scope == scope && i.ScopeID == scopeID })
	return nil
}

func TestSetBudgetAndOverview(t *testing.T) {
	ctx := context.Background()
	s, _, _, w := newTest()
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	ada, bob := hired(t, s, "Ada", 0), hired(t, s, "Bob", 0)
	w.projects = map[uint64]string{12: "Shop", 13: "Secret"}
	runs := s.runs.(*fakeRuns)
	add := func(agentID, projectID uint64, in, out int64, at time.Time) {
		runs.next++
		runs.rows[runs.next] = domain.Run{ID: runs.next, GuildID: 1, AgentID: agentID, ProjectID: projectID, Status: domain.RunSucceeded,
			StartedAt: &at, FinishedAt: &at, Usage: domain.Usage{InputTokens: in, OutputTokens: out, DurationMS: 61_500}}
	}
	add(ada.ID, 12, 600, 100, now.Add(-time.Hour))
	add(ada.ID, 0, 100, 0, now.Add(-time.Hour))
	add(bob.ID, 13, 50, 50, now.Add(-time.Hour))
	add(ada.ID, 12, 5000, 0, now.AddDate(0, -1, 0)) // last month

	terms := domain.BudgetTerms{Amount: 1000, WarnPercent: 80, HardStop: true, Notify: true}
	b, err := s.SetBudget(ctx, 1, 7, BudgetInput{Scope: domain.AgentScope, ScopeID: ada.ID, Metric: domain.TokensMetric, BudgetTerms: terms}, nil)
	if err != nil || b.Observed != 800 || b.Status != domain.BudgetWarning || b.Window != domain.CalendarMonthUTC || b.ScopeName != "Ada" {
		t.Fatalf("agent budget: %+v %v", b, err)
	}
	if !b.WindowStart.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("window start %s", b.WindowStart)
	}
	// Set into its Warning, it opens a soft incident right after.
	if n := len(w.events); n < 2 || w.events[n-1].Action != "budget.soft_threshold_crossed" {
		t.Fatalf("activity %v", w.actions)
	}
	if up := w.events[len(w.events)-2]; up.Action != "budget.updated" || up.Entity != "budget" || up.EntityID != b.ID || up.ActorID != 7 || up.AgentName != "Ada" {
		t.Errorf("activity %+v", up)
	}
	// The same scope, metric and window again changes that one Budget.
	terms.Amount = 700
	if b2, err := s.SetBudget(ctx, 1, 8, BudgetInput{Scope: domain.AgentScope, ScopeID: ada.ID, Metric: domain.TokensMetric, Window: "calendar_month_utc", BudgetTerms: terms}, nil); err != nil ||
		b2.ID != b.ID || b2.Status != domain.BudgetHardStop || b2.UpdatedBy != 8 || b2.CreatedBy != 7 {
		t.Fatalf("lowered: %+v %v", b2, err)
	}
	// A Project's Budget counts its Runs for good; run time in whole seconds.
	p, err := s.SetBudget(ctx, 1, 7, BudgetInput{Scope: domain.ProjectScope, ScopeID: 12, Metric: domain.RunTimeMetric, BudgetTerms: terms}, nil)
	if err != nil || p.Window != domain.Lifetime || p.Observed != 123 || !p.WindowStart.IsZero() {
		t.Fatalf("project budget: %+v %v", p, err)
	}
	if _, err := s.SetBudget(ctx, 1, 7, BudgetInput{Scope: domain.ProjectScope, ScopeID: 13, Metric: domain.RunsMetric, BudgetTerms: terms}, nil); err != nil {
		t.Fatal(err)
	}
	if g, err := s.SetBudget(ctx, 1, 7, BudgetInput{Scope: domain.GuildScope, ScopeID: 1, Metric: domain.RunsMetric, BudgetTerms: terms}, nil); err != nil || g.Observed != 3 || g.ScopeName != "Guild 1" {
		t.Fatalf("guild budget: %+v %v", g, err)
	}

	var fe *domain.FieldError
	for name, in := range map[string]BudgetInput{
		"another guild":       {Scope: domain.GuildScope, ScopeID: 2, Metric: domain.RunsMetric, BudgetTerms: terms},
		"another guild agent": {Scope: domain.AgentScope, ScopeID: 999, Metric: domain.RunsMetric, BudgetTerms: terms},
		"unknown project":     {Scope: domain.ProjectScope, ScopeID: 99, Metric: domain.RunsMetric, BudgetTerms: terms},
		"unknown window":      {Scope: domain.AgentScope, ScopeID: ada.ID, Metric: domain.RunsMetric, Window: "weekly", BudgetTerms: terms},
	} {
		if _, err := s.SetBudget(ctx, 1, 7, in, nil); !errors.As(err, &fe) {
			t.Errorf("%s: %v", name, err)
		}
	}

	hide13 := func(ids []uint64) ([]uint64, error) {
		return slices.DeleteFunc(slices.Clone(ids), func(id uint64) bool { return id == 13 }), nil
	}
	if _, err := s.SetBudget(ctx, 1, 7, BudgetInput{Scope: domain.ProjectScope, ScopeID: 13, Metric: domain.TokensMetric, BudgetTerms: terms}, hide13); !errors.As(err, &fe) {
		t.Errorf("a budget for a project the person may not view: %v", err)
	}
	o, err := s.BudgetOverview(ctx, 1, hide13)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, b := range o.Budgets {
		got = append(got, b.ScopeName+"/"+string(b.Metric)+"/"+string(b.Status))
	}
	if want := []string{"Guild 1/runs/ok", "Ada/tokens/hard_stop", "Shop/run_time/ok"}; !slices.Equal(got, want) {
		t.Errorf("overview %v, want %v", got, want)
	}

	// A terminated Agent's Budgets go, as do a deleted Project's.
	if _, err := s.Terminate(ctx, 1, Actor{ID: 7, Permissions: hirer}, ada.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ForgetProject(ctx, 12); err != nil {
		t.Fatal(err)
	}
	if o, _ := s.BudgetOverview(ctx, 1, nil); len(o.Budgets) != 2 {
		t.Errorf("after terminate and delete: %+v", o.Budgets)
	}
	if _, err := s.SetBudget(ctx, 1, 7, BudgetInput{Scope: domain.AgentScope, ScopeID: ada.ID, Metric: domain.RunsMetric, BudgetTerms: terms}, nil); !errors.As(err, &fe) {
		t.Errorf("a budget for a terminated agent: %v", err)
	}
}

func TestHardStop(t *testing.T) {
	ctx := context.Background()
	s, store, _, w := newTest()
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	ada, bob := hired(t, s, "Ada", 0), hired(t, s, "Bob", 0)
	w.projects = map[uint64]string{12: "Shop", 13: "Lab"}
	w.issues = map[uint64]IssueBrief{
		30: {ID: 30, ProjectID: 12, Identifier: "G-30", AgentAssigneeID: ada.ID},
		31: {ID: 31, ProjectID: 12, Identifier: "G-31", AgentAssigneeID: bob.ID},
		32: {ID: 32, ProjectID: 13, Identifier: "G-32", AgentAssigneeID: bob.ID},
	}
	runs := s.runs.(*fakeRuns)
	finish := func(agentID, projectID uint64) domain.Run {
		runs.next++
		r := domain.Run{ID: runs.next, GuildID: 1, AgentID: agentID, ProjectID: projectID, Status: domain.RunSucceeded, StartedAt: &now, FinishedAt: &now}
		runs.rows[r.ID] = r
		s.evaluateRun(ctx, r)
		return r
	}
	wake := func(agentID, issueID uint64) (domain.Run, error) {
		r, _, err := s.Wake(ctx, 1, agentID, WakeInput{Source: domain.OnDemand, Reason: domain.Manual, IssueID: issueID, ActorID: 7})
		return r, err
	}
	terms := domain.BudgetTerms{Amount: 2, WarnPercent: 50, HardStop: true, Notify: true}
	if _, err := s.SetBudget(ctx, 1, 7, BudgetInput{Scope: domain.AgentScope, ScopeID: ada.ID, Metric: domain.RunsMetric, BudgetTerms: terms}, nil); err != nil {
		t.Fatal(err)
	}
	budgets := s.budgets.(*fakeBudgets)
	if len(budgets.incidents) != 0 {
		t.Fatalf("incidents at 0: %+v", budgets.incidents)
	}

	// The Warning opens one soft incident per window.
	finish(ada.ID, 12)
	if len(budgets.incidents) != 1 || budgets.incidents[0].Threshold != domain.SoftThreshold || w.last.Action != "budget.soft_threshold_crossed" || w.last.ActorID != 0 {
		t.Fatalf("soft: %+v %+v", budgets.incidents, w.last)
	}
	queued, err := wake(ada.ID, 30)
	if err != nil {
		t.Fatal(err)
	}
	runs.next++
	running := domain.Run{ID: runs.next, GuildID: 1, AgentID: ada.ID, Status: domain.RunRunning, StartedAt: &now}
	runs.rows[running.ID] = running

	// The Hard stop opens one hard incident with its Approval, resolves the
	// soft one, pauses the Agent by budget and cancels its queued Run only.
	finish(ada.ID, 12)
	if len(budgets.incidents) != 2 || len(w.overrides) != 1 {
		t.Fatalf("hard: %+v %+v", budgets.incidents, w.overrides)
	}
	soft, hard := budgets.incidents[0], budgets.incidents[1]
	if soft.Status != domain.IncidentResolved || hard.Threshold != domain.HardThreshold || hard.Status != domain.IncidentOpen || hard.ApprovalID != 201 || hard.Observed != 2 {
		t.Fatalf("incidents: %+v %+v", soft, hard)
	}
	if a := store.rows[ada.ID]; a.Status != domain.Paused || a.PauseReason != domain.PausedByBudget {
		t.Fatalf("ada: %+v", a)
	}
	if r := runs.rows[queued.ID]; r.Status != domain.RunCancelled || r.Error != HardStopReason {
		t.Fatalf("queued run: %+v", r)
	}
	if runs.rows[running.ID].Status != domain.RunRunning {
		t.Fatal("the running run was cancelled")
	}
	if !slices.Contains(w.actions, "budget.hard_threshold_crossed") || !slices.Contains(w.actions, "agent.paused") {
		t.Errorf("actions %v", w.actions)
	}
	// Once per window: the running Run finishing opens nothing more.
	finish(ada.ID, 12)
	if len(budgets.incidents) != 2 || len(w.overrides) != 1 {
		t.Fatalf("hard again: %+v", budgets.incidents)
	}
	if _, err := wake(ada.ID, 30); err == nil {
		t.Error("a budget-paused agent woke")
	}
	if _, err := s.Resume(ctx, 1, Actor{ID: 7, Permissions: hirer}, ada.ID); !errors.Is(err, ErrBudgetStillExceeded) {
		t.Errorf("resume while exceeded: %v", err)
	}

	// A project Budget stops that Project's Runs only, and pauses nobody.
	if _, err := s.SetBudget(ctx, 1, 7, BudgetInput{Scope: domain.ProjectScope, ScopeID: 13, Metric: domain.RunsMetric,
		BudgetTerms: domain.BudgetTerms{Amount: 1, WarnPercent: 80, HardStop: true, Notify: true}}, nil); err != nil {
		t.Fatal(err)
	}
	bobQueued, err := wake(bob.ID, 32)
	if err != nil {
		t.Fatal(err)
	}
	finish(bob.ID, 13)
	if r := runs.rows[bobQueued.ID]; r.Status != domain.RunCancelled || r.Error != HardStopReason {
		t.Fatalf("bob's queued run in the lab: %+v", r)
	}
	if store.rows[bob.ID].Status == domain.Paused {
		t.Fatal("a project budget paused bob")
	}
	var block *BudgetBlock
	if _, err := wake(bob.ID, 32); !errors.As(err, &block) || block.Scope != domain.ProjectScope || block.ScopeName != "Lab" ||
		block.Reason != "Project cannot start new runs because its budget's hard stop is reached." {
		t.Fatalf("bob in the lab: %v", err)
	}
	if err := s.IssueCommented(ctx, 1, 32, bob.ID, 0, 7, false, false); err != nil {
		t.Errorf("a hook's wake in a stopped scope is dropped: %v", err)
	}
	shop, err := wake(bob.ID, 31)
	if err != nil {
		t.Fatalf("bob in the shop: %v", err)
	}
	o, err := s.BudgetOverview(ctx, 1, nil)
	if err != nil || len(o.Incidents) != 2 || o.PausedAgents != 1 || o.StoppedProjects != 1 || o.Incidents[0].ScopeName == "" {
		t.Fatalf("overview: %+v %v", o, err)
	}

	// Raising the Budget above its Observed amount resumes the Agent and
	// approves its Approval by the person.
	terms.Amount = 5
	if _, err := s.SetBudget(ctx, 1, 8, BudgetInput{Scope: domain.AgentScope, ScopeID: ada.ID, Metric: domain.RunsMetric, BudgetTerms: terms}, nil); err != nil {
		t.Fatal(err)
	}
	if a := store.rows[ada.ID]; a.Status != domain.Idle || a.PauseReason != "" {
		t.Fatalf("ada after the raise: %+v", a)
	}
	if budgets.incidents[1].Status != domain.IncidentResolved || !w.decisions[201] {
		t.Fatalf("after the raise: %+v %v", budgets.incidents[1], w.decisions)
	}
	if _, err := wake(ada.ID, 30); err != nil {
		t.Fatalf("ada after the raise: %v", err)
	}

	// A guild Budget at its Hard stop blocks every Agent, guild first.
	if _, err := s.SetBudget(ctx, 1, 7, BudgetInput{Scope: domain.GuildScope, ScopeID: 1, Metric: domain.RunsMetric,
		BudgetTerms: domain.BudgetTerms{Amount: 1, WarnPercent: 80, HardStop: true, Notify: true}}, nil); err != nil {
		t.Fatal(err)
	}
	if b, err := s.budgetBlock(ctx, 1, bob.ID, 13); err != nil || b == nil || b.Scope != domain.GuildScope {
		t.Fatalf("guild block: %+v %v", b, err)
	}
	// A deleted Project's Budget goes, and its waiting Approval is cancelled.
	labApproval := budgets.incidents[2].ApprovalID
	if err := s.ForgetProject(ctx, 13); err != nil || !slices.Contains(w.cancelled, labApproval) {
		t.Fatalf("forget the lab: %v, cancelled %v (want %d)", err, w.cancelled, labApproval)
	}
	// The guild Budget cancelled the queued Runs; one queued past it is
	// cancelled at its claim.
	if r := runs.rows[shop.ID]; r.Status != domain.RunCancelled {
		t.Fatalf("bob's shop run after the guild stop: %+v", r)
	}
	runs.next++
	late := domain.Run{ID: runs.next, GuildID: 1, AgentID: bob.ID, IssueID: 31, Status: domain.RunQueued, WakeCount: 1, NextSeq: 1}
	runs.rows[late.ID] = late
	if _, err := s.ClaimRun(ctx, Desktop{ID: 3, MemberID: 7}, late.ID); !errors.As(err, &block) || block.Scope != domain.GuildScope {
		t.Fatalf("claim in a stopped guild: %v", err)
	}
	if r := runs.rows[late.ID]; r.Status != domain.RunCancelled || r.Error != blockReasons[domain.GuildScope] {
		t.Fatalf("the late run: %+v", r)
	}
}

func TestResolveBudgetIncident(t *testing.T) {
	ctx := context.Background()
	s, store, _, w := newTest()
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	ada := hired(t, s, "Ada", 0)
	runs := s.runs.(*fakeRuns)
	budgets := s.budgets.(*fakeBudgets)
	finish := func() {
		runs.next++
		r := domain.Run{ID: runs.next, GuildID: 1, AgentID: ada.ID, Status: domain.RunSucceeded, StartedAt: &now, FinishedAt: &now}
		runs.rows[r.ID] = r
		s.evaluateRun(ctx, r)
	}
	hardStop := func(amount int64) domain.BudgetIncident {
		t.Helper()
		in := BudgetInput{Scope: domain.AgentScope, ScopeID: ada.ID, Metric: domain.RunsMetric,
			BudgetTerms: domain.BudgetTerms{Amount: amount, WarnPercent: 80, HardStop: true, Notify: true}}
		if _, err := s.SetBudget(ctx, 1, 7, in, nil); err != nil {
			t.Fatal(err)
		}
		for range amount - int64(len(runs.rows)) {
			finish()
		}
		i := budgets.incidents[len(budgets.incidents)-1]
		if i.Threshold != domain.HardThreshold || i.Status != domain.IncidentOpen || store.rows[ada.ID].PauseReason != domain.PausedByBudget {
			t.Fatalf("no hard stop at %d: %+v %+v", amount, i, store.rows[ada.ID])
		}
		return i
	}
	hard := hardStop(2)

	// Refusals: another Guild's, an amount not above the Observed amount,
	// a soft incident.
	if _, err := s.ResolveBudgetIncident(ctx, 2, 7, hard.ID, RaiseBudgetAndResume, 5, "", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("another guild's: %v", err)
	}
	var fe *domain.FieldError
	if _, err := s.ResolveBudgetIncident(ctx, 1, 7, hard.ID, RaiseBudgetAndResume, 2, "", nil); !errors.As(err, &fe) || fe.Field != "amount" {
		t.Errorf("amount equal to observed: %v", err)
	}
	soft := hard
	soft.ID, soft.Threshold, soft.ApprovalID = 99, domain.SoftThreshold, 0
	budgets.incidents = append(budgets.incidents, soft)
	if _, err := s.ResolveBudgetIncident(ctx, 1, 7, soft.ID, KeepPaused, 0, "", nil); !errors.Is(err, ErrSoftIncident) {
		t.Errorf("soft: %v", err)
	}
	budgets.incidents = budgets.incidents[:len(budgets.incidents)-1]

	// Raising resolves it, approves its Approval with the note and resumes
	// the Agent.
	got, err := s.ResolveBudgetIncident(ctx, 1, 8, hard.ID, RaiseBudgetAndResume, 4, "one more week", nil)
	if err != nil || got.Status != domain.IncidentResolved || got.ScopeName != "Ada" {
		t.Fatalf("raise: %+v %v", got, err)
	}
	if a := store.rows[ada.ID]; a.Status != domain.Idle || a.PauseReason != "" {
		t.Fatalf("ada after the raise: %+v", a)
	}
	if !w.decisions[hard.ApprovalID] || budgets.rows[0].Amount != 4 {
		t.Fatalf("after the raise: %v %+v", w.decisions, budgets.rows[0])
	}
	if w.last.Action != "budget.incident_resolved" || w.last.ActorID != 8 || w.last.Details["amount"] != int64(4) {
		t.Errorf("activity %+v", w.last)
	}
	if _, err := s.ResolveBudgetIncident(ctx, 1, 8, hard.ID, KeepPaused, 0, "", nil); err == nil {
		t.Error("a resolved incident was resolved again")
	}

	// Keeping paused dismisses it and rejects its Approval; the Agent stays
	// paused and cannot be resumed.
	for range 2 {
		finish()
	}
	hard = budgets.incidents[len(budgets.incidents)-1]
	if hard.Threshold != domain.HardThreshold || hard.Status != domain.IncidentOpen {
		t.Fatalf("second hard stop: %+v", budgets.incidents)
	}
	got, err = s.ResolveBudgetIncident(ctx, 1, 8, hard.ID, KeepPaused, 0, "", nil)
	if err != nil || got.Status != domain.IncidentDismissed {
		t.Fatalf("keep paused: %+v %v", got, err)
	}
	if approved, ok := w.decisions[hard.ApprovalID]; !ok || approved {
		t.Errorf("approval after keep paused: %v", w.decisions)
	}
	if a := store.rows[ada.ID]; a.Status != domain.Paused || a.PauseReason != domain.PausedByBudget {
		t.Fatalf("ada after keep paused: %+v", a)
	}
	if w.last.Details["amount"] != nil || w.last.Details["action"] != "keep_paused" {
		t.Errorf("activity %+v", w.last)
	}
}
