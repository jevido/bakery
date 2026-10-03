package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000044AddDescriptionToEnvironments lets people describe the
// Environments they now add themselves, and keeps their names unique
// within a Project regardless of case, as Coolify does. Every Project has
// only its production Environment so far, so the index holds.
type M20260930000044AddDescriptionToEnvironments struct{}

func (r *M20260930000044AddDescriptionToEnvironments) Signature() string {
	return "20260930000044_add_description_to_environments"
}

func (r *M20260930000044AddDescriptionToEnvironments) Up() error {
	return sqls(
		`ALTER TABLE environments ADD COLUMN description text NOT NULL DEFAULT ''`,
		`CREATE UNIQUE INDEX environments_project_id_lower_name_unique ON environments (project_id, lower(name))`,
	)
}

func (r *M20260930000044AddDescriptionToEnvironments) Down() error {
	return sqls(
		`DROP INDEX IF EXISTS environments_project_id_lower_name_unique`,
		`ALTER TABLE environments DROP COLUMN IF EXISTS description`,
	)
}

func sqls(statements ...string) error {
	for _, q := range statements {
		if err := facades.Schema().Sql(q); err != nil {
			return err
		}
	}
	return nil
}
