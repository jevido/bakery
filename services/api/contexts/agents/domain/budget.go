package domain

import (
	"slices"
	"time"
)

// BudgetScope is what a Budget caps: the whole Guild, one Agent or one
// Project.
type BudgetScope string

const (
	GuildScope   BudgetScope = "guild"
	AgentScope   BudgetScope = "agent"
	ProjectScope BudgetScope = "project"
)

// BudgetScopes are the glossary's Budget scopes.
var BudgetScopes = []BudgetScope{GuildScope, AgentScope, ProjectScope}

// ParseBudgetScope reads a Budget scope type.
func ParseBudgetScope(s string) (BudgetScope, error) {
	if slices.Contains(BudgetScopes, BudgetScope(s)) {
		return BudgetScope(s), nil
	}
	return "", invalid("scope_type", "must be one of %s", joined(BudgetScopes))
}

// BudgetMetric is what a Budget counts: tokens (input plus output), Runs,
// or run time in seconds.
type BudgetMetric string

const (
	TokensMetric  BudgetMetric = "tokens"
	RunsMetric    BudgetMetric = "runs"
	RunTimeMetric BudgetMetric = "run_time"
)

// BudgetMetrics are the glossary's Budget metrics.
var BudgetMetrics = []BudgetMetric{TokensMetric, RunsMetric, RunTimeMetric}

// ParseBudgetMetric reads a Budget metric.
func ParseBudgetMetric(s string) (BudgetMetric, error) {
	if slices.Contains(BudgetMetrics, BudgetMetric(s)) {
		return BudgetMetric(s), nil
	}
	return "", invalid("metric", "must be one of %s", joined(BudgetMetrics))
}

// BudgetWindow is the stretch of time a Budget counts over.
type BudgetWindow string

const (
	CalendarMonthUTC BudgetWindow = "calendar_month_utc"
	Lifetime         BudgetWindow = "lifetime"
)

// BudgetWindows are the glossary's Budget windows.
var BudgetWindows = []BudgetWindow{CalendarMonthUTC, Lifetime}

// ParseBudgetWindow reads a Budget window; none is the scope's default,
// lifetime for a Project and the calendar month otherwise, as Paperclip's
// upsertPolicy.
func ParseBudgetWindow(s string, scope BudgetScope) (BudgetWindow, error) {
	if s == "" {
		if scope == ProjectScope {
			return Lifetime, nil
		}
		return CalendarMonthUTC, nil
	}
	if slices.Contains(BudgetWindows, BudgetWindow(s)) {
		return BudgetWindow(s), nil
	}
	return "", invalid("window", "must be one of %s", joined(BudgetWindows))
}

// Bounds is the window that holds at: [start, end) of at's calendar month
// in UTC, or zero times for lifetime, which has neither.
func (w BudgetWindow) Bounds(at time.Time) (start, end time.Time) {
	if w != CalendarMonthUTC {
		return time.Time{}, time.Time{}
	}
	at = at.UTC()
	start = time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}

// BudgetStatus is where a Budget's Observed amount stands against it.
type BudgetStatus string

const (
	BudgetOK       BudgetStatus = "ok"
	BudgetWarning  BudgetStatus = "warning"
	BudgetHardStop BudgetStatus = "hard_stop"
)

// DefaultWarnPercent is a Budget's Warning when none is given.
const DefaultWarnPercent = 80

// Budget caps what the Runs in one Budget scope may use per Budget window.
// Amount 0 caps nothing.
type Budget struct {
	ID          uint64
	GuildID     uint64
	Scope       BudgetScope
	ScopeID     uint64
	Metric      BudgetMetric
	Window      BudgetWindow
	Amount      int64
	WarnPercent int
	HardStop    bool
	Notify      bool
	CreatedBy   uint64
	UpdatedBy   uint64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// BudgetTerms are what a person sets on a Budget.
type BudgetTerms struct {
	Amount      int64
	WarnPercent int
	HardStop    bool
	Notify      bool
}

// NewBudget is a Budget for the scope, metric and window with the terms.
// A guild scope is the Guild itself.
func NewBudget(guildID uint64, scope BudgetScope, scopeID uint64, metric BudgetMetric, window BudgetWindow, t BudgetTerms, by uint64, at time.Time) (Budget, error) {
	if scope == GuildScope && scopeID != guildID {
		return Budget{}, invalid("scope_id", "a guild budget's scope is the guild itself")
	}
	if scopeID == 0 {
		return Budget{}, invalid("scope_id", "is required")
	}
	b := Budget{GuildID: guildID, Scope: scope, ScopeID: scopeID, Metric: metric, Window: window, CreatedBy: by, CreatedAt: at}
	return b, b.Set(t, by, at)
}

// Set changes the Budget's terms: an amount of at least 0 and a Warning
// of 1–99 percent.
func (b *Budget) Set(t BudgetTerms, by uint64, at time.Time) error {
	if t.Amount < 0 {
		return invalid("amount", "must be 0 or more")
	}
	if t.WarnPercent < 1 || t.WarnPercent > 99 {
		return invalid("warn_percent", "must be between 1 and 99")
	}
	b.Amount, b.WarnPercent, b.HardStop, b.Notify = t.Amount, t.WarnPercent, t.HardStop, t.Notify
	b.UpdatedBy, b.UpdatedAt = by, at
	return nil
}

// WarningAt is the Observed amount at which the Budget warns: its Warning
// percent of the amount, rounded up.
func (b Budget) WarningAt() int64 {
	return (b.Amount*int64(b.WarnPercent) + 99) / 100
}

// Status is the Budget status at the Observed amount, as Paperclip's
// budgetStatusFromObserved: a Budget of 0 is always ok, and one without
// Hard stop at most warns.
func (b Budget) Status(observed int64) BudgetStatus {
	switch {
	case b.Amount <= 0:
		return BudgetOK
	case observed >= b.Amount && b.HardStop:
		return BudgetHardStop
	case observed >= b.WarningAt():
		return BudgetWarning
	}
	return BudgetOK
}

// Threshold is which of a Budget's thresholds a Budget incident records:
// its Warning or its Hard stop.
type Threshold string

const (
	SoftThreshold Threshold = "soft"
	HardThreshold Threshold = "hard"
)

// IncidentStatus is where a Budget incident stands.
type IncidentStatus string

const (
	IncidentOpen      IncidentStatus = "open"
	IncidentResolved  IncidentStatus = "resolved"
	IncidentDismissed IncidentStatus = "dismissed"
)

// BudgetIncident records one threshold of one Budget crossed in one
// Budget window (zero bounds for lifetime), with the amount and Observed
// amount when it opened. A hard one has its budget_override_required
// Approval.
type BudgetIncident struct {
	ID          uint64
	GuildID     uint64
	BudgetID    uint64
	Scope       BudgetScope
	ScopeID     uint64
	Metric      BudgetMetric
	Window      BudgetWindow
	WindowStart time.Time
	WindowEnd   time.Time
	Threshold   Threshold
	Amount      int64
	Observed    int64
	Status      IncidentStatus
	ApprovalID  uint64
	ResolvedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// OpenIncident is an open Budget incident of the Budget at the threshold,
// observed in the window that holds at.
func OpenIncident(b Budget, t Threshold, observed int64, at time.Time) BudgetIncident {
	start, end := b.Window.Bounds(at)
	return BudgetIncident{
		GuildID: b.GuildID, BudgetID: b.ID, Scope: b.Scope, ScopeID: b.ScopeID, Metric: b.Metric, Window: b.Window,
		WindowStart: start, WindowEnd: end, Threshold: t, Amount: b.Amount, Observed: observed, Status: IncidentOpen,
		CreatedAt: at, UpdatedAt: at,
	}
}

// InWindow reports whether the incident belongs to the Budget's window
// that holds at.
func (i BudgetIncident) InWindow(at time.Time) bool {
	start, _ := i.Window.Bounds(at)
	return i.WindowStart.Equal(start)
}

// IncidentStatusError refuses a change the incident's status does not
// allow.
type IncidentStatusError struct {
	Status IncidentStatus
	Action string
}

func (e *IncidentStatusError) Error() string {
	return "a " + string(e.Status) + " budget incident cannot be " + e.Action
}

// Resolve closes an open incident: its Budget was raised above the
// Observed amount.
func (i *BudgetIncident) Resolve(at time.Time) error {
	return i.close(IncidentResolved, "resolved", at)
}

// Dismiss closes an open incident while its scope stays stopped: the
// Board kept it paused.
func (i *BudgetIncident) Dismiss(at time.Time) error {
	return i.close(IncidentDismissed, "dismissed", at)
}

func (i *BudgetIncident) close(to IncidentStatus, verb string, at time.Time) error {
	if i.Status != IncidentOpen {
		return &IncidentStatusError{Status: i.Status, Action: verb}
	}
	i.Status, i.ResolvedAt, i.UpdatedAt = to, &at, at
	return nil
}
