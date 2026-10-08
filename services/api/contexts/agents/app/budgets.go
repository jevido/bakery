package app

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// Budgets keeps Budgets.
type Budgets interface {
	// Budgets lists the Guild's Budgets.
	Budgets(ctx context.Context, guildID uint64) ([]domain.Budget, error)
	// SaveBudget creates the Budget, or changes the one for its Guild,
	// scope, metric and window, and answers it as stored.
	SaveBudget(ctx context.Context, b domain.Budget) (domain.Budget, error)
	// DeleteBudgetsOf removes the Budgets of the scope, in every Guild.
	DeleteBudgetsOf(ctx context.Context, scope domain.BudgetScope, scopeID uint64) error
}

// BudgetInput is a Budget as typed: the scope, metric and window it caps
// (Window "" for the scope's default) and its terms.
type BudgetInput struct {
	Scope   domain.BudgetScope
	ScopeID uint64
	Metric  domain.BudgetMetric
	Window  string
	domain.BudgetTerms
}

// BudgetSummary is a Budget with its scope's name, its Observed amount and
// status in its current window (zero bounds for lifetime).
type BudgetSummary struct {
	domain.Budget
	ScopeName   string
	Observed    int64
	Status      domain.BudgetStatus
	WindowStart time.Time
	WindowEnd   time.Time
}

// BudgetOverview is the Guild's Budgets the person may see, with their
// open Budget incidents and stopped scopes.
type BudgetOverview struct {
	Budgets         []BudgetSummary
	PausedAgents    int
	StoppedProjects int
}

// SetBudget creates or changes the one Budget of the Guild for the scope,
// metric and window, by the person. The scope must be the Guild, one of
// its Agents that is not terminated or one of its Projects the person may
// view (visible).
func (s *Service) SetBudget(ctx context.Context, guildID, memberID uint64, in BudgetInput, visible Visible) (BudgetSummary, error) {
	window, err := domain.ParseBudgetWindow(in.Window, in.Scope)
	if err != nil {
		return BudgetSummary{}, err
	}
	name, err := s.scopeName(ctx, guildID, in.Scope, in.ScopeID, visible)
	if err != nil {
		return BudgetSummary{}, err
	}
	at := s.now()
	all, err := s.budgets.Budgets(ctx, guildID)
	if err != nil {
		return BudgetSummary{}, err
	}
	i := slices.IndexFunc(all, func(b domain.Budget) bool {
		return b.Scope == in.Scope && b.ScopeID == in.ScopeID && b.Metric == in.Metric && b.Window == window
	})
	var b domain.Budget
	if i >= 0 {
		b = all[i]
		err = b.Set(in.BudgetTerms, memberID, at)
	} else {
		b, err = domain.NewBudget(guildID, in.Scope, in.ScopeID, in.Metric, window, in.BudgetTerms, memberID, at)
	}
	if err != nil {
		return BudgetSummary{}, err
	}
	if b, err = s.budgets.SaveBudget(ctx, b); err != nil {
		return BudgetSummary{}, err
	}
	act := Activity{GuildID: guildID, ActorID: memberID, Entity: "budget", EntityID: b.ID, Action: "budget.updated", AgentName: name, Details: map[string]any{
		"scope_type": string(b.Scope), "scope_id": b.ScopeID, "metric": string(b.Metric), "window": string(b.Window),
		"amount": b.Amount, "warn_percent": b.WarnPercent, "hard_stop": b.HardStop, "notify": b.Notify,
	}}
	if err := s.work.RecordActivity(ctx, act); err != nil {
		s.Logf("agents: recording budget.updated of budget %d: %v", b.ID, err)
	}
	totals := map[domain.BudgetWindow][]RunTotal{}
	return s.summarize(ctx, b, name, totals, at)
}

// scopeName names the Budget scope, or refuses one outside the Guild on
// scope_id.
func (s *Service) scopeName(ctx context.Context, guildID uint64, scope domain.BudgetScope, id uint64, visible Visible) (string, error) {
	outside := &domain.FieldError{Field: "scope_id", Message: "must be this guild, one of its agents or one of its projects"}
	switch scope {
	case domain.GuildScope:
		if id != guildID {
			return "", outside
		}
		return s.guilds.GuildName(ctx, guildID)
	case domain.AgentScope:
		a, ok, err := s.agents.Agent(ctx, id)
		if err != nil {
			return "", err
		}
		if !ok || a.GuildID != guildID || a.Status == domain.Terminated {
			return "", outside
		}
		return a.Name, nil
	}
	ids := []uint64{id}
	if visible != nil {
		var err error
		if ids, err = visible(ids); err != nil {
			return "", err
		}
	}
	names, err := s.repositories.ProjectNames(ctx, guildID, ids)
	if err != nil {
		return "", err
	}
	name, ok := names[id]
	if !ok {
		return "", outside
	}
	return name, nil
}

// summarize adds the Budget's Observed amount in its window at now, from
// the Guild's Run totals since the window began (cached per window in
// totals).
func (s *Service) summarize(ctx context.Context, b domain.Budget, name string, totals map[domain.BudgetWindow][]RunTotal, now time.Time) (BudgetSummary, error) {
	start, end := b.Window.Bounds(now)
	ts, ok := totals[b.Window]
	if !ok {
		q := CostRange{GuildID: b.GuildID}
		if !start.IsZero() {
			q.From = &start
		}
		var err error
		if ts, err = s.runs.RunTotals(ctx, q); err != nil {
			return BudgetSummary{}, err
		}
		totals[b.Window] = ts
	}
	var f Figures
	for _, t := range ts {
		if b.Scope == domain.AgentScope && t.AgentID != b.ScopeID || b.Scope == domain.ProjectScope && t.ProjectID != b.ScopeID {
			continue
		}
		f.add(t.Figures)
	}
	observed := observedOf(b.Metric, f)
	return BudgetSummary{Budget: b, ScopeName: name, Observed: observed, Status: b.Status(observed), WindowStart: start, WindowEnd: end}, nil
}

// observedOf is what the metric counts in the Figures: input plus output
// tokens, Runs, or whole seconds of run time.
func observedOf(m domain.BudgetMetric, f Figures) int64 {
	switch m {
	case domain.RunsMetric:
		return f.Runs
	case domain.RunTimeMetric:
		return f.RunTimeMS / 1000
	}
	return f.Tokens()
}

// BudgetOverview is the Guild's Budgets, guild first, then Agents and
// Projects by name. A Budget of a Project the person may not view
// (visible) is left out.
func (s *Service) BudgetOverview(ctx context.Context, guildID uint64, visible Visible) (BudgetOverview, error) {
	all, err := s.budgets.Budgets(ctx, guildID)
	if err != nil {
		return BudgetOverview{}, err
	}
	out := BudgetOverview{Budgets: []BudgetSummary{}}
	if len(all) == 0 {
		return out, nil
	}
	names := map[domain.BudgetScope]map[uint64]string{domain.GuildScope: {}, domain.AgentScope: {}, domain.ProjectScope: {}}
	var projectIDs []uint64
	for _, b := range all {
		switch b.Scope {
		case domain.GuildScope:
			if len(names[domain.GuildScope]) == 0 {
				name, err := s.guilds.GuildName(ctx, guildID)
				if err != nil {
					return BudgetOverview{}, err
				}
				names[domain.GuildScope][guildID] = name
			}
		case domain.ProjectScope:
			if !slices.Contains(projectIDs, b.ScopeID) {
				projectIDs = append(projectIDs, b.ScopeID)
			}
		}
	}
	agents, err := s.agents.Agents(ctx, guildID)
	if err != nil {
		return BudgetOverview{}, err
	}
	for _, a := range agents {
		names[domain.AgentScope][a.ID] = a.Name
	}
	if len(projectIDs) > 0 && visible != nil {
		if projectIDs, err = visible(projectIDs); err != nil {
			return BudgetOverview{}, err
		}
	}
	if len(projectIDs) > 0 {
		if names[domain.ProjectScope], err = s.repositories.ProjectNames(ctx, guildID, projectIDs); err != nil {
			return BudgetOverview{}, err
		}
	}
	at := s.now()
	totals := map[domain.BudgetWindow][]RunTotal{}
	for _, b := range all {
		name, ok := names[b.Scope][b.ScopeID]
		if !ok {
			continue
		}
		sum, err := s.summarize(ctx, b, name, totals, at)
		if err != nil {
			return BudgetOverview{}, err
		}
		out.Budgets = append(out.Budgets, sum)
	}
	order := map[domain.BudgetScope]int{domain.GuildScope: 0, domain.AgentScope: 1, domain.ProjectScope: 2}
	slices.SortStableFunc(out.Budgets, func(x, y BudgetSummary) int {
		return cmp.Or(cmp.Compare(order[x.Scope], order[y.Scope]), cmp.Compare(x.ScopeName, y.ScopeName),
			cmp.Compare(x.Metric, y.Metric), cmp.Compare(x.Window, y.Window))
	})
	return out, nil
}

// ForgetProject removes the Budgets of a deleted Project.
func (s *Service) ForgetProject(ctx context.Context, projectID uint64) error {
	return s.budgets.DeleteBudgetsOf(ctx, domain.ProjectScope, projectID)
}
