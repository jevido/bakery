package migrations

// M20260930000070CreateGuildMasterOffersTable holds the Transfer offers of
// the Guild Master (guilds). At most one is open per Guild. An open one
// past expires_at counts as expired and is written so when next read.
type M20260930000070CreateGuildMasterOffersTable struct{}

func (r *M20260930000070CreateGuildMasterOffersTable) Signature() string {
	return "20260930000070_create_guild_master_offers_table"
}

func (r *M20260930000070CreateGuildMasterOffersTable) Up() error {
	return sqls(
		`CREATE TABLE guild_master_offers (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
			from_member_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
			to_member_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
			status varchar(16) NOT NULL DEFAULT 'open'
				CHECK (status IN ('open', 'accepted', 'declined', 'withdrawn', 'expired')),
			created_at timestamptz NOT NULL,
			expires_at timestamptz NOT NULL
		)`,
		`CREATE UNIQUE INDEX guild_master_offers_one_open ON guild_master_offers (guild_id) WHERE status = 'open'`,
		`CREATE INDEX guild_master_offers_to_member_id_index ON guild_master_offers (to_member_id)`,
	)
}

func (r *M20260930000070CreateGuildMasterOffersTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS guild_master_offers`)
}
