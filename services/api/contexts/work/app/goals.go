// Package app holds the work use cases: plan Goals and Issues and talk
// about Issues in Comments.
package app

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// ErrNotFound is a Goal or an Issue that does not exist, or that the
// person asking may not see.
var ErrNotFound = errors.New("not found")

// Goals keeps Goals.
type Goals interface {
	// Goals lists the Guild's Goals, oldest first.
	Goals(ctx context.Context, guildID uint64) ([]domain.Goal, error)
	Goal(ctx context.Context, id uint64) (domain.Goal, bool, error)
	CreateGoal(ctx context.Context, g domain.Goal) (domain.Goal, error)
	SaveGoal(ctx context.Context, g domain.Goal) error
	// DeleteGoal deletes the Goal and, in the same transaction, moves its
	// Sub-goals under the Goal's own parent.
	DeleteGoal(ctx context.Context, g domain.Goal) error
}

// Guilds is what work asks guilds: whether a Member belongs to a Guild
// (guilds.IsMember) and a Guild's Issue prefix (guilds.IssuePrefix).
type Guilds interface {
	IsMember(ctx context.Context, guildID, memberID uint64) (bool, error)
	IssuePrefix(ctx context.Context, guildID uint64) (string, error)
}

type Service struct {
	goals    Goals
	issues   Issues
	comments Comments
	docs     Documents
	guilds   Guilds
	projects Projects
	activity Activity
	now      func() time.Time
	// Logf logs what a request cannot report, such as an Activity event
	// that was not recorded.
	Logf func(format string, args ...any)
}

func NewService(goals Goals, issues Issues, comments Comments, docs Documents, guilds Guilds, projects Projects, activity Activity) *Service {
	return &Service{goals: goals, issues: issues, comments: comments, docs: docs, guilds: guilds, projects: projects, activity: activity, now: time.Now, Logf: log.Printf}
}

// GoalInput is a new Goal as typed. Empty Level and Status take their
// defaults; ParentID and OwnerID are 0 for none.
type GoalInput struct {
	Title       string
	Description string
	Level       string
	Status      string
	ParentID    uint64
	OwnerID     uint64
}

// GoalPatch changes the fields that are not nil. A ParentID or OwnerID of
// 0 removes the parent or the owner.
type GoalPatch struct {
	Title       *string
	Description *string
	Level       *string
	Status      *string
	ParentID    *uint64
	OwnerID     *uint64
}

// Goals lists the Guild's Goals, oldest first.
func (s *Service) Goals(ctx context.Context, guildID uint64) ([]domain.Goal, error) {
	return s.goals.Goals(ctx, guildID)
}

// GoalInGuild reports whether the Goal exists and belongs to the Guild.
func (s *Service) GoalInGuild(ctx context.Context, id, guildID uint64) (bool, error) {
	g, found, err := s.goals.Goal(ctx, id)
	return found && g.GuildID == guildID, err
}

func (s *Service) Goal(ctx context.Context, id uint64) (domain.Goal, error) {
	g, found, err := s.goals.Goal(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	return g, err
}

// CreateGoal creates a Goal in the Guild by the Member.
func (s *Service) CreateGoal(ctx context.Context, guildID, memberID uint64, in GoalInput) (domain.Goal, error) {
	level, status := domain.TaskLevel, domain.Planned
	var err error
	if in.Level != "" {
		if level, err = domain.ParseGoalLevel(in.Level); err != nil {
			return domain.Goal{}, err
		}
	}
	if in.Status != "" {
		if status, err = domain.ParseGoalStatus(in.Status); err != nil {
			return domain.Goal{}, err
		}
	}
	g, err := domain.NewGoal(guildID, in.Title, in.Description, level, status)
	if err != nil {
		return domain.Goal{}, err
	}
	if err := s.moveUnder(ctx, &g, in.ParentID); err != nil {
		return domain.Goal{}, err
	}
	if err := s.setOwner(ctx, &g, in.OwnerID); err != nil {
		return domain.Goal{}, err
	}
	g, err = s.goals.CreateGoal(ctx, g)
	if err != nil {
		return domain.Goal{}, err
	}
	s.publish(ctx, domain.GoalCreated{Happened: s.happened(memberID), Goal: g})
	return g, nil
}

// ChangeGoal changes the Goal by the Member.
func (s *Service) ChangeGoal(ctx context.Context, memberID, id uint64, p GoalPatch) (domain.Goal, error) {
	g, err := s.Goal(ctx, id)
	if err != nil {
		return domain.Goal{}, err
	}
	before := g
	if p.Title != nil {
		if err := g.Rename(*p.Title); err != nil {
			return domain.Goal{}, err
		}
	}
	if p.Description != nil {
		g.Describe(*p.Description)
	}
	if p.Level != nil {
		l, err := domain.ParseGoalLevel(*p.Level)
		if err != nil {
			return domain.Goal{}, err
		}
		g.SetLevel(l)
	}
	if p.Status != nil {
		st, err := domain.ParseGoalStatus(*p.Status)
		if err != nil {
			return domain.Goal{}, err
		}
		g.SetStatus(st)
	}
	if p.ParentID != nil {
		if err := s.moveUnder(ctx, &g, *p.ParentID); err != nil {
			return domain.Goal{}, err
		}
	}
	if p.OwnerID != nil {
		if err := s.setOwner(ctx, &g, *p.OwnerID); err != nil {
			return domain.Goal{}, err
		}
	}
	if err := s.goals.SaveGoal(ctx, g); err != nil {
		return domain.Goal{}, err
	}
	g, err = s.Goal(ctx, id)
	if err != nil {
		return domain.Goal{}, err
	}
	if e := (domain.GoalChanged{Happened: s.happened(memberID), Before: before, After: g}); len(e.Changes()) > 0 {
		s.publish(ctx, e)
	}
	return g, nil
}

// DeleteGoal deletes the Goal by the Member; its Sub-goals move under its
// parent.
func (s *Service) DeleteGoal(ctx context.Context, memberID, id uint64) error {
	g, err := s.Goal(ctx, id)
	if err != nil {
		return err
	}
	if err := s.goals.DeleteGoal(ctx, g); err != nil {
		return err
	}
	s.publish(ctx, domain.GoalDeleted{Happened: s.happened(memberID), Goal: g})
	return nil
}

// moveUnder looks up the parent Goal and its ancestors and hands them to
// the Goal to check; 0 removes the parent.
func (s *Service) moveUnder(ctx context.Context, g *domain.Goal, parentID uint64) error {
	if parentID == 0 {
		return g.MoveUnder(nil, nil)
	}
	all, err := s.goals.Goals(ctx, g.GuildID)
	if err != nil {
		return err
	}
	byID := make(map[uint64]domain.Goal, len(all))
	for _, x := range all {
		byID[x.ID] = x
	}
	parent, ok := byID[parentID]
	if !ok {
		// Another Guild's Goal is answered as one that does not exist.
		return &domain.FieldError{Field: "parent_id", Message: "parent goal not found"}
	}
	var ancestors []uint64
	for up := parent.ParentID; up != 0 && len(ancestors) <= len(all); up = byID[up].ParentID {
		ancestors = append(ancestors, up)
	}
	return g.MoveUnder(&parent, ancestors)
}

func (s *Service) setOwner(ctx context.Context, g *domain.Goal, memberID uint64) error {
	if memberID != 0 {
		ok, err := s.guilds.IsMember(ctx, g.GuildID, memberID)
		if err != nil {
			return err
		}
		if !ok {
			return &domain.FieldError{Field: "owner_id", Message: "owner must be a member of this guild"}
		}
	}
	g.SetOwner(memberID)
	return nil
}
