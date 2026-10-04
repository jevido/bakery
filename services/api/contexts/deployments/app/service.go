package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	// StopRoute is routing's; nil leaves the Route of a stopped
	// Application alone.
	StopRoute func(ctx context.Context, applicationID uint64) error
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
	return s.deploy(ctx, applicationID, false)
}

// DeployWithoutCache queues a Deployment that builds every layer again.
func (s *Service) DeployWithoutCache(ctx context.Context, applicationID uint64) (domain.Deployment, error) {
	return s.deploy(ctx, applicationID, true)
}

func (s *Service) deploy(ctx context.Context, applicationID uint64, forceRebuild bool) (domain.Deployment, error) {
	if _, err := s.applications(ctx, applicationID); err != nil {
		return domain.Deployment{}, err
	}
	d := domain.NewDeployment(applicationID, domain.TriggerManual)
	d.ForceRebuild = forceRebuild
	return s.queue(ctx, d)
}

// Restart queues a Deployment that starts the Image of the Application's
// newest finished Deployment again with today's settings, without cloning
// or building, as Coolify's restart does.
func (s *Service) Restart(ctx context.Context, applicationID uint64) (domain.Deployment, error) {
	if _, err := s.applications(ctx, applicationID); err != nil {
		return domain.Deployment{}, err
	}
	all, err := s.store.ByApplication(ctx, applicationID, 50)
	if err != nil {
		return domain.Deployment{}, err
	}
	var last *domain.Deployment
	for i := range all {
		if all[i].Preview == 0 && all[i].Status == domain.Finished && (last == nil || all[i].ID > last.ID) {
			last = &all[i]
		}
	}
	d, err := domain.NewRestart(last)
	if err != nil {
		return d, err
	}
	if err := s.checkImage(ctx, d); err != nil {
		return domain.Deployment{}, err
	}
	return s.queue(ctx, d)
}

// Stop takes the Application's Route out of its Proxy and stops and
// removes its own Containers (its Previews keep running) on every Server
// it ran on. Nothing brings it back but a Deploy, Restart or Rollback.
func (s *Service) Stop(ctx context.Context, applicationID uint64) error {
	a, err := s.applications(ctx, applicationID)
	if err != nil {
		return err
	}
	if _, active, err := s.store.Active(ctx, applicationID); err != nil {
		return err
	} else if active {
		return ErrDeploymentInProgress
	}
	if s.runtimes == nil {
		return errors.New("no runtime to stop containers with")
	}
	if s.StopRoute != nil {
		if err := s.StopRoute(ctx, applicationID); err != nil {
			return fmt.Errorf("taking the route out of the proxy: %w", err)
		}
	}
	servers, err := s.store.ServerIDs(ctx, applicationID)
	if err != nil {
		return err
	}
	if !slices.Contains(servers, a.ServerID) {
		servers = append(servers, a.ServerID)
	}
	for _, id := range servers {
		rt, err := s.runtimes(ctx, id)
		if err != nil {
			return err
		}
		if _, err := rt.Stop(ctx, applicationID); err != nil {
			return fmt.Errorf("stopping on %s: %w", rt.ServerName(), err)
		}
	}
	return nil
}

// ApplicationStatus is what Status reports: Coolify's status string, and
// whether a Container (running or not) is there to remove.
type ApplicationStatus struct {
	Status           domain.ApplicationStatus
	ContainerPresent bool
}

// Status reads the Application's own Containers on its Target server and,
// when one runs and the Application has a Healthcheck, probes the newest
// running one once.
func (s *Service) Status(ctx context.Context, applicationID uint64) (ApplicationStatus, error) {
	a, err := s.applications(ctx, applicationID)
	if err != nil {
		return ApplicationStatus{}, err
	}
	if s.runtimes == nil {
		return ApplicationStatus{}, errors.New("no runtime to read containers with")
	}
	rt, err := s.runtimes(ctx, a.ServerID)
	if err != nil {
		return ApplicationStatus{}, err
	}
	states, running, err := rt.States(ctx, applicationID)
	if err != nil {
		return ApplicationStatus{}, err
	}
	health := domain.UnknownHealth
	if running != "" && a.HealthCheck.Enabled {
		url := fmt.Sprintf("http://127.0.0.1:%d%s", a.Port, a.HealthCheck.Path)
		timeout := time.Duration(max(a.HealthCheck.Timeout, 1)) * time.Second
		if ok, _, err := rt.Probe(ctx, running, url, timeout); ok && err == nil {
			health = domain.Healthy
		} else {
			health = domain.Unhealthy
		}
	}
	return ApplicationStatus{Status: domain.StatusOf(states, health), ContainerPresent: len(states) > 0}, nil
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
	if err := s.checkImage(ctx, d); err != nil {
		return domain.Deployment{}, err
	}
	return s.queue(ctx, d)
}

// checkImage makes sure the Image d starts is still on d's Server.
func (s *Service) checkImage(ctx context.Context, d domain.Deployment) error {
	if s.runtimes == nil {
		return errors.New("no runtime to check images with")
	}
	rt, err := s.runtimes(ctx, d.ServerID)
	if err != nil {
		return err
	}
	ok, err := rt.ImageExists(ctx, d.Image)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w (%s)", ErrImageGone, d.Image)
	}
	return nil
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

// History returns a page of the Application's Deployment history, and how
// many Deployments match in all.
func (s *Service) History(ctx context.Context, applicationID uint64, q HistoryQuery) ([]domain.Deployment, int, error) {
	if _, err := s.applications(ctx, applicationID); err != nil {
		return nil, 0, err
	}
	return s.store.History(ctx, applicationID, q)
}

// HistoryFacets returns what the Application's Deployment history can be
// filtered on.
func (s *Service) HistoryFacets(ctx context.Context, applicationID uint64) (HistoryFacets, error) {
	if _, err := s.applications(ctx, applicationID); err != nil {
		return HistoryFacets{}, err
	}
	return s.store.HistoryFacets(ctx, applicationID)
}

// RetainedImage is a Deployment whose Image a Rollback can start again.
type RetainedImage struct {
	domain.Deployment
	// Current is the newest finished Deployment's Image: the one the
	// Application runs, or a Restart starts.
	Current bool
}

// RetainedImages returns the Deployments a Rollback can go back to: those
// domain.RetainedImages names whose Image is still on their Server, newest
// first.
func (s *Service) RetainedImages(ctx context.Context, applicationID uint64) ([]RetainedImage, error) {
	if _, err := s.applications(ctx, applicationID); err != nil {
		return nil, err
	}
	if s.runtimes == nil {
		return nil, errors.New("no runtime to check images with")
	}
	all, err := s.store.ByApplication(ctx, applicationID, -1)
	if err != nil {
		return nil, err
	}
	var out []RetainedImage
	runtimes := map[uint64]Runtime{}
	for i, d := range domain.RetainedImages(all) {
		rt, ok := runtimes[d.ServerID]
		if !ok {
			if rt, err = s.runtimes(ctx, d.ServerID); err != nil {
				return nil, err
			}
			runtimes[d.ServerID] = rt
		}
		exists, err := rt.ImageExists(ctx, d.Image)
		if err != nil {
			return nil, err
		}
		if exists {
			out = append(out, RetainedImage{Deployment: d, Current: i == 0})
		}
	}
	return out, nil
}

// LogAfter returns up to limit log lines with an id above afterID.
func (s *Service) LogAfter(ctx context.Context, deploymentID, afterID uint64, limit int) ([]domain.LogLine, error) {
	return s.logs.After(ctx, deploymentID, afterID, limit)
}
