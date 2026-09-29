// Package app holds the projects use cases.
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jevido/bakery/services/api/contexts/projects/domain"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrProjectNotEmpty = errors.New("delete the project's applications first")
)

// Store keeps projects, environments, applications and env vars. Env var
// values are encrypted by the Store; the service only sees plain values.
type Store interface {
	// CreateProject stores the Project and its production Environment in
	// one transaction.
	CreateProject(ctx context.Context, p domain.Project) (domain.Project, error)
	Projects(ctx context.Context) ([]domain.Project, error)
	// Project returns the Project with its Environments and their
	// Applications.
	Project(ctx context.Context, id uint64) (domain.Project, bool, error)
	UpdateProject(ctx context.Context, p domain.Project) error
	// DeleteProject refuses (ErrProjectNotEmpty) while it has Applications.
	DeleteProject(ctx context.Context, id uint64) error

	Environment(ctx context.Context, id uint64) (domain.Environment, bool, error)

	SlugTaken(ctx context.Context, slug string) (bool, error)
	// DomainTaken reports whether another Application than exceptID has the
	// Domain.
	DomainTaken(ctx context.Context, domain string, exceptID uint64) (bool, error)
	CreateApplication(ctx context.Context, a domain.Application) (domain.Application, error)
	Application(ctx context.Context, id uint64) (domain.Application, bool, error)
	UpdateApplication(ctx context.Context, a domain.Application) error
	DeleteApplication(ctx context.Context, id uint64) error

	EnvVars(ctx context.Context, applicationID uint64) ([]domain.EnvVar, error)
	ReplaceEnvVars(ctx context.Context, applicationID uint64, vars []domain.EnvVar) error
}

// NewDeployKey generates a Deploy key with the comment on its public half.
type NewDeployKey func(comment string) (domain.DeployKey, error)

type Service struct {
	store        Store
	newDeployKey NewDeployKey
	domainSuffix string
	// reservedDomain is the dashboard's own domain; no Application may take
	// it. Empty when there is none.
	reservedDomain string
	onDeleted      []func(ctx context.Context, applicationID uint64)
}

// NewService takes the suffix default Domains get (`<slug>.<suffix>`) and
// the domain reserved for the Bakery dashboard (empty for none).
func NewService(store Store, newDeployKey NewDeployKey, domainSuffix, reservedDomain string) *Service {
	return &Service{store: store, newDeployKey: newDeployKey, domainSuffix: domainSuffix, reservedDomain: strings.ToLower(reservedDomain)}
}

// keepDeployKey gives an SSH Source a Deploy key if it has none, and takes it
// away from any other Source: an SSH Source always has one, an https Source
// never carries an unused one.
func (s *Service) keepDeployKey(a *domain.Application) error {
	if !domain.IsSSHSource(a.GitURL) {
		a.DeployKey = domain.DeployKey{}
		return nil
	}
	if a.DeployKey.Public != "" {
		return nil
	}
	k, err := s.newDeployKey("bakery-" + a.Slug)
	if err != nil {
		return fmt.Errorf("generating the deploy key: %w", err)
	}
	a.DeployKey = k
	return nil
}

// OnApplicationDeleted registers a handler for the ApplicationDeleted event.
func (s *Service) OnApplicationDeleted(f func(ctx context.Context, applicationID uint64)) {
	s.onDeleted = append(s.onDeleted, f)
}

func (s *Service) CreateProject(ctx context.Context, name, description string) (domain.Project, error) {
	p, err := domain.NewProject(name, description)
	if err != nil {
		return domain.Project{}, err
	}
	return s.store.CreateProject(ctx, p)
}

func (s *Service) Projects(ctx context.Context) ([]domain.Project, error) {
	return s.store.Projects(ctx)
}

func (s *Service) Project(ctx context.Context, id uint64) (domain.Project, error) {
	p, found, err := s.store.Project(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	return p, err
}

func (s *Service) UpdateProject(ctx context.Context, id uint64, name, description string) (domain.Project, error) {
	if _, err := s.Project(ctx, id); err != nil {
		return domain.Project{}, err
	}
	p, err := domain.NewProject(name, description)
	if err != nil {
		return domain.Project{}, err
	}
	p.ID = id
	if err := s.store.UpdateProject(ctx, p); err != nil {
		return domain.Project{}, err
	}
	return s.Project(ctx, id)
}

func (s *Service) DeleteProject(ctx context.Context, id uint64) error {
	if _, err := s.Project(ctx, id); err != nil {
		return err
	}
	return s.store.DeleteProject(ctx, id)
}

func (s *Service) CreateApplication(ctx context.Context, environmentID uint64, in domain.ApplicationInput) (domain.Application, error) {
	env, found, err := s.store.Environment(ctx, environmentID)
	if err != nil {
		return domain.Application{}, err
	}
	if !found {
		return domain.Application{}, ErrNotFound
	}
	in, err = in.Normalize()
	if err != nil {
		return domain.Application{}, err
	}
	slug, err := s.freeSlug(ctx, domain.Slugify(in.Name))
	if err != nil {
		return domain.Application{}, err
	}
	a := domain.Application{
		EnvironmentID:  env.ID,
		ProjectID:      env.ProjectID,
		Name:           in.Name,
		Slug:           slug,
		GitURL:         in.GitURL,
		GitBranch:      in.GitBranch,
		DockerfilePath: in.DockerfilePath,
		Port:           in.Port,
		Domain:         in.Domain,
		HealthCheck:    domain.DefaultHealthCheck(),
	}
	if in.HealthCheck != nil {
		a.HealthCheck = *in.HealthCheck
	}
	if a.Domain == "" {
		a.Domain = domain.DefaultDomain(slug, s.domainSuffix)
	}
	if err := s.checkDomain(ctx, a.Domain, 0); err != nil {
		return domain.Application{}, err
	}
	if err := s.keepDeployKey(&a); err != nil {
		return domain.Application{}, err
	}
	return s.store.CreateApplication(ctx, a)
}

// freeSlug returns base, or base-2, base-3, ... whichever is free first.
func (s *Service) freeSlug(ctx context.Context, base string) (string, error) {
	for i := 1; i < 100; i++ {
		slug := base
		if i > 1 {
			slug = fmt.Sprintf("%s-%d", base, i)
		}
		taken, err := s.store.SlugTaken(ctx, slug)
		if err != nil {
			return "", err
		}
		if !taken {
			return slug, nil
		}
	}
	return "", &domain.FieldError{Field: "name", Message: "too many applications with this name"}
}

func (s *Service) checkDomain(ctx context.Context, d string, exceptID uint64) error {
	if s.reservedDomain != "" && d == s.reservedDomain {
		return &domain.FieldError{Field: "domain", Message: "domain is reserved for the Bakery dashboard"}
	}
	taken, err := s.store.DomainTaken(ctx, d, exceptID)
	if err != nil {
		return err
	}
	if taken {
		return &domain.FieldError{Field: "domain", Message: d + " is already used by another application"}
	}
	return nil
}

func (s *Service) Application(ctx context.Context, id uint64) (domain.Application, error) {
	a, found, err := s.store.Application(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	return a, err
}

// UpdateApplication changes everything but the Slug, which names Images and
// Containers and so stays fixed. An empty Domain goes back to the default.
func (s *Service) UpdateApplication(ctx context.Context, id uint64, in domain.ApplicationInput) (domain.Application, error) {
	a, err := s.Application(ctx, id)
	if err != nil {
		return domain.Application{}, err
	}
	in, err = in.Normalize()
	if err != nil {
		return domain.Application{}, err
	}
	a.Name, a.GitURL, a.GitBranch, a.DockerfilePath, a.Port, a.Domain =
		in.Name, in.GitURL, in.GitBranch, in.DockerfilePath, in.Port, in.Domain
	if in.HealthCheck != nil {
		a.HealthCheck = *in.HealthCheck
	}
	if a.Domain == "" {
		a.Domain = domain.DefaultDomain(a.Slug, s.domainSuffix)
	}
	if err := s.checkDomain(ctx, a.Domain, a.ID); err != nil {
		return domain.Application{}, err
	}
	if err := s.keepDeployKey(&a); err != nil {
		return domain.Application{}, err
	}
	if err := s.store.UpdateApplication(ctx, a); err != nil {
		return domain.Application{}, err
	}
	return a, nil
}

// RegenerateDeployKey replaces an SSH Application's Deploy key; the old
// public half stops working once removed from the repository.
func (s *Service) RegenerateDeployKey(ctx context.Context, id uint64) (domain.Application, error) {
	a, err := s.Application(ctx, id)
	if err != nil {
		return domain.Application{}, err
	}
	if !domain.IsSSHSource(a.GitURL) {
		return domain.Application{}, &domain.FieldError{Field: "git_url", Message: "only an application with an SSH git URL has a deploy key"}
	}
	a.DeployKey = domain.DeployKey{}
	if err := s.keepDeployKey(&a); err != nil {
		return domain.Application{}, err
	}
	if err := s.store.UpdateApplication(ctx, a); err != nil {
		return domain.Application{}, err
	}
	return a, nil
}

func (s *Service) DeleteApplication(ctx context.Context, id uint64) error {
	if _, err := s.Application(ctx, id); err != nil {
		return err
	}
	if err := s.store.DeleteApplication(ctx, id); err != nil {
		return err
	}
	for _, f := range s.onDeleted {
		f(ctx, id)
	}
	return nil
}

func (s *Service) EnvVars(ctx context.Context, applicationID uint64) ([]domain.EnvVar, error) {
	if _, err := s.Application(ctx, applicationID); err != nil {
		return nil, err
	}
	return s.store.EnvVars(ctx, applicationID)
}

func (s *Service) ReplaceEnvVars(ctx context.Context, applicationID uint64, vars []domain.EnvVar) error {
	if _, err := s.Application(ctx, applicationID); err != nil {
		return err
	}
	if err := domain.CheckEnvVars(vars); err != nil {
		return err
	}
	return s.store.ReplaceEnvVars(ctx, applicationID, vars)
}
