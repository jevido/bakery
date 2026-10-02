package app

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

// PreviewView is a Preview with where it is served and its newest
// Deployment.
type PreviewView struct {
	domain.Preview
	// Domain is the Preview domain, empty when the Application has none.
	Domain string
	Latest *domain.Deployment
}

// Previews lists the Application's Previews, open ones first.
func (s *Service) Previews(ctx context.Context, applicationID uint64) ([]PreviewView, error) {
	a, err := s.applications(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	list, err := s.previews.ByApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	out := make([]PreviewView, len(list))
	for i, p := range list {
		out[i].Preview = p
		if len(a.Domains) > 0 {
			out[i].Domain = domain.PreviewDomain(p.Number, a.Domains[0])
		}
		latest, err := s.store.ByPreview(ctx, applicationID, p.Number, 1)
		if err != nil {
			return nil, err
		}
		if len(latest) > 0 {
			out[i].Latest = &latest[0]
		}
	}
	return out, nil
}

// ClosePreview cancels the Preview's active Deployment, marks it closed,
// and removes its Preview route and, on every Server its Deployments ran
// on, its Containers, Volumes and Images. A step that fails is logged and
// the others still run. Closing a closed Preview removes again whatever is
// left.
func (s *Service) ClosePreview(ctx context.Context, applicationID uint64, number int) error {
	p, found, err := s.previews.ByNumber(ctx, applicationID, number)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	ds, err := s.store.ByPreview(ctx, applicationID, number, -1)
	if err != nil {
		return err
	}
	for _, d := range ds {
		if d.Status.Active() {
			if _, err := s.Cancel(ctx, d.ID); err != nil && !errors.Is(err, ErrNotCancellable) {
				s.Log("deployments: cancelling deployment %d of preview #%d: %v", d.ID, number, err)
			}
		}
	}
	if p.State == domain.PreviewOpen {
		if err := p.Close(time.Now()); err != nil {
			return err
		}
		if _, err := s.previews.Save(ctx, p); err != nil {
			return err
		}
	}
	s.removePreview(context.WithoutCancel(ctx), applicationID, number, ds)
	if s.Comments != nil && p.CommentID != "" {
		if _, err := s.Comments.Comment(context.WithoutCancel(ctx), applicationID, number, s.Comments.RemovedBody()); err != nil {
			s.Log("deployments: commenting on pull request #%d of application %d: %v", number, applicationID, err)
		}
	}
	return nil
}

// removePreview removes what a closed Preview ran.
func (s *Service) removePreview(ctx context.Context, applicationID uint64, number int, ds []domain.Deployment) {
	if s.DropPreviewRoute != nil {
		if err := s.DropPreviewRoute(ctx, applicationID, number); err != nil {
			s.Log("deployments: dropping the route of preview #%d of application %d: %v", number, applicationID, err)
		}
	}
	if s.runtimes == nil {
		return
	}
	var servers []uint64
	for _, d := range ds {
		if !slices.Contains(servers, d.ServerID) {
			servers = append(servers, d.ServerID)
		}
	}
	for _, server := range servers {
		rt, err := s.runtimes(ctx, server)
		if err != nil {
			s.Log("deployments: removing preview #%d of application %d: %v", number, applicationID, err)
			continue
		}
		if _, err := rt.RemovePreview(ctx, applicationID, number); err != nil {
			s.Log("deployments: removing preview #%d of application %d on %s: %v", number, applicationID, rt.ServerName(), err)
		}
		for _, d := range ds {
			if d.ServerID != server || d.Image == "" {
				continue
			}
			if _, err := rt.RemoveImage(ctx, d.Image); err != nil {
				s.Log("deployments: removing image %s of preview #%d: %v", d.Image, number, err)
			}
		}
	}
}

// DeletePreviews removes every Preview of a deleted Application. Its
// Containers, Volumes and routes go with the Application's own.
func (s *Service) DeletePreviews(ctx context.Context, applicationID uint64) error {
	return s.previews.DeleteForApplication(ctx, applicationID)
}
