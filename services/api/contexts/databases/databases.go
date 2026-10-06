// Package databases is what the router and the boot code may use from the
// databases context: its routes, the log stream, Recover and the
// BackupFinished event. Nothing else in contexts/databases is for outside use.
package databases

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/databases/app"
	"github.com/jevido/bakery/services/api/contexts/databases/domain"
	databaseshttp "github.com/jevido/bakery/services/api/contexts/databases/http"
	"github.com/jevido/bakery/services/api/contexts/databases/infra"
	"github.com/jevido/bakery/services/api/contexts/guilds"
	"github.com/jevido/bakery/services/api/contexts/projects"
	"github.com/jevido/bakery/services/api/contexts/servers"
)

var (
	once    sync.Once
	service *app.Service
	runtime infra.Runtime
	// shutdown closes when the context Recover got ends, ending log
	// streams so a stopping API does not wait on open tabs.
	shutdown = make(chan struct{})
)

// environments translates projects' Environment into this context's.
func environments(ctx context.Context, id uint64) (app.Environment, error) {
	e, err := projects.Environment(ctx, id)
	if errors.Is(err, projects.ErrNotFound) {
		return app.Environment{}, app.ErrNotFound
	}
	return app.Environment{ID: e.ID, ProjectID: e.ProjectID, GuildID: e.GuildID}, err
}

// publicHost is the host Public URLs name.
func publicHost() string {
	cfg := facades.Config()
	for _, h := range []string{cfg.GetString("bakery.databases.public_host"), cfg.GetString("bakery.dashboard.domain")} {
		if h != "" {
			return h
		}
	}
	return "localhost"
}

func svc() *app.Service {
	once.Do(func() {
		cfg := facades.Config()
		runtime = infra.Runtime{
			Podman:     podman.Default(),
			Network:    cfg.GetString("bakery.network"),
			PublicBind: cfg.GetString("bakery.databases.public_bind"),
		}
		service = app.NewService(infra.Store{}, runtime, environments, publicHost())
		service.Log = facades.Log().Errorf
		service.Files = infra.BackupFiles{Dir: cfg.GetString("bakery.backups.dir")}
		service.S3 = func(st domain.S3Storage) app.S3Client { return infra.S3{Storage: st} }
		service.BackupExecutionFinished = publishBackupExecutionFinished
		projects.OnProjectDeleting(service.InUse)
		projects.OnEnvironmentDeleting(service.InUseInEnvironment)
		servers.OnContainerOwner("database", belongs(service.EnvironmentOfDatabase))
	})
	return service
}

// inGuild answers 404 for a route whose {id} Environment (through the id
// lookup) is outside the Current guild.
func inGuild(name string, environmentOf func(ctx context.Context, id uint64) (uint64, error)) contractshttp.Middleware {
	return guilds.Owns(name, belongs(environmentOf))
}

// belongs reports whether the Environment of the id (through the lookup)
// is in the Guild.
func belongs(environmentOf func(ctx context.Context, id uint64) (uint64, error)) func(ctx context.Context, id, guildID uint64) (bool, error) {
	return func(ctx context.Context, id, guildID uint64) (bool, error) {
		envID, err := environmentOf(ctx, id)
		if errors.Is(err, app.ErrNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return projects.EnvironmentInGuild(ctx, envID, guildID)
	}
}

var (
	environmentInGuild = guilds.Owns("environment", projects.EnvironmentInGuild)
	projectInGuild     = guilds.Owns("project", projects.ProjectInGuild)
	databaseInGuild    = inGuild("database", func(ctx context.Context, id uint64) (uint64, error) {
		return svc().EnvironmentOfDatabase(ctx, id)
	})
	scheduledBackupInGuild = inGuild("scheduled-backup", func(ctx context.Context, id uint64) (uint64, error) {
		return svc().EnvironmentOfScheduledBackup(ctx, id)
	})
	s3StorageInGuild = guilds.Owns("s3-storage", func(ctx context.Context, id, guildID uint64) (bool, error) {
		return svc().S3StorageInGuild(ctx, id, guildID)
	})
	backupExecutionInGuild = inGuild("backup-execution", func(ctx context.Context, id uint64) (uint64, error) {
		return svc().EnvironmentOfBackupExecution(ctx, id)
	})
)

// Routes registers the databases API behind guilds.Auth, every route keyed
// by an Environment, Project, Database, Scheduled backup or Backup execution
// answering 404 outside the Current guild; listing S3 storages also needs
// guilds.Secrets, changing them guilds.Admin.
func Routes(r route.Router) {
	c := databaseshttp.NewController(svc(), guilds.Current)
	r.Middleware(guilds.Auth, environmentInGuild).Post("/api/environments/{id}/databases", c.Create)
	r.Middleware(guilds.Auth, projectInGuild).Get("/api/projects/{id}/databases", c.ForProject)
	r.Middleware(guilds.Auth, databaseInGuild).Group(func(r route.Router) {
		r.Get("/api/databases/{id}", c.Show)
		r.Patch("/api/databases/{id}", c.Update)
		r.Delete("/api/databases/{id}", c.Delete)
		r.Get("/api/databases/{id}/backup-executions", c.BackupExecutions)
		r.Post("/api/databases/{id}/backup-executions", c.BackUp)
		r.Get("/api/databases/{id}/scheduled-backups", c.ScheduledBackups)
		r.Post("/api/databases/{id}/scheduled-backups", c.CreateScheduledBackup)
	})
	r.Middleware(guilds.Auth, scheduledBackupInGuild).Group(func(r route.Router) {
		r.Get("/api/scheduled-backups/{id}", c.ShowScheduledBackup)
		r.Patch("/api/scheduled-backups/{id}", c.UpdateScheduledBackup)
		r.Delete("/api/scheduled-backups/{id}", c.DeleteScheduledBackup)
		r.Get("/api/scheduled-backups/{id}/backup-executions", c.ScheduledBackupExecutions)
		r.Post("/api/scheduled-backups/{id}/backup-executions", c.BackUpScheduledBackup)
	})
	r.Middleware(guilds.Auth, backupExecutionInGuild).Group(func(r route.Router) {
		r.Post("/api/backup-executions/{id}/restore", c.Restore)
		r.Delete("/api/backup-executions/{id}", c.DeleteBackupExecution)
	})
	// Coolify's deploy actions: an API token needs deploy for them.
	r.Middleware(guilds.Deploy, databaseInGuild).Group(func(r route.Router) {
		r.Post("/api/databases/{id}/start", c.Start)
		r.Post("/api/databases/{id}/stop", c.Stop)
		r.Post("/api/databases/{id}/restart", c.Restart)
	})
	// Members pick an S3 storage for a Scheduled backup, so they may list
	// them (no secret keys are shown); only admins change them.
	r.Middleware(guilds.Auth, guilds.Secrets).Group(func(r route.Router) {
		r.Get("/api/s3-storages", c.S3Storages)
	})
	r.Middleware(guilds.Auth, s3StorageInGuild, guilds.Admin).Group(func(r route.Router) {
		r.Post("/api/s3-storages", c.CreateS3Storage)
		r.Post("/api/s3-storages/check", c.CheckS3Storage)
		r.Patch("/api/s3-storages/{id}", c.UpdateS3Storage)
		r.Delete("/api/s3-storages/{id}", c.DeleteS3Storage)
	})
}

// StreamRoutes registers the log stream and Backup execution downloads, behind
// guilds.Auth (404 outside the Current guild) but outside the request
// timeout. A Backup execution holds the
// Database's data, so its download is a Secret.
func StreamRoutes(r route.Router) {
	c := databaseshttp.NewStreamController(svc(), shutdown)
	r.Middleware(guilds.Auth, databaseInGuild).Get("/api/databases/{id}/logs", c.Logs)
	r.Middleware(guilds.Auth, guilds.Secrets, backupExecutionInGuild).Get("/api/backup-executions/{id}/download", c.Download)
}

// Recover starts, in the background, every Database that should run and has
// no Container, retrying while Podman or the database is unreachable, and
// then the scheduler of Backup executions.
func Recover(ctx context.Context) {
	s := svc()
	go func() {
		<-ctx.Done()
		close(shutdown)
	}()

	go func() {
		for attempt := 1; ; attempt++ {
			err := runtime.Podman.EnsureNetwork(ctx, runtime.Network)
			if err == nil {
				err = s.Recover(ctx)
			}
			if err == nil {
				// Only now: Recover marks Backup executions left running as failed,
				// which must not hit one the scheduler just started.
				go schedule(ctx, s)
				return
			}
			facades.Log().Errorf("databases: recovering (attempt %d): %v", attempt, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(min(time.Duration(attempt)*2*time.Second, 30*time.Second)):
			}
		}
	}()
}

// schedule runs the backup scheduler at the start of every minute until
// ctx ends.
func schedule(ctx context.Context, s *app.Service) {
	for {
		now := time.Now()
		select {
		case <-ctx.Done():
			return
		case <-time.After(now.Truncate(time.Minute).Add(time.Minute).Sub(now)):
		}
		if err := s.Tick(ctx, time.Now()); err != nil && ctx.Err() == nil {
			facades.Log().Errorf("databases: backup scheduler: %v", err)
		}
	}
}

// BackupExecutionFinished is a Backup execution that ended; one Recover fails after a restart
// is not announced.
type BackupExecutionFinished struct {
	BackupExecutionID uint64
	// GuildID is the Guild the Database belongs to; 0 when its Environment
	// is gone.
	GuildID      uint64
	DatabaseID   uint64
	DatabaseName string
	Type         string
	Succeeded    bool
	// Reason is why it failed.
	Reason string
	// Trigger is "manual" or "scheduled".
	Trigger   string
	SizeBytes int64
	// OffSite says the Backup execution was also uploaded to its S3 storage.
	OffSite    bool
	FinishedAt time.Time
}

var (
	backupMu                  sync.Mutex
	onBackupExecutionFinished []func(ctx context.Context, e BackupExecutionFinished)
)

// OnBackupExecutionFinished registers f to hear of every BackupFinished. It runs in
// its own goroutine, so it can neither hold up nor break a Backup execution.
func OnBackupExecutionFinished(f func(ctx context.Context, e BackupExecutionFinished)) {
	backupMu.Lock()
	defer backupMu.Unlock()
	onBackupExecutionFinished = append(onBackupExecutionFinished, f)
}

func publishBackupExecutionFinished(ctx context.Context, d domain.Database, b domain.BackupExecution) {
	env, err := environments(ctx, d.EnvironmentID)
	if err != nil && !errors.Is(err, app.ErrNotFound) {
		facades.Log().Errorf("databases: the guild of database %d: %v", d.ID, err)
	}
	e := BackupExecutionFinished{
		BackupExecutionID: b.ID, GuildID: env.GuildID, DatabaseID: d.ID, DatabaseName: d.Name, Type: string(d.Type),
		Succeeded: b.Status == domain.ExecutionSucceeded, Reason: b.Error, Trigger: string(b.Trigger),
		SizeBytes: b.SizeBytes, OffSite: b.S3, FinishedAt: b.FinishedAt,
	}
	backupMu.Lock()
	subscribers := slices.Clone(onBackupExecutionFinished)
	backupMu.Unlock()
	for _, f := range subscribers {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					facades.Log().Errorf("databases: a BackupFinished subscriber panicked: %v", r)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			f(ctx, e)
		}()
	}
}
