package domain

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func names(vars []RoutineVariable) []string {
	out := []string{}
	for _, v := range vars {
		out = append(out, v.Name)
	}
	return out
}

func TestVariableNames(t *testing.T) {
	got := VariableNames("Triage {{repo}} on {{ date }}", "See {{pr\\_url}}, {{repo}} and {{1bad}} {{ unknown_Thing }}")
	if want := []string{"repo", "date", "pr_url", "unknown_Thing"}; !slices.Equal(got, want) {
		t.Fatalf("VariableNames = %v, want %v", got, want)
	}
}

func TestRoutineVariablesFollowThePlaceholders(t *testing.T) {
	r, err := NewRoutine(1, ByMember(7), "Triage {{repo}} alert {{timestamp}}", "Due {{dueDate}}, {{date}}")
	if err != nil {
		t.Fatal(err)
	}
	if got := names(r.Variables); !slices.Equal(got, []string{"repo", "dueDate"}) {
		t.Fatalf("variables %v: the built-ins are never listed", got)
	}
	if r.Variables[0].Type != TextVariable || !r.Variables[0].Required || r.Variables[0].Default != nil || r.Variables[1].Type != DateVariable {
		t.Fatalf("new variables %+v", r.Variables)
	}
	if err := r.SetVariables([]RoutineVariable{{Name: "repo", Type: SelectVariable, Options: []string{"shop", "blog"}, Default: "blog", Required: true}, {Name: "gone", Type: TextVariable}}); err != nil {
		t.Fatal(err)
	}
	if got := names(r.Variables); !slices.Equal(got, []string{"repo", "dueDate"}) || r.Variables[0].Type != SelectVariable {
		t.Fatalf("after SetVariables %+v: a name that is not a placeholder is dropped", r.Variables)
	}
	// A title edit keeps an existing definition and drops a placeholder
	// that went.
	if err := r.Rename("Triage {{repo}} and {{area}}"); err != nil {
		t.Fatal(err)
	}
	if err := r.Describe(""); err != nil {
		t.Fatal(err)
	}
	if got := names(r.Variables); !slices.Equal(got, []string{"repo", "area"}) || r.Variables[0].Default != "blog" {
		t.Fatalf("after editing %+v", r.Variables)
	}
	if got := r.RequiredWithoutDefault(); !slices.Equal(got, []string{"area"}) {
		t.Fatalf("RequiredWithoutDefault = %v", got)
	}
	var fe *FieldError
	if err := r.CheckSchedulable("trigger"); !errors.As(err, &fe) || fe.Field != "trigger" || fe.Message != "Scheduled routines require defaults for required variables: area" {
		t.Fatalf("CheckSchedulable = %v", err)
	}
}

func TestSetVariablesRefuses(t *testing.T) {
	r, _ := NewRoutine(1, ByMember(7), "{{a}}", "")
	for name, def := range map[string]RoutineVariable{
		"select without options":  {Name: "a", Type: SelectVariable},
		"default not of its type": {Name: "a", Type: NumberVariable, Default: "many"},
		"unknown type":            {Name: "a", Type: "colour"},
	} {
		if err := r.SetVariables([]RoutineVariable{def}); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if err := r.SetVariables([]RoutineVariable{{Name: "a", Type: TextVariable}, {Name: "a", Type: TextVariable}}); err == nil {
		t.Error("twice: no error")
	}
	if err := r.SetVariables([]RoutineVariable{{Name: "a", Type: NumberVariable, Default: "3"}}); err != nil || r.Variables[0].Default != 3.0 {
		t.Errorf("a numeric default is kept as a number: %+v, %v", r.Variables, err)
	}
}

func TestNormalizeVariable(t *testing.T) {
	sel := RoutineVariable{Name: "repo", Type: SelectVariable, Options: []string{"shop", "blog"}}
	for _, tc := range []struct {
		v    RoutineVariable
		raw  any
		want any
		ok   bool
	}{
		{RoutineVariable{Name: "b", Type: BooleanVariable}, true, true, true},
		{RoutineVariable{Name: "b", Type: BooleanVariable}, " Yes ", true, true},
		{RoutineVariable{Name: "b", Type: BooleanVariable}, 0.0, false, true},
		{RoutineVariable{Name: "b", Type: BooleanVariable}, "maybe", nil, false},
		{RoutineVariable{Name: "n", Type: NumberVariable}, 2.5, 2.5, true},
		{RoutineVariable{Name: "n", Type: NumberVariable}, " 42 ", 42.0, true},
		{RoutineVariable{Name: "n", Type: NumberVariable}, "", nil, false},
		{RoutineVariable{Name: "n", Type: NumberVariable}, true, nil, false},
		{RoutineVariable{Name: "d", Type: DateVariable}, "2026-02-28", "2026-02-28", true},
		{RoutineVariable{Name: "d", Type: DateVariable}, "2026-02-30", nil, false},
		{RoutineVariable{Name: "d", Type: DateVariable}, "2026-2-3", nil, false},
		{RoutineVariable{Name: "d", Type: DateVariable}, 20260228.0, nil, false},
		{sel, "blog", "blog", true},
		{sel, "other", nil, false},
		{RoutineVariable{Name: "t", Type: TextVariable}, 3.0, "3", true},
		{RoutineVariable{Name: "t", Type: TextareaVariable}, map[string]any{"a": 1.0}, `{"a":1}`, true},
		{RoutineVariable{Name: "t", Type: TextVariable}, nil, nil, true},
	} {
		got, err := tc.v.Normalize(tc.raw)
		if tc.ok != (err == nil) || got != tc.want {
			t.Errorf("%s %v: %v, %v", tc.v.Type, tc.raw, got, err)
		}
		var fe *FieldError
		if err != nil && (!errors.As(err, &fe) || fe.Field != "variables."+tc.v.Name) {
			t.Errorf("%s %v: error %v names no variable", tc.v.Type, tc.raw, err)
		}
	}
}

func TestResolveVariables(t *testing.T) {
	now := time.Date(2026, 4, 28, 14, 17, 0, 0, time.FixedZone("CEST", 2*3600))
	vars := []RoutineVariable{
		{Name: "repo", Type: TextVariable, Required: true},
		{Name: "count", Type: NumberVariable, Default: 1.0, Required: true},
		{Name: "note", Type: TextVariable},
	}
	got, err := ResolveVariables(vars, WebhookSource, map[string]any{"repo": "shop", "variables": map[string]any{"count": "4"}}, nil, now)
	if err != nil || got["repo"] != "shop" || got["count"] != 4.0 || got["date"] != "2026-04-28" || got["timestamp"] != "April 28, 2026 at 12:17 PM UTC" {
		t.Fatalf("webhook payload: %v, %v", got, err)
	}
	if _, ok := got["note"]; ok {
		t.Errorf("an optional variable without a value is left out: %v", got)
	}
	// Only a webhook reads the payload's own fields; given values win.
	if _, err := ResolveVariables(vars, APISource, map[string]any{"repo": "shop"}, nil, now); err == nil {
		t.Error("api with repo only at the top of the payload: no error")
	}
	got, err = ResolveVariables(vars, ManualSource, map[string]any{"variables": map[string]any{"repo": "a"}}, map[string]any{"repo": "b"}, now)
	if err != nil || got["repo"] != "b" || got["count"] != 1.0 {
		t.Fatalf("given: %v, %v", got, err)
	}
	var fe *FieldError
	if _, err := ResolveVariables(vars, ManualSource, nil, map[string]any{"repo": "  "}, now); !errors.As(err, &fe) || fe.Field != "variables.repo" || fe.Message != "Missing routine variables: repo" {
		t.Fatalf("missing: %v", err)
	}
	if _, err := ResolveVariables(vars, ManualSource, nil, map[string]any{"repo": "x", "count": "lots"}, now); !errors.As(err, &fe) || fe.Field != "variables.count" {
		t.Fatalf("wrong type: %v", err)
	}
}

func TestInterpolate(t *testing.T) {
	got := Interpolate("Triage {{ repo }} {{pr\\_url}} {{unknown}} on {{date}} x{{n}} {{ok}}", map[string]any{"repo": "shop", "pr_url": "u", "date": "2026-04-28", "n": 2.0, "ok": true})
	if want := "Triage shop u {{unknown}} on 2026-04-28 x2 true"; got != want {
		t.Fatalf("Interpolate = %q, want %q", got, want)
	}
}
