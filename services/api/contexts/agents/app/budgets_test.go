package app

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// fakeBudgets keeps Budgets in memory, upserting as the unique key does.
type fakeBudgets struct {
	rows []domain.Budget
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
	if w.last.Action != "budget.updated" || w.last.Entity != "budget" || w.last.EntityID != b.ID || w.last.ActorID != 7 || w.last.AgentName != "Ada" {
		t.Errorf("activity %+v", w.last)
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
