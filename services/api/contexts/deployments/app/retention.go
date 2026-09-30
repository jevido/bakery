package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

// PruneImages applies Image retention to every Application: the Images
// domain.ImagesToPrune lets go are removed, unless a Container still uses
// one. It returns the bytes freed; an Image that fails to go is reported
// and the rest carry on.
func (s *Service) PruneImages(ctx context.Context) (int64, error) {
	if s.removeImage == nil {
		return 0, errors.New("no runtime to remove images with")
	}
	ids, err := s.store.ApplicationIDs(ctx)
	if err != nil {
		return 0, err
	}
	var reclaimed int64
	var errs []error
	for _, id := range ids {
		// -1: every Deployment of the Application, not a page of them.
		ds, err := s.store.ByApplication(ctx, id, -1)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, image := range domain.ImagesToPrune(ds) {
			n, err := s.removeImage(ctx, image)
			if err != nil {
				errs = append(errs, fmt.Errorf("removing %s: %w", image, err))
				continue
			}
			reclaimed += n
		}
	}
	return reclaimed, errors.Join(errs...)
}
