package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type Service struct {
	store        Store
	logs         Logs
	applications Applications
	knownHosts   KnownHosts
	wake         chan struct{}

	mu sync.Mutex
	// running holds the cancel function of each Deployment the Worker is
	// running in this process, until its Route moves.
	running map[uint64]context.CancelCauseFunc
}

func NewService(store Store, logs Logs, applications Applications, knownHosts KnownHosts) *Service {
	return &Service{store: store, logs: logs, applications: applications, knownHosts: knownHosts, wake: make(chan struct{}, 1), running: map[uint64]context.CancelCauseFunc{}}
}

func (s *Service) KnownHosts(ctx context.Context) ([]domain.KnownHost, error) {
	return s.knownHosts.List(ctx)
}

// ForgetKnownHost lets the next clone from the host trust its key again.
func (s *Service) ForgetKnownHost(ctx context.Context, id uint64) error {
	found, err := s.knownHosts.Forget(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	return err
}

// Deploy queues a Deployment of the Application and wakes the Worker.
func (s *Service) Deploy(ctx context.Context, applicationID uint64) (domain.Deployment, error) {
	if _, err := s.applications(ctx, applicationID); err != nil {
		return domain.Deployment{}, err
	}
	return s.queue(ctx, applicationID, domain.TriggerManual)
}

// queue queues a Deployment with the trigger and wakes the Worker.
func (s *Service) queue(ctx context.Context, applicationID uint64, trigger domain.Trigger) (domain.Deployment, error) {
	d, err := s.store.Queue(ctx, applicationID, trigger)
	if err != nil {
		return domain.Deployment{}, err
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return d, nil
}

// Cancel ends a queued Deployment at once, and stops a running one: the
// Worker then removes what it made and ends it cancelled. It returns the
// Deployment as it is now.
func (s *Service) Cancel(ctx context.Context, id uint64) (domain.Deployment, error) {
	d, err := s.Deployment(ctx, id)
	if err != nil {
		return d, err
	}
	if d.Status == domain.Queued {
		ok, err := s.store.CancelQueued(ctx, id)
		if err != nil {
			return d, err
		}
		if ok {
			return s.Deployment(ctx, id)
		}
		// Claimed in the meantime: cancel it as a running one.
	}
	// A just-claimed Deployment is registered a moment after its claim.
	for range 10 {
		if d, err = s.Deployment(ctx, id); err != nil {
			return d, err
		}
		if !d.Status.Active() {
			return d, fmt.Errorf("%w: it is %s", ErrNotCancellable, d.Status)
		}
		s.mu.Lock()
		cancel := s.running[id]
		s.mu.Unlock()
		if cancel != nil {
			cancel(domain.ErrCancelled)
			return d, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return d, fmt.Errorf("%w: its route is already moving", ErrNotCancellable)
}

// register makes a running Deployment cancellable; release ends that.
func (s *Service) register(id uint64, cancel context.CancelCauseFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running[id] = cancel
}

func (s *Service) release(id uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.running, id)
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
