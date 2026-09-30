// Package http exposes the projects context over HTTP.
package http

import (
	"context"
	"errors"
	"strconv"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/projects/app"
	"github.com/jevido/bakery/services/api/contexts/projects/domain"
)

type Controller struct {
	service *app.Service
	// localServer is the Local server's id, shown where an Application's
	// Target server is 0.
	localServer func(ctx context.Context) (uint64, error)
}

func NewController(service *app.Service, localServer func(ctx context.Context) (uint64, error)) *Controller {
	return &Controller{service: service, localServer: localServer}
}

// localID is the Local server's id, 0 when it cannot be read (the
// dashboard then shows no Server rather than failing the page).
func (c *Controller) localID(ctx contractshttp.Context) uint64 {
	id, err := c.localServer(ctx.Context())
	if err != nil {
		facades.Log().Errorf("projects: reading the local server: %v", err)
	}
	return id
}

type applicationJSON struct {
	ID               uint64   `json:"id"`
	ProjectID        uint64   `json:"project_id"`
	EnvironmentID    uint64   `json:"environment_id"`
	Name             string   `json:"name"`
	Slug             string   `json:"slug"`
	BuildPack        string   `json:"build_pack"`
	ImageReference   string   `json:"image_reference"`
	PublishDirectory string   `json:"publish_directory"`
	GitURL           string   `json:"git_url"`
	GitBranch        string   `json:"git_branch"`
	DockerfilePath   string   `json:"dockerfile_path"`
	Port             int      `json:"port"`
	Domains          []string `json:"domains"`
	// DeployKeyPublic is empty for an https Source.
	DeployKeyPublic string `json:"deploy_key_public"`
	// The registry password itself is never returned.
	RegistryUsername    string `json:"registry_username"`
	HasRegistryPassword bool   `json:"has_registry_password"`
	// PublicURL is where the Application is reached through the Proxy on
	// its primary Domain, with the Proxy's HTTPS port when it is not 443
	// (development); PublicURLs the same for every Domain.
	PublicURL      string             `json:"public_url"`
	PublicURLs     []string           `json:"public_urls"`
	HealthCheck    healthCheckJSON    `json:"health_check"`
	Storages       []storageJSON      `json:"storages"`
	ResourceLimits resourceLimitsJSON `json:"resource_limits"`
	// ServerID is the Target server's id, the Local server's included.
	ServerID uint64 `json:"server_id"`
}

// resourceLimitsJSON uses null for unlimited.
type resourceLimitsJSON struct {
	MemoryMB *int     `json:"memory_mb"`
	CPUs     *float64 `json:"cpus"`
}

func resourceLimitsToJSON(l domain.ResourceLimits) resourceLimitsJSON {
	var out resourceLimitsJSON
	if l.MemoryMB != 0 {
		out.MemoryMB = &l.MemoryMB
	}
	if l.CPUs != 0 {
		out.CPUs = &l.CPUs
	}
	return out
}

func (r resourceLimitsJSON) domain() domain.ResourceLimits {
	var l domain.ResourceLimits
	if r.MemoryMB != nil {
		l.MemoryMB = *r.MemoryMB
	}
	if r.CPUs != nil {
		l.CPUs = *r.CPUs
	}
	return l
}

type storageJSON struct {
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
}

func storagesToJSON(storages []domain.Storage) []storageJSON {
	out := make([]storageJSON, len(storages))
	for i, s := range storages {
		out[i] = storageJSON(s)
	}
	return out
}

type healthCheckJSON struct {
	Enabled     bool   `json:"enabled"`
	Path        string `json:"path"`
	Interval    int    `json:"interval"`
	Timeout     int    `json:"timeout"`
	Retries     int    `json:"retries"`
	StartPeriod int    `json:"start_period"`
}

func (h healthCheckJSON) domain() domain.HealthCheck {
	return domain.HealthCheck{Enabled: h.Enabled, Path: h.Path, Interval: h.Interval, Timeout: h.Timeout, Retries: h.Retries, StartPeriod: h.StartPeriod}
}

func applicationToJSON(a domain.Application, localServer uint64) applicationJSON {
	server := a.ServerID
	if server == 0 {
		server = localServer
	}
	return applicationJSON{
		ServerID: server,
		ID:       a.ID, ProjectID: a.ProjectID, EnvironmentID: a.EnvironmentID, Name: a.Name, Slug: a.Slug,
		BuildPack: string(a.BuildPack), ImageReference: a.ImageReference, PublishDirectory: a.PublishDirectory,
		GitURL: a.GitURL, GitBranch: a.GitBranch, DockerfilePath: a.DockerfilePath, Port: a.Port, Domains: a.Domains,
		DeployKeyPublic: a.DeployKey.Public, PublicURL: publicURL(a.PrimaryDomain()), PublicURLs: publicURLs(a.Domains),
		RegistryUsername: a.RegistryCredentials.Username, HasRegistryPassword: a.RegistryCredentials.Username != "", // both or neither
		Storages: storagesToJSON(a.Storages), ResourceLimits: resourceLimitsToJSON(a.ResourceLimits),
		HealthCheck: healthCheckJSON{
			Enabled: a.HealthCheck.Enabled, Path: a.HealthCheck.Path, Interval: a.HealthCheck.Interval,
			Timeout: a.HealthCheck.Timeout, Retries: a.HealthCheck.Retries, StartPeriod: a.HealthCheck.StartPeriod,
		},
	}
}

func publicURLs(domains []string) []string {
	out := make([]string, len(domains))
	for i, d := range domains {
		out[i] = publicURL(d)
	}
	return out
}

func publicURL(d string) string {
	port := facades.Config().GetInt("bakery.proxy.https_port", 443)
	if port == 443 {
		return "https://" + d
	}
	return "https://" + d + ":" + strconv.Itoa(port)
}

type environmentJSON struct {
	ID           uint64            `json:"id"`
	Name         string            `json:"name"`
	Applications []applicationJSON `json:"applications"`
}

type projectJSON struct {
	ID           uint64            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Environments []environmentJSON `json:"environments,omitempty"`
}

func projectToJSON(p domain.Project, localServer uint64) projectJSON {
	out := projectJSON{ID: p.ID, Name: p.Name, Description: p.Description}
	for _, e := range p.Environments {
		ej := environmentJSON{ID: e.ID, Name: e.Name, Applications: []applicationJSON{}}
		for _, a := range e.Applications {
			ej.Applications = append(ej.Applications, applicationToJSON(a, localServer))
		}
		out.Environments = append(out.Environments, ej)
	}
	return out
}

// fail maps the context's errors to responses.
func fail(ctx contractshttp.Context, err error) contractshttp.Response {
	var fe *domain.FieldError
	switch {
	case errors.As(err, &fe):
		return respond.Invalid(ctx, fe.Field, fe.Message)
	case errors.Is(err, app.ErrNotFound):
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	case errors.Is(err, app.ErrProjectNotEmpty):
		return respond.Error(ctx, contractshttp.StatusConflict, err.Error())
	}
	return respond.ServerError(ctx, err)
}

func id(ctx contractshttp.Context) (uint64, bool) {
	v, err := strconv.ParseUint(ctx.Request().Route("id"), 10, 64)
	return v, err == nil
}

func notFound(ctx contractshttp.Context) contractshttp.Response {
	return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
}

type projectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (c *Controller) ListProjects(ctx contractshttp.Context) contractshttp.Response {
	ps, err := c.service.Projects(ctx.Context())
	if err != nil {
		return fail(ctx, err)
	}
	out := make([]projectJSON, len(ps))
	for i, p := range ps {
		out[i] = projectToJSON(p, c.localID(ctx))
	}
	return ctx.Response().Success().Json(contractshttp.Json{"projects": out})
}

func (c *Controller) CreateProject(ctx contractshttp.Context) contractshttp.Response {
	var req projectRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	p, err := c.service.CreateProject(ctx.Context(), req.Name, req.Description)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusCreated, contractshttp.Json{"project": projectToJSON(p, c.localID(ctx))})
}

func (c *Controller) ShowProject(ctx contractshttp.Context) contractshttp.Response {
	pid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	p, err := c.service.Project(ctx.Context(), pid)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"project": projectToJSON(p, c.localID(ctx))})
}

func (c *Controller) UpdateProject(ctx contractshttp.Context) contractshttp.Response {
	pid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req projectRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	p, err := c.service.UpdateProject(ctx.Context(), pid, req.Name, req.Description)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"project": projectToJSON(p, c.localID(ctx))})
}

func (c *Controller) DeleteProject(ctx contractshttp.Context) contractshttp.Response {
	pid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	if err := c.service.DeleteProject(ctx.Context(), pid); err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().NoContent()
}

type applicationRequest struct {
	Name string `json:"name"`
	// BuildPack omitted keeps the current one (dockerfile for a new
	// Application).
	BuildPack        string   `json:"build_pack"`
	ImageReference   string   `json:"image_reference"`
	PublishDirectory string   `json:"publish_directory"`
	GitURL           string   `json:"git_url"`
	GitBranch        string   `json:"git_branch"`
	DockerfilePath   string   `json:"dockerfile_path"`
	Port             int      `json:"port"`
	Domains          []string `json:"domains"`
	// HealthCheck omitted keeps the current one.
	HealthCheck *healthCheckJSON `json:"health_check"`
	// RegistryCredentials omitted keeps the current ones; an empty username
	// removes them; an empty password keeps the stored one.
	RegistryCredentials *registryCredentialsJSON `json:"registry_credentials"`
	// Storages omitted keeps the current ones.
	Storages *[]storageJSON `json:"storages"`
	// ResourceLimits omitted keeps the current ones.
	ResourceLimits *resourceLimitsJSON `json:"resource_limits"`
	// ServerID is the Target server; only read when the Application is
	// created. Omitted or 0 is the Local server.
	ServerID uint64 `json:"server_id"`
}

type registryCredentialsJSON struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (r applicationRequest) input() domain.ApplicationInput {
	in := domain.ApplicationInput{
		Name: r.Name, GitURL: r.GitURL, GitBranch: r.GitBranch,
		BuildPack: domain.BuildPack(r.BuildPack), ImageReference: r.ImageReference, PublishDirectory: r.PublishDirectory,
		DockerfilePath: r.DockerfilePath, Port: r.Port, Domains: r.Domains, ServerID: r.ServerID,
	}
	if r.HealthCheck != nil {
		h := r.HealthCheck.domain()
		in.HealthCheck = &h
	}
	if r.ResourceLimits != nil {
		l := r.ResourceLimits.domain()
		in.ResourceLimits = &l
	}
	if r.Storages != nil {
		storages := make([]domain.Storage, len(*r.Storages))
		for i, s := range *r.Storages {
			storages[i] = domain.Storage(s)
		}
		in.Storages = &storages
	}
	if r.RegistryCredentials != nil {
		in.RegistryCredentials = &domain.RegistryCredentials{Username: r.RegistryCredentials.Username, Password: r.RegistryCredentials.Password}
	}
	return in
}

func (c *Controller) CreateApplication(ctx contractshttp.Context) contractshttp.Response {
	envID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req applicationRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	a, err := c.service.CreateApplication(ctx.Context(), envID, req.input())
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusCreated, contractshttp.Json{"application": applicationToJSON(a, c.localID(ctx))})
}

func (c *Controller) ShowApplication(ctx contractshttp.Context) contractshttp.Response {
	aid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	a, err := c.service.Application(ctx.Context(), aid)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"application": applicationToJSON(a, c.localID(ctx))})
}

func (c *Controller) UpdateApplication(ctx contractshttp.Context) contractshttp.Response {
	aid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req applicationRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	a, err := c.service.UpdateApplication(ctx.Context(), aid, req.input())
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"application": applicationToJSON(a, c.localID(ctx))})
}

// RegenerateDeployKey answers with the Application and its new public key.
func (c *Controller) RegenerateDeployKey(ctx contractshttp.Context) contractshttp.Response {
	aid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	a, err := c.service.RegenerateDeployKey(ctx.Context(), aid)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"application": applicationToJSON(a, c.localID(ctx))})
}

func (c *Controller) DeleteApplication(ctx contractshttp.Context) contractshttp.Response {
	aid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	if err := c.service.DeleteApplication(ctx.Context(), aid); err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().NoContent()
}

type envVarJSON struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Build   bool   `json:"build"`
	Runtime bool   `json:"runtime"`
}

// envVarInput leaves the scope out to mean runtime only.
type envVarInput struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Build   *bool  `json:"build"`
	Runtime *bool  `json:"runtime"`
}

type envRequest struct {
	Env []envVarInput `json:"env"`
}

func (r envRequest) vars() []domain.EnvVar {
	vars := make([]domain.EnvVar, len(r.Env))
	for i, v := range r.Env {
		vars[i] = domain.EnvVar{Name: v.Name, Value: v.Value, Runtime: true}
		if v.Build != nil {
			vars[i].Build = *v.Build
		}
		if v.Runtime != nil {
			vars[i].Runtime = *v.Runtime
		}
	}
	return vars
}

func envToJSON(vars []domain.EnvVar) []envVarJSON {
	out := make([]envVarJSON, len(vars))
	for i, v := range vars {
		out[i] = envVarJSON{Name: v.Name, Value: v.Value, Build: v.Build, Runtime: v.Runtime}
	}
	return out
}

type inheritedJSON struct {
	envVarJSON
	From       string `json:"from"`
	Overridden bool   `json:"overridden"`
}

// ShowEnv answers with the Application's own variables and the Shared ones
// it inherits.
func (c *Controller) ShowEnv(ctx contractshttp.Context) contractshttp.Response {
	aid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	own, inherited, err := c.service.Variables(ctx.Context(), aid)
	if err != nil {
		return fail(ctx, err)
	}
	in := make([]inheritedJSON, len(inherited))
	for i, v := range inherited {
		in[i] = inheritedJSON{envVarJSON: envToJSON([]domain.EnvVar{v.EnvVar})[0], From: v.From, Overridden: v.Overridden}
	}
	return ctx.Response().Success().Json(contractshttp.Json{"env": envToJSON(own), "inherited": in})
}

// replaceVariables binds the whole set and hands it to replace.
func replaceVariables(ctx contractshttp.Context, replace func(ctx context.Context, owner uint64, vars []domain.EnvVar) error) contractshttp.Response {
	oid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req envRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	vars := req.vars()
	if err := replace(ctx.Context(), oid, vars); err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"env": envToJSON(vars)})
}

func showVariables(ctx contractshttp.Context, read func(ctx context.Context, owner uint64) ([]domain.EnvVar, error)) contractshttp.Response {
	oid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	vars, err := read(ctx.Context(), oid)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"env": envToJSON(vars)})
}

func (c *Controller) ReplaceEnv(ctx contractshttp.Context) contractshttp.Response {
	return replaceVariables(ctx, c.service.ReplaceEnvVars)
}

func (c *Controller) ShowProjectVariables(ctx contractshttp.Context) contractshttp.Response {
	return showVariables(ctx, c.service.ProjectVariables)
}

func (c *Controller) ReplaceProjectVariables(ctx contractshttp.Context) contractshttp.Response {
	return replaceVariables(ctx, c.service.ReplaceProjectVariables)
}

func (c *Controller) ShowEnvironmentVariables(ctx contractshttp.Context) contractshttp.Response {
	return showVariables(ctx, c.service.EnvironmentVariables)
}

func (c *Controller) ReplaceEnvironmentVariables(ctx contractshttp.Context) contractshttp.Response {
	return replaceVariables(ctx, c.service.ReplaceEnvironmentVariables)
}
