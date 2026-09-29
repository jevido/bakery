package app

import (
	"context"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type Service struct {
	store        Store
	logs         Logs
	applications Applications
	wake         chan struct{}
}

func NewService(store Store, logs Logs, applications Applications) *Service {
	return &Service{store: store, logs: logs, applications: applications, wake: make(chan struct{}, 1)}
}

// Deploy queues a Deployment of the Application and wakes the Worker.
func (s *Service) Deploy(ctx context.Context, applicationID uint64) (domain.Deployment, error) {
	if _, err := s.applications(ctx, applicationID); err != nil {
		return domain.Deployment{}, err
	}
	d, err := s.store.Queue(ctx, applicationID)
	if err != nil {
		return domain.Deployment{}, err
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return d, nil
}

// Wake is signalled when there may be work; the Worker listens on it.
func (s *Service) Wake() <-chan struct{} { return s.wake }

func (s *Service) Deployment(ctx context.Context, id uint64) (domain.Deployment, error) {
	d, found, err := s.store.ByID(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	return d, err
}

func (s *Service) Deployments(ctx context.Context, applicationID uint64) ([]domain.Deployment, error) {
	if _, err := s.applications(ctx, applicationID); err != nil {
		return nil, err
	}
	return s.store.ByApplication(ctx, applicationID, 50)
}

// LogAfter returns up to limit log lines with an id above afterID.
func (s *Service) LogAfter(ctx context.Context, deploymentID, afterID uint64, limit int) ([]domain.LogLine, error) {
	return s.logs.After(ctx, deploymentID, afterID, limit)
}
