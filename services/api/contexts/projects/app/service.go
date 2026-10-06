// Package app holds the projects use cases.
package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jevido/bakery/services/api/contexts/projects/domain"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrProjectNotEmpty = errors.New("delete the project's applications, databases and services first")
	// ErrEnvironmentNotEmpty is returned when an Environment that still has
	// Applications, Databases or Services is deleted.
	ErrEnvironmentNotEmpty = errors.New("the environment has resources")
)

// Store keeps projects, environments, applications and Environment variables.
// Variable values are encrypted by the Store; the service only sees plain values.
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
	// EnvironmentNameTaken reports whether another Environment of the
	// Project than exceptID has the name, case-insensitively.
	EnvironmentNameTaken(ctx context.Context, projectID uint64, name string, exceptID uint64) (bool, error)
	CreateEnvironment(ctx context.Context, e domain.Environment) (domain.Environment, error)
	UpdateEnvironment(ctx context.Context, e domain.Environment) error
	// DeleteEnvironment refuses (ErrEnvironmentNotEmpty) while it has
	// Applications.
	DeleteEnvironment(ctx context.Context, id uint64) error

	SlugTaken(ctx context.Context, slug string) (bool, error)
	// DomainsTaken returns those of the Domains that an Application other
	// than exceptID has.
	DomainsTaken(ctx context.Context, domains []string, exceptID uint64) ([]string, error)
	CreateApplication(ctx context.Context, a domain.Application) (domain.Application, error)
	Application(ctx context.Context, id uint64) (domain.Application, bool, error)
	UpdateApplication(ctx context.Context, a domain.Application) error
	DeleteApplication(ctx context.Context, id uint64) error

	// Variables returns the variables of one level (domain.FromProject,
	// FromEnvironment or FromApplication) of the owner with that id.
	Variables(ctx context.Context, level string, ownerID uint64) ([]domain.EnvironmentVariable, error)
	ReplaceVariables(ctx context.Context, level string, ownerID uint64, vars []domain.EnvironmentVariable) error
}

// NewDeployKey generates a Deploy key with the comment on its public half.
type NewDeployKey func(comment string) (domain.DeployKey, error)

type Service struct {
	store        Store
	newDeployKey NewDeployKey
	domainSuffix string
	// reservedDomain is the dashboard's own domain; no Application may take
	// it. Empty when there is none.
	reservedDomain   string
	onDeleted        []func(ctx context.Context, e ApplicationDeleted)
	onDomainsChanged []func(ctx context.Context, applicationID uint64, domains []string)
	onDeleting       []func(ctx context.Context, projectID uint64) (bool, error)
	onEnvDeleting    []func(ctx context.Context, environmentID uint64) (bool, error)
	onDomainCheck    []func(ctx context.Context, domain string) (bool, error)
	// ServerExists and LocalServer are servers' Exists and LocalID, asked
	// when an Application is created with a Target server; nil allows only
	// the Local server.
	ServerExists func(ctx context.Context, id uint64) (bool, error)
	LocalServer  func(ctx context.Context) (uint64, error)
}

// targetServer checks the Target server an Application is created with and
// returns it as stored: the Local server, by its id or as 0, is 0.
func (s *Service) targetServer(ctx context.Context, id uint64) (uint64, error) {
	if id == 0 {
		return 0, nil
	}
	if s.LocalServer != nil {
		local, err := s.LocalServer(ctx)
		if err != nil {
			return 0, err
		}
		if id == local {
			return 0, nil
		}
	}
	if s.ServerExists == nil {
		return 0, &domain.FieldError{Field: "server_id", Message: "no such server"}
	}
	ok, err := s.ServerExists(ctx, id)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, &domain.FieldError{Field: "server_id", Message: "no such server"}
	}
	return id, nil
}

// NewService takes the suffix default Domains get (`<slug>.<suffix>`) and
// the domain reserved for the Bakery dashboard (empty for none).
func NewService(store Store, newDeployKey NewDeployKey, domainSuffix, reservedDomain string) *Service {
	return &Service{store: store, newDeployKey: newDeployKey, domainSuffix: domainSuffix, reservedDomain: strings.ToLower(reservedDomain)}
}

// keepDeployKey gives an SSH Git repository a Deploy key if it has none, and
// takes it away from any other: an SSH Git repository always has one, an
// https one never carries an unused one.
func (s *Service) keepDeployKey(a *domain.Application) error {
	if !domain.IsSSHRepository(a.GitURL) {
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

// ApplicationDeleted is the event DeleteApplication publishes: what is to be
// removed with the Application besides its Containers, Routes and
// Deployments, as the person deleting it chose.
type ApplicationDeleted struct {
	ApplicationID uint64
	// DeleteVolumes removes its volumes, and with them its stored data.
	DeleteVolumes bool
	// DeleteImages removes the Images its Deployments built or pulled.
	DeleteImages bool
}

// OnApplicationDeleted registers a handler for the ApplicationDeleted event.
func (s *Service) OnApplicationDeleted(f func(ctx context.Context, e ApplicationDeleted)) {
	s.onDeleted = append(s.onDeleted, f)
}

// OnApplicationDomainsChanged registers a handler for the
// ApplicationDomainsChanged event.
func (s *Service) OnApplicationDomainsChanged(f func(ctx context.Context, applicationID uint64, domains []string)) {
	s.onDomainsChanged = append(s.onDomainsChanged, f)
}

// OnProjectDeleting registers a check DeleteProject asks first: another
// context that keeps something in the Project answers true while it does,
// and the deletion is refused.
func (s *Service) OnProjectDeleting(inUse func(ctx context.Context, projectID uint64) (bool, error)) {
	s.onDeleting = append(s.onDeleting, inUse)
}

// OnEnvironmentDeleting registers a check DeleteEnvironment asks first:
// another context that keeps something in the Environment answers true while
// it does, and the deletion is refused.
func (s *Service) OnEnvironmentDeleting(inUse func(ctx context.Context, environmentID uint64) (bool, error)) {
	s.onEnvDeleting = append(s.onEnvDeleting, inUse)
}

// OnDomainCheck registers a check asked for every Domain an Application is
// given: another context that serves the Domain answers true, and the
// Application is refused it. An error aborts the create or update.
func (s *Service) OnDomainCheck(inUse func(ctx context.Context, domain string) (bool, error)) {
	s.onDomainCheck = append(s.onDomainCheck, inUse)
}

// DomainInUse reports whether an Application has the Domain or it is the
// dashboard's own.
func (s *Service) DomainInUse(ctx context.Context, d string) (bool, error) {
	d = strings.ToLower(strings.TrimSpace(d))
	if s.reservedDomain != "" && d == s.reservedDomain {
		return true, nil
	}
	taken, err := s.store.DomainsTaken(ctx, []string{d}, 0)
	return len(taken) > 0, err
}

// CreateProject makes a Project in the Guild.
func (s *Service) CreateProject(ctx context.Context, guildID uint64, name, description string) (domain.Project, error) {
	p, err := domain.NewProject(name, description)
	if err != nil {
		return domain.Project{}, err
	}
	p.GuildID = guildID
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
	for _, inUse := range s.onDeleting {
		used, err := inUse(ctx, id)
		if err != nil {
			return err
		}
		if used {
			return ErrProjectNotEmpty
		}
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
	if strings.TrimSpace(in.Name) == "" {
		in.Name = domain.GeneratedApplicationName(in.BuildPack, in.GitURL, in.GitBranch, randomSuffix())
	}
	in, err = in.Normalize()
	if err != nil {
		return domain.Application{}, err
	}
	server, err := s.targetServer(ctx, in.ServerID)
	if err != nil {
		return domain.Application{}, err
	}
	slug, err := s.freeSlug(ctx, domain.Slugify(in.Name))
	if err != nil {
		return domain.Application{}, err
	}
	a := domain.Application{
		ServerID:         server,
		EnvironmentID:    env.ID,
		ProjectID:        env.ProjectID,
		GuildID:          env.GuildID,
		Name:             in.Name,
		Slug:             slug,
		BuildPack:        in.BuildPack,
		DockerImage:      in.DockerImage,
		PublishDirectory: in.PublishDirectory,
		GitURL:           in.GitURL,
		GitBranch:        in.GitBranch,
		DockerfilePath:   in.DockerfilePath,
		Port:             in.Port,
		Domains:          in.Domains,
		HealthCheck:      domain.DefaultHealthCheck(),
	}
	if in.Description != nil {
		a.Description = *in.Description
	}
	if in.HealthCheck != nil {
		a.HealthCheck = *in.HealthCheck
	}
	if in.RegistryCredentials != nil {
		a.RegistryCredentials = *in.RegistryCredentials
	}
	if in.Storages != nil {
		a.Storages = *in.Storages
	}
	if in.ResourceLimits != nil {
		a.ResourceLimits = *in.ResourceLimits
	}
	if len(a.Domains) == 0 {
		a.Domains = []string{domain.DefaultDomain(slug, s.domainSuffix)}
	}
	if err := s.checkDomains(ctx, a.Domains, 0); err != nil {
		return domain.Application{}, err
	}
	if err := s.keepDeployKey(&a); err != nil {
		return domain.Application{}, err
	}
	return s.store.CreateApplication(ctx, a)
}

// randomSuffix is the random part of a generated name: 8 lowercase letters
// and digits (Coolify uses 24; 8 keeps the slug and default domain readable).
func randomSuffix() string { return strings.ToLower(rand.Text()[:8]) }

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

func (s *Service) checkDomains(ctx context.Context, domains []string, exceptID uint64) error {
	for _, d := range domains {
		if s.reservedDomain != "" && d == s.reservedDomain {
			return &domain.FieldError{Field: "domains", Message: d + " is reserved for The Bakery dashboard"}
		}
	}
	taken, err := s.store.DomainsTaken(ctx, domains, exceptID)
	if err != nil {
		return err
	}
	if len(taken) > 0 {
		return &domain.FieldError{Field: "domains", Message: taken[0] + " is already used by another application"}
	}
	for _, d := range domains {
		for _, inUse := range s.onDomainCheck {
			used, err := inUse(ctx, d)
			if err != nil {
				return err
			}
			if used {
				return &domain.FieldError{Field: "domains", Message: d + " is already used by another application or service"}
			}
		}
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
// Containers and so stays fixed. No Domains goes back to the default one.
// When the Domains changed, ApplicationDomainsChanged is fired after the
// update is stored.
func (s *Service) UpdateApplication(ctx context.Context, id uint64, in domain.ApplicationInput) (domain.Application, error) {
	a, err := s.Application(ctx, id)
	if err != nil {
		return domain.Application{}, err
	}
	if in.BuildPack == "" {
		in.BuildPack = a.BuildPack
	}
	if c := in.RegistryCredentials; c != nil && c.Username != "" && c.Password == "" {
		// The form does not show the stored password; saving without
		// typing it again keeps it.
		in.RegistryCredentials = &domain.RegistryCredentials{Username: c.Username, Password: a.RegistryCredentials.Password}
	}
	in, err = in.Normalize()
	if err != nil {
		return domain.Application{}, err
	}
	a.BuildPack, a.DockerImage, a.PublishDirectory = in.BuildPack, in.DockerImage, in.PublishDirectory
	if in.RegistryCredentials != nil {
		a.RegistryCredentials = *in.RegistryCredentials
	}
	if in.Description != nil {
		a.Description = *in.Description
	}
	before := a.Domains
	a.Name, a.GitURL, a.GitBranch, a.DockerfilePath, a.Port, a.Domains =
		in.Name, in.GitURL, in.GitBranch, in.DockerfilePath, in.Port, in.Domains
	if in.HealthCheck != nil {
		a.HealthCheck = *in.HealthCheck
	}
	if in.Storages != nil {
		a.Storages = *in.Storages
	}
	if in.ResourceLimits != nil {
		a.ResourceLimits = *in.ResourceLimits
	}
	if len(a.Domains) == 0 {
		a.Domains = []string{domain.DefaultDomain(a.Slug, s.domainSuffix)}
	}
	if err := s.checkDomains(ctx, a.Domains, a.ID); err != nil {
		return domain.Application{}, err
	}
	if err := s.keepDeployKey(&a); err != nil {
		return domain.Application{}, err
	}
	if err := s.store.UpdateApplication(ctx, a); err != nil {
		return domain.Application{}, err
	}
	if !slices.Equal(before, a.Domains) {
		for _, f := range s.onDomainsChanged {
			f(ctx, a.ID, slices.Clone(a.Domains))
		}
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
	if !domain.IsSSHRepository(a.GitURL) {
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

// DeleteApplication deletes the Application and publishes
// ApplicationDeleted with the volumes and Images removed or kept as asked.
func (s *Service) DeleteApplication(ctx context.Context, id uint64, deleteVolumes, deleteImages bool) error {
	if _, err := s.Application(ctx, id); err != nil {
		return err
	}
	if err := s.store.DeleteApplication(ctx, id); err != nil {
		return err
	}
	e := ApplicationDeleted{ApplicationID: id, DeleteVolumes: deleteVolumes, DeleteImages: deleteImages}
	for _, f := range s.onDeleted {
		f(ctx, e)
	}
	return nil
}

func (s *Service) EnvironmentVariables(ctx context.Context, applicationID uint64) ([]domain.EnvironmentVariable, error) {
	if _, err := s.Application(ctx, applicationID); err != nil {
		return nil, err
	}
	return s.store.Variables(ctx, domain.FromApplication, applicationID)
}

func (s *Service) ReplaceEnvironmentVariables(ctx context.Context, applicationID uint64, vars []domain.EnvironmentVariable) error {
	if _, err := s.Application(ctx, applicationID); err != nil {
		return err
	}
	return s.replace(ctx, domain.FromApplication, applicationID, vars)
}

func (s *Service) replace(ctx context.Context, level string, ownerID uint64, vars []domain.EnvironmentVariable) error {
	if err := domain.CheckEnvironmentVariables(vars); err != nil {
		return err
	}
	return s.store.ReplaceVariables(ctx, level, ownerID, vars)
}

func (s *Service) ProjectSharedVariables(ctx context.Context, projectID uint64) ([]domain.EnvironmentVariable, error) {
	if _, err := s.Project(ctx, projectID); err != nil {
		return nil, err
	}
	return s.store.Variables(ctx, domain.FromProject, projectID)
}

func (s *Service) ReplaceProjectSharedVariables(ctx context.Context, projectID uint64, vars []domain.EnvironmentVariable) error {
	if _, err := s.Project(ctx, projectID); err != nil {
		return err
	}
	return s.replace(ctx, domain.FromProject, projectID, vars)
}

// Environment returns the Environment (without its Applications), or
// ErrNotFound.
func (s *Service) Environment(ctx context.Context, id uint64) (domain.Environment, error) {
	return s.environment(ctx, id)
}

func (s *Service) environment(ctx context.Context, id uint64) (domain.Environment, error) {
	e, found, err := s.store.Environment(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	return e, err
}

// EnvironmentInProject returns the Environment with its Applications, and
// its Project (whose Environments are all listed), or ErrNotFound.
func (s *Service) EnvironmentInProject(ctx context.Context, id uint64) (domain.Project, domain.Environment, error) {
	e, err := s.environment(ctx, id)
	if err != nil {
		return domain.Project{}, domain.Environment{}, err
	}
	p, err := s.Project(ctx, e.ProjectID)
	if err != nil {
		return domain.Project{}, domain.Environment{}, err
	}
	for _, env := range p.Environments {
		if env.ID == id {
			return p, env, nil
		}
	}
	return domain.Project{}, domain.Environment{}, ErrNotFound
}

// CreateEnvironment adds an Environment to the Project; its name is unique
// within the Project, case-insensitively.
func (s *Service) CreateEnvironment(ctx context.Context, projectID uint64, name, description string) (domain.Environment, error) {
	p, err := s.Project(ctx, projectID)
	if err != nil {
		return domain.Environment{}, err
	}
	e, err := domain.NewEnvironment(name, description)
	if err != nil {
		return domain.Environment{}, err
	}
	e.ProjectID, e.GuildID = projectID, p.GuildID
	if err := s.checkEnvironmentName(ctx, e, 0); err != nil {
		return domain.Environment{}, err
	}
	return s.store.CreateEnvironment(ctx, e)
}

// UpdateEnvironment renames or describes the Environment.
func (s *Service) UpdateEnvironment(ctx context.Context, id uint64, name, description string) (domain.Environment, error) {
	current, err := s.environment(ctx, id)
	if err != nil {
		return domain.Environment{}, err
	}
	e, err := domain.NewEnvironment(name, description)
	if err != nil {
		return domain.Environment{}, err
	}
	e.ID, e.ProjectID = id, current.ProjectID
	if err := s.checkEnvironmentName(ctx, e, id); err != nil {
		return domain.Environment{}, err
	}
	if err := s.store.UpdateEnvironment(ctx, e); err != nil {
		return domain.Environment{}, err
	}
	return s.environment(ctx, id)
}

func (s *Service) checkEnvironmentName(ctx context.Context, e domain.Environment, exceptID uint64) error {
	taken, err := s.store.EnvironmentNameTaken(ctx, e.ProjectID, e.Name, exceptID)
	if err != nil {
		return err
	}
	if taken {
		return &domain.FieldError{Field: "name", Message: fmt.Sprintf("the project already has an environment named %s", e.Name)}
	}
	return nil
}

// DeleteEnvironment deletes an Environment without Applications, Databases
// or Services, with its Shared variables. The last Environment of a Project
// may go too, as in Coolify.
func (s *Service) DeleteEnvironment(ctx context.Context, id uint64) error {
	if _, err := s.environment(ctx, id); err != nil {
		return err
	}
	for _, inUse := range s.onEnvDeleting {
		used, err := inUse(ctx, id)
		if err != nil {
			return err
		}
		if used {
			return ErrEnvironmentNotEmpty
		}
	}
	return s.store.DeleteEnvironment(ctx, id)
}

func (s *Service) EnvironmentSharedVariables(ctx context.Context, environmentID uint64) ([]domain.EnvironmentVariable, error) {
	if _, err := s.environment(ctx, environmentID); err != nil {
		return nil, err
	}
	return s.store.Variables(ctx, domain.FromEnvironment, environmentID)
}

func (s *Service) ReplaceEnvironmentSharedVariables(ctx context.Context, environmentID uint64, vars []domain.EnvironmentVariable) error {
	if _, err := s.environment(ctx, environmentID); err != nil {
		return err
	}
	return s.replace(ctx, domain.FromEnvironment, environmentID, vars)
}

// Variables returns an Application's own variables and the Shared ones it
// inherits from its Environment and Project.
func (s *Service) Variables(ctx context.Context, applicationID uint64) (own []domain.EnvironmentVariable, inherited []domain.InheritedVariable, err error) {
	a, err := s.Application(ctx, applicationID)
	if err != nil {
		return nil, nil, err
	}
	project, environment, own, err := s.levels(ctx, a)
	if err != nil {
		return nil, nil, err
	}
	return own, domain.Inherited(project, environment, own), nil
}

// MergedVariables is what an Application's build and Container get.
func (s *Service) MergedVariables(ctx context.Context, a domain.Application) (build, runtime map[string]string, err error) {
	project, environment, own, err := s.levels(ctx, a)
	if err != nil {
		return nil, nil, err
	}
	build, runtime = domain.Merge(project, environment, own)
	return build, runtime, nil
}

func (s *Service) levels(ctx context.Context, a domain.Application) (project, environment, own []domain.EnvironmentVariable, err error) {
	if project, err = s.store.Variables(ctx, domain.FromProject, a.ProjectID); err != nil {
		return
	}
	if environment, err = s.store.Variables(ctx, domain.FromEnvironment, a.EnvironmentID); err != nil {
		return
	}
	own, err = s.store.Variables(ctx, domain.FromApplication, a.ID)
	return
}
