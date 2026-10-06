package migrations

// M20260930000063AddGuildToNotificationChannels makes every Notification channel belong to one Guild;
// those from before Guilds belong to the first one. The name is unique
// within a Guild.
type M20260930000063AddGuildToNotificationChannels struct{}

func (r *M20260930000063AddGuildToNotificationChannels) Signature() string {
	return "20260930000063_add_guild_to_notification_channels"
}

func (r *M20260930000063AddGuildToNotificationChannels) Up() error {
	return sqls(
		`ALTER TABLE notification_channels ADD COLUMN guild_id bigint REFERENCES guilds (id)`,
		`INSERT INTO guilds (name, description, created_at, updated_at)
		SELECT 'Default', '', now(), now()
		WHERE EXISTS (SELECT 1 FROM notification_channels) AND NOT EXISTS (SELECT 1 FROM guilds)`,
		`UPDATE notification_channels SET guild_id = (SELECT min(id) FROM guilds)`,
		`ALTER TABLE notification_channels ALTER COLUMN guild_id SET NOT NULL`,
		`ALTER TABLE notification_channels DROP CONSTRAINT notification_channels_name_unique`,
		`ALTER TABLE notification_channels ADD CONSTRAINT notification_channels_guild_id_name_unique UNIQUE (guild_id, name)`,
	)
}

func (r *M20260930000063AddGuildToNotificationChannels) Down() error {
	return sqls(
		`ALTER TABLE notification_channels DROP COLUMN IF EXISTS guild_id`,
		`ALTER TABLE notification_channels ADD CONSTRAINT notification_channels_name_unique UNIQUE (name)`,
	)
}
