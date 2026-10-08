// Package deployments is what the router and the boot code may use from the
// deployments context: its routes, the Worker and the DeploymentFinished
// event, and for the work context OpenPullRequest, the Pull request and
// Preview hooks and PreviewURL. Nothing else in contexts/deployments is for
// outside use.
package deployments

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/deployments/app"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
	deploymentshttp "github.com/jevido/bakery/services/api/contexts/deployments/http"
	"github.com/jevido/bakery/services/api/contexts/deployments/infra"
	"github.com/jevido/bakery/services/api/contexts/guilds"
	"github.com/jevido/bakery/services/api/contexts/projects"
	"github.com/jevido/bakery/services/api/contexts/routing"
	"github.com/jevido/bakery/services/api/contexts/servers"
)

var (
	once     sync.Once
	service  *app.Service
	webhooks *app.Webhooks
	// runtime is the Runtime config every Server's Runtime starts from.
	runtime infra.Runtime
)

var (
	localMu sync.Mutex
	localID uint64
)

// localServer is the Local server's id, read once it can be.
func localServer() uint64 {
	localMu.Lock()
	defer localMu.Unlock()
	if localID == 0 {
		id, err := servers.LocalID(context.Background())
		if err != nil {
			facades.Log().Errorf("deployments: reading the local server: %v", err)
		}
		localID = id
	}
	return localID
}

// runtimeOn is the Runtime of a Server (0 the Local server), through
// servers' pooled Server connection.
func runtimeOn(ctx context.Context, serverID uint64) (infra.Runtime, error) {
	conn, err := servers.Connect(ctx, serverID)
	if err != nil {
		return infra.Runtime{}, err
	}
	r := runtime
	r.Server, r.Podman = conn.Name, conn.Podman
	return r, nil
}

func runtimes(ctx context.Context, serverID uint64) (app.Runtime, error) {
	return runtimeOn(ctx, serverID)
}

// applications translates projects' snapshot into this context's language.
func applications(ctx context.Context, id uint64) (app.Application, error) {
	s, err := projects.ApplicationForDeploy(ctx, id)
	if err != nil {
		return app.Application{}, err
	}
	storages := make([]app.Storage, len(s.Storages))
	for i, st := range s.Storages {
		storages[i] = app.Storage(st)
	}
	return app.Application{
		ID: s.ID, GuildID: s.GuildID, Slug: s.Slug, BuildPack: s.BuildPack, DockerImage: s.DockerImage, PublishDirectory: s.PublishDirectory,
		RegistryUsername: s.RegistryUsername, RegistryPassword: s.RegistryPassword,
		GitURL: s.GitURL, GitBranch: s.GitBranch,
		ServerID: s.ServerID, DockerfilePath: s.DockerfilePath, Port: s.Port, Domains: s.Domains, BuildVariables: s.BuildVariables, RuntimeVariables: s.RuntimeVariables, DeployKey: s.DeployKey,
		HealthCheck: app.HealthCheck(s.HealthCheck), Storages: storages,
		MemoryMB: s.MemoryMB, CPUs: s.CPUs,
	}, nil
}

func svc() *app.Service {
	once.Do(func() {
		deploymentshttp.LocalServer = localServer
		service = app.NewService(infra.Store{}, infra.Logs{}, applications, infra.KnownHosts{}, infra.Previews{})
		webhooks = app.NewWebhooks(service, infra.Webhooks{})
		webhooks.Hosts = infra.GitHosts{}
		webhooks.PullRequestChanged = publishPullRequest
		service.PreviewDeploying = func(_ context.Context, applicationID uint64, number int) {
			publish("Preview deploying", &onPreviewDeploying, PreviewEvent{ApplicationID: applicationID, Number: number})
		}
		service.PreviewRemoved = func(_ context.Context, applicationID uint64, number int) {
			publish("Preview removed", &onPreviewRemoved, PreviewEvent{ApplicationID: applicationID, Number: number})
		}
		service.DropPreviewRoute = routing.DropPreviewRoute
		service.StopRoute = routing.StopRoute
		service.Comments = app.NewCommenter(infra.Webhooks{}, infra.Previews{}, infra.GitHosts{})
		service.Comments.URL = publicURL
		deploymentshttp.PublicURL = publicURL
		service.Log = facades.Log().Errorf
		runtime = infra.Runtime{
			Network:            facades.Config().GetString("bakery.network"),
			StartTimeout:       30 * time.Second,
			Settle:             2 * time.Second,
			InsecureRegistries: splitList(facades.Config().GetString("bakery.insecure_registries")),
		}
		// Image retention during Cleanup of a Server: the Images of all but
		// the newest five finished Deployments there go, unless a Container
		// still uses one. It works once StartWorker ran.
		servers.OnCleanup(service.PruneImages)
		servers.OnServerResources("application", applicationsOn)
		projects.OnApplicationDeleted(func(ctx context.Context, e projects.ApplicationDeleted) {
			applicationID := e.ApplicationID
			// Every Server its Deployments ran on; a Server that cannot be
			// reached keeps what is there, which its Cleanup leaves alone.
			ids, err := (infra.Store{}).ServerIDs(ctx, applicationID)
			if err != nil {
				facades.Log().Errorf("deployments: servers of application %d: %v", applicationID, err)
			}
			// The Images, read before the Deployments that name them go.
			var all []domain.Deployment
			if e.DeleteImages {
				if all, err = (infra.Store{}).ByApplication(ctx, applicationID, -1); err != nil {
					facades.Log().Errorf("deployments: images of application %d: %v", applicationID, err)
				}
			}
			for _, id := range ids {
				rt, err := runtimeOn(ctx, id)
				if err != nil {
					facades.Log().Errorf("deployments: removing application %d: %v", applicationID, err)
					continue
				}
				if err := rt.RemoveAll(ctx, applicationID); err != nil {
					facades.Log().Errorf("deployments: removing containers of application %d on %s: %v", applicationID, rt.Server, err)
					continue
				}
				if e.DeleteVolumes {
					if err := rt.RemoveVolumes(ctx, applicationID); err != nil {
						facades.Log().Errorf("deployments: removing volumes of application %d on %s: %v", applicationID, rt.Server, err)
					}
				}
				seen := map[string]bool{}
				for _, d := range all {
					if d.ServerID != id || d.Image == "" || seen[d.Image] {
						continue
					}
					seen[d.Image] = true
					if _, err := rt.RemoveImage(ctx, d.Image); err != nil {
						facades.Log().Errorf("deployments: removing image %s on %s: %v", d.Image, rt.Server, err)
					}
				}
			}
			if err := service.DeletePreviews(ctx, applicationID); err != nil {
				facades.Log().Errorf("deployments: deleting previews of application %d: %v", applicationID, err)
			}
			if err := (infra.Store{}).DeleteForApplication(ctx, applicationID); err != nil {
				facades.Log().Errorf("deployments: deleting deployments of application %d: %v", applicationID, err)
			}
			if err := webhooks.DeleteForApplication(ctx, applicationID); err != nil {
				facades.Log().Errorf("deployments: deleting the webhook of application %d: %v", applicationID, err)
			}
		})
	})
	return service
}

// publicURL is where a Domain is served: the Local server's Proxy on its
// configured HTTPS port, a Remote server's always on 443 (as projects
// shows an Application's).
func publicURL(d string, serverID uint64) string {
	port := facades.Config().GetInt("bakery.proxy.https_port", 443)
	if port == 443 || serverID != 0 {
		return "https://" + d
	}
	return "https://" + d + ":" + strconv.Itoa(port)
}

func isApplicationNotFound(err error) bool { return errors.Is(err, projects.ErrNotFound) }

// applicationsOn lists the Applications on a Server for its Resources list,
// each with its latest own Deployment's state as its status.
func applicationsOn(ctx context.Context, serverID uint64, in []servers.ResourceProject) ([]servers.ServerResource, error) {
	var envIDs []uint64
	for _, p := range in {
		for id := range p.Environments {
			envIDs = append(envIDs, id)
		}
	}
	list, err := projects.ApplicationsOnServer(ctx, serverID, envIDs)
	if err != nil {
		return nil, err
	}
	out := make([]servers.ServerResource, len(list))
	for i, a := range list {
		status, err := service.LatestStatus(ctx, a.ID)
		if err != nil {
			return nil, err
		}
		out[i] = servers.ServerResource{ID: a.ID, Name: a.Name, EnvironmentID: a.EnvironmentID, Status: status}
	}
	return out, nil
}

// Routes registers the deployments API behind guilds.Auth (Known hosts
// behind manage_servers, the Webhook with its secret behind see_secrets,
// changes behind manage_applications, deploy actions behind deploy), every
// route keyed by an Application or a Deployment
// answering 404 outside the Current guild, and the Webhook endpoint git
// hosts call without a Session (the signature is its authentication; it is
// found by its secret, in whatever Guild).
func Routes(r route.Router) {
	c := deploymentshttp.NewController(svc(), isApplicationNotFound, guilds.Current)
	wc := deploymentshttp.NewWebhookController(webhooks, isApplicationNotFound)
	r.Post("/api/webhooks/applications/{id}", wc.Receive)
	// The Webhook answers with its secret, even after a change.
	r.Middleware(guilds.Auth, applicationInProject, guilds.Can("see_secrets")).Get("/api/applications/{id}/webhook", wc.Show)
	// Known hosts were the admins' alone, reading them as well.
	r.Middleware(guilds.Auth, guilds.Can("manage_servers")).Group(func(r route.Router) {
		r.Get("/api/known-hosts", c.KnownHosts)
		r.Delete("/api/known-hosts/{id}", c.ForgetKnownHost)
	})
	r.Middleware(guilds.Auth, applicationInProject).Group(func(r route.Router) {
		r.Get("/api/applications/{id}/status", c.Status)
		r.Get("/api/applications/{id}/previews", c.Previews)
		r.Get("/api/applications/{id}/deployments", c.List)
		r.Get("/api/applications/{id}/images", c.Images)
	})
	r.Middleware(guilds.Auth, applicationInProject, guilds.Can("manage_applications")).Group(func(r route.Router) {
		r.Patch("/api/applications/{id}/webhook", wc.Update)
		r.Post("/api/applications/{id}/webhook/secret", wc.RotateSecret)
		r.Delete("/api/applications/{id}/previews/{number}", c.DeletePreview)
	})
	r.Middleware(guilds.Auth, deploymentInProject).Group(func(r route.Router) {
		r.Get("/api/deployments/{id}", c.Show)
	})
	// Coolify's deploy actions: an API token needs deploy for them.
	r.Middleware(guilds.Deploy, applicationInProject, guilds.Can("deploy")).Group(func(r route.Router) {
		r.Post("/api/applications/{id}/deploy", c.Deploy)
		r.Post("/api/applications/{id}/restart", c.Restart)
		r.Post("/api/applications/{id}/stop", c.Stop)
		r.Post("/api/applications/{id}/previews/{number}/deploy", c.DeployPreview)
	})
	r.Middleware(guilds.Deploy, deploymentInProject, guilds.Can("deploy")).Group(func(r route.Router) {
		r.Post("/api/deployments/{id}/cancel", c.Cancel)
		r.Post("/api/deployments/{id}/rollback", c.Rollback)
	})
}

// applicationInProject and deploymentInProject answer 404 for an {id}
// Application, or a Deployment of an Application, outside the Current
// guild or in a Project the request may not view.
var (
	applicationInProject = guilds.InProject("application", projects.ProjectOf("application"))
	deploymentInProject  = guilds.InProject("deployment", func(ctx context.Context, id uint64) (uint64, uint64, bool, error) {
		d, err := svc().Deployment(ctx, id)
		if errors.Is(err, app.ErrNotFound) {
			return 0, 0, false, nil
		}
		if err != nil {
			return 0, 0, false, err
		}
		return projects.ProjectOf("application")(ctx, d.ApplicationID)
	})
)

var shutdown = make(chan struct{})

// StreamRoutes registers the live log streams, behind guilds.Auth but
// outside the request timeout, each answering 404 outside the Current guild.
func StreamRoutes(r route.Router) {
	isNotFound := func(err error) bool { return errors.Is(err, projects.ErrNotFound) || errors.Is(err, app.ErrNotFound) }
	c := deploymentshttp.NewStreamController(svc(), followContainer, isNotFound, shutdown)
	r.Middleware(guilds.Auth, deploymentInProject).Get("/api/deployments/{id}/log", c.DeploymentLog)
	r.Middleware(guilds.Auth, applicationInProject).Get("/api/applications/{id}/logs", c.ContainerLogs)
}

// followContainer reads the Application's running Container on its Target
// server, each line prefixed with its time, as Coolify's runtime logs are.
func followContainer(ctx context.Context, applicationID uint64, follow bool, tail int, out func(stream, line string)) (bool, error) {
	a, err := projects.ApplicationForDeploy(ctx, applicationID)
	if err != nil {
		return false, err
	}
	rt, err := runtimeOn(ctx, a.ServerID)
	if err != nil {
		return false, err
	}
	name, found, err := rt.Running(ctx, applicationID)
	if err != nil || !found {
		return false, err
	}
	return true, rt.Podman.TimestampedLogs(ctx, name, follow, tail, func(stream, line string) {
		if stream == "stderr" {
			out("err", line)
		} else {
			out("out", line)
		}
	})
}

// StartWorker fails Deployments a previous process left half done, then
// runs the Worker until ctx ends.
func StartWorker(ctx context.Context) {
	workDir := filepath.Join(os.TempDir(), "bakery-builds")
	w := app.NewWorker(svc(), infra.Git{KnownHosts: infra.KnownHosts{}}, runtimes, routing.SwitchRoute, workDir)
	w.PreviewRouter = routing.SwitchPreviewRoute
	w.Planner = infra.Nixpacks{Binary: facades.Config().GetString("bakery.nixpacks")}
	w.Log = facades.Log().Errorf
	w.Finished = publishFinished
	go func() {
		<-ctx.Done()
		close(shutdown)
	}()
	go func() {
		for attempt := 1; ; attempt++ {
			if err := w.Recover(ctx); err == nil {
				break
			} else {
				facades.Log().Errorf("deployments: recovering (attempt %d): %v", attempt, err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
		}
		w.Run(ctx)
	}()
}

// splitList splits a comma-separated setting, dropping empty items.
func splitList(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// The reasons OpenPullRequest cannot open a Pull request.
var (
	ErrNoGitHostToken  = app.ErrNoGitHostToken
	ErrUnknownGitHost  = domain.ErrUnknownGitHost
	ErrBranchNotPushed = app.ErrBranchNotPushed
	ErrNoRepository    = app.ErrNoRepository
)

// OpenedPullRequest is a Pull request on the Application's git host:
// its Provider ("github", "gitea", "forgejo" or "gitlab"), number (GitLab:
// iid), web link and title. Created is false when it was already open.
type OpenedPullRequest struct {
	Provider string
	Number   int
	URL      string
	Title    string
	Created  bool
}

// OpenPullRequest opens a Pull request from the head branch into the
// Application's branch, through its git host's REST API with the
// Application's Git host token, or answers the one already open from that
// branch. The caller has checked who may.
func OpenPullRequest(ctx context.Context, applicationID uint64, head, title, body string) (OpenedPullRequest, error) {
	svc()
	pr, err := webhooks.OpenPullRequest(ctx, applicationID, head, title, body)
	if err != nil {
		return OpenedPullRequest{}, err
	}
	return OpenedPullRequest{Provider: string(pr.Provider), Number: pr.Number, URL: pr.URL, Title: pr.Title, Created: pr.Created}, nil
}

// DeploymentFinished is a Deployment that ended succeeded or failed; a
// cancelled one, or one failed because a restart interrupted it, is not
// announced.
type DeploymentFinished struct {
	DeploymentID uint64
	// GuildID is the Guild the Application belongs to; 0 when it could not
	// be read.
	GuildID         uint64
	ApplicationID   uint64
	ApplicationSlug string
	// Preview is the Preview number of a Preview Deployment, else 0.
	Preview   int
	Succeeded bool
	// Reason is why it failed.
	Reason        string
	Branch        string
	CommitSHA     string
	CommitMessage string
	// Trigger is "manual", "webhook", "rollback" or "restart".
	Trigger    string
	Rollback   bool
	FinishedAt time.Time
}

var (
	hooksMu            sync.Mutex
	onFinished         []func(ctx context.Context, e DeploymentFinished)
	onPullRequest      []func(ctx context.Context, e PullRequestEvent)
	onPreviewDeploying []func(ctx context.Context, e PreviewEvent)
	onPreviewRemoved   []func(ctx context.Context, e PreviewEvent)
)

// OnDeploymentFinished registers f to hear of every DeploymentFinished. It
// runs in its own goroutine, so it can neither hold up nor break the
// Worker.
func OnDeploymentFinished(f func(ctx context.Context, e DeploymentFinished)) {
	register(&onFinished, f)
}

func register[E any](list *[]func(ctx context.Context, e E), f func(ctx context.Context, e E)) {
	hooksMu.Lock()
	defer hooksMu.Unlock()
	*list = append(*list, f)
}

// publish runs every subscriber of the list in its own goroutine, so none
// can hold up or break what announced e.
func publish[E any](name string, list *[]func(ctx context.Context, e E), e E) {
	hooksMu.Lock()
	subscribers := slices.Clone(*list)
	hooksMu.Unlock()
	for _, f := range subscribers {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					facades.Log().Errorf("deployments: a %s subscriber panicked: %v", name, r)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			f(ctx, e)
		}()
	}
}

func publishFinished(_ context.Context, d domain.Deployment, a app.Application) {
	e := DeploymentFinished{
		DeploymentID: d.ID, GuildID: a.GuildID, ApplicationID: d.ApplicationID, ApplicationSlug: a.Slug, Preview: d.Preview,
		Succeeded: d.Status == domain.Finished, Reason: d.Error, Branch: d.Branch,
		CommitSHA: d.CommitSHA, CommitMessage: d.CommitMessage, Trigger: string(d.Trigger),
		Rollback: d.Trigger == domain.TriggerRollback, FinishedAt: time.Now(),
	}
	if d.FinishedAt != nil {
		e.FinishedAt = *d.FinishedAt
	}
	publish("DeploymentFinished", &onFinished, e)
}

// PullRequestEvent is a Pull request of an Application's repository opened
// (or reopened), pushed to or closed, as a verified Webhook call told it,
// whatever the Previews switch and the base branch.
type PullRequestEvent struct {
	GuildID       uint64
	ApplicationID uint64
	// Provider is the git host's wire key, e.g. "forgejo".
	Provider string
	// Number is the Pull request's number (GitLab: its iid).
	Number int
	// Action is "opened", "pushed" or "closed".
	Action string
	// Merged is true for one closed by merging it.
	Merged bool
	// Branch is its head branch.
	Branch string
	URL    string
	Title  string
}

// OnPullRequest registers f to hear of every PullRequestEvent, in its own
// goroutine.
func OnPullRequest(f func(ctx context.Context, e PullRequestEvent)) {
	register(&onPullRequest, f)
}

func publishPullRequest(_ context.Context, a app.Application, p domain.Provider, pr domain.PullRequest) {
	publish("PullRequestEvent", &onPullRequest, PullRequestEvent{
		GuildID: a.GuildID, ApplicationID: a.ID, Provider: string(p), Number: pr.Number, Action: string(pr.Action),
		Merged: pr.Merged, Branch: pr.Branch, URL: pr.URL, Title: pr.Title,
	})
}

// PreviewEvent is something that happened to the Preview of an
// Application's Pull request number.
type PreviewEvent struct {
	ApplicationID uint64
	Number        int
}

// OnPreviewDeploying registers f to hear of every Preview Deployment
// queued; DeploymentFinished, with its Preview number, tells how it ended.
// f runs in its own goroutine.
func OnPreviewDeploying(f func(ctx context.Context, e PreviewEvent)) {
	register(&onPreviewDeploying, f)
}

// OnPreviewRemoved registers f to hear of every closed Preview whose
// Containers and route were removed, in its own goroutine.
func OnPreviewRemoved(f func(ctx context.Context, e PreviewEvent)) {
	register(&onPreviewRemoved, f)
}

// PreviewURL is where the Application's Preview number is served, as the
// Previews page shows it; found is false for an unknown Preview or an
// Application without a Domain.
func PreviewURL(ctx context.Context, applicationID uint64, number int) (url string, found bool, err error) {
	views, err := svc().Previews(ctx, applicationID)
	if err != nil {
		return "", false, err
	}
	for _, p := range views {
		if p.Number != number || p.Domain == "" {
			continue
		}
		var server uint64
		if p.Latest != nil {
			server = p.Latest.ServerID
		}
		return publicURL(p.Domain, server), true, nil
	}
	return "", false, nil
}
