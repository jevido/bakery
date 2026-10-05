// Package app holds the services use cases.
package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jevido/bakery/services/api/contexts/services/domain"
)

var (
	ErrNotFound = errors.New("not found")
	// ErrBusy refuses an action while another one runs for the Service.
	ErrBusy = errors.New("the service is busy with another action; try again when it is done")
)

// Store keeps Services; variable values are encrypted by the Store.
type Store interface {
	Create(ctx context.Context, s domain.Service) (domain.Service, error)
	Get(ctx context.Context, id uint64) (domain.Service, bool, error)
	ForProject(ctx context.Context, projectID uint64) ([]domain.Service, error)
	// Wanted lists the Services whose desired state is running.
	Wanted(ctx context.Context) ([]domain.Service, error)
	Save(ctx context.Context, s domain.Service) error
	SetLastError(ctx context.Context, id uint64, msg string) error
	SetDesiredState(ctx context.Context, id uint64, state domain.DesiredState) error
	Delete(ctx context.Context, id uint64) error
	SlugTaken(ctx context.Context, slug string) (bool, error)
	CountForProject(ctx context.Context, projectID uint64) (int64, error)
	CountForEnvironment(ctx context.Context, environmentID uint64) (int64, error)
	DomainTaken(ctx context.Context, domain string, exceptServiceID uint64) (bool, error)
}

// Runtime runs a Service's Components. A stopped Service has no
// Containers.
type Runtime interface {
	// Up (re)creates every Container from the resolved Compose file.
	Up(ctx context.Context, s domain.Service, resolved domain.Compose, pull bool) error
	// Down removes the Containers; volumes stay.
	Down(ctx context.Context, s domain.Service) error
	// Remove removes the Containers, the network and the volumes.
	Remove(ctx context.Context, s domain.Service) error
	Statuses(ctx context.Context, s domain.Service) (map[string]domain.Status, map[string]string, error)
	Logs(ctx context.Context, s domain.Service, component string, follow bool, tail int, out func(stream, line string)) (bool, error)
}

// Route is a Public Component served on its Domains.
type Route struct {
	Component string
	Domains   []string
	Container string
	Port      int
}

// Routes is routing as this context uses it.
type Routes interface {
	Set(ctx context.Context, serviceID uint64, routes []Route) error
	Drop(ctx context.Context, serviceID uint64) error
}

// Environment is where a Service is placed, as projects told us.
type Environment struct {
	ID        uint64
	ProjectID uint64
}

// Environments looks up an Environment, returning ErrNotFound for an
// unknown one.
type Environments func(ctx context.Context, id uint64) (Environment, error)

// DomainInUse asks projects whether an Application (or the dashboard) has
// the Domain.
type DomainInUse func(ctx context.Context, domain string) (bool, error)

// opTimeout bounds one background action, pulls included.
const opTimeout = 15 * time.Minute

// Service runs Up in the background, one action at a time per Service,
// because pulling images can take longer than a request.
type Service struct {
	store        Store
	runtime      Runtime
	routes       Routes
	environments Environments
	domainInUse  DomainInUse
	domainSuffix string
	Generate     domain.Generate
	// Log reports background failures; nil discards them.
	Log func(format string, args ...any)
	// catalog is the Service templates, set with SetTemplates.
	catalog []domain.Template

	mu   sync.Mutex
	busy map[uint64]context.CancelFunc
	wg   sync.WaitGroup
}

func NewService(store Store, runtime Runtime, routes Routes, environments Environments, domainInUse DomainInUse, domainSuffix string, generate domain.Generate) *Service {
	return &Service{
		store: store, runtime: runtime, routes: routes, environments: environments,
		domainInUse: domainInUse, domainSuffix: domainSuffix, Generate: generate,
		busy: map[uint64]context.CancelFunc{},
	}
}

// SetTemplates gives the Service its catalog of templates.
func (s *Service) SetTemplates(list []domain.Template) { s.catalog = list }

// View is a Service as the Owner sees it.
type View struct {
	domain.Service
	Status     domain.ServiceStatus
	Busy       bool
	Components []ComponentView
}

// ComponentView is a Component with what its Container is doing.
type ComponentView struct {
	domain.Component
	Status domain.Status
	Detail string
	// GeneratedDomain is the Domain The Bakery gives a Public Component,
	// for the dashboard's Generate domain; "" when not public.
	GeneratedDomain string
}

// Input is what the Owner creates a Service from.
type Input struct {
	Name        string
	Compose     string
	TemplateKey string
}

// Change is what the Owner changes; nil fields stay.
type Change struct {
	Name        *string
	Description *string
	Compose     *string
	Domains     map[string][]string
	Variables   map[string]string
}

func (s *Service) get(ctx context.Context, id uint64) (domain.Service, error) {
	sv, found, err := s.store.Get(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	return sv, err
}

// newService makes the Service with the first Slug that is free and whose
// default Domains nobody has, so a second "whoami" becomes whoami-2 with
// its own Domain instead of a refusal.
func (s *Service) newService(ctx context.Context, env Environment, in Input) (domain.Service, error) {
	base := domain.Slugify(in.Name)
	for i := 1; i < 100; i++ {
		slug := base
		if i > 1 {
			slug = fmt.Sprintf("%s-%d", base, i)
		}
		taken, err := s.store.SlugTaken(ctx, slug)
		if err != nil {
			return domain.Service{}, err
		}
		if taken {
			continue
		}
		sv, err := domain.NewService(env.ID, env.ProjectID, in.Name, slug, in.Compose, s.domainSuffix, s.Generate)
		if err != nil {
			return domain.Service{}, err
		}
		sv.TemplateKey = in.TemplateKey
		err = s.checkDomains(ctx, sv)
		var fe *domain.FieldError
		if errors.As(err, &fe) && fe.Field == "domains" {
			continue
		}
		return sv, err
	}
	return domain.Service{}, &domain.FieldError{Field: "name", Message: "too many services with this name"}
}

// checkDomains refuses a Domain an Application, the dashboard or another
// Service has.
func (s *Service) checkDomains(ctx context.Context, sv domain.Service) error {
	for _, d := range sv.Domains() {
		used, err := s.domainInUse(ctx, d)
		if err != nil {
			return err
		}
		if !used {
			used, err = s.store.DomainTaken(ctx, d, sv.ID)
			if err != nil {
				return err
			}
		}
		if used {
			return &domain.FieldError{Field: "domains", Message: d + " is already used by another application or service"}
		}
	}
	return nil
}

// randomSuffix is the random part of a generated name: 8 lowercase letters
// and digits (Coolify uses 24; 8 keeps the slug and default domains readable).
func randomSuffix() string { return strings.ToLower(rand.Text()[:8]) }

// Create stores a new Service and brings it Up in the background.
func (s *Service) Create(ctx context.Context, environmentID uint64, in Input) (View, error) {
	env, err := s.environments(ctx, environmentID)
	if err != nil {
		return View{}, err
	}
	if strings.TrimSpace(in.Name) == "" && in.TemplateKey == "" {
		in.Name = domain.GeneratedName(randomSuffix())
	}
	sv, err := s.newService(ctx, env, in)
	if err != nil {
		return View{}, err
	}
	sv, err = s.store.Create(ctx, sv)
	if err != nil {
		return View{}, err
	}
	if err := s.background(sv.ID, "deploy", true); err != nil {
		return View{}, err
	}
	return s.view(ctx, sv)
}

// Update changes the Service. Compose file and variable changes apply on
// the next Up; changed Domains of a running Service move at once when the
// Compose file stays the same.
func (s *Service) Update(ctx context.Context, id uint64, ch Change) (View, error) {
	sv, err := s.get(ctx, id)
	if err != nil {
		return View{}, err
	}
	if ch.Compose != nil && *ch.Compose != sv.ComposeFile {
		if err := sv.ChangeCompose(*ch.Compose, s.domainSuffix, s.Generate); err != nil {
			return View{}, err
		}
	} else {
		ch.Compose = nil
	}
	for component, domains := range ch.Domains {
		if err := sv.SetDomains(component, domains); err != nil {
			return View{}, err
		}
	}
	for name, value := range ch.Variables {
		if err := sv.SetVariable(name, value); err != nil {
			return View{}, err
		}
	}
	if ch.Name != nil {
		if err := sv.Rename(*ch.Name); err != nil {
			return View{}, err
		}
	}
	if ch.Description != nil {
		if err := sv.Describe(*ch.Description); err != nil {
			return View{}, err
		}
	}
	if err := s.checkDomains(ctx, sv); err != nil {
		return View{}, err
	}
	if err := s.store.Save(ctx, sv); err != nil {
		return View{}, err
	}
	if len(ch.Domains) > 0 && ch.Compose == nil && sv.DesiredState == domain.Running {
		if err := s.routes.Set(ctx, sv.ID, routesOf(sv)); err != nil {
			return View{}, err
		}
	}
	return s.view(ctx, sv)
}

func routesOf(sv domain.Service) []Route {
	var out []Route
	for _, c := range sv.Components {
		if c.Public {
			out = append(out, Route{Component: c.Name, Domains: c.Domains, Container: domain.ContainerName(sv.ID, c.Name), Port: c.Port})
		}
	}
	return out
}

// background starts Up for the Service unless another action runs for it.
// The Service is read again when the action starts, so it runs what is
// stored.
func (s *Service) background(id uint64, what string, pull bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	s.mu.Lock()
	if _, ok := s.busy[id]; ok {
		s.mu.Unlock()
		cancel()
		return ErrBusy
	}
	s.busy[id] = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()
		err := s.up(ctx, id, pull)
		s.mu.Lock()
		delete(s.busy, id)
		s.mu.Unlock()
		msg := ""
		if err != nil && !errors.Is(err, context.Canceled) {
			msg = fmt.Sprintf("%s failed: %v", what, err)
			if s.Log != nil {
				s.Log("services: %s of service %d: %v", what, id, err)
			}
		}
		if errors.Is(err, ErrNotFound) || errors.Is(err, context.Canceled) {
			return // deleted or stopped meanwhile
		}
		if err := s.store.SetLastError(context.Background(), id, msg); err != nil && s.Log != nil {
			s.Log("services: storing the outcome of service %d: %v", id, err)
		}
	}()
	return nil
}

func (s *Service) up(ctx context.Context, id uint64, pull bool) error {
	sv, err := s.get(ctx, id)
	if err != nil {
		return err
	}
	resolved, err := sv.Resolved()
	if err != nil {
		return err
	}
	if err := s.runtime.Up(ctx, sv, resolved, pull); err != nil {
		return err
	}
	return s.routes.Set(ctx, sv.ID, routesOf(sv))
}

// cancel ends whatever runs for the Service and waits until it has let go.
func (s *Service) cancel(id uint64) {
	s.mu.Lock()
	cancel, ok := s.busy[id]
	s.mu.Unlock()
	if !ok {
		return
	}
	cancel()
	for {
		s.mu.Lock()
		_, still := s.busy[id]
		s.mu.Unlock()
		if !still {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Wait waits for every background action to finish (tests).
func (s *Service) Wait() { s.wg.Wait() }

func (s *Service) setDesired(ctx context.Context, id uint64, state domain.DesiredState) (domain.Service, error) {
	sv, err := s.get(ctx, id)
	if err != nil {
		return sv, err
	}
	sv.DesiredState = state
	return sv, s.store.SetDesiredState(ctx, id, state)
}

// Start (and Restart) recreate every Container in the background from the
// images there are; Redeploy pulls them again first.
func (s *Service) Start(ctx context.Context, id uint64) (View, error) {
	return s.upNow(ctx, id, "start", false)
}

func (s *Service) Restart(ctx context.Context, id uint64) (View, error) {
	return s.upNow(ctx, id, "restart", false)
}

func (s *Service) Redeploy(ctx context.Context, id uint64) (View, error) {
	return s.upNow(ctx, id, "redeploy", true)
}

func (s *Service) upNow(ctx context.Context, id uint64, what string, pull bool) (View, error) {
	s.mu.Lock()
	_, busy := s.busy[id]
	s.mu.Unlock()
	if busy {
		return View{}, ErrBusy
	}
	sv, err := s.setDesired(ctx, id, domain.Running)
	if err != nil {
		return View{}, err
	}
	if err := s.background(id, what, pull); err != nil {
		return View{}, err
	}
	return s.view(ctx, sv)
}

// Stop ends whatever runs for the Service and removes its Containers at
// once; its volumes and Service routes stay.
func (s *Service) Stop(ctx context.Context, id uint64) (View, error) {
	sv, err := s.setDesired(ctx, id, domain.Stopped)
	if err != nil {
		return View{}, err
	}
	s.cancel(id)
	if err := s.runtime.Down(ctx, sv); err != nil {
		return View{}, err
	}
	if err := s.store.SetLastError(ctx, id, ""); err != nil {
		return View{}, err
	}
	sv.LastError = ""
	return s.view(ctx, sv)
}

// Delete drops the Service routes, removes the Containers, the network and
// the volumes with all data, then the Service.
func (s *Service) Delete(ctx context.Context, id uint64) error {
	sv, err := s.get(ctx, id)
	if err != nil {
		return err
	}
	s.cancel(id)
	if err := s.routes.Drop(ctx, id); err != nil {
		return err
	}
	if err := s.runtime.Remove(ctx, sv); err != nil {
		return err
	}
	return s.store.Delete(ctx, id)
}

func (s *Service) view(ctx context.Context, sv domain.Service) (View, error) {
	statuses, details, err := s.runtime.Statuses(ctx, sv)
	if err != nil {
		return View{}, err
	}
	s.mu.Lock()
	_, busy := s.busy[sv.ID]
	s.mu.Unlock()
	v := View{Service: sv, Status: sv.Summarize(statuses, busy), Busy: busy}
	first := true
	for _, c := range sv.Components {
		cv := ComponentView{Component: c, Status: statuses[c.Name], Detail: details[c.Name]}
		if c.Public {
			cv.GeneratedDomain = domain.DefaultDomain(sv.Slug, c.Name, first, s.domainSuffix)
			first = false
		}
		v.Components = append(v.Components, cv)
	}
	return v, nil
}

func (s *Service) Get(ctx context.Context, id uint64) (View, error) {
	sv, err := s.get(ctx, id)
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, sv)
}

// ForProject lists the Project's Services with their status.
func (s *Service) ForProject(ctx context.Context, projectID uint64) ([]View, error) {
	list, err := s.store.ForProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(list))
	for _, sv := range list {
		v, err := s.view(ctx, sv)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// InUse reports whether the Project has Services, for projects'
// OnProjectDeleting.
func (s *Service) InUse(ctx context.Context, projectID uint64) (bool, error) {
	n, err := s.store.CountForProject(ctx, projectID)
	return n > 0, err
}

// InUseInEnvironment reports whether the Environment has Services, for
// projects' OnEnvironmentDeleting.
func (s *Service) InUseInEnvironment(ctx context.Context, environmentID uint64) (bool, error) {
	n, err := s.store.CountForEnvironment(ctx, environmentID)
	return n > 0, err
}

// DomainInUse reports whether a Service has the Domain, for projects'
// OnDomainCheck.
func (s *Service) DomainInUse(ctx context.Context, d string) (bool, error) {
	return s.store.DomainTaken(ctx, d, 0)
}

// Recover brings Up, in the background and without pulling, every Service
// that should run but has a Component without a Container.
func (s *Service) Recover(ctx context.Context) error {
	list, err := s.store.Wanted(ctx)
	if err != nil {
		return err
	}
	for _, sv := range list {
		statuses, _, err := s.runtime.Statuses(ctx, sv)
		if err != nil {
			return err
		}
		for _, st := range statuses {
			if st == domain.StatusMissing {
				_ = s.background(sv.ID, "start", false)
				break
			}
		}
	}
	return nil
}

// Logs follows a Component's Container; found is false without one.
func (s *Service) Logs(ctx context.Context, id uint64, component string, tail int, out func(stream, line string)) (bool, error) {
	sv, err := s.get(ctx, id)
	if err != nil {
		return false, err
	}
	if _, ok := sv.Component(component); !ok {
		return false, ErrNotFound
	}
	return s.runtime.Logs(ctx, sv, component, true, tail, out)
}
