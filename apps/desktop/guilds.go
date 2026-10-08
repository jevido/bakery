package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jevido/bakery/apps/desktop/bakery"
	"github.com/jevido/bakery/apps/desktop/store"
)

// refreshEvery is how often the Guilds and Agents the window shows are read
// again from their Bakery, so a Guild created or an Agent hired in the
// dashboard appears without a reload.
const refreshEvery = 15 * time.Second

// shown is what the window last asked for: the Bakery whose Guilds the rail
// shows and the Guild and tab whose Agents the page lists. Only these are
// refreshed.
type shown struct {
	address string
	guildID uint64
	status  string
}

// cached is one answer kept in memory, as JSON to see whether a refresh
// changed it.
type cached struct {
	at   time.Time
	json []byte
}

// GuildsEvent is the `guilds` event: the Guilds of the Bakery at Address.
type GuildsEvent struct {
	Address string         `json:"address"`
	Guilds  []bakery.Guild `json:"guilds"`
}

// AgentsEvent is the `agents` event: the Agents of one Guild on one tab.
type AgentsEvent struct {
	Address string         `json:"address"`
	GuildID uint64         `json:"guild_id"`
	Status  string         `json:"status"`
	Agents  []bakery.Agent `json:"agents"`
}

// keyed is a client for the connected Bakery at address; a Bakery already
// known to be signed out answers ErrSignedOut without a request.
func (d *Desktop) keyed(address string) (*bakery.Client, error) {
	f, err := d.store.Load()
	if err != nil {
		return nil, err
	}
	b, ok := f.Find(address)
	if !ok {
		return nil, store.ErrUnknown
	}
	if b.SignedOut {
		return nil, bakery.ErrSignedOut
	}
	return bakery.New(b.Address, b.Key), nil
}

// usable says whether the Bakery at address is connected and not signed
// out, so a cached answer may be shown for it.
func (d *Desktop) usable(address string) bool {
	_, err := d.keyed(address)
	return err == nil
}

// ask runs one request to the Bakery at address and records a 401 as that
// Bakery being signed out.
func ask[T any](d *Desktop, address string, request func(context.Context, *bakery.Client) (T, error)) (T, error) {
	var zero T
	c, err := d.keyed(address)
	if err != nil {
		return zero, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := request(ctx, c)
	if errors.Is(err, bakery.ErrSignedOut) {
		d.signedOut(address)
	}
	return out, err
}

// remember keeps v as the answer for key and says whether it differs from
// the one kept before.
func (d *Desktop) remember(key string, v any) bool {
	b, err := json.Marshal(v)
	if err != nil {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	old, ok := d.cache[key]
	d.cache[key] = cached{at: time.Now(), json: b}
	return !ok || !bytes.Equal(old.json, b)
}

// recall decodes the answer kept for key into out when it is younger than
// refreshEvery.
func (d *Desktop) recall(key string, out any) bool {
	d.mu.Lock()
	c, ok := d.cache[key]
	d.mu.Unlock()
	return ok && time.Since(c.at) < refreshEvery && json.Unmarshal(c.json, out) == nil
}

func guildsKey(address string) string { return "guilds " + address }

func agentsKey(address string, guildID uint64, status string) string {
	return fmt.Sprintf("agents %s %d %s", address, guildID, status)
}

// show records what the window shows and starts the refresh the first time.
func (d *Desktop) show(change func(*shown)) {
	d.mu.Lock()
	change(&d.shown)
	d.mu.Unlock()
	d.refreshing.Do(func() { go d.refreshLoop() })
}

// Me is the person the Desktop key acts as on the Bakery at address.
func (d *Desktop) Me(address string) (bakery.Member, error) {
	return ask(d, address, func(ctx context.Context, c *bakery.Client) (bakery.Member, error) { return c.Me(ctx) })
}

// Guilds lists the person's Guilds on the Bakery at address, and keeps
// them current with `guilds` events while the window shows them.
func (d *Desktop) Guilds(address string) ([]bakery.Guild, error) {
	d.show(func(s *shown) {
		if s.address != address {
			*s = shown{address: address}
		}
	})
	var out []bakery.Guild
	if d.usable(address) && d.recall(guildsKey(address), &out) {
		return out, nil
	}
	out, err := ask(d, address, func(ctx context.Context, c *bakery.Client) ([]bakery.Guild, error) { return c.Guilds(ctx) })
	if err != nil {
		return nil, err
	}
	d.remember(guildsKey(address), out)
	return out, nil
}

// Agents lists the Agents of the Guild guildID on the Bakery at address on
// the tab status, and keeps them current with `agents` events while the
// window shows them.
func (d *Desktop) Agents(address string, guildID uint64, status string) ([]bakery.Agent, error) {
	d.show(func(s *shown) { *s = shown{address: address, guildID: guildID, status: status} })
	key := agentsKey(address, guildID, status)
	var out []bakery.Agent
	if d.usable(address) && d.recall(key, &out) {
		return out, nil
	}
	out, err := ask(d, address, func(ctx context.Context, c *bakery.Client) ([]bakery.Agent, error) {
		return c.Agents(ctx, guildID, status)
	})
	if err != nil {
		return nil, err
	}
	d.remember(key, out)
	return out, nil
}

// Agent reads one Agent of the Guild guildID on the Bakery at address.
func (d *Desktop) Agent(address string, guildID, id uint64) (bakery.Agent, error) {
	return ask(d, address, func(ctx context.Context, c *bakery.Client) (bakery.Agent, error) {
		return c.Agent(ctx, guildID, id)
	})
}

func (d *Desktop) refreshLoop() {
	tick := time.NewTicker(refreshEvery)
	defer tick.Stop()
	for range tick.C {
		d.refresh()
	}
}

// refresh reads what the window shows again and sends a `guilds` or
// `agents` event for what changed. A failed request waits for the next
// tick; a signed-out Bakery stops being refreshed.
func (d *Desktop) refresh() {
	d.mu.Lock()
	s := d.shown
	d.mu.Unlock()
	if s.address == "" {
		return
	}
	guilds, err := ask(d, s.address, func(ctx context.Context, c *bakery.Client) ([]bakery.Guild, error) { return c.Guilds(ctx) })
	if errors.Is(err, bakery.ErrSignedOut) || errors.Is(err, store.ErrUnknown) {
		d.mu.Lock()
		if d.shown == s {
			d.shown = shown{}
		}
		d.mu.Unlock()
		return
	}
	if err == nil && d.remember(guildsKey(s.address), guilds) {
		d.events.Emit("guilds", GuildsEvent{Address: s.address, Guilds: guilds})
	}
	if s.guildID == 0 {
		return
	}
	agents, err := ask(d, s.address, func(ctx context.Context, c *bakery.Client) ([]bakery.Agent, error) {
		return c.Agents(ctx, s.guildID, s.status)
	})
	if err == nil && d.remember(agentsKey(s.address, s.guildID, s.status), agents) {
		d.events.Emit("agents", AgentsEvent{Address: s.address, GuildID: s.guildID, Status: s.status, Agents: agents})
	}
}
