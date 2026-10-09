package app

import (
	"context"
	"errors"
	"slices"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// RunRequest is one Routine run asked for: its source, the Routine trigger
// it names (0 for none) and who asked (nobody for a Schedule).
type RunRequest struct {
	Source    domain.RoutineRunSource
	TriggerID uint64
	Actor     domain.Actor
}

// everyProject sees every Project: a Routine run creates its Execution
// Issue in the Routine's Project whoever triggered it, since the route
// already checked that they may view the Routine.
func everyProject(ids []uint64) ([]uint64, error) { return ids, nil }

// RunRoutine records a Routine run of the Guild's Routine and dispatches
// it, as Paperclip's dispatchRoutineRun: linked to the Routine's Live
// execution Issue under coalesce_if_active or skip_if_active, else a new
// Execution Issue assigned to its Agent assignee, which wakes it. An
// Agent may only run a Routine assigned to itself. The Routine's lock
// keeps two Routine runs of it from both finding no Live execution Issue.
func (s *Service) RunRoutine(ctx context.Context, guildID, routineID uint64, req RunRequest) (domain.RoutineRun, error) {
	unlock, err := s.routines.LockRoutine(ctx, routineID)
	if err != nil {
		return domain.RoutineRun{}, err
	}
	defer unlock()
	r, found, err := s.routines.Routine(ctx, routineID)
	if err != nil {
		return domain.RoutineRun{}, err
	}
	if !found || r.GuildID != guildID {
		return domain.RoutineRun{}, ErrNotFound
	}
	if req.Actor.AgentID != 0 && r.AssigneeAgentID != req.Actor.AgentID {
		return domain.RoutineRun{}, ErrNotOwnRoutine
	}
	var trigger *domain.RoutineTrigger
	if req.TriggerID != 0 {
		t, found, err := s.routines.Trigger(ctx, req.TriggerID)
		if err != nil {
			return domain.RoutineRun{}, err
		}
		if !found || t.GuildID != guildID {
			return domain.RoutineRun{}, &domain.FieldError{Field: "trigger_id", Message: "trigger not found"}
		}
		trigger = &t
	}
	now := s.now()
	rr, err := domain.ReceiveRoutineRun(r, trigger, req.Source, req.Actor, now)
	if err != nil {
		return domain.RoutineRun{}, err
	}
	if rr, err = s.routines.CreateRoutineRun(ctx, rr); err != nil {
		return domain.RoutineRun{}, err
	}
	if err := s.dispatch(ctx, r, &rr); err != nil {
		return domain.RoutineRun{}, err
	}
	if err := s.routines.SaveRoutineRun(ctx, rr); err != nil {
		return domain.RoutineRun{}, err
	}
	if err := s.routines.RoutineTriggered(ctx, r.ID, now); err != nil {
		return domain.RoutineRun{}, err
	}
	r.LastTriggeredAt = &now
	if trigger != nil {
		trigger.LastFiredAt, trigger.LastResult = &now, string(rr.Status)
		if err := s.routines.SaveTrigger(ctx, *trigger); err != nil {
			return domain.RoutineRun{}, err
		}
	}
	if rr.Source != domain.ManualSource {
		s.publish(ctx, domain.RoutineRunTriggered{Happened: s.happened(req.Actor), Routine: r, Run: rr})
	}
	return rr, nil
}

// dispatch links the received Routine run to the Routine's Live execution
// Issue, as its Concurrency policy says, or creates its Execution Issue.
// An Issue that cannot be created fails the Routine run with the reason.
func (s *Service) dispatch(ctx context.Context, r domain.Routine, rr *domain.RoutineRun) error {
	live, found, err := s.liveExecutionIssue(ctx, r)
	if err != nil {
		return err
	}
	if found && rr.LinkToLive(r.ConcurrencyPolicy, live, s.now()) {
		return nil
	}
	i, err := s.CreateIssue(ctx, r.GuildID, rr.TriggeredBy, IssueInput{
		Title: r.Title, Description: r.Description, Status: string(domain.Todo), Priority: string(r.Priority),
		AssigneeAgentID: r.AssigneeAgentID, ProjectID: r.ProjectID, GoalID: r.GoalID, ParentID: r.ParentIssueID,
		originRoutineID: r.ID, originRoutineRunID: rr.ID,
	}, everyProject)
	if err != nil {
		var fe *domain.FieldError
		if !errors.As(err, &fe) {
			s.Logf("work: creating the execution issue of routine %d: %v", r.ID, err)
		}
		rr.Fail(err.Error(), s.now())
		return nil
	}
	rr.IssueCreated(i.ID)
	return nil
}

// liveExecutionIssue finds the Routine's Live execution Issue: its newest
// open Execution Issue that has a queued or running Run.
func (s *Service) liveExecutionIssue(ctx context.Context, r domain.Routine) (domain.Issue, bool, error) {
	if s.IssuesLive == nil {
		return domain.Issue{}, false, nil
	}
	is, err := s.issues.OpenExecutionIssues(ctx, r.ID)
	if err != nil || len(is) == 0 {
		return domain.Issue{}, false, err
	}
	ids := make([]uint64, len(is))
	for n, i := range is {
		ids[n] = i.ID
	}
	live, err := s.IssuesLive(ctx, r.GuildID, ids)
	if err != nil {
		return domain.Issue{}, false, err
	}
	for _, i := range is {
		if live[i.ID] {
			return i, true, nil
		}
	}
	return domain.Issue{}, false, nil
}

// followExecutionIssue moves the Routine run that created the Execution
// Issue with its new status. The Issue's change already happened, so a
// failure is logged.
func (s *Service) followExecutionIssue(ctx context.Context, i domain.Issue, deleted bool) {
	if i.OriginRoutineRunID == 0 {
		return
	}
	rr, found, err := s.routines.RoutineRun(ctx, i.OriginRoutineRunID)
	if err == nil && found {
		if deleted {
			rr.IssueDeleted(s.now())
			err = s.routines.SaveRoutineRun(ctx, rr)
		} else if rr.Follow(i.Status, s.now()) {
			err = s.routines.SaveRoutineRun(ctx, rr)
		}
	}
	if err != nil {
		s.Logf("work: following execution issue %d in its routine run %d: %v", i.ID, i.OriginRoutineRunID, err)
	}
}

// RoutineRuns lists the Guild's Routine runs newest first, at most limit:
// of the one Routine, which the person must be able to view, or with
// routineID 0 of every Routine they may view.
func (s *Service) RoutineRuns(ctx context.Context, guildID, routineID uint64, limit int, visible Visible) ([]domain.RoutineRun, error) {
	var ids []uint64
	if routineID != 0 {
		if _, err := s.Routine(ctx, guildID, routineID, visible); err != nil {
			return nil, err
		}
		ids = []uint64{routineID}
	} else {
		rs, err := s.Routines(ctx, guildID, RoutineFilter{}, visible)
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 {
		return []domain.RoutineRun{}, nil
	}
	return s.routines.RoutineRuns(ctx, ids, limit)
}

// LastRoutineRuns answers the newest Routine run of each of the Routines
// that has one.
func (s *Service) LastRoutineRuns(ctx context.Context, routineIDs []uint64) (map[uint64]domain.RoutineRun, error) {
	return s.routines.LastRoutineRuns(ctx, routineIDs)
}

// RoutineTitles names the Guild's Routines among ids.
func (s *Service) RoutineTitles(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error) {
	out := map[uint64]string{}
	if len(ids) == 0 {
		return out, nil
	}
	rs, err := s.routines.Routines(ctx, guildID)
	if err != nil {
		return nil, err
	}
	for _, r := range rs {
		if slices.Contains(ids, r.ID) {
			out[r.ID] = r.Title
		}
	}
	return out, nil
}
