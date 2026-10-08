package app

import (
	"context"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// Activity keeps the Guild's Activity events.
type Activity interface {
	Record(ctx context.Context, e domain.ActivityEvent) error
}

// happened is the Member's domain event at this moment.
func (s *Service) happened(memberID uint64) domain.Happened {
	return domain.Happened{ActorID: memberID, At: s.now()}
}

// publish records the domain event as an Activity event. The change it
// describes is already stored, so a failure is logged, not returned.
func (s *Service) publish(ctx context.Context, e domain.Event) {
	a := e.Activity()
	if err := s.activity.Record(ctx, a); err != nil {
		s.Logf("work: recording %s of %s %d: %v", a.Action, a.EntityType, a.EntityID, err)
	}
}
