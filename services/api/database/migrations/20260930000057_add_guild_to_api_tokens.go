package migrations

// M20260930000057AddGuildToAPITokens binds every API token (identity) to the
// Guild it was made in; the tokens from before Guilds belong to the first
// one. A name is unique per Member and Guild.
type M20260930000057AddGuildToAPITokens struct{}

func (r *M20260930000057AddGuildToAPITokens) Signature() string {
	return "20260930000057_add_guild_to_api_tokens"
}

func (r *M20260930000057AddGuildToAPITokens) Up() error {
	return sqls(
		`ALTER TABLE api_tokens ADD COLUMN guild_id bigint REFERENCES guilds (id) ON DELETE CASCADE`,
		`UPDATE api_tokens SET guild_id = (SELECT min(id) FROM guilds)`,
		// Only possible without any Guild, which means without Members.
		`DELETE FROM api_tokens WHERE guild_id IS NULL`,
		`ALTER TABLE api_tokens ALTER COLUMN guild_id SET NOT NULL`,
		`ALTER TABLE api_tokens DROP CONSTRAINT IF EXISTS api_tokens_user_id_name_unique`,
		`ALTER TABLE api_tokens ADD CONSTRAINT api_tokens_user_id_guild_id_name_unique UNIQUE (user_id, guild_id, name)`,
		`CREATE INDEX api_tokens_guild_id_index ON api_tokens (guild_id)`,
	)
}

// Down keeps one token per Member and name, the oldest, so the old
// constraint fits again.
func (r *M20260930000057AddGuildToAPITokens) Down() error {
	return sqls(
		`DELETE FROM api_tokens a USING api_tokens b WHERE a.user_id = b.user_id AND a.name = b.name AND a.id > b.id`,
		`ALTER TABLE api_tokens DROP CONSTRAINT IF EXISTS api_tokens_user_id_guild_id_name_unique`,
		`ALTER TABLE api_tokens ADD CONSTRAINT api_tokens_user_id_name_unique UNIQUE (user_id, name)`,
		`ALTER TABLE api_tokens DROP COLUMN IF EXISTS guild_id`,
	)
}
