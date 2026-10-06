package app

import (
	"context"
	"errors"
	"sync"

	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

// HealthChanged is what a Server probe found changed about one Server.
type HealthChanged struct {
	Server domain.Server
	Change domain.HealthChange
	// Reason is why the Server could not be reached.
	Reason              string
	DiskUsed, DiskTotal int64
}

// probing is how many Servers are probed at the same time.
const probing = 4

// ProbeAll runs a Server probe of every Server whose latest Validation
// passed: it reads the Server metrics (which needs a connection) and
// records the outcome, calling OnHealthChanged for what changed.
func (s *Service) ProbeAll(ctx context.Context) error {
	servers, err := s.store.List(ctx, 0)
	if err != nil {
		return err
	}
	sem := make(chan struct{}, probing)
	var wg sync.WaitGroup
	for _, srv := range servers {
		if !srv.Probed() {
			continue
		}
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			s.probe(ctx, srv.ID)
		}()
	}
	wg.Wait()
	return nil
}

func (s *Service) probe(ctx context.Context, id uint64) {
	m, err := s.Metrics(ctx, id)
	if ctx.Err() != nil {
		return
	}
	var unreachable *ErrUnreachable
	if err != nil && !errors.As(err, &unreachable) {
		s.log("servers: probing server %d: %v", id, err)
		return
	}
	// Read again: a Validation or an edit may have happened meanwhile.
	srv, err := s.Get(ctx, id)
	if err != nil {
		s.log("servers: probing server %d: %v", id, err)
		return
	}
	disk := m.Server
	changes := srv.RecordProbe(unreachable == nil, disk.DiskUsed, disk.DiskTotal)
	if err := s.store.Save(ctx, srv); err != nil {
		s.log("servers: saving the probe of %s: %v", srv.Name, err)
		return
	}
	if s.OnHealthChanged == nil {
		return
	}
	for _, c := range changes {
		e := HealthChanged{Server: srv, Change: c, DiskUsed: disk.DiskUsed, DiskTotal: disk.DiskTotal}
		if unreachable != nil {
			e.Reason = unreachable.Reason
		}
		s.OnHealthChanged(ctx, e)
	}
}
