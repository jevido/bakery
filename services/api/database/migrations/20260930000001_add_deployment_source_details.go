package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000001AddDeploymentSourceDetails records what a Deployment built
// (branch, commit subject and author) and what started it.
type M20260930000001AddDeploymentSourceDetails struct{}

func (r *M20260930000001AddDeploymentSourceDetails) Signature() string {
	return "20260930000001_add_deployment_source_details"
}

func (r *M20260930000001AddDeploymentSourceDetails) Up() error {
	return facades.Schema().Table("deployments", func(t schema.Blueprint) {
		t.String("branch").Default("")
		t.Text("commit_message").Default("")
		t.String("commit_author").Default("")
		t.String("trigger").Default("manual")
	})
}

func (r *M20260930000001AddDeploymentSourceDetails) Down() error {
	return facades.Schema().DropColumns("deployments", []string{"branch", "commit_message", "commit_author", "trigger"})
}
