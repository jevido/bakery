package bootstrap

import (
	contractsfoundation "github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/foundation"

	"github.com/jevido/bakery/services/api/config"
	"github.com/jevido/bakery/services/api/contexts/guilds"
	"github.com/jevido/bakery/services/api/contexts/identity"
	"github.com/jevido/bakery/services/api/routes"
)

func Boot() contractsfoundation.Application {
	guilds.Boot()
	return foundation.Setup().
		WithMigrations(Migrations).
		WithCommands(identity.Commands).
		WithRouting(func() {
			routes.Api()
			routes.Grpc()
		}).
		WithProviders(Providers).
		WithConfig(config.Boot).
		Create()
}
