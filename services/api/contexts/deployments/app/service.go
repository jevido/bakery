package app

import (
	"context"
	"errors"
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
	previews     PreviewStore
	wake         chan struct{}
	// runtimes reach each Server's Podman, for Rollback's Image check,
	// Image retention and removing Previews; set by the Worker.
	runtimes Runtimes
	// DropPreviewRoute is routing's; nil leaves Preview routes alone.
	DropPreviewRoute func(ctx context.Context, applicationID uint64, preview int) error
	// Comments writes Preview comments; nil writes none.
	Comments *Commenter
	// Log reports what removing a Preview could not do.
	Log func(format string, args ...any)

	mu sync.Mutex
	// running holds the cancel function of each Deployment the Worker is
	// running in this process, until its Route moves.
	running map[uint64]context.CancelCauseFunc
}

func NewService(store Store, logs Logs, applications Applications, knownHosts KnownHosts, previews PreviewStore) *Service {
	return &Service{store: store, logs: logs, applications: applications, knownHosts: knownHosts, previews: previews, Log: func(string, ...any) {}, wake: make(chan struct{}, 1), running: map[uint64]context.CancelCauseFunc{}}
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
	return s.queue(ctx, domain.NewDeployment(applicationID, domain.TriggerManual))
}

// DeployPreview queues a Deployment of the Application's open Preview
// number and wakes the Worker.
func (s *Service) DeployPreview(ctx context.Context, applicationID uint64, number int, trigger domain.Trigger) (domain.Deployment, error) {
	a, err := s.applications(ctx, applicationID)
	if err != nil {
		return domain.Deployment{}, err
	}
	if a.BuildPack == BuildPackDockerImage {
		return domain.Deployment{}, ErrNoPreviews
	}
	p, found, err := s.previews.ByNumber(ctx, applicationID, number)
	if err != nil {
		return domain.Deployment{}, err
	}
	if !found {
		return domain.Deployment{}, ErrNotFound
	}
	if p.State != domain.PreviewOpen {
		return domain.Deployment{}, domain.ErrPreviewClosed
	}
	return s.queue(ctx, domain.NewPreviewDeployment(applicationID, number, trigger))
}

// Rollback queues a Deployment that starts the given Deployment's Image
// again with today's runtime settings.
func (s *Service) Rollback(ctx context.Context, id uint64) (domain.Deployment, error) {
	of, err := s.Deployment(ctx, id)
	if err != nil {
		return of, err
	}
	d, err := domain.NewRollback(of)
	if err != nil {
		return d, err
	}
	if s.runtimes == nil {
		return domain.Deployment{}, errors.New("no runtime to check images with")
	}
	rt, err := s.runtimes(ctx, d.ServerID)
	if err != nil {
		return domain.Deployment{}, err
	}
	ok, err := rt.ImageExists(ctx, d.Image)
	if err != nil {
		return domain.Deployment{}, err
	}
	if !ok {
		return domain.Deployment{}, fmt.Errorf("%w (%s)", ErrImageGone, d.Image)
	}
	return s.queue(ctx, d)
}

// queue stores a queued Deployment and wakes the Worker.
func (s *Service) queue(ctx context.Context, d domain.Deployment) (domain.Deployment, error) {
	d, err := s.store.Queue(ctx, d)
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
