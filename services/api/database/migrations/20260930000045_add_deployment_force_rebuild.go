package migrations

// M20260930000045AddDeploymentForceRebuild records a Deployment that builds
// without the layer cache (Coolify's force_rebuild), so the Worker that
// claims it later knows.
type M20260930000045AddDeploymentForceRebuild struct{}

func (r *M20260930000045AddDeploymentForceRebuild) Signature() string {
	return "20260930000045_add_deployment_force_rebuild"
}

func (r *M20260930000045AddDeploymentForceRebuild) Up() error {
	return sqls(`ALTER TABLE deployments ADD COLUMN force_rebuild boolean NOT NULL DEFAULT false`)
}

func (r *M20260930000045AddDeploymentForceRebuild) Down() error {
	return sqls(`ALTER TABLE deployments DROP COLUMN IF EXISTS force_rebuild`)
}
