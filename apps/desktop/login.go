package main

import (
	"errors"
	"flag"
	"fmt"
)

// runLogin is Paperclip's `auth login` for the Desktop app: it starts a
// Desktop sign-in at --server, prints the approve link (and opens it unless
// --no-browser), waits until it is approved, expired or cancelled, and
// stores the Bakery as the window's Connect does. It is the way in without
// a window, for the headless runner.
func runLogin(args []string) error {
	flags := flag.NewFlagSet("login", flag.ContinueOnError)
	server := flags.String("server", "", "the Bakery's address, e.g. https://bakery.example.com")
	noBrowser := flags.Bool("no-browser", false, "print the approve link without opening a browser")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *server == "" {
		return errors.New("login needs --server <address>")
	}
	bakeries, err := openStore()
	if err != nil {
		return err
	}
	events := NewEvents()
	done, stop := events.Subscribe()
	defer stop()
	open := openInBrowser
	if *noBrowser {
		open = nil
	}
	d := NewDesktop(events, bakeries, open)
	start, err := d.Connect(*server)
	if err != nil {
		return err
	}
	fmt.Printf("Approve The Bakery desktop app in your browser:\n\n  %s\n\nWaiting until %s…\n", start.ApprovalURL, start.ExpiresAt.Local().Format("15:04:05"))
	for e := range done {
		state, ok := e.Data.(ConnectState)
		if e.Name != "connect" || !ok || state.ID != start.ID {
			continue
		}
		switch state.Status {
		case connectApproved:
			fmt.Printf("Connected to %s. The key is in %s.\n", state.Address, bakeries.Path)
			return nil
		case connectFailed:
			return errors.New(state.Error)
		default:
			return fmt.Errorf("the sign-in was %s; run login again", state.Status)
		}
	}
	return nil
}
