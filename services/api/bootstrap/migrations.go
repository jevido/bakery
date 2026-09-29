package bootstrap

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/database/migrations"
)

func Migrations() []schema.Migration {
	return []schema.Migration{
		&migrations.M20210101000001CreateJobsTable{},
		&migrations.M20260929000001CreateUsersTable{},
		&migrations.M20260929000002CreateProjectsTables{},
		&migrations.M20260929000003CreateRoutesTable{},
		&migrations.M20260929000004CreateDeploymentsTables{},
		&migrations.M20260930000001AddDeploymentSourceDetails{},
	}
}
