package migrations

// M20260930000096AddWebhookProvider remembers the Provider that last called
// a Webhook with a verified call, so The Bakery knows which REST API opens
// Pull requests for the Application. NULL until the first such call.
type M20260930000096AddWebhookProvider struct{}

func (r *M20260930000096AddWebhookProvider) Signature() string {
	return "20260930000096_add_webhook_provider"
}

func (r *M20260930000096AddWebhookProvider) Up() error {
	return sqls(`ALTER TABLE webhooks ADD COLUMN provider varchar(16) NULL`)
}

func (r *M20260930000096AddWebhookProvider) Down() error {
	return sqls(`ALTER TABLE webhooks DROP COLUMN IF EXISTS provider`)
}
