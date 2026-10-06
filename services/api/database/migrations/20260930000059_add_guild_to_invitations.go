package migrations

// M20260930000059AddGuildToInvitations moves the Invitations to guilds: each
// invites into one Guild, those from before Guilds into the first one. An
// email has at most one open Invitation per Guild.
type M20260930000059AddGuildToInvitations struct{}

func (r *M20260930000059AddGuildToInvitations) Signature() string {
	return "20260930000059_add_guild_to_invitations"
}

func (r *M20260930000059AddGuildToInvitations) Up() error {
	return sqls(
		`ALTER TABLE invitations ADD COLUMN guild_id bigint REFERENCES guilds (id) ON DELETE CASCADE`,
		`UPDATE invitations SET guild_id = (SELECT min(id) FROM guilds)`,
		// Only possible without any Guild, which means without Members.
		`DELETE FROM invitations WHERE guild_id IS NULL`,
		`ALTER TABLE invitations ALTER COLUMN guild_id SET NOT NULL`,
		`DROP INDEX IF EXISTS invitations_one_open`,
		`CREATE UNIQUE INDEX invitations_one_open ON invitations (guild_id, email)
			WHERE accepted_at IS NULL AND revoked_at IS NULL`,
	)
}

// Down keeps, per email, only the newest open Invitation open, so the old
// index fits again.
func (r *M20260930000059AddGuildToInvitations) Down() error {
	return sqls(
		`UPDATE invitations a SET revoked_at = now() FROM invitations b
			WHERE a.email = b.email AND a.id < b.id
			AND a.accepted_at IS NULL AND a.revoked_at IS NULL
			AND b.accepted_at IS NULL AND b.revoked_at IS NULL`,
		`DROP INDEX IF EXISTS invitations_one_open`,
		`CREATE UNIQUE INDEX invitations_one_open ON invitations (email)
			WHERE accepted_at IS NULL AND revoked_at IS NULL`,
		`ALTER TABLE invitations DROP COLUMN IF EXISTS guild_id`,
	)
}
