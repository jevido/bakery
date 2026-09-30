package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

// PruneImages applies Image retention to every Application whose
// Deployments ran on the Server (0 the Local server): the Images
// domain.ImagesToPrune lets go are removed there, unless a Container still
// uses one. It returns the bytes freed; an Image that fails to go is
// reported and the rest carry on.
func (s *Service) PruneImages(ctx context.Context, serverID uint64) (int64, error) {
	if s.runtimes == nil {
		return 0, errors.New("no runtime to remove images with")
	}
	ids, err := s.store.ApplicationIDs(ctx)
	if err != nil {
		return 0, err
	}
	var reclaimed int64
	var errs []error
	var rt Runtime
	for _, id := range ids {
		// -1: every Deployment of the Application, not a page of them.
		all, err := s.store.ByApplication(ctx, id, -1)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		var ds []domain.Deployment
		for _, d := range all {
			if d.ServerID == serverID {
				ds = append(ds, d)
			}
		}
		if len(ds) == 0 {
			continue
		}
		if rt == nil {
			if rt, err = s.runtimes(ctx, serverID); err != nil {
				return 0, err
			}
		}
		for _, image := range domain.ImagesToPrune(ds) {
			n, err := rt.RemoveImage(ctx, image)
			if err != nil {
				errs = append(errs, fmt.Errorf("removing %s: %w", image, err))
				continue
			}
			reclaimed += n
		}
	}
	return reclaimed, errors.Join(errs...)
}
