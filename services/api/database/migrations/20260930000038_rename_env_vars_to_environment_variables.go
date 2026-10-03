package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000038RenameEnvVarsToEnvironmentVariables gives the variable
// tables Coolify's names. The Shared variable tables move aside first,
// because environment_variables held the Environment's Shared variables
// and is the name an Application's own Environment variables take now.
// Postgres keeps rows, indexes, constraints and sequences across a rename.
type M20260930000038RenameEnvVarsToEnvironmentVariables struct{}

func (r *M20260930000038RenameEnvVarsToEnvironmentVariables) Signature() string {
	return "20260930000038_rename_env_vars_to_environment_variables"
}

func (r *M20260930000038RenameEnvVarsToEnvironmentVariables) Up() error {
	return renameTables([][2]string{
		{"environment_variables", "environment_shared_variables"},
		{"project_variables", "project_shared_variables"},
		{"env_vars", "environment_variables"},
	})
}

func (r *M20260930000038RenameEnvVarsToEnvironmentVariables) Down() error {
	return renameTables([][2]string{
		{"environment_variables", "env_vars"},
		{"project_shared_variables", "project_variables"},
		{"environment_shared_variables", "environment_variables"},
	})
}

func renameTables(renames [][2]string) error {
	for _, r := range renames {
		if err := facades.Schema().Rename(r[0], r[1]); err != nil {
			return err
		}
	}
	return nil
}
