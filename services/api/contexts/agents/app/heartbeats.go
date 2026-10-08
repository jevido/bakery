package app

import (
	"context"
	"time"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// TickHeartbeats starts the timer Wakes that are due: each Agent whose
// Heartbeat is due and that has an open Issue assigned to it is claimed
// once for this interval, and only the process whose claim hit wakes it,
// without an Issue and with its Hirer as the one asking. An Agent without
// open Issues is skipped and stays due. One Agent failing is logged and
// the others still tick.
func (s *Service) TickHeartbeats(ctx context.Context) (woken int, err error) {
	now := s.now()
	due, err := s.agents.DueHeartbeats(ctx, now)
	if err != nil {
		return 0, err
	}
	for _, a := range due {
		ok, err := s.tickHeartbeat(ctx, a, now)
		if err != nil {
			s.Logf("agents: heartbeat of agent %d: %v", a.ID, err)
			continue
		}
		if ok {
			woken++
		}
	}
	return woken, nil
}

func (s *Service) tickHeartbeat(ctx context.Context, a domain.Agent, now time.Time) (bool, error) {
	if !a.HeartbeatDue(now) {
		return false, nil
	}
	open, err := s.work.OpenIssuesOfAgent(ctx, a.GuildID, a.ID)
	if err != nil || len(open) == 0 {
		return false, err
	}
	claimed, err := s.agents.ClaimHeartbeat(ctx, a.ID, a.LastHeartbeatAt, now)
	if err != nil || !claimed {
		return false, err
	}
	if _, _, err := s.wake(ctx, a, WakeInput{Source: domain.Timer, Reason: domain.HeartbeatTimer, ActorID: a.HirerID}); err != nil {
		return false, err
	}
	return true, nil
}
