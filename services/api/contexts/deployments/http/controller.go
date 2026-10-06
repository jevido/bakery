// Package http exposes deployments over HTTP.
package http

import (
	"errors"
	"strconv"
	"strings"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/deployments/app"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type Controller struct {
	service *app.Service
	// isNotFound recognises projects' "no such application".
	isNotFound func(error) bool
	// guild is the Current guild of a request.
	guild func(ctx contractshttp.Context) uint64
}

func NewController(service *app.Service, isNotFound func(error) bool, guild func(ctx contractshttp.Context) uint64) *Controller {
	return &Controller{service: service, isNotFound: isNotFound, guild: guild}
}

// LocalServer returns the Local server's id, shown where a Deployment's
// Server is 0; nil or failing shows 0.
var LocalServer func() uint64

// PublicURL is where a Domain on a Server (0 the Local server) is reached.
var PublicURL func(domain string, serverID uint64) string

type deploymentJSON struct {
	ID            uint64     `json:"id"`
	ApplicationID uint64     `json:"application_id"`
	Preview       int        `json:"preview"`
	ServerID      uint64     `json:"server_id"`
	Status        string     `json:"status"`
	Active        bool       `json:"active"`
	Trigger       string     `json:"trigger"`
	Branch        string     `json:"branch"`
	CommitSHA     string     `json:"commit_sha"`
	CommitMessage string     `json:"commit_message"`
	CommitAuthor  string     `json:"commit_author"`
	SourceImage   string     `json:"source_image"`
	Image         string     `json:"image"`
	Container     string     `json:"container"`
	RollbackOf    *uint64    `json:"rollback_of"`
	ForceRebuild  bool       `json:"force_rebuild"`
	Error         string     `json:"error"`
	CreatedAt     time.Time  `json:"created_at"`
	StartedAt     *time.Time `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"`
}

func ToJSON(d domain.Deployment) deploymentJSON {
	server := d.ServerID
	if server == 0 && LocalServer != nil {
		server = LocalServer()
	}
	return deploymentJSON{
		ID: d.ID, ApplicationID: d.ApplicationID, Preview: d.Preview, ServerID: server, Status: string(d.Status), Active: d.Status.Active(),
		Trigger: string(d.Trigger), Branch: d.Branch, CommitSHA: d.CommitSHA,
		CommitMessage: d.CommitMessage, CommitAuthor: d.CommitAuthor, SourceImage: d.SourceImage, Image: d.Image, Container: d.Container, RollbackOf: d.RollbackOf, ForceRebuild: d.ForceRebuild, Error: d.Error,
		CreatedAt: d.CreatedAt, StartedAt: d.StartedAt, FinishedAt: d.FinishedAt,
	}
}

func (c *Controller) fail(ctx contractshttp.Context, err error) contractshttp.Response {
	switch {
	case errors.Is(err, app.ErrNotFound), c.isNotFound(err):
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	case errors.Is(err, domain.ErrAlreadyQueued), errors.Is(err, app.ErrNotCancellable),
		errors.Is(err, domain.ErrNotRollbackTarget), errors.Is(err, app.ErrImageGone),
		errors.Is(err, domain.ErrPreviewRollback), errors.Is(err, domain.ErrPreviewClosed), errors.Is(err, app.ErrNoPreviews),
		errors.Is(err, domain.ErrNothingToRestart), errors.Is(err, app.ErrDeploymentInProgress):
		return respond.Error(ctx, contractshttp.StatusConflict, err.Error())
	}
	return respond.ServerError(ctx, err)
}

// RouteID reads the {id} route parameter.
func RouteID(ctx contractshttp.Context) (uint64, bool) {
	v, err := strconv.ParseUint(ctx.Request().Route("id"), 10, 64)
	return v, err == nil
}

func (c *Controller) Deploy(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	deploy := c.service.Deploy
	// force_rebuild is the field Coolify's dashboard sends, force its API's.
	if ctx.Request().InputBool("force_rebuild") || ctx.Request().InputBool("force") {
		deploy = c.service.DeployWithoutCache
	}
	d, err := deploy(ctx.Context(), id)
	if err != nil {
		return c.fail(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusCreated, contractshttp.Json{"deployment": ToJSON(d)})
}

// Restart queues a Deployment that starts the running Image again.
func (c *Controller) Restart(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	d, err := c.service.Restart(ctx.Context(), id)
	if err != nil {
		return c.fail(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusCreated, contractshttp.Json{"deployment": ToJSON(d)})
}

func statusJSON(s app.ApplicationStatus) contractshttp.Json {
	return contractshttp.Json{"status": string(s.Status), "container_present": s.ContainerPresent, "container": s.Container}
}

// Stop stops the Application and answers with its status after.
func (c *Controller) Stop(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	if err := c.service.Stop(ctx.Context(), id); err != nil {
		return c.fail(ctx, err)
	}
	s, err := c.service.Status(ctx.Context(), id)
	if err != nil {
		return c.fail(ctx, err)
	}
	return ctx.Response().Success().Json(statusJSON(s))
}

// Status answers with the Application's status from its Containers.
func (c *Controller) Status(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	s, err := c.service.Status(ctx.Context(), id)
	if err != nil {
		return c.fail(ctx, err)
	}
	return ctx.Response().Success().Json(statusJSON(s))
}

// List answers a page of the Application's Deployment history, newest
// first: `skip`, `take` (1–100, default 50), `sort=oldest`, `search`, and
// comma-separated `status` (finished, failed, in_progress, queued,
// cancelled), `source` (manual, pull-request, webhook, rollback, restart),
// `server` ids and a `pull_request` number. `count` is how many match in
// all; `filters=1` adds the values present to filter on.
func (c *Controller) List(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	r := ctx.Request()
	q := app.HistoryQuery{
		Skip:    max(0, r.InputInt("skip", 0)),
		Take:    min(100, max(1, r.InputInt("take", 50))),
		Oldest:  r.Input("sort") == "oldest",
		Search:  r.Input("search"),
		Preview: max(0, r.InputInt("pull_request", 0)),
	}
	for _, v := range list(r.Input("status")) {
		if v == "in_progress" {
			q.Statuses = append(q.Statuses, domain.Cloning, domain.Building, domain.Starting)
		} else {
			q.Statuses = append(q.Statuses, domain.Status(v))
		}
	}
	for _, v := range list(r.Input("source")) {
		q.Sources = append(q.Sources, app.HistorySource(v))
	}
	for _, v := range list(r.Input("server")) {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			continue
		}
		q.ServerIDs = append(q.ServerIDs, n)
		// Deployments on the Local server store 0.
		if LocalServer != nil && n == LocalServer() {
			q.ServerIDs = append(q.ServerIDs, 0)
		}
	}
	ds, total, err := c.service.History(ctx.Context(), id, q)
	if err != nil {
		return c.fail(ctx, err)
	}
	out := make([]deploymentJSON, len(ds))
	for i, d := range ds {
		out[i] = ToJSON(d)
	}
	body := contractshttp.Json{"deployments": out, "count": total}
	if r.InputBool("filters") {
		f, err := c.service.HistoryFacets(ctx.Context(), id)
		if err != nil {
			return c.fail(ctx, err)
		}
		body["filters"] = facetsJSON(f)
	}
	return ctx.Response().Success().Json(body)
}

func list(v string) []string {
	var out []string
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

type historyFiltersJSON struct {
	Statuses  []string `json:"statuses"`
	Sources   []string `json:"sources"`
	ServerIDs []uint64 `json:"server_ids"`
	Previews  []int    `json:"pull_requests"`
}

// facetsJSON names the statuses as the status filter takes them, with the
// running steps as one in_progress.
func facetsJSON(f app.HistoryFacets) historyFiltersJSON {
	out := historyFiltersJSON{Statuses: []string{}, Sources: []string{}, ServerIDs: []uint64{}, Previews: f.Previews}
	if out.Previews == nil {
		out.Previews = []int{}
	}
	seen := map[string]bool{}
	for _, s := range f.Statuses {
		v := string(s)
		if s.Active() && s != domain.Queued {
			v = "in_progress"
		}
		if !seen[v] {
			seen[v] = true
			out.Statuses = append(out.Statuses, v)
		}
	}
	for _, s := range f.Sources {
		out.Sources = append(out.Sources, string(s))
	}
	servers := map[uint64]bool{}
	for _, id := range f.ServerIDs {
		if id == 0 && LocalServer != nil {
			id = LocalServer()
		}
		if !servers[id] {
			servers[id] = true
			out.ServerIDs = append(out.ServerIDs, id)
		}
	}
	return out
}

// Images answers the Deployments whose Image a Rollback can start again,
// newest first, the current one marked.
func (c *Controller) Images(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	images, err := c.service.RetainedImages(ctx.Context(), id)
	if err != nil {
		return c.fail(ctx, err)
	}
	type imageJSON struct {
		Image      string         `json:"image"`
		Current    bool           `json:"current"`
		Deployment deploymentJSON `json:"deployment"`
	}
	out := make([]imageJSON, len(images))
	for i, img := range images {
		out[i] = imageJSON{Image: img.Image, Current: img.Current, Deployment: ToJSON(img.Deployment)}
	}
	return ctx.Response().Success().Json(contractshttp.Json{"images": out})
}

func (c *Controller) Show(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	d, err := c.service.Deployment(ctx.Context(), id)
	if err != nil {
		return c.fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"deployment": ToJSON(d)})
}

func (c *Controller) Rollback(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	d, err := c.service.Rollback(ctx.Context(), id)
	if err != nil {
		return c.fail(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusCreated, contractshttp.Json{"deployment": ToJSON(d)})
}

// Cancel answers 200 with a Deployment cancelled at once (it was queued),
// or 202 with a running one that the Worker is stopping.
func (c *Controller) Cancel(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	d, err := c.service.Cancel(ctx.Context(), id)
	if err != nil {
		return c.fail(ctx, err)
	}
	status := contractshttp.StatusOK
	if d.Status.Active() {
		status = contractshttp.StatusAccepted
	}
	return ctx.Response().Json(status, contractshttp.Json{"deployment": ToJSON(d)})
}

type knownHostJSON struct {
	ID           uint64    `json:"id"`
	Host         string    `json:"host"`
	Fingerprints []string  `json:"fingerprints"`
	CreatedAt    time.Time `json:"created_at"`
}

func (c *Controller) KnownHosts(ctx contractshttp.Context) contractshttp.Response {
	hosts, err := c.service.KnownHosts(ctx.Context(), c.guild(ctx))
	if err != nil {
		return c.fail(ctx, err)
	}
	out := make([]knownHostJSON, len(hosts))
	for i, h := range hosts {
		out[i] = knownHostJSON{ID: h.ID, Host: h.Host, Fingerprints: h.Fingerprints, CreatedAt: h.CreatedAt}
		if out[i].Fingerprints == nil {
			out[i].Fingerprints = []string{}
		}
	}
	return ctx.Response().Success().Json(contractshttp.Json{"known_hosts": out})
}

func (c *Controller) ForgetKnownHost(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	if err := c.service.ForgetKnownHost(ctx.Context(), c.guild(ctx), id); err != nil {
		return c.fail(ctx, err)
	}
	return ctx.Response().NoContent()
}
