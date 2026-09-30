package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000030AddUserRole gives every Member a Role (identity). The one
// account that existed before is the Owner; the partial index keeps it the
// only one.
type M20260930000030AddUserRole struct{}

func (r *M20260930000030AddUserRole) Signature() string {
	return "20260930000030_add_user_role"
}

func (r *M20260930000030AddUserRole) Up() error {
	for _, stmt := range []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS role varchar(16) NOT NULL DEFAULT 'member'`,
		`UPDATE users SET role = 'owner' WHERE id = (SELECT min(id) FROM users)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS users_one_owner ON users (role) WHERE role = 'owner'`,
	} {
		if err := facades.Schema().Sql(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (r *M20260930000030AddUserRole) Down() error {
	for _, stmt := range []string{
		`DROP INDEX IF EXISTS users_one_owner`,
		`ALTER TABLE users DROP COLUMN IF EXISTS role`,
	} {
		if err := facades.Schema().Sql(stmt); err != nil {
			return err
		}
	}
	return nil
}
