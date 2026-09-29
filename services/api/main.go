package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jevido/bakery/services/api/bootstrap"
)

func main() {
	app := bootstrap.Boot()

	// `go run . artisan ...` runs a command and exits; only the server
	// starts the runtime (proxy, deployment worker).
	if len(os.Args) < 2 || os.Args[1] != "artisan" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		bootstrap.StartRuntime(ctx)
	}

	app.Start()
}
