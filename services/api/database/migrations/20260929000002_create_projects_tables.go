package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260929000002CreateProjectsTables holds the projects context: projects,
// environments, applications and their env vars.
type M20260929000002CreateProjectsTables struct{}

func (r *M20260929000002CreateProjectsTables) Signature() string {
	return "20260929000002_create_projects_tables"
}

func (r *M20260929000002CreateProjectsTables) Up() error {
	s := facades.Schema()
	if err := s.Create("projects", func(t schema.Blueprint) {
		t.ID()
		t.String("name")
		t.Text("description").Default("")
		t.TimestampsTz()
	}); err != nil {
		return err
	}
	if err := s.Create("environments", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("project_id")
		t.String("name")
		t.TimestampsTz()
		t.Foreign("project_id").References("id").On("projects").CascadeOnDelete()
		t.Unique("project_id", "name")
	}); err != nil {
		return err
	}
	if err := s.Create("applications", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("environment_id")
		t.String("name")
		t.String("slug")
		t.String("git_url")
		t.String("git_branch")
		t.String("dockerfile_path")
		t.Integer("port")
		t.String("domain")
		t.TimestampsTz()
		t.Foreign("environment_id").References("id").On("environments").RestrictOnDelete()
		t.Unique("slug")
		t.Unique("domain")
	}); err != nil {
		return err
	}
	return s.Create("env_vars", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("application_id")
		t.String("name")
		t.Text("value_encrypted")
		t.TimestampsTz()
		t.Foreign("application_id").References("id").On("applications").CascadeOnDelete()
		t.Unique("application_id", "name")
	})
}

func (r *M20260929000002CreateProjectsTables) Down() error {
	for _, table := range []string{"env_vars", "applications", "environments", "projects"} {
		if err := facades.Schema().DropIfExists(table); err != nil {
			return err
		}
	}
	return nil
}
