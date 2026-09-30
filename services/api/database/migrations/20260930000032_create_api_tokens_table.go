package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000032CreateAPITokensTable holds the API tokens (identity): only
// the SHA-256 of each, removed with their Member.
type M20260930000032CreateAPITokensTable struct{}

func (r *M20260930000032CreateAPITokensTable) Signature() string {
	return "20260930000032_create_api_tokens_table"
}

func (r *M20260930000032CreateAPITokensTable) Up() error {
	if facades.Schema().HasTable("api_tokens") {
		return nil
	}
	return facades.Schema().Create("api_tokens", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("user_id")
		t.String("name", 64)
		t.String("token_hash", 64)
		t.Boolean("read_only").Default(false)
		t.TimestampTz("last_used_at").Nullable()
		t.TimestampsTz()
		t.Unique("token_hash")
		t.Unique("user_id", "name")
		t.Foreign("user_id").References("id").On("users").CascadeOnDelete()
	})
}

func (r *M20260930000032CreateAPITokensTable) Down() error {
	return facades.Schema().DropIfExists("api_tokens")
}
