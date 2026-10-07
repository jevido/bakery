package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestParseGoalLevel(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want GoalLevel
		ok   bool
	}{
		{"guild", GuildLevel, true},
		{"agent", AgentLevel, true},
		{"task", TaskLevel, true},
		{"team", "", false},
		{"company", "", false},
		{"", "", false},
	} {
		got, err := ParseGoalLevel(tc.in)
		if (err == nil) != tc.ok || got != tc.want {
			t.Errorf("ParseGoalLevel(%q) = %q, %v", tc.in, got, err)
		}
	}
}

func TestParseGoalStatus(t *testing.T) {
	for _, tc := range []struct {
		in string
		ok bool
	}{
		{"planned", true}, {"active", true}, {"achieved", true}, {"cancelled", true},
		{"done", false}, {"", false}, {"Active", false},
	} {
		_, err := ParseGoalStatus(tc.in)
		if (err == nil) != tc.ok {
			t.Errorf("ParseGoalStatus(%q) error = %v", tc.in, err)
		}
	}
}

func TestNewGoalTitle(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
		ok             bool
	}{
		{"trimmed", "  Ship it  ", "Ship it", true},
		{"empty", "", "", false},
		{"blank", "   ", "", false},
		{"200 characters", strings.Repeat("é", 200), strings.Repeat("é", 200), true},
		{"201 characters", strings.Repeat("a", 201), "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, err := NewGoal(1, tc.in, "", TaskLevel, Planned)
			if (err == nil) != tc.ok || g.Title != tc.want {
				t.Fatalf("NewGoal(%q) = %q, %v", tc.in, g.Title, err)
			}
			var fe *FieldError
			if err != nil && (!errors.As(err, &fe) || fe.Field != "title") {
				t.Fatalf("error %v is not a title FieldError", err)
			}
		})
	}
}

func TestMoveUnder(t *testing.T) {
	root := Goal{ID: 1, GuildID: 7}
	child := Goal{ID: 2, GuildID: 7, ParentID: 1}
	grandchild := Goal{ID: 3, GuildID: 7, ParentID: 2}
	other := Goal{ID: 9, GuildID: 8}
	for _, tc := range []struct {
		name      string
		goal      Goal
		parent    *Goal
		ancestors []uint64
		want      uint64
		cycle     bool
		fail      bool
	}{
		{"no parent", child, nil, nil, 0, false, false},
		{"under a sibling tree", Goal{ID: 4, GuildID: 7}, &grandchild, []uint64{2, 1}, 3, false, false},
		{"new goal under root", Goal{GuildID: 7}, &root, nil, 1, false, false},
		{"under itself", root, &root, nil, 0, true, true},
		{"under its child", root, &child, []uint64{1}, 0, true, true},
		{"under its grandchild", root, &grandchild, []uint64{2, 1}, 0, true, true},
		{"another guild", root, &other, nil, 0, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := tc.goal
			err := g.MoveUnder(tc.parent, tc.ancestors)
			if (err != nil) != tc.fail || errors.Is(err, ErrGoalCycle) != tc.cycle {
				t.Fatalf("MoveUnder = %v", err)
			}
			if err == nil && g.ParentID != tc.want {
				t.Fatalf("ParentID = %d, want %d", g.ParentID, tc.want)
			}
		})
	}
}
