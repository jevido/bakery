package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

// method is one /rpc entry: it decodes its own arguments from the request's
// JSON array and answers a result to encode.
type method func(args []json.RawMessage) (any, error)

// methods lists every Desktop method `serve` exposes. Only these are
// callable over HTTP; a new Desktop method is added here and in
// frontend/src/lib/desktop.ts.
func (d *Desktop) methods() map[string]method {
	return map[string]method{
		"Version": func(args []json.RawMessage) (any, error) {
			if err := decodeArgs(args); err != nil {
				return nil, err
			}
			return d.Version(), nil
		},
		"Bakeries": func(args []json.RawMessage) (any, error) {
			if err := decodeArgs(args); err != nil {
				return nil, err
			}
			return d.Bakeries()
		},
		"Activate": func(args []json.RawMessage) (any, error) {
			var address string
			if err := decodeArgs(args, &address); err != nil {
				return nil, err
			}
			return nil, d.Activate(address)
		},
		"Connect": func(args []json.RawMessage) (any, error) {
			var address string
			if err := decodeArgs(args, &address); err != nil {
				return nil, err
			}
			return d.Connect(address)
		},
		"ConnectStatus": func(args []json.RawMessage) (any, error) {
			var id uint64
			if err := decodeArgs(args, &id); err != nil {
				return nil, err
			}
			return d.ConnectStatus(id)
		},
		"CancelConnect": func(args []json.RawMessage) (any, error) {
			var id uint64
			if err := decodeArgs(args, &id); err != nil {
				return nil, err
			}
			return nil, d.CancelConnect(id)
		},
		"Me": func(args []json.RawMessage) (any, error) {
			var address string
			if err := decodeArgs(args, &address); err != nil {
				return nil, err
			}
			return d.Me(address)
		},
		"Guilds": func(args []json.RawMessage) (any, error) {
			var address string
			if err := decodeArgs(args, &address); err != nil {
				return nil, err
			}
			return d.Guilds(address)
		},
		"Agents": func(args []json.RawMessage) (any, error) {
			var address, status string
			var guildID uint64
			if err := decodeArgs(args, &address, &guildID, &status); err != nil {
				return nil, err
			}
			return d.Agents(address, guildID, status)
		},
		"Agent": func(args []json.RawMessage) (any, error) {
			var address string
			var guildID, id uint64
			if err := decodeArgs(args, &address, &guildID, &id); err != nil {
				return nil, err
			}
			return d.Agent(address, guildID, id)
		},
		"Disconnect": func(args []json.RawMessage) (any, error) {
			var address string
			if err := decodeArgs(args, &address); err != nil {
				return nil, err
			}
			return nil, d.Disconnect(address)
		},
		"LocalRuns": func(args []json.RawMessage) (any, error) {
			if err := decodeArgs(args); err != nil {
				return nil, err
			}
			return d.LocalRuns(), nil
		},
		"Runs": func(args []json.RawMessage) (any, error) {
			var address string
			var guildID, id uint64
			if err := decodeArgs(args, &address, &guildID, &id); err != nil {
				return nil, err
			}
			return d.Runs(address, guildID, id)
		},
		"RunEvents": func(args []json.RawMessage) (any, error) {
			var address string
			var guildID, id uint64
			var after int64
			if err := decodeArgs(args, &address, &guildID, &id, &after); err != nil {
				return nil, err
			}
			return d.RunEvents(address, guildID, id, after)
		},
		"FollowRun": func(args []json.RawMessage) (any, error) {
			var address string
			var guildID, id uint64
			if err := decodeArgs(args, &address, &guildID, &id); err != nil {
				return nil, err
			}
			return nil, d.FollowRun(address, guildID, id)
		},
		"UnfollowRun": func(args []json.RawMessage) (any, error) {
			var address string
			var guildID, id uint64
			if err := decodeArgs(args, &address, &guildID, &id); err != nil {
				return nil, err
			}
			d.UnfollowRun(address, guildID, id)
			return nil, nil
		},
	}
}

// errBadArgs marks arguments that do not fit the method: a 400, not a 500.
var errBadArgs = errors.New("bad arguments")

// decodeArgs decodes args into targets, one each, and refuses any other count.
func decodeArgs(args []json.RawMessage, targets ...any) error {
	if len(args) != len(targets) {
		return fmt.Errorf("%w: want %d, got %d", errBadArgs, len(targets), len(args))
	}
	for i, t := range targets {
		if err := json.Unmarshal(args[i], t); err != nil {
			return fmt.Errorf("%w: argument %d: %v", errBadArgs, i, err)
		}
	}
	return nil
}

// pingEvery keeps an idle /rpc/events stream open through proxies and lets
// the frontend see it is still connected.
const pingEvery = 15 * time.Second

// newServer serves the built frontend at / and the Desktop's methods at
// POST /rpc/<Method> (a JSON array of arguments in; {"result": …} or
// {"error": "…"} out) and its live updates at GET /rpc/events (SSE).
func newServer(d *Desktop, assets fs.FS) http.Handler {
	methods := d.methods()
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(assets))
	mux.HandleFunc("GET /rpc/events", func(w http.ResponseWriter, r *http.Request) {
		streamEvents(w, r, d.events)
	})
	mux.HandleFunc("POST /rpc/{method}", func(w http.ResponseWriter, r *http.Request) {
		call, ok := methods[r.PathValue("method")]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown method"})
			return
		}
		var args []json.RawMessage
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&args); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the body must be a JSON array of arguments"})
			return
		}
		result, err := call(args)
		switch {
		case errors.Is(err, errBadArgs):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		case err != nil:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		default:
			writeJSON(w, http.StatusOK, map[string]any{"result": result})
		}
	})
	return mux
}

func streamEvents(w http.ResponseWriter, r *http.Request, events *Events) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	ch, cancel := events.Subscribe()
	defer cancel()
	send := func(e Event) bool {
		data, err := json.Marshal(e.Data)
		if err != nil {
			return true
		}
		// Event names never hold a newline; data is one line of JSON.
		_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", strings.ReplaceAll(e.Name, "\n", ""), data)
		flusher.Flush()
		return err == nil
	}
	if !send(Event{Name: "ping", Data: nil}) {
		return
	}
	tick := time.NewTicker(pingEvery)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			if !send(Event{Name: "ping", Data: nil}) {
				return
			}
		case e := <-ch:
			if !send(e) {
				return
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
