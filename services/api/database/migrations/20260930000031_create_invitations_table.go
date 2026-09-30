package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000031CreateInvitationsTable holds the Invitations (identity):
// only a hash of each link's token, and at most one open Invitation per
// email.
type M20260930000031CreateInvitationsTable struct{}

func (r *M20260930000031CreateInvitationsTable) Signature() string {
	return "20260930000031_create_invitations_table"
}

func (r *M20260930000031CreateInvitationsTable) Up() error {
	if !facades.Schema().HasTable("invitations") {
		err := facades.Schema().Create("invitations", func(t schema.Blueprint) {
			t.ID()
			t.String("email")
			t.String("role", 16)
			t.String("token_hash", 64)
			t.UnsignedBigInteger("invited_by").Nullable()
			t.TimestampTz("expires_at")
			t.TimestampTz("accepted_at").Nullable()
			t.TimestampTz("revoked_at").Nullable()
			t.TimestampsTz()
			t.Unique("token_hash")
			t.Foreign("invited_by").References("id").On("users").NullOnDelete()
		})
		if err != nil {
			return err
		}
	}
	return facades.Schema().Sql(`CREATE UNIQUE INDEX IF NOT EXISTS invitations_one_open ON invitations (email)
		WHERE accepted_at IS NULL AND revoked_at IS NULL`)
}

func (r *M20260930000031CreateInvitationsTable) Down() error {
	return facades.Schema().DropIfExists("invitations")
}
