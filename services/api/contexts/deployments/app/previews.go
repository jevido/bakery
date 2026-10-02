package app

import (
	"context"
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

// ClosePreview marks the Preview closed. Closing a closed one is fine.
func (s *Service) ClosePreview(ctx context.Context, applicationID uint64, number int) error {
	p, found, err := s.previews.ByNumber(ctx, applicationID, number)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	if p.State == domain.PreviewClosed {
		return nil
	}
	if err := p.Close(time.Now()); err != nil {
		return err
	}
	_, err = s.previews.Save(ctx, p)
	return err
}
