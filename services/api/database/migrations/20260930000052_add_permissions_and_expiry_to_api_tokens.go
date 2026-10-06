package migrations

// M20260930000052AddPermissionsAndExpiryToAPITokens gives API tokens
// Coolify's permissions and an expiry in place of the read-only flag. A
// read-only token becomes read, every other one root; both stay capped by
// their Member's current Role, so each token does exactly what it did. The
// description grows to Coolify's 255 characters.
type M20260930000052AddPermissionsAndExpiryToAPITokens struct{}

func (r *M20260930000052AddPermissionsAndExpiryToAPITokens) Signature() string {
	return "20260930000052_add_permissions_and_expiry_to_api_tokens"
}

func (r *M20260930000052AddPermissionsAndExpiryToAPITokens) Up() error {
	return sqls(
		`ALTER TABLE api_tokens ADD COLUMN permissions text NOT NULL DEFAULT 'read'`,
		`ALTER TABLE api_tokens ADD COLUMN expires_at timestamptz NULL`,
		`UPDATE api_tokens SET permissions = CASE WHEN read_only THEN 'read' ELSE 'root' END`,
		`ALTER TABLE api_tokens DROP COLUMN read_only`,
		`ALTER TABLE api_tokens ALTER COLUMN name TYPE varchar(255)`,
	)
}

// Down keeps a token read-only only when it was exactly read; a name longer
// than the old 64 characters is cut short.
func (r *M20260930000052AddPermissionsAndExpiryToAPITokens) Down() error {
	return sqls(
		`ALTER TABLE api_tokens ADD COLUMN read_only boolean NOT NULL DEFAULT false`,
		`UPDATE api_tokens SET read_only = (permissions = 'read')`,
		`ALTER TABLE api_tokens DROP COLUMN permissions`,
		`ALTER TABLE api_tokens DROP COLUMN expires_at`,
		`ALTER TABLE api_tokens ALTER COLUMN name TYPE varchar(64) USING left(name, 64)`,
	)
}
