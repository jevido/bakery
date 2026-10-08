package app

import (
	"context"
	"slices"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// Activity keeps the Guild's Activity events.
type Activity interface {
	Record(ctx context.Context, e domain.ActivityEvent) error
	// Activity lists the Guild's events that q keeps, newest first.
	Activity(ctx context.Context, guildID uint64, q ActivityQuery) ([]domain.ActivityEvent, error)
	// IssueActivity lists every event about the Issue, oldest first.
	IssueActivity(ctx context.Context, issueID uint64) ([]domain.ActivityEvent, error)
}

// happened is the Actor's domain event at this moment.
func (s *Service) happened(by domain.Actor) domain.Happened {
	return domain.Happened{Actor: by, At: s.now()}
}

// publish records the domain event as an Activity event. The change it
// describes is already stored, so a failure is logged, not returned.
func (s *Service) publish(ctx context.Context, e domain.Event) {
	a := e.Activity()
	if err := s.activity.Record(ctx, a); err != nil {
		s.Logf("work: recording %s of %s %d: %v", a.Action, a.EntityType, a.EntityID, err)
	}
}

// RecordAgentActivity stores an event the agents context made about one
// of its Agents. Unlike work's own events, a failure is the caller's to
// handle.
func (s *Service) RecordAgentActivity(ctx context.Context, e domain.AgentEvent) error {
	e, err := e.Validated()
	if err != nil {
		return err
	}
	if e.At.IsZero() {
		e.At = s.now()
	}
	return s.activity.Record(ctx, e.Activity())
}

// The most Activity events one page answers, and how many without a limit.
const (
	MaxActivity     = 200
	DefaultActivity = 50
)

// ActivityQuery is what a page of the Guild's Activity keeps: events about
// EntityType ("" for both), by Actor (nobody for anyone), older than Before (0
// for the newest), at most Limit of them.
type ActivityQuery struct {
	EntityType string
	Actor      domain.Actor
	Before     uint64
	Limit      int
}

// Activity is a page of the Guild's Activity, newest first, without the
// events about Issues the person may not view. An Issue that still exists
// is judged by the Project it is in now, a deleted one by the Project it
// was in. Hidden events are dropped after the store answers, so it asks
// again until the page is full or no older events are left.
func (s *Service) Activity(ctx context.Context, guildID uint64, q ActivityQuery, visible Visible) ([]domain.ActivityEvent, error) {
	if q.Limit <= 0 || q.Limit > MaxActivity {
		q.Limit = DefaultActivity
	}
	out := []domain.ActivityEvent{}
	for {
		page, err := s.activity.Activity(ctx, guildID, q)
		if err != nil {
			return nil, err
		}
		kept, err := s.visibleActivity(ctx, page, visible)
		if err != nil {
			return nil, err
		}
		for _, e := range kept {
			if len(out) == q.Limit {
				return out, nil
			}
			out = append(out, e)
		}
		if len(page) < q.Limit || len(out) == q.Limit {
			return out, nil
		}
		q.Before = page[len(page)-1].ID
	}
}

// visibleActivity keeps the events the person may see, in their order:
// those about an Issue or a Budget in a Project they may not view go.
func (s *Service) visibleActivity(ctx context.Context, es []domain.ActivityEvent, visible Visible) ([]domain.ActivityEvent, error) {
	var issueIDs []uint64
	for _, e := range es {
		if e.EntityType == domain.IssueEntity {
			issueIDs = append(issueIDs, e.EntityID)
		}
	}
	is, err := s.issues.IssuesByID(ctx, issueIDs)
	if err != nil {
		return nil, err
	}
	current := make(map[uint64]uint64, len(is))
	for _, i := range is {
		current[i.ID] = i.ProjectID
	}
	// An event about anything but an Issue is in a Project only when it
	// says so: one about a Project's Budget.
	projectOf := func(e domain.ActivityEvent) uint64 {
		if p, ok := current[e.EntityID]; ok && e.EntityType == domain.IssueEntity {
			return p
		}
		return e.ProjectID
	}
	var projectIDs []uint64
	for _, e := range es {
		if p := projectOf(e); p != 0 && !slices.Contains(projectIDs, p) {
			projectIDs = append(projectIDs, p)
		}
	}
	var seen []uint64
	if len(projectIDs) > 0 {
		if seen, err = visible(projectIDs); err != nil {
			return nil, err
		}
	}
	out := make([]domain.ActivityEvent, 0, len(es))
	for _, e := range es {
		if p := projectOf(e); p == 0 || slices.Contains(seen, p) {
			out = append(out, e)
		}
	}
	return out, nil
}

// IssueActivity is every event about the Issue, oldest first; an Issue
// that is another Guild's or that the person may not view is ErrNotFound.
func (s *Service) IssueActivity(ctx context.Context, guildID uint64, ref string, visible Visible) ([]domain.ActivityEvent, error) {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return nil, err
	}
	return s.activity.IssueActivity(ctx, i.ID)
}
