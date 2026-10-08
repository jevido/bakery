// The Bakery's Desktop app: a Wails 3 window, or with `serve` the same
// frontend and Go methods over local HTTP for a headless browser. See README.md.
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jevido/bakery/apps/desktop/mcp"
	"github.com/jevido/bakery/apps/desktop/runner"
	"github.com/jevido/bakery/apps/desktop/store"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// The built frontend (frontend/dist) is embedded into the binary.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	args := os.Args[1:]
	var err error
	switch {
	case len(args) == 0:
		err = runWindow()
	case args[0] == "serve":
		err = runServe(args[1:])
	case args[0] == "login":
		err = runLogin(args[1:])
	case args[0] == "runner":
		err = runRunner()
	case args[0] == "mcp":
		err = runMCP()
	case args[0] == "version":
		fmt.Println(version)
	default:
		err = fmt.Errorf("unknown command %q; commands: serve [--addr 127.0.0.1:4991] [--no-runner], runner, mcp, login --server <address> [--no-browser], version", args[0])
	}
	if err != nil {
		log.Fatal(err)
	}
}

// openStore opens bakeries.json where store.DefaultPath puts it.
func openStore() (*store.Store, error) {
	path, err := store.DefaultPath()
	if err != nil {
		return nil, err
	}
	return store.New(path), nil
}

// newRunner is the Runner for the Bakeries in bakeries, with its Runs'
// working directories beside bakeries.json and its updates sent to events
// (nil in `runner`, which has no frontend).
func newRunner(bakeries *store.Store, events *Events) *runner.Runner {
	r := &runner.Runner{Store: bakeries, Home: filepath.Dir(bakeries.Path)}
	if events != nil {
		r.Events = events.Emit
	}
	return r
}

// openInBrowser opens url in the system browser through Wails' browser
// helper, which needs no running app.
func openInBrowser(url string) error {
	return (&application.BrowserManager{}).OpenURL(url)
}

func runWindow() error {
	bakeries, err := openStore()
	if err != nil {
		return err
	}
	events := NewEvents()
	r := newRunner(bakeries, events)
	app := application.New(application.Options{
		Name:        "The Bakery",
		Description: "Runs your Bakery Agents on this computer",
		Services:    []application.Service{application.NewService(NewDesktop(events, bakeries, openInBrowser, r))},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac:         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	events.SetWindow(func(name string, data any) { app.Event.Emit(name, data) })
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	r.Start(ctx)
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "The Bakery",
		Width:  1280,
		Height: 800,
		// Paperclip's dark background, so the window does not flash white.
		BackgroundColour: application.NewRGB(9, 9, 11),
		URL:              "/",
	})
	return app.Run()
}

func runServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := flags.String("addr", "127.0.0.1:4991", "loopback address to listen on")
	noRunner := flags.Bool("no-runner", false, "do not run this person's Runs")
	if err := flags.Parse(args); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return err
	}
	// serve runs the person's Desktop as them: never reachable from another machine.
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("serve listens on a loopback address only, not %q", host)
	}
	bakeries, err := openStore()
	if err != nil {
		return err
	}
	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		return err
	}
	// net.Listen fails when the port is taken, instead of moving to another.
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	log.Printf("The Bakery desktop app serving on http://%s", ln.Addr())
	events := NewEvents()
	var r *runner.Runner
	if !*noRunner {
		r = newRunner(bakeries, events)
		r.Start(context.Background())
	}
	err = http.Serve(ln, newServer(NewDesktop(events, bakeries, nil, r), dist))
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// runRunner runs only the Runner, until SIGINT or SIGTERM. The claude
// processes it started are stopped and their Runs left running on the
// Bakery, which marks them lost and queues them again.
func runRunner() error {
	bakeries, err := openStore()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	r := newRunner(bakeries, nil)
	log.Printf("The Bakery desktop app running Runs for the Bakeries in %s", bakeries.Path)
	r.Start(ctx)
	<-ctx.Done()
	r.Wait()
	log.Print("runner stopped")
	return nil
}

// runMCP serves The Bakery's MCP server on stdio for the `claude` of one
// Run, configured from the environment the Runner gives it.
func runMCP() error {
	cfg, err := mcp.ConfigFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return mcp.Run(ctx, cfg, version)
}
