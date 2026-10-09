package app

import (
	"context"
	"errors"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// ErrNotOwnRoutine is an Agent creating or changing a Routine that is not
// assigned to itself, or handing one to another Agent.
var ErrNotOwnRoutine = errors.New("agents can only manage their own routines")

// Routines keeps Routines.
type Routines interface {
	// Routines lists the Guild's Routines, newest first.
	Routines(ctx context.Context, guildID uint64) ([]domain.Routine, error)
	Routine(ctx context.Context, id uint64) (domain.Routine, bool, error)
	CreateRoutine(ctx context.Context, r domain.Routine) (domain.Routine, error)
	SaveRoutine(ctx context.Context, r domain.Routine) error
	// RoutinesOfAgent lists the Guild's Routines the Agent is the Agent
	// assignee of, archived ones too.
	RoutinesOfAgent(ctx context.Context, guildID, agentID uint64) ([]domain.Routine, error)
	DeleteRoutinesOfProject(ctx context.Context, projectID uint64) error
}

// RoutineFilter is what a list of Routines keeps. Nil fields keep every
// Routine; an id of 0 keeps the Routines without one.
type RoutineFilter struct {
	ProjectID       *uint64
	AssigneeAgentID *uint64
	Status          *domain.RoutineStatus
}

func (f RoutineFilter) keeps(r domain.Routine) bool {
	return (f.ProjectID == nil || *f.ProjectID == r.ProjectID) &&
		(f.AssigneeAgentID == nil || *f.AssigneeAgentID == r.AssigneeAgentID) &&
		(f.Status == nil || *f.Status == r.Status)
}

// RoutineInput is a new Routine as typed. Empty Priority, Status and
// policies take their defaults; the ids are 0 for none.
type RoutineInput struct {
	Title             string
	Description       string
	Priority          string
	Status            string
	ConcurrencyPolicy string
	CatchUpPolicy     string
	ProjectID         uint64
	GoalID            uint64
	ParentIssueID     uint64
	AssigneeAgentID   uint64
}

// RoutinePatch changes the fields that are not nil. An id of 0 removes
// the Project, Goal, parent Issue or Agent assignee.
type RoutinePatch struct {
	Title             *string
	Description       *string
	Priority          *string
	Status            *string
	ConcurrencyPolicy *string
	CatchUpPolicy     *string
	ProjectID         *uint64
	GoalID            *uint64
	ParentIssueID     *uint64
	AssigneeAgentID   *uint64
}

// Routines lists the Guild's Routines the filter keeps, newest first,
// leaving out those in a Project the person may not view.
func (s *Service) Routines(ctx context.Context, guildID uint64, f RoutineFilter, visible Visible) ([]domain.Routine, error) {
	all, err := s.routines.Routines(ctx, guildID)
	if err != nil {
		return nil, err
	}
	var projectIDs []uint64
	for _, r := range all {
		if r.ProjectID != 0 {
			projectIDs = append(projectIDs, r.ProjectID)
		}
	}
	seen := map[uint64]bool{}
	if len(projectIDs) > 0 {
		ids, err := visible(projectIDs)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			seen[id] = true
		}
	}
	out := []domain.Routine{}
	for _, r := range all {
		if f.keeps(r) && (r.ProjectID == 0 || seen[r.ProjectID]) {
			out = append(out, r)
		}
	}
	return out, nil
}

// RoutineProject finds the Project (0 for none) and Guild of the Routine;
// found is false when there is none.
func (s *Service) RoutineProject(ctx context.Context, id uint64) (projectID, guildID uint64, found bool, err error) {
	r, found, err := s.routines.Routine(ctx, id)
	return r.ProjectID, r.GuildID, found, err
}

// Routine returns the Guild's Routine, which the person must be able to
// view.
func (s *Service) Routine(ctx context.Context, guildID, id uint64, visible Visible) (domain.Routine, error) {
	r, found, err := s.routines.Routine(ctx, id)
	if err != nil {
		return domain.Routine{}, err
	}
	if !found || r.GuildID != guildID {
		return domain.Routine{}, ErrNotFound
	}
	if ok, err := s.mayView(r.ProjectID, visible); err != nil || !ok {
		if err == nil {
			err = ErrNotFound
		}
		return domain.Routine{}, err
	}
	return r, nil
}

// CreateRoutine creates a Routine in the Guild by the Member or Agent; an
// Agent only one assigned to itself.
func (s *Service) CreateRoutine(ctx context.Context, guildID uint64, by domain.Actor, in RoutineInput, visible Visible) (domain.Routine, error) {
	if by.AgentID != 0 && in.AssigneeAgentID != by.AgentID {
		return domain.Routine{}, ErrNotOwnRoutine
	}
	r, err := domain.NewRoutine(guildID, by, in.Title, in.Description)
	if err != nil {
		return domain.Routine{}, err
	}
	nonEmpty := func(v string) *string {
		if v == "" {
			return nil
		}
		return &v
	}
	p := RoutinePatch{
		Priority: nonEmpty(in.Priority), Status: nonEmpty(in.Status),
		ConcurrencyPolicy: nonEmpty(in.ConcurrencyPolicy), CatchUpPolicy: nonEmpty(in.CatchUpPolicy),
		ProjectID: &in.ProjectID, GoalID: &in.GoalID, ParentIssueID: &in.ParentIssueID, AssigneeAgentID: &in.AssigneeAgentID,
	}
	if err := s.applyRoutine(ctx, &r, p, visible); err != nil {
		return domain.Routine{}, err
	}
	if r, err = s.routines.CreateRoutine(ctx, r); err != nil {
		return domain.Routine{}, err
	}
	s.publish(ctx, domain.RoutineCreated{Happened: s.happened(by), Routine: r})
	return r, nil
}

// ChangeRoutine changes the Routine by the Member or Agent, pausing,
// resuming and archiving it included; an Agent only one assigned to
// itself, and never to another Agent.
func (s *Service) ChangeRoutine(ctx context.Context, guildID uint64, by domain.Actor, id uint64, p RoutinePatch, visible Visible) (domain.Routine, error) {
	r, err := s.Routine(ctx, guildID, id, visible)
	if err != nil {
		return domain.Routine{}, err
	}
	if by.AgentID != 0 && (r.AssigneeAgentID != by.AgentID || p.AssigneeAgentID != nil && *p.AssigneeAgentID != by.AgentID) {
		return domain.Routine{}, ErrNotOwnRoutine
	}
	if r.Archived() {
		return domain.Routine{}, domain.ErrRoutineArchived
	}
	before := r
	if p.Title != nil {
		if err := r.Rename(*p.Title); err != nil {
			return domain.Routine{}, err
		}
	}
	if p.Description != nil {
		if err := r.Describe(*p.Description); err != nil {
			return domain.Routine{}, err
		}
	}
	if err := s.applyRoutine(ctx, &r, p, visible); err != nil {
		return domain.Routine{}, err
	}
	if err := s.routines.SaveRoutine(ctx, r); err != nil {
		return domain.Routine{}, err
	}
	after, found, err := s.routines.Routine(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	if err != nil {
		return domain.Routine{}, err
	}
	switch e := (domain.RoutineChanged{Happened: s.happened(by), Before: before, After: after}); {
	case after.Archived():
		s.publish(ctx, domain.RoutineArchived{Happened: e.Happened, Routine: after})
	case len(e.Changes()) > 0:
		s.publish(ctx, e)
	}
	return after, nil
}

// applyRoutine sets what a patch names besides the title and description,
// each checked.
func (s *Service) applyRoutine(ctx context.Context, r *domain.Routine, p RoutinePatch, visible Visible) error {
	if p.Priority != nil {
		v, err := domain.ParsePriority(*p.Priority)
		if err != nil {
			return err
		}
		r.SetPriority(v)
	}
	if p.Status != nil {
		v, err := domain.ParseRoutineStatus(*p.Status)
		if err != nil {
			return err
		}
		if err := r.SetStatus(v); err != nil {
			return err
		}
	}
	if p.ConcurrencyPolicy != nil {
		v, err := domain.ParseConcurrencyPolicy(*p.ConcurrencyPolicy)
		if err != nil {
			return err
		}
		r.SetConcurrencyPolicy(v)
	}
	if p.CatchUpPolicy != nil {
		v, err := domain.ParseCatchUpPolicy(*p.CatchUpPolicy)
		if err != nil {
			return err
		}
		r.SetCatchUpPolicy(v)
	}
	if p.AssigneeAgentID != nil && *p.AssigneeAgentID != r.AssigneeAgentID {
		if err := s.assignRoutine(ctx, r, *p.AssigneeAgentID); err != nil {
			return err
		}
	}
	if p.ProjectID != nil {
		// A Routine's Project is checked as an Issue's.
		i := domain.Issue{GuildID: r.GuildID}
		if err := s.placeIn(ctx, &i, *p.ProjectID, visible); err != nil {
			return err
		}
		r.PlaceIn(i.ProjectID)
	}
	if p.GoalID != nil {
		if err := s.routineGoal(ctx, r, *p.GoalID); err != nil {
			return err
		}
	}
	if p.ParentIssueID != nil {
		return s.routineParent(ctx, r, *p.ParentIssueID, visible)
	}
	return nil
}

// assignRoutine hands the Routine to one of the Guild's Agents that is not
// terminated; 0 makes it a Draft.
func (s *Service) assignRoutine(ctx context.Context, r *domain.Routine, agentID uint64) error {
	if agentID != 0 {
		a, err := s.AssigneeAgents(ctx, r.GuildID, []uint64{agentID})
		if err != nil {
			return err
		}
		if found, ok := a[agentID]; !ok || found.Terminated {
			return &domain.FieldError{Field: "assignee_agent_id", Message: "assignee must be an agent of this guild that is not terminated"}
		}
	}
	r.AssignAgent(agentID)
	return nil
}

func (s *Service) routineGoal(ctx context.Context, r *domain.Routine, goalID uint64) error {
	if goalID == 0 {
		return r.ServeGoal(nil)
	}
	g, found, err := s.goals.Goal(ctx, goalID)
	if err != nil {
		return err
	}
	if !found {
		return &domain.FieldError{Field: "goal_id", Message: "goal not found"}
	}
	return r.ServeGoal(&g)
}

// routineParent makes one of the Guild's Issues the person may see the
// parent of the Routine's Execution Issues; 0 is none.
func (s *Service) routineParent(ctx context.Context, r *domain.Routine, issueID uint64, visible Visible) error {
	if issueID == 0 {
		return r.MoveUnder(nil)
	}
	notFound := &domain.FieldError{Field: "parent_issue_id", Message: "parent issue not found"}
	i, found, err := s.issues.Issue(ctx, issueID)
	if err != nil {
		return err
	}
	if !found {
		return notFound
	}
	if ok, err := s.mayView(i.ProjectID, visible); err != nil || !ok {
		if err == nil {
			err = notFound
		}
		return err
	}
	return r.MoveUnder(&i)
}

// unassignRoutines makes the terminated Agent's Routines that are not
// archived Drafts, each change recorded with the actor who terminated it.
func (s *Service) unassignRoutines(ctx context.Context, guildID, agentID, actorID uint64) error {
	rs, err := s.routines.RoutinesOfAgent(ctx, guildID, agentID)
	if err != nil {
		return err
	}
	for _, r := range rs {
		if r.Archived() {
			continue
		}
		e := domain.RoutineChanged{Happened: s.happened(domain.ByMember(actorID)), Before: r}
		r.AssignAgent(0)
		if err := s.routines.SaveRoutine(ctx, r); err != nil {
			return err
		}
		after, found, err := s.routines.Routine(ctx, r.ID)
		if err != nil {
			return err
		}
		if found {
			e.After = after
			s.publish(ctx, e)
		}
	}
	return nil
}
