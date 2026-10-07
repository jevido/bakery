// Package domain is the work context's model: Goals, and later Issues and
// their Comments. It depends on nothing outside the standard library.
package domain

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// FieldError is a broken rule on one input field.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }

func invalid(field, format string, args ...any) error {
	return &FieldError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// ErrGoalCycle is a parent Goal that is the Goal itself or one of its
// Sub-goals.
var ErrGoalCycle error = &FieldError{Field: "parent_id", Message: "a goal cannot sit under itself or one of its sub-goals"}

// MaxTitle is the longest title of a Goal or an Issue, in characters.
const MaxTitle = 200

// GoalLevel is how wide a Goal reaches.
type GoalLevel string

const (
	GuildLevel GoalLevel = "guild"
	AgentLevel GoalLevel = "agent"
	TaskLevel  GoalLevel = "task"
)

// GoalLevels lists every Goal level, widest first.
var GoalLevels = []GoalLevel{GuildLevel, AgentLevel, TaskLevel}

// ParseGoalLevel reads a Goal level by its wire key.
func ParseGoalLevel(s string) (GoalLevel, error) {
	if l := GoalLevel(s); slices.Contains(GoalLevels, l) {
		return l, nil
	}
	return "", invalid("level", "level must be guild, agent or task")
}

// GoalStatus is where a Goal stands.
type GoalStatus string

const (
	Planned   GoalStatus = "planned"
	Active    GoalStatus = "active"
	Achieved  GoalStatus = "achieved"
	Cancelled GoalStatus = "cancelled"
)

// GoalStatuses lists every Goal status in the order a Goal moves through
// them.
var GoalStatuses = []GoalStatus{Planned, Active, Achieved, Cancelled}

// ParseGoalStatus reads a Goal status by its wire key.
func ParseGoalStatus(s string) (GoalStatus, error) {
	if st := GoalStatus(s); slices.Contains(GoalStatuses, st) {
		return st, nil
	}
	return "", invalid("status", "status must be planned, active, achieved or cancelled")
}

// Goal is an outcome a Guild works toward. ParentID and OwnerID are 0 for
// none.
type Goal struct {
	ID          uint64
	GuildID     uint64
	ParentID    uint64
	OwnerID     uint64
	Title       string
	Description string
	Level       GoalLevel
	Status      GoalStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewGoal is a Goal of the Guild without a parent or an owner yet.
func NewGoal(guildID uint64, title, description string, level GoalLevel, status GoalStatus) (Goal, error) {
	g := Goal{GuildID: guildID, Description: description, Level: level, Status: status}
	if err := g.Rename(title); err != nil {
		return Goal{}, err
	}
	return g, nil
}

// Title trims a title and checks it is 1 to MaxTitle characters.
func Title(field, title string) (string, error) {
	t := strings.TrimSpace(title)
	if t == "" {
		return "", invalid(field, "title is required")
	}
	if utf8.RuneCountInString(t) > MaxTitle {
		return "", invalid(field, "title is at most %d characters", MaxTitle)
	}
	return t, nil
}

func (g *Goal) Rename(title string) error {
	t, err := Title("title", title)
	if err != nil {
		return err
	}
	g.Title = t
	return nil
}

// Describe sets the Markdown description; empty is none.
func (g *Goal) Describe(description string) { g.Description = description }

func (g *Goal) SetLevel(l GoalLevel) { g.Level = l }

func (g *Goal) SetStatus(s GoalStatus) { g.Status = s }

// SetOwner hands the Goal to a Member, already known to be one of the
// Guild's; 0 is no owner.
func (g *Goal) SetOwner(memberID uint64) { g.OwnerID = memberID }

// MoveUnder makes parent the Goal's parent, nil for none. ancestors are
// the ids of parent's own ancestors, nearest first: the Goal may be none
// of them, nor parent itself.
func (g *Goal) MoveUnder(parent *Goal, ancestors []uint64) error {
	if parent == nil {
		g.ParentID = 0
		return nil
	}
	if parent.GuildID != g.GuildID {
		return invalid("parent_id", "parent goal not found")
	}
	if g.ID != 0 && (parent.ID == g.ID || slices.Contains(ancestors, g.ID)) {
		return ErrGoalCycle
	}
	g.ParentID = parent.ID
	return nil
}
