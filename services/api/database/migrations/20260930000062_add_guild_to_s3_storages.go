package migrations

// M20260930000062AddGuildToS3Storages makes every S3 storage belong to one Guild;
// those from before Guilds belong to the first one. The name is unique
// within a Guild.
type M20260930000062AddGuildToS3Storages struct{}

func (r *M20260930000062AddGuildToS3Storages) Signature() string {
	return "20260930000062_add_guild_to_s3_storages"
}

func (r *M20260930000062AddGuildToS3Storages) Up() error {
	return sqls(
		`ALTER TABLE s3_storages ADD COLUMN guild_id bigint REFERENCES guilds (id)`,
		`INSERT INTO guilds (name, description, created_at, updated_at)
		SELECT 'Default', '', now(), now()
		WHERE EXISTS (SELECT 1 FROM s3_storages) AND NOT EXISTS (SELECT 1 FROM guilds)`,
		`UPDATE s3_storages SET guild_id = (SELECT min(id) FROM guilds)`,
		`ALTER TABLE s3_storages ALTER COLUMN guild_id SET NOT NULL`,
		`ALTER TABLE s3_storages DROP CONSTRAINT s3_storages_name_unique`,
		`ALTER TABLE s3_storages ADD CONSTRAINT s3_storages_guild_id_name_unique UNIQUE (guild_id, name)`,
	)
}

func (r *M20260930000062AddGuildToS3Storages) Down() error {
	return sqls(
		`ALTER TABLE s3_storages DROP COLUMN IF EXISTS guild_id`,
		`ALTER TABLE s3_storages ADD CONSTRAINT s3_storages_name_unique UNIQUE (name)`,
	)
}
