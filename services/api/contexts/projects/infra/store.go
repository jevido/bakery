// Package infra stores the projects context with the Goravel ORM and
// encrypts env var values with Goravel's Crypt (APP_KEY).
package infra

import (
	"context"
	"errors"
	"strings"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/projects/app"
	"github.com/jevido/bakery/services/api/contexts/projects/domain"
)

type projectRecord struct {
	ID          uint64 `gorm:"primaryKey"`
	Name        string
	Description string
	orm.Timestamps
}

func (projectRecord) TableName() string { return "projects" }

type environmentRecord struct {
	ID        uint64 `gorm:"primaryKey"`
	ProjectID uint64
	Name      string
	orm.Timestamps
}

func (environmentRecord) TableName() string { return "environments" }

type applicationRecord struct {
	ID                        uint64 `gorm:"primaryKey"`
	EnvironmentID             uint64
	Name                      string
	Slug                      string
	BuildPack                 string
	ImageReference            string
	PublishDirectory          string
	GitURL                    string `gorm:"column:git_url"`
	GitBranch                 string
	DockerfilePath            string
	Port                      int
	Domain                    string
	DeployKeyPublic           string
	DeployKeyPrivateEncrypted string
	RegistryUsername          string
	RegistryPasswordEncrypted string
	HealthCheckEnabled        bool
	HealthCheckPath           string
	HealthCheckInterval       int
	HealthCheckTimeout        int
	HealthCheckRetries        int
	HealthCheckStartPeriod    int
	orm.Timestamps
}

func (applicationRecord) TableName() string { return "applications" }

// toDomain leaves the Deploy key's private half out; only Application(id)
// decrypts it.
func (r applicationRecord) toDomain(projectID uint64) domain.Application {
	return domain.Application{
		ID: r.ID, EnvironmentID: r.EnvironmentID, ProjectID: projectID, Name: r.Name, Slug: r.Slug,
		BuildPack: domain.BuildPack(r.BuildPack), ImageReference: r.ImageReference, PublishDirectory: r.PublishDirectory,
		GitURL: r.GitURL, GitBranch: r.GitBranch, DockerfilePath: r.DockerfilePath, Port: r.Port, Domain: r.Domain,
		DeployKey:           domain.DeployKey{Public: r.DeployKeyPublic},
		RegistryCredentials: domain.RegistryCredentials{Username: r.RegistryUsername},
		HealthCheck: domain.HealthCheck{
			Enabled: r.HealthCheckEnabled, Path: r.HealthCheckPath, Interval: r.HealthCheckInterval,
			Timeout: r.HealthCheckTimeout, Retries: r.HealthCheckRetries, StartPeriod: r.HealthCheckStartPeriod,
		},
	}
}

func healthCheckColumns(h domain.HealthCheck) map[string]any {
	return map[string]any{
		"health_check_enabled": h.Enabled, "health_check_path": h.Path, "health_check_interval": h.Interval,
		"health_check_timeout": h.Timeout, "health_check_retries": h.Retries, "health_check_start_period": h.StartPeriod,
	}
}

func encryptRegistryPassword(c domain.RegistryCredentials) (string, error) {
	if c.Password == "" {
		return "", nil
	}
	return facades.Crypt().EncryptString(c.Password)
}

func encryptPrivate(k domain.DeployKey) (string, error) {
	if k.Private == "" {
		return "", nil
	}
	return facades.Crypt().EncryptString(k.Private)
}

// variableRecord is a row of env_vars, environment_variables or
// project_variables; Owner is the application, environment or project id.
type variableRecord struct {
	ID             uint64 `gorm:"primaryKey"`
	Owner          uint64 `gorm:"-"`
	Name           string
	ValueEncrypted string
	Build          bool
	Runtime        bool
	orm.Timestamps
}

// variableTables maps a variable level to its table and owner column.
var variableTables = map[string][2]string{
	domain.FromApplication: {"env_vars", "application_id"},
	domain.FromEnvironment: {"environment_variables", "environment_id"},
	domain.FromProject:     {"project_variables", "project_id"},
}

type Store struct{}

func (Store) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func (s Store) CreateProject(ctx context.Context, p domain.Project) (domain.Project, error) {
	rec := projectRecord{Name: p.Name, Description: p.Description}
	env := environmentRecord{Name: domain.DefaultEnvironment}
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		if err := tx.Create(&rec); err != nil {
			return err
		}
		env.ProjectID = rec.ID
		return tx.Create(&env)
	})
	if err != nil {
		return domain.Project{}, err
	}
	p.ID = rec.ID
	p.Environments = []domain.Environment{{ID: env.ID, ProjectID: rec.ID, Name: env.Name}}
	return p, nil
}

func (s Store) Projects(ctx context.Context) ([]domain.Project, error) {
	var recs []projectRecord
	if err := s.query(ctx).OrderBy("name").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Project, len(recs))
	for i, r := range recs {
		out[i] = domain.Project{ID: r.ID, Name: r.Name, Description: r.Description}
	}
	return out, nil
}

func (s Store) Project(ctx context.Context, id uint64) (domain.Project, bool, error) {
	var rec projectRecord
	if found, err := first(s.query(ctx).Where("id", id), &rec); err != nil || !found {
		return domain.Project{}, found, err
	}
	p := domain.Project{ID: rec.ID, Name: rec.Name, Description: rec.Description}

	var envs []environmentRecord
	if err := s.query(ctx).Where("project_id", id).OrderBy("id").Find(&envs); err != nil {
		return domain.Project{}, false, err
	}
	envIDs := make([]any, len(envs))
	for i, e := range envs {
		envIDs[i] = e.ID
	}
	var apps []applicationRecord
	if len(envIDs) > 0 {
		if err := s.query(ctx).WhereIn("environment_id", envIDs).OrderBy("name").Find(&apps); err != nil {
			return domain.Project{}, false, err
		}
	}
	for _, e := range envs {
		env := domain.Environment{ID: e.ID, ProjectID: id, Name: e.Name, Applications: []domain.Application{}}
		for _, a := range apps {
			if a.EnvironmentID == e.ID {
				env.Applications = append(env.Applications, a.toDomain(id))
			}
		}
		p.Environments = append(p.Environments, env)
	}
	return p, true, nil
}

func (s Store) UpdateProject(ctx context.Context, p domain.Project) error {
	_, err := s.query(ctx).Model(&projectRecord{}).Where("id", p.ID).Update(map[string]any{"name": p.Name, "description": p.Description})
	return err
}

func (s Store) DeleteProject(ctx context.Context, id uint64) error {
	return facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		n, err := tx.Model(&applicationRecord{}).
			Where("environment_id IN (SELECT id FROM environments WHERE project_id = ?)", id).Count()
		if err != nil {
			return err
		}
		if n > 0 {
			return app.ErrProjectNotEmpty
		}
		_, err = tx.Where("id", id).Delete(&projectRecord{})
		return err
	})
}

func (s Store) Environment(ctx context.Context, id uint64) (domain.Environment, bool, error) {
	var rec environmentRecord
	found, err := first(s.query(ctx).Where("id", id), &rec)
	return domain.Environment{ID: rec.ID, ProjectID: rec.ProjectID, Name: rec.Name}, found, err
}

func (s Store) SlugTaken(ctx context.Context, slug string) (bool, error) {
	n, err := s.query(ctx).Model(&applicationRecord{}).Where("slug", slug).Count()
	return n > 0, err
}

func (s Store) DomainTaken(ctx context.Context, d string, exceptID uint64) (bool, error) {
	n, err := s.query(ctx).Model(&applicationRecord{}).Where("domain", d).Where("id <> ?", exceptID).Count()
	return n > 0, err
}

func (s Store) CreateApplication(ctx context.Context, a domain.Application) (domain.Application, error) {
	private, err := encryptPrivate(a.DeployKey)
	if err != nil {
		return domain.Application{}, err
	}
	password, err := encryptRegistryPassword(a.RegistryCredentials)
	if err != nil {
		return domain.Application{}, err
	}
	rec := applicationRecord{
		EnvironmentID: a.EnvironmentID, Name: a.Name, Slug: a.Slug, GitURL: a.GitURL, GitBranch: a.GitBranch,
		BuildPack: string(a.BuildPack), ImageReference: a.ImageReference, PublishDirectory: a.PublishDirectory,
		DockerfilePath: a.DockerfilePath, Port: a.Port, Domain: a.Domain,
		DeployKeyPublic: a.DeployKey.Public, DeployKeyPrivateEncrypted: private,
		RegistryUsername: a.RegistryCredentials.Username, RegistryPasswordEncrypted: password,
		HealthCheckEnabled: a.HealthCheck.Enabled, HealthCheckPath: a.HealthCheck.Path,
		HealthCheckInterval: a.HealthCheck.Interval, HealthCheckTimeout: a.HealthCheck.Timeout,
		HealthCheckRetries: a.HealthCheck.Retries, HealthCheckStartPeriod: a.HealthCheck.StartPeriod,
	}
	if err := s.query(ctx).Create(&rec); err != nil {
		return domain.Application{}, uniqueViolation(err)
	}
	return rec.toDomain(a.ProjectID), nil
}

func (s Store) Application(ctx context.Context, id uint64) (domain.Application, bool, error) {
	var rec applicationRecord
	if found, err := first(s.query(ctx).Where("id", id), &rec); err != nil || !found {
		return domain.Application{}, found, err
	}
	var env environmentRecord
	if _, err := first(s.query(ctx).Where("id", rec.EnvironmentID), &env); err != nil {
		return domain.Application{}, false, err
	}
	a := rec.toDomain(env.ProjectID)
	if rec.DeployKeyPrivateEncrypted != "" {
		private, err := facades.Crypt().DecryptString(rec.DeployKeyPrivateEncrypted)
		if err != nil {
			return domain.Application{}, false, errors.New("cannot decrypt the deploy key of " + rec.Slug + " (was APP_KEY changed?)")
		}
		a.DeployKey.Private = private
	}
	if rec.RegistryPasswordEncrypted != "" {
		password, err := facades.Crypt().DecryptString(rec.RegistryPasswordEncrypted)
		if err != nil {
			return domain.Application{}, false, errors.New("cannot decrypt the registry password of " + rec.Slug + " (was APP_KEY changed?)")
		}
		a.RegistryCredentials.Password = password
	}
	return a, true, nil
}

// UpdateApplication writes the Deploy key as given: the service passes the
// Application it read (private half included), so an unchanged key is
// rewritten, not lost.
func (s Store) UpdateApplication(ctx context.Context, a domain.Application) error {
	private, err := encryptPrivate(a.DeployKey)
	if err != nil {
		return err
	}
	password, err := encryptRegistryPassword(a.RegistryCredentials)
	if err != nil {
		return err
	}
	columns := map[string]any{
		"name": a.Name, "git_url": a.GitURL, "git_branch": a.GitBranch,
		"build_pack": string(a.BuildPack), "image_reference": a.ImageReference, "publish_directory": a.PublishDirectory,
		"dockerfile_path": a.DockerfilePath, "port": a.Port, "domain": a.Domain,
		"deploy_key_public": a.DeployKey.Public, "deploy_key_private_encrypted": private,
		"registry_username": a.RegistryCredentials.Username, "registry_password_encrypted": password,
	}
	for k, v := range healthCheckColumns(a.HealthCheck) {
		columns[k] = v
	}
	_, err = s.query(ctx).Model(&applicationRecord{}).Where("id", a.ID).Update(columns)
	return uniqueViolation(err)
}

func (s Store) DeleteApplication(ctx context.Context, id uint64) error {
	_, err := s.query(ctx).Where("id", id).Delete(&applicationRecord{})
	return err
}

func (s Store) Variables(ctx context.Context, level string, ownerID uint64) ([]domain.EnvVar, error) {
	t := variableTables[level]
	var recs []variableRecord
	if err := s.query(ctx).Table(t[0]).Where(t[1], ownerID).OrderBy("name").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.EnvVar, len(recs))
	for i, r := range recs {
		value, err := facades.Crypt().DecryptString(r.ValueEncrypted)
		if err != nil {
			return nil, errors.New("cannot decrypt variable " + r.Name + " (was APP_KEY changed?)")
		}
		out[i] = domain.EnvVar{Name: r.Name, Value: value, Build: r.Build, Runtime: r.Runtime}
	}
	return out, nil
}

func (s Store) ReplaceVariables(ctx context.Context, level string, ownerID uint64, vars []domain.EnvVar) error {
	t := variableTables[level]
	rows := make([]map[string]any, len(vars))
	now := time.Now()
	for i, v := range vars {
		enc, err := facades.Crypt().EncryptString(v.Value)
		if err != nil {
			return err
		}
		rows[i] = map[string]any{
			t[1]: ownerID, "name": v.Name, "value_encrypted": enc, "build": v.Build, "runtime": v.Runtime,
			"created_at": now, "updated_at": now,
		}
	}
	return facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		if _, err := tx.Table(t[0]).Where(t[1], ownerID).Delete(); err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Table(t[0]).Create(&rows)
	})
}

func first(q contractsorm.Query, dest any) (bool, error) {
	if err := q.FirstOrFail(dest); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// uniqueViolation turns a unique index hit (two requests racing past the
// service's own check) into the field error the service would have given.
func uniqueViolation(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if !strings.Contains(msg, "23505") && !strings.Contains(msg, "duplicate key") {
		return err
	}
	if strings.Contains(msg, "domain") {
		return &domain.FieldError{Field: "domain", Message: "domain is already used by another application"}
	}
	return &domain.FieldError{Field: "name", Message: "an application with this name was just created; try again"}
}
