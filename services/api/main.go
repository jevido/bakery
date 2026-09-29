package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/goravel/framework/database/migration"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/bootstrap"
)

func main() {
	app := bootstrap.Boot()

	// MIGRATE_ON_START (set by `task api:dev`) migrates before serving. This
	// calls the migrator directly because `artisan migrate` exits 0 even
	// when a migration fails, which would start the API on half a schema.
	if len(os.Args) == 1 && os.Getenv("MIGRATE_ON_START") == "true" {
		migrator := migration.NewMigrator(facades.Artisan(), facades.Schema(), facades.Config().GetString("database.migrations.table"))
		if err := migrator.Run(); err != nil {
			log.Fatalf("migrating before start: %v", err)
		}
	}

	// `go run . artisan ...` runs a command and exits; only the server
	// starts the runtime (proxy, deployment worker).
	if len(os.Args) < 2 || os.Args[1] != "artisan" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		bootstrap.StartRuntime(ctx)
	}

	app.Start()
}
