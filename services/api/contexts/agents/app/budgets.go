package app

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

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
	// BudgetIncidents lists the Guild's Budget incidents of the Budget (0
	// for any) in the statuses (any for none), oldest first.
	BudgetIncidents(ctx context.Context, guildID, budgetID uint64, statuses []domain.IncidentStatus) ([]domain.BudgetIncident, error)
	// OpenIncident stores the new incident; opened is false when one not
	// dismissed already holds its Budget, window, threshold and amount.
	OpenIncident(ctx context.Context, i domain.BudgetIncident) (stored domain.BudgetIncident, opened bool, err error)
	// OpenIncidentsOf lists the open Budget incidents of the scope, in
	// every Guild.
	OpenIncidentsOf(ctx context.Context, scope domain.BudgetScope, scopeID uint64) ([]domain.BudgetIncident, error)
	// SaveIncident stores the incident's status, Approval and resolution
	// only while it is still open; saved is false otherwise.
	SaveIncident(ctx context.Context, i domain.BudgetIncident) (saved bool, err error)
}

// HardStopReason is the error of a queued Run a Hard stop cancelled.
const HardStopReason = "Cancelled because the budget's hard stop was reached."

// ErrBudgetStillExceeded refuses to Resume an Agent paused by budget while
// its own Budget is still at its Hard stop.
var ErrBudgetStillExceeded = errors.New("budget still exceeded")

// BudgetBlock refuses a Wake or a claim in a stopped scope: the Budget
// scope at its Hard stop, named, and why, as Paperclip's
// getInvocationBlock.
type BudgetBlock struct {
	Scope     domain.BudgetScope
	ScopeID   uint64
	ScopeName string
	Reason    string
}

func (b *BudgetBlock) Error() string { return b.Reason }

// blockReasons are the reasons of a BudgetBlock, per scope.
var blockReasons = map[domain.BudgetScope]string{
	domain.GuildScope:   "Guild cannot start new runs because its budget's hard stop is reached.",
	domain.AgentScope:   "Agent is paused because its budget's hard stop was reached.",
	domain.ProjectScope: "Project cannot start new runs because its budget's hard stop is reached.",
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

// IncidentSummary is a Budget incident with its scope's name.
type IncidentSummary struct {
	domain.BudgetIncident
	ScopeName string
}

// BudgetOverview is the Guild's Budgets the person may see, with their
// open Budget incidents, the Agents paused by budget and the Projects at a
// Hard stop.
type BudgetOverview struct {
	Budgets         []BudgetSummary
	Incidents       []IncidentSummary
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
	sum, err := s.summarize(ctx, b, name, totals, at)
	if err != nil {
		return BudgetSummary{}, err
	}
	if err := s.evaluate(ctx, sum, memberID); err != nil {
		s.Logf("agents: evaluating budget %d: %v", b.ID, err)
	}
	return sum, nil
}

// countsToward reports whether a Run of the Agent for the Project (0 for
// none) counts toward the Budget.
func countsToward(b domain.Budget, agentID, projectID uint64) bool {
	switch b.Scope {
	case domain.AgentScope:
		return b.ScopeID == agentID
	case domain.ProjectScope:
		return projectID != 0 && b.ScopeID == projectID
	}
	return true
}

// evaluateRun evaluates every Budget the finished Run counts toward; a
// failure is logged, as the Run is stored already.
func (s *Service) evaluateRun(ctx context.Context, r domain.Run) {
	all, err := s.budgets.Budgets(ctx, r.GuildID)
	if err != nil {
		s.Logf("agents: reading the budgets after run %d: %v", r.ID, err)
		return
	}
	totals := map[domain.BudgetWindow][]RunTotal{}
	for _, b := range all {
		if !countsToward(b, r.AgentID, r.ProjectID) {
			continue
		}
		name, err := s.scopeName(ctx, b.GuildID, b.Scope, b.ScopeID, nil)
		if err != nil {
			s.Logf("agents: naming the scope of budget %d: %v", b.ID, err)
			continue
		}
		sum, err := s.summarize(ctx, b, name, totals, s.now())
		if err == nil {
			err = s.evaluate(ctx, sum, 0)
		}
		if err != nil {
			s.Logf("agents: evaluating budget %d after run %d: %v", b.ID, r.ID, err)
		}
	}
}

// evaluate opens the Budget's incidents its Observed amount calls for in
// its current window, stopping the scope at a Hard stop, and resolves the
// open ones a raise lifted, deciding their Approvals approved by the
// person (0 when a Run finished) and resuming an Agent it paused.
func (s *Service) evaluate(ctx context.Context, b BudgetSummary, memberID uint64) error {
	at := s.now()
	all, err := s.budgets.BudgetIncidents(ctx, b.GuildID, b.ID, []domain.IncidentStatus{domain.IncidentOpen, domain.IncidentResolved})
	if err != nil {
		return err
	}
	held := map[domain.Threshold]bool{}
	var open []domain.BudgetIncident
	for _, i := range all {
		if !i.InWindow(at) {
			continue
		}
		// One a raise resolved does not hold the new amount; an open one
		// still asks the Board, whatever its amount.
		if i.Amount == b.Amount || i.Status == domain.IncidentOpen {
			held[i.Threshold] = true
		}
		if i.Status == domain.IncidentOpen {
			open = append(open, i)
		}
	}
	for _, i := range open {
		// A hard one is lifted below the Hard stop; a soft one is replaced
		// by a hard one, ends below the Warning, or was seen by the person
		// who changed the Budget.
		if i.Threshold == domain.HardThreshold && b.Status != domain.BudgetHardStop ||
			i.Threshold == domain.SoftThreshold && (b.Status != domain.BudgetWarning || memberID != 0) {
			if err := s.resolveIncident(ctx, i, memberID); err != nil {
				return err
			}
		}
	}
	switch b.Status {
	case domain.BudgetWarning:
		if b.Notify && !held[domain.SoftThreshold] {
			if _, err := s.openIncident(ctx, b, domain.SoftThreshold); err != nil {
				return err
			}
		}
	case domain.BudgetHardStop:
		if held[domain.HardThreshold] {
			return nil
		}
		opened, err := s.openIncident(ctx, b, domain.HardThreshold)
		if err != nil || !opened {
			return err
		}
		return s.stop(ctx, b)
	}
	if b.Scope == domain.AgentScope {
		return s.resumeFromBudget(ctx, b, memberID)
	}
	return nil
}

// openIncident opens a Budget incident at the threshold, with its
// budget_override_required Approval when it is hard, and records it.
// opened is false when another request opened it first.
func (s *Service) openIncident(ctx context.Context, b BudgetSummary, t domain.Threshold) (bool, error) {
	i, opened, err := s.budgets.OpenIncident(ctx, domain.OpenIncident(b.Budget, t, b.Observed, s.now()))
	if err != nil || !opened {
		return false, err
	}
	details := map[string]any{
		"budget_id": b.ID, "scope_type": string(b.Scope), "scope_id": b.ScopeID, "metric": string(b.Metric),
		"amount": b.Amount, "observed": b.Observed,
	}
	action := "budget.soft_threshold_crossed"
	if t == domain.HardThreshold {
		action = "budget.hard_threshold_crossed"
		if i.ApprovalID, err = s.work.RequestBudgetOverride(ctx, b, i); err != nil {
			s.Logf("agents: asking the board about budget incident %d: %v", i.ID, err)
		} else if _, err := s.budgets.SaveIncident(ctx, i); err != nil {
			return true, err
		}
		details["approval_id"] = i.ApprovalID
	}
	act := Activity{GuildID: b.GuildID, Entity: "budget_incident", EntityID: i.ID, Action: action, AgentName: b.ScopeName, Details: details}
	if err := s.work.RecordActivity(ctx, act); err != nil {
		s.Logf("agents: recording %s of budget incident %d: %v", action, i.ID, err)
	}
	return true, nil
}

// resolveIncident resolves an open incident whose Budget was raised, and
// approves its Approval by the person who raised it.
func (s *Service) resolveIncident(ctx context.Context, i domain.BudgetIncident, memberID uint64) error {
	if err := i.Resolve(s.now()); err != nil {
		return err
	}
	saved, err := s.budgets.SaveIncident(ctx, i)
	if err != nil || !saved {
		return err
	}
	if i.ApprovalID != 0 && memberID != 0 {
		if err := s.work.DecideBudgetOverride(ctx, i.GuildID, memberID, i.ApprovalID, true, ""); err != nil {
			s.Logf("agents: approving the budget override %d: %v", i.ApprovalID, err)
		}
	}
	return nil
}

// stop stops the Budget's scope at its Hard stop: an agent scope pauses
// its Agent by budget, letting its running Run finish, and every scope
// cancels its queued Runs.
func (s *Service) stop(ctx context.Context, b BudgetSummary) error {
	q := RunQuery{GuildID: b.GuildID, Statuses: []domain.RunStatus{domain.RunQueued}}
	if b.Scope == domain.AgentScope {
		q.AgentID = b.ScopeID
		a, ok, err := s.agents.Agent(ctx, b.ScopeID)
		if err != nil {
			return err
		}
		if ok && a.PauseForBudget(s.now()) == nil {
			if err := s.agents.SaveAgent(ctx, a); err != nil {
				return err
			}
			s.record(ctx, domain.AgentPaused{Agent: a})
		}
	}
	rs, err := s.runs.Runs(ctx, q)
	if err != nil {
		return err
	}
	agents := map[uint64]domain.Agent{}
	for _, r := range rs {
		if b.Scope == domain.ProjectScope {
			if r.IssueID == 0 {
				continue
			}
			i, ok, err := s.work.IssueForRun(ctx, r.GuildID, r.IssueID)
			if err != nil {
				return err
			}
			if !ok || i.ProjectID != b.ScopeID {
				continue
			}
		}
		a, ok := agents[r.AgentID]
		if !ok {
			if a, _, err = s.agents.Agent(ctx, r.AgentID); err != nil {
				return err
			}
			agents[r.AgentID] = a
		}
		var se *domain.RunStatusError
		if _, err := s.cancelRunBecause(ctx, a, r, 0, HardStopReason); err != nil && !errors.As(err, &se) {
			return err
		}
	}
	return nil
}

// resumeFromBudget resumes the agent scope's Agent paused by budget once
// none of its own Budgets is at its Hard stop, by the person who raised it
// (0 for none).
func (s *Service) resumeFromBudget(ctx context.Context, b BudgetSummary, memberID uint64) error {
	a, ok, err := s.agents.Agent(ctx, b.ScopeID)
	if err != nil || !ok || a.Status != domain.Paused || a.PauseReason != domain.PausedByBudget {
		return err
	}
	if block, err := s.agentBlock(ctx, a); err != nil || block != nil {
		return err
	}
	if err := a.Resume(s.now()); err != nil {
		return err
	}
	if err := s.agents.SaveAgent(ctx, a); err != nil {
		return err
	}
	s.record(ctx, domain.AgentResumed{Agent: a, ActorID: memberID})
	return nil
}

// agentBlock is the BudgetBlock of one of the Agent's own Budgets at its
// Hard stop, nil for none.
func (s *Service) agentBlock(ctx context.Context, a domain.Agent) (*BudgetBlock, error) {
	return s.blockIn(ctx, a.GuildID, func(b domain.Budget) bool { return b.Scope == domain.AgentScope && b.ScopeID == a.ID })
}

// budgetBlock refuses a Run of the Agent for the Project (0 for none) in
// a stopped scope, checking the Guild, then the Agent, then the Project,
// as Paperclip's getInvocationBlock; nil when nothing stops it.
func (s *Service) budgetBlock(ctx context.Context, guildID, agentID, projectID uint64) (*BudgetBlock, error) {
	return s.blockIn(ctx, guildID, func(b domain.Budget) bool { return countsToward(b, agentID, projectID) })
}

func (s *Service) blockIn(ctx context.Context, guildID uint64, counts func(domain.Budget) bool) (*BudgetBlock, error) {
	all, err := s.budgets.Budgets(ctx, guildID)
	if err != nil {
		return nil, err
	}
	order := map[domain.BudgetScope]int{domain.GuildScope: 0, domain.AgentScope: 1, domain.ProjectScope: 2}
	slices.SortStableFunc(all, func(x, y domain.Budget) int { return cmp.Compare(order[x.Scope], order[y.Scope]) })
	totals := map[domain.BudgetWindow][]RunTotal{}
	at := s.now()
	for _, b := range all {
		if b.Amount <= 0 || !b.HardStop || !counts(b) {
			continue
		}
		sum, err := s.summarize(ctx, b, "", totals, at)
		if err != nil {
			return nil, err
		}
		if sum.Status != domain.BudgetHardStop {
			continue
		}
		name, err := s.scopeName(ctx, guildID, b.Scope, b.ScopeID, nil)
		if err != nil {
			var fe *domain.FieldError
			if !errors.As(err, &fe) {
				return nil, err
			}
		}
		return &BudgetBlock{Scope: b.Scope, ScopeID: b.ScopeID, ScopeName: name, Reason: blockReasons[b.Scope]}, nil
	}
	return nil, nil
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
	out := BudgetOverview{Budgets: []BudgetSummary{}, Incidents: []IncidentSummary{}}
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
		if a.Status == domain.Paused && a.PauseReason == domain.PausedByBudget {
			out.PausedAgents++
		}
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
	stopped := map[uint64]bool{}
	for _, b := range out.Budgets {
		if b.Scope == domain.ProjectScope && b.Status == domain.BudgetHardStop {
			stopped[b.ScopeID] = true
		}
	}
	out.StoppedProjects = len(stopped)
	open, err := s.budgets.BudgetIncidents(ctx, guildID, 0, []domain.IncidentStatus{domain.IncidentOpen})
	if err != nil {
		return BudgetOverview{}, err
	}
	for _, i := range open {
		// An incident of a Budget left out (a hidden Project's) is too.
		if name, ok := names[i.Scope][i.ScopeID]; ok {
			out.Incidents = append(out.Incidents, IncidentSummary{BudgetIncident: i, ScopeName: name})
		}
	}
	slices.SortStableFunc(out.Incidents, func(x, y IncidentSummary) int { return y.CreatedAt.Compare(x.CreatedAt) })
	return out, nil
}

// ForgetProject removes the Budgets of a deleted Project.
func (s *Service) ForgetProject(ctx context.Context, projectID uint64) error {
	return s.forgetBudgets(ctx, domain.ProjectScope, projectID, 0)
}

// forgetBudgets removes the scope's Budgets, with their incidents, and
// cancels the budget_override_required Approvals still waiting on them, by
// the person (0 for none).
func (s *Service) forgetBudgets(ctx context.Context, scope domain.BudgetScope, scopeID, actorID uint64) error {
	open, err := s.budgets.OpenIncidentsOf(ctx, scope, scopeID)
	if err != nil {
		return err
	}
	for _, i := range open {
		if i.ApprovalID == 0 {
			continue
		}
		if err := s.work.CancelApproval(ctx, i.GuildID, actorID, i.ApprovalID); err != nil {
			s.Logf("agents: cancelling the budget override %d: %v", i.ApprovalID, err)
		}
	}
	return s.budgets.DeleteBudgetsOf(ctx, scope, scopeID)
}

// IncidentAction is what the Board does with a hard Budget incident.
type IncidentAction string

const (
	RaiseBudgetAndResume IncidentAction = "raise_budget_and_resume"
	KeepPaused           IncidentAction = "keep_paused"
)

// ParseIncidentAction is the IncidentAction on the wire, or a FieldError
// on action.
func ParseIncidentAction(s string) (IncidentAction, error) {
	switch a := IncidentAction(s); a {
	case RaiseBudgetAndResume, KeepPaused:
		return a, nil
	}
	return "", &domain.FieldError{Field: "action", Message: "must be raise_budget_and_resume or keep_paused"}
}

// ResolveBudgetIncident is the person's answer to one of the Guild's open
// hard Budget incidents, as Paperclip's resolveIncident. Raising sets the
// Budget's amount, which must exceed its Observed amount now, resolves its
// open incidents, approves their Approvals and resumes an Agent it paused;
// keeping paused dismisses the incident and rejects its Approval, and the
// scope stays stopped. An incident of a Project the person may not view
// (visible) is not found.
func (s *Service) ResolveBudgetIncident(ctx context.Context, guildID, memberID, incidentID uint64, action IncidentAction, amount int64, note string, visible Visible) (IncidentSummary, error) {
	// work keeps a Decision note to this length; checked here, as its
	// Decision follows the incident's change.
	if utf8.RuneCountInString(strings.TrimSpace(note)) > MaxDecisionNote {
		return IncidentSummary{}, &domain.FieldError{Field: "decision_note", Message: "is at most 20000 characters"}
	}
	all, err := s.budgets.BudgetIncidents(ctx, guildID, 0, nil)
	if err != nil {
		return IncidentSummary{}, err
	}
	at := slices.IndexFunc(all, func(i domain.BudgetIncident) bool { return i.ID == incidentID })
	if at < 0 {
		return IncidentSummary{}, ErrNotFound
	}
	i := all[at]
	name, err := s.scopeName(ctx, guildID, i.Scope, i.ScopeID, visible)
	if err != nil {
		var fe *domain.FieldError
		if errors.As(err, &fe) {
			return IncidentSummary{}, ErrNotFound
		}
		return IncidentSummary{}, err
	}
	if i.Status != domain.IncidentOpen {
		return IncidentSummary{}, &domain.IncidentStatusError{Status: i.Status, Action: "resolved"}
	}
	if i.Threshold != domain.HardThreshold {
		return IncidentSummary{}, ErrSoftIncident
	}
	details := map[string]any{"action": string(action), "amount": nil, "scope_type": string(i.Scope), "scope_id": i.ScopeID}
	switch action {
	case RaiseBudgetAndResume:
		if err := s.raiseBudget(ctx, &i, memberID, amount, name, note); err != nil {
			return IncidentSummary{}, err
		}
		details["amount"] = amount
	case KeepPaused:
		if err := s.closeIncident(ctx, &i, memberID, false, note); err != nil {
			return IncidentSummary{}, err
		}
	default:
		return IncidentSummary{}, &domain.FieldError{Field: "action", Message: "must be raise_budget_and_resume or keep_paused"}
	}
	act := Activity{GuildID: guildID, ActorID: memberID, Entity: "budget_incident", EntityID: i.ID, Action: "budget.incident_resolved", AgentName: name, Details: details}
	if err := s.work.RecordActivity(ctx, act); err != nil {
		s.Logf("agents: recording budget.incident_resolved of budget incident %d: %v", i.ID, err)
	}
	return IncidentSummary{BudgetIncident: i, ScopeName: name}, nil
}

// MaxDecisionNote is the longest note a person may give with a
// resolution, in characters: work's longest Decision note.
const MaxDecisionNote = 20000

// ErrSoftIncident refuses to resolve a soft Budget incident by hand: it is
// resolved when its Budget changes.
var ErrSoftIncident = errors.New("only a hard budget incident can be resolved")

// raiseBudget sets the incident's Budget to the amount, which must exceed
// its Observed amount now, resolves the incident with the note and then
// evaluates the Budget, which resolves its other open incidents and
// resumes an Agent it paused.
func (s *Service) raiseBudget(ctx context.Context, i *domain.BudgetIncident, memberID uint64, amount int64, name, note string) error {
	all, err := s.budgets.Budgets(ctx, i.GuildID)
	if err != nil {
		return err
	}
	at := slices.IndexFunc(all, func(b domain.Budget) bool { return b.ID == i.BudgetID })
	if at < 0 {
		return ErrNotFound
	}
	b := all[at]
	now := s.now()
	sum, err := s.summarize(ctx, b, name, map[domain.BudgetWindow][]RunTotal{}, now)
	if err != nil {
		return err
	}
	if amount <= sum.Observed {
		return &domain.FieldError{Field: "amount", Message: "new budget must exceed the observed amount"}
	}
	terms := domain.BudgetTerms{Amount: amount, WarnPercent: b.WarnPercent, HardStop: b.HardStop, Notify: b.Notify}
	if err := b.Set(terms, memberID, now); err != nil {
		return err
	}
	if b, err = s.budgets.SaveBudget(ctx, b); err != nil {
		return err
	}
	act := Activity{GuildID: b.GuildID, ActorID: memberID, Entity: "budget", EntityID: b.ID, Action: "budget.updated", AgentName: name, Details: map[string]any{
		"scope_type": string(b.Scope), "scope_id": b.ScopeID, "metric": string(b.Metric), "window": string(b.Window),
		"amount": b.Amount, "warn_percent": b.WarnPercent, "hard_stop": b.HardStop, "notify": b.Notify,
	}}
	if err := s.work.RecordActivity(ctx, act); err != nil {
		s.Logf("agents: recording budget.updated of budget %d: %v", b.ID, err)
	}
	if err := s.closeIncident(ctx, i, memberID, true, note); err != nil {
		return err
	}
	sum.Budget = b
	sum.Status = b.Status(sum.Observed)
	return s.evaluate(ctx, sum, memberID)
}

// closeIncident resolves (approved) or dismisses the open incident and
// decides its Approval by the person with the note. Another request that
// closed it first refuses it.
func (s *Service) closeIncident(ctx context.Context, i *domain.BudgetIncident, memberID uint64, approved bool, note string) error {
	close := i.Dismiss
	if approved {
		close = i.Resolve
	}
	if err := close(s.now()); err != nil {
		return err
	}
	saved, err := s.budgets.SaveIncident(ctx, *i)
	if err != nil {
		return err
	}
	if !saved {
		return &domain.IncidentStatusError{Status: domain.IncidentResolved, Action: "resolved again"}
	}
	if i.ApprovalID != 0 {
		if err := s.work.DecideBudgetOverride(ctx, i.GuildID, memberID, i.ApprovalID, approved, note); err != nil {
			s.Logf("agents: deciding the budget override %d: %v", i.ApprovalID, err)
		}
	}
	return nil
}
