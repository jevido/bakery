// The Bakery's Desktop app: a Wails 3 window, or with `serve` the same
// frontend and Go methods over local HTTP for a headless browser. See README.md.
package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"

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
	case args[0] == "version":
		fmt.Println(version)
	default:
		err = fmt.Errorf("unknown command %q; commands: serve [--addr 127.0.0.1:4991], version", args[0])
	}
	if err != nil {
		log.Fatal(err)
	}
}

func runWindow() error {
	events := NewEvents()
	app := application.New(application.Options{
		Name:        "The Bakery",
		Description: "Runs your Bakery Agents on this computer",
		Services:    []application.Service{application.NewService(NewDesktop(events))},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac:         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	events.SetWindow(func(name string, data any) { app.Event.Emit(name, data) })
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
	err = http.Serve(ln, newServer(NewDesktop(NewEvents()), dist))
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
