package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260929000004CreateDeploymentsTables holds Deployments and their logs.
// No foreign key to applications: deployments learns of deletions through
// the ApplicationDeleted event.
type M20260929000004CreateDeploymentsTables struct{}

func (r *M20260929000004CreateDeploymentsTables) Signature() string {
	return "20260929000004_create_deployments_tables"
}

func (r *M20260929000004CreateDeploymentsTables) Up() error {
	s := facades.Schema()
	if err := s.Create("deployments", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("application_id")
		t.String("status")
		t.String("commit_sha").Default("")
		t.String("image").Default("")
		t.String("container_name").Default("")
		t.Text("error").Default("")
		t.DateTimeTz("started_at").Nullable()
		t.DateTimeTz("finished_at").Nullable()
		t.TimestampsTz()
		t.Index("application_id", "id")
		t.Index("status", "id")
	}); err != nil {
		return err
	}
	// One active Deployment per Application, enforced by the database so two
	// Deploy clicks cannot both queue. Through Schema, so it runs inside the
	// migration's transaction where the table already exists.
	if err := s.Sql(`CREATE UNIQUE INDEX deployments_one_active
		ON deployments (application_id)
		WHERE status IN ('queued', 'cloning', 'building', 'starting')`); err != nil {
		return err
	}
	return s.Create("deployment_logs", func(t schema.Blueprint) {
		t.BigIncrements("id")
		t.UnsignedBigInteger("deployment_id")
		t.String("stream", 8)
		t.Text("line")
		t.DateTimeTz("created_at").UseCurrent()
		t.Foreign("deployment_id").References("id").On("deployments").CascadeOnDelete()
		t.Index("deployment_id", "id")
	})
}

func (r *M20260929000004CreateDeploymentsTables) Down() error {
	for _, table := range []string{"deployment_logs", "deployments"} {
		if err := facades.Schema().DropIfExists(table); err != nil {
			return err
		}
	}
	return nil
}
