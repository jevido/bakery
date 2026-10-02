// Package deployments is what the router and the boot code may use from the
// deployments context: its routes, the Worker and the DeploymentFinished
// event. Nothing else in contexts/deployments is for outside use.
package deployments

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/deployments/app"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
	deploymentshttp "github.com/jevido/bakery/services/api/contexts/deployments/http"
	"github.com/jevido/bakery/services/api/contexts/deployments/infra"
	"github.com/jevido/bakery/services/api/contexts/identity"
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
		ID: s.ID, Slug: s.Slug, BuildPack: s.BuildPack, ImageReference: s.ImageReference, PublishDirectory: s.PublishDirectory,
		RegistryUsername: s.RegistryUsername, RegistryPassword: s.RegistryPassword,
		GitURL: s.GitURL, GitBranch: s.GitBranch,
		ServerID: s.ServerID, DockerfilePath: s.DockerfilePath, Port: s.Port, Domains: s.Domains, BuildEnv: s.BuildEnv, RuntimeEnv: s.RuntimeEnv, DeployKey: s.DeployKey,
		HealthCheck: app.HealthCheck(s.HealthCheck), Storages: storages,
		MemoryMB: s.MemoryMB, CPUs: s.CPUs,
	}, nil
}

func svc() *app.Service {
	once.Do(func() {
		deploymentshttp.LocalServer = localServer
		service = app.NewService(infra.Store{}, infra.Logs{}, applications, infra.KnownHosts{}, infra.Previews{})
		webhooks = app.NewWebhooks(service, infra.Webhooks{})
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
		projects.OnApplicationDeleted(func(ctx context.Context, applicationID uint64) {
			// Every Server its Deployments ran on; a Server that cannot be
			// reached keeps what is there, which its Cleanup leaves alone.
			ids, err := (infra.Store{}).ServerIDs(ctx, applicationID)
			if err != nil {
				facades.Log().Errorf("deployments: servers of application %d: %v", applicationID, err)
			}
			for _, id := range ids {
				rt, err := runtimeOn(ctx, id)
				if err != nil {
					facades.Log().Errorf("deployments: removing application %d: %v", applicationID, err)
					continue
				}
				if err := rt.RemoveAll(ctx, applicationID); err != nil {
					facades.Log().Errorf("deployments: removing containers of application %d on %s: %v", applicationID, rt.Server, err)
				} else if err := rt.RemoveVolumes(ctx, applicationID); err != nil {
					facades.Log().Errorf("deployments: removing volumes of application %d on %s: %v", applicationID, rt.Server, err)
				}
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

func isApplicationNotFound(err error) bool { return errors.Is(err, projects.ErrNotFound) }

// Routes registers the deployments API behind identity.Auth (Known hosts
// also behind identity.Admin, the Webhook with its secret behind
// identity.Secrets), and the
// Webhook endpoint git hosts call without a Session (the signature is its
// authentication).
func Routes(r route.Router) {
	c := deploymentshttp.NewController(svc(), isApplicationNotFound)
	wc := deploymentshttp.NewWebhookController(webhooks, isApplicationNotFound)
	r.Post("/api/webhooks/applications/{id}", wc.Receive)
	// The Webhook answers with its secret, even after a change.
	r.Middleware(identity.Auth, identity.Secrets).Group(func(r route.Router) {
		r.Get("/api/applications/{id}/webhook", wc.Show)
	})
	r.Middleware(identity.Auth, identity.Admin).Group(func(r route.Router) {
		r.Get("/api/known-hosts", c.KnownHosts)
		r.Delete("/api/known-hosts/{id}", c.ForgetKnownHost)
	})
	r.Middleware(identity.Auth).Group(func(r route.Router) {
		r.Patch("/api/applications/{id}/webhook", wc.Update)
		r.Post("/api/applications/{id}/webhook/secret", wc.RotateSecret)
		r.Post("/api/applications/{id}/deploy", c.Deploy)
		r.Get("/api/applications/{id}/previews", c.Previews)
		r.Post("/api/applications/{id}/previews/{number}/deploy", c.DeployPreview)
		r.Delete("/api/applications/{id}/previews/{number}", c.DeletePreview)
		r.Get("/api/applications/{id}/deployments", c.List)
		r.Get("/api/deployments/{id}", c.Show)
		r.Post("/api/deployments/{id}/cancel", c.Cancel)
		r.Post("/api/deployments/{id}/rollback", c.Rollback)
	})
}

var shutdown = make(chan struct{})

// StreamRoutes registers the live log streams, behind identity.Auth but
// outside the request timeout.
func StreamRoutes(r route.Router) {
	isNotFound := func(err error) bool { return errors.Is(err, projects.ErrNotFound) || errors.Is(err, app.ErrNotFound) }
	c := deploymentshttp.NewStreamController(svc(), followContainer, isNotFound, shutdown)
	r.Middleware(identity.Auth).Group(func(r route.Router) {
		r.Get("/api/deployments/{id}/log", c.DeploymentLog)
		r.Get("/api/applications/{id}/logs", c.ContainerLogs)
	})
}

// followContainer follows the Application's running Container on its
// Target server.
func followContainer(ctx context.Context, applicationID uint64, tail int, out func(stream, line string)) (bool, error) {
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
	return true, rt.Podman.Logs(ctx, name, true, tail, func(stream, line string) {
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

// DeploymentFinished is a Deployment that ended succeeded or failed; a
// cancelled one, or one failed because a restart interrupted it, is not
// announced.
type DeploymentFinished struct {
	DeploymentID    uint64
	ApplicationID   uint64
	ApplicationSlug string
	Succeeded       bool
	// Reason is why it failed.
	Reason        string
	Branch        string
	CommitSHA     string
	CommitMessage string
	// Trigger is "manual", "webhook" or "rollback".
	Trigger    string
	Rollback   bool
	FinishedAt time.Time
}

var (
	finishedMu sync.Mutex
	onFinished []func(ctx context.Context, e DeploymentFinished)
)

// OnDeploymentFinished registers f to hear of every DeploymentFinished. It
// runs in its own goroutine, so it can neither hold up nor break the
// Worker.
func OnDeploymentFinished(f func(ctx context.Context, e DeploymentFinished)) {
	finishedMu.Lock()
	defer finishedMu.Unlock()
	onFinished = append(onFinished, f)
}

func publishFinished(_ context.Context, d domain.Deployment, slug string) {
	e := DeploymentFinished{
		DeploymentID: d.ID, ApplicationID: d.ApplicationID, ApplicationSlug: slug,
		Succeeded: d.Status == domain.Finished, Reason: d.Error, Branch: d.Branch,
		CommitSHA: d.CommitSHA, CommitMessage: d.CommitMessage, Trigger: string(d.Trigger),
		Rollback: d.RollbackOf != nil, FinishedAt: time.Now(),
	}
	if d.FinishedAt != nil {
		e.FinishedAt = *d.FinishedAt
	}
	finishedMu.Lock()
	subscribers := slices.Clone(onFinished)
	finishedMu.Unlock()
	for _, f := range subscribers {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					facades.Log().Errorf("deployments: a DeploymentFinished subscriber panicked: %v", r)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			f(ctx, e)
		}()
	}
}
