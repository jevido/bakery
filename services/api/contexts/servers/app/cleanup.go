package app

import (
	"context"
	"time"

	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

// ImageRetention removes the Deployment images the deployments context no
// longer keeps on one Server (0 for the Local server, as every other context
// names it), returning the bytes freed.
type ImageRetention func(ctx context.Context, serverID uint64) (int64, error)

// Pruner is what a Connection gives for Cleanup.
type Pruner interface {
	// PruneDanglingImages removes dangling images labelled bakery.managed
	// and returns the bytes freed.
	PruneDanglingImages(ctx context.Context) (int64, error)
}

const cleanupTimeout = 10 * time.Minute

// CleanUp frees the Server's disk: Image retention for the Applications on
// it, then dangling Bakery images. It records when it ran and what it
// freed.
func (s *Service) CleanUp(ctx context.Context, id uint64) (domain.Cleanup, error) {
	srv, err := s.Get(ctx, id)
	if err != nil {
		return domain.Cleanup{}, err
	}
	cctx, cancel := context.WithTimeout(ctx, cleanupTimeout)
	defer cancel()
	conn, err := s.connector.Connect(cctx, srv)
	if err != nil {
		return domain.Cleanup{}, &ErrUnreachable{Reason: err.Error()}
	}
	defer conn.Close()
	var reclaimed int64
	if s.Retention != nil {
		n, err := s.Retention(cctx, srv.RefID())
		if err != nil {
			// What could be removed was; the rest waits for the next run.
			s.log("servers: image retention on %s: %v", srv.Name, err)
		}
		reclaimed += n
	}
	n, err := conn.PruneDanglingImages(cctx)
	if err != nil {
		return domain.Cleanup{}, &ErrUnreachable{Reason: err.Error()}
	}
	c := domain.Cleanup{At: s.now(), Reclaimed: reclaimed + n}
	srv.RecordCleanup(c)
	return c, s.store.Save(ctx, srv)
}

// CleanUpAll cleans up every Reachable Server, reporting failures.
func (s *Service) CleanUpAll(ctx context.Context) error {
	servers, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	for _, srv := range servers {
		if srv.Status != domain.Reachable {
			continue
		}
		if _, err := s.CleanUp(ctx, srv.ID); err != nil {
			s.log("servers: cleanup of %s: %v", srv.Name, err)
		}
	}
	return nil
}
