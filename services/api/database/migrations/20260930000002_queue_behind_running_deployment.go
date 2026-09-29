package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000002QueueBehindRunningDeployment replaces "one active
// Deployment per Application" with one queued and one running, so a
// Deployment can wait behind the one being built.
type M20260930000002QueueBehindRunningDeployment struct{}

func (r *M20260930000002QueueBehindRunningDeployment) Signature() string {
	return "20260930000002_queue_behind_running_deployment"
}

func (r *M20260930000002QueueBehindRunningDeployment) Up() error {
	for _, stmt := range []string{
		`DROP INDEX IF EXISTS deployments_one_active`,
		`CREATE UNIQUE INDEX deployments_one_queued ON deployments (application_id) WHERE status = 'queued'`,
		`CREATE UNIQUE INDEX deployments_one_running ON deployments (application_id)
			WHERE status IN ('cloning', 'building', 'starting')`,
	} {
		if err := facades.Schema().Sql(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (r *M20260930000002QueueBehindRunningDeployment) Down() error {
	for _, stmt := range []string{
		`DROP INDEX IF EXISTS deployments_one_queued`,
		`DROP INDEX IF EXISTS deployments_one_running`,
		`CREATE UNIQUE INDEX deployments_one_active ON deployments (application_id)
			WHERE status IN ('queued', 'cloning', 'building', 'starting')`,
	} {
		if err := facades.Schema().Sql(stmt); err != nil {
			return err
		}
	}
	return nil
}
