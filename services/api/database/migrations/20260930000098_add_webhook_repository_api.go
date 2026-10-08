package migrations

// M20260930000098AddWebhookRepositoryAPI remembers the repository's REST
// base as the Webhook's last verified call named it, so a Pull request can
// be opened for an Application cloned over SSH from a git host whose web UI
// is not on 443. NULL until such a call.
type M20260930000098AddWebhookRepositoryAPI struct{}

func (r *M20260930000098AddWebhookRepositoryAPI) Signature() string {
	return "20260930000098_add_webhook_repository_api"
}

func (r *M20260930000098AddWebhookRepositoryAPI) Up() error {
	return sqls(`ALTER TABLE webhooks ADD COLUMN repository_api text NULL`)
}

func (r *M20260930000098AddWebhookRepositoryAPI) Down() error {
	return sqls(`ALTER TABLE webhooks DROP COLUMN IF EXISTS repository_api`)
}
