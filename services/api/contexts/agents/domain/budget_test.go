package domain

import (
	"errors"
	"testing"
	"time"
)

func TestParseBudgetTerms(t *testing.T) {
	for _, s := range []string{"guild", "agent", "project"} {
		if _, err := ParseBudgetScope(s); err != nil {
			t.Errorf("scope %s: %v", s, err)
		}
	}
	var fe *FieldError
	if _, err := ParseBudgetScope("company"); !errors.As(err, &fe) || fe.Field != "scope_type" {
		t.Errorf("unknown scope: %v", err)
	}
	if _, err := ParseBudgetMetric("cents"); !errors.As(err, &fe) || fe.Field != "metric" {
		t.Errorf("unknown metric: %v", err)
	}
	if _, err := ParseBudgetWindow("weekly", AgentScope); !errors.As(err, &fe) || fe.Field != "window" {
		t.Errorf("unknown window: %v", err)
	}
	for scope, want := range map[BudgetScope]BudgetWindow{GuildScope: CalendarMonthUTC, AgentScope: CalendarMonthUTC, ProjectScope: Lifetime} {
		if w, err := ParseBudgetWindow("", scope); err != nil || w != want {
			t.Errorf("default window of %s: %s %v", scope, w, err)
		}
	}
}

func TestBudgetWindowBounds(t *testing.T) {
	at := time.Date(2026, 12, 31, 23, 30, 0, 0, time.FixedZone("x", -2*3600))
	start, end := CalendarMonthUTC.Bounds(at)
	if !start.Equal(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)) || !end.Equal(time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("month of %s: %s – %s", at, start, end)
	}
	if start, end := Lifetime.Bounds(at); !start.IsZero() || !end.IsZero() {
		t.Errorf("lifetime has bounds: %s – %s", start, end)
	}
}

func TestNewBudget(t *testing.T) {
	at := time.Now()
	terms := BudgetTerms{Amount: 100, WarnPercent: DefaultWarnPercent, HardStop: true, Notify: true}
	var fe *FieldError
	for name, c := range map[string]struct {
		scope   BudgetScope
		scopeID uint64
		terms   BudgetTerms
		field   string
	}{
		"guild scope of another guild": {GuildScope, 2, terms, "scope_id"},
		"no agent":                     {AgentScope, 0, terms, "scope_id"},
		"negative amount":              {AgentScope, 4, BudgetTerms{Amount: -1, WarnPercent: 80}, "amount"},
		"warning of 0":                 {AgentScope, 4, BudgetTerms{Amount: 1, WarnPercent: 0}, "warn_percent"},
		"warning of 100":               {AgentScope, 4, BudgetTerms{Amount: 1, WarnPercent: 100}, "warn_percent"},
	} {
		if _, err := NewBudget(1, c.scope, c.scopeID, TokensMetric, CalendarMonthUTC, c.terms, 7, at); !errors.As(err, &fe) || fe.Field != c.field {
			t.Errorf("%s: %v", name, err)
		}
	}
	b, err := NewBudget(1, GuildScope, 1, RunsMetric, Lifetime, terms, 7, at)
	if err != nil || b.Amount != 100 || b.CreatedBy != 7 || b.UpdatedBy != 7 || !b.HardStop {
		t.Errorf("guild budget: %+v %v", b, err)
	}
}

func TestBudgetStatus(t *testing.T) {
	for _, c := range []struct {
		amount   int64
		warn     int
		observed int64
		want     BudgetStatus
	}{
		{0, 80, 1_000_000, BudgetOK},
		{100, 80, 0, BudgetOK},
		{100, 80, 79, BudgetOK},
		{100, 80, 80, BudgetWarning},
		{100, 80, 99, BudgetWarning},
		{100, 80, 100, BudgetHardStop},
		{100, 80, 250, BudgetHardStop},
		// The Warning rounds up: 80% of 7 is 5.6, so 5 is still ok.
		{7, 80, 5, BudgetOK},
		{7, 80, 6, BudgetWarning},
		{1, 1, 0, BudgetOK},
		{1, 1, 1, BudgetHardStop},
	} {
		b := Budget{Amount: c.amount, WarnPercent: c.warn}
		if got := b.Status(c.observed); got != c.want {
			t.Errorf("%d at %d%%, observed %d: %s, want %s", c.amount, c.warn, c.observed, got, c.want)
		}
	}
}
