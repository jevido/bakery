package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000036AddPreviews gives deployments Previews: a Deployment
// belongs to the Application itself (preview 0) or to one Preview, one
// queued per (Application, Preview); the Webhook gets the Previews switch
// and the Git host token.
type M20260930000036AddPreviews struct{}

func (r *M20260930000036AddPreviews) Signature() string {
	return "20260930000036_add_previews"
}

func (r *M20260930000036AddPreviews) Up() error {
	for _, stmt := range []string{
		`ALTER TABLE deployments ADD COLUMN IF NOT EXISTS preview integer NOT NULL DEFAULT 0`,
		`DROP INDEX IF EXISTS deployments_one_queued`,
		`CREATE UNIQUE INDEX deployments_one_queued ON deployments (application_id, preview) WHERE status = 'queued'`,
		`CREATE TABLE IF NOT EXISTS previews (
			id bigserial PRIMARY KEY,
			application_id bigint NOT NULL,
			number integer NOT NULL,
			branch varchar(255) NOT NULL DEFAULT '',
			title varchar(500) NOT NULL DEFAULT '',
			url varchar(1000) NOT NULL DEFAULT '',
			provider varchar(20) NOT NULL DEFAULT '',
			api varchar(1000) NOT NULL DEFAULT '',
			state varchar(10) NOT NULL DEFAULT 'open',
			comment_id varchar(100) NOT NULL DEFAULT '',
			closed_at timestamptz,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now(),
			UNIQUE (application_id, number)
		)`,
		`ALTER TABLE webhooks ADD COLUMN IF NOT EXISTS previews boolean NOT NULL DEFAULT false`,
		`ALTER TABLE webhooks ADD COLUMN IF NOT EXISTS git_host_token_encrypted text NOT NULL DEFAULT ''`,
	} {
		if err := facades.Schema().Sql(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (r *M20260930000036AddPreviews) Down() error {
	for _, stmt := range []string{
		`ALTER TABLE webhooks DROP COLUMN IF EXISTS git_host_token_encrypted`,
		`ALTER TABLE webhooks DROP COLUMN IF EXISTS previews`,
		`DROP TABLE IF EXISTS previews`,
		`DELETE FROM deployments WHERE preview <> 0`,
		`DROP INDEX IF EXISTS deployments_one_queued`,
		`CREATE UNIQUE INDEX deployments_one_queued ON deployments (application_id) WHERE status = 'queued'`,
		`ALTER TABLE deployments DROP COLUMN IF EXISTS preview`,
	} {
		if err := facades.Schema().Sql(stmt); err != nil {
			return err
		}
	}
	return nil
}
