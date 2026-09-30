package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000035AddTwoFactor gives Members Two-factor authentication and
// Sessions valid from (identity). A secret without two_factor_enabled_at is
// pending: made, never confirmed with a code.
type M20260930000035AddTwoFactor struct{}

func (r *M20260930000035AddTwoFactor) Signature() string {
	return "20260930000035_add_two_factor"
}

func (r *M20260930000035AddTwoFactor) Up() error {
	for _, stmt := range []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS two_factor_secret_encrypted text`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS two_factor_enabled_at timestamptz`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS two_factor_last_step bigint NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS sessions_valid_from timestamptz`,
		`CREATE TABLE IF NOT EXISTS recovery_codes (
			id bigserial PRIMARY KEY,
			user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
			code_hash varchar(64) NOT NULL UNIQUE,
			created_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS recovery_codes_user_id ON recovery_codes (user_id)`,
	} {
		if err := facades.Schema().Sql(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (r *M20260930000035AddTwoFactor) Down() error {
	for _, stmt := range []string{
		`DROP TABLE IF EXISTS recovery_codes`,
		`ALTER TABLE users DROP COLUMN IF EXISTS sessions_valid_from`,
		`ALTER TABLE users DROP COLUMN IF EXISTS two_factor_last_step`,
		`ALTER TABLE users DROP COLUMN IF EXISTS two_factor_enabled_at`,
		`ALTER TABLE users DROP COLUMN IF EXISTS two_factor_secret_encrypted`,
	} {
		if err := facades.Schema().Sql(stmt); err != nil {
			return err
		}
	}
	return nil
}
