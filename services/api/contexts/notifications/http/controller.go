// Package http is the notifications JSON API, for admins.
package http

import (
	"errors"
	"net/url"
	"strconv"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/notifications/app"
	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
)

type Controller struct {
	service *app.Service
}

func NewController(service *app.Service) *Controller {
	return &Controller{service: service}
}

// settingsJSON is a channel's settings as shown: no secret ever, only
// whether one is set, and of a secret URL only its host.
type settingsJSON struct {
	Host        string   `json:"host,omitempty"`
	Port        int      `json:"port,omitempty"`
	Security    string   `json:"security,omitempty"`
	Username    string   `json:"username,omitempty"`
	HasPassword bool     `json:"has_password"`
	From        string   `json:"from,omitempty"`
	To          []string `json:"to,omitempty"`
	URL         string   `json:"url,omitempty"`
	URLHost     string   `json:"url_host,omitempty"`
	ChatID      string   `json:"chat_id,omitempty"`
	HasBotToken bool     `json:"has_bot_token"`
	Topic       string   `json:"topic,omitempty"`
	HasToken    bool     `json:"has_token"`
	HasSecret   bool     `json:"has_secret"`
}

type channelJSON struct {
	ID         uint64       `json:"id"`
	Name       string       `json:"name"`
	Kind       string       `json:"kind"`
	Settings   settingsJSON `json:"settings"`
	EventKinds []string     `json:"event_kinds"`
	CreatedAt  time.Time    `json:"created_at"`
}

func urlHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

func toJSON(c domain.Channel) channelJSON {
	s := c.Settings
	out := channelJSON{ID: c.ID, Name: c.Name, Kind: string(c.Kind), EventKinds: []string{}, CreatedAt: c.CreatedAt}
	for _, k := range c.EventKinds {
		out.EventKinds = append(out.EventKinds, string(k))
	}
	j := settingsJSON{
		Host: s.Host, Port: s.Port, Security: string(s.Security), Username: s.Username, HasPassword: s.Password != "",
		From: s.From, To: s.To, ChatID: s.ChatID, HasBotToken: s.BotToken != "", Topic: s.Topic,
		HasToken: s.Token != "", HasSecret: s.Secret != "",
	}
	switch c.Kind {
	case domain.Ntfy:
		// An ntfy server's address is no secret; the token is.
		j.URL = s.URL
	case domain.Discord, domain.Slack, domain.Webhook:
		// The path of an incoming webhook URL often is the credential.
		j.URLHost = urlHost(s.URL)
	}
	out.Settings = j
	return out
}

type settingsRequest struct {
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Security string   `json:"security"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`
	URL      string   `json:"url"`
	BotToken string   `json:"bot_token"`
	ChatID   string   `json:"chat_id"`
	Topic    string   `json:"topic"`
	Token    string   `json:"token"`
	Secret   string   `json:"secret"`
}

// channelRequest is a channel as typed. Secrets are write-only: empty
// keeps the stored one. Missing event_kinds means the defaults on a new
// channel and no change on an existing one.
type channelRequest struct {
	Name       string          `json:"name"`
	Kind       string          `json:"kind"`
	Settings   settingsRequest `json:"settings"`
	EventKinds *[]string       `json:"event_kinds"`
}

func (r channelRequest) input() domain.Input {
	s := r.Settings
	in := domain.Input{Name: r.Name, Kind: domain.Kind(r.Kind), Settings: domain.Settings{
		Host: s.Host, Port: s.Port, Security: domain.Security(s.Security), Username: s.Username, Password: s.Password,
		From: s.From, To: s.To, URL: s.URL, BotToken: s.BotToken, ChatID: s.ChatID, Topic: s.Topic, Token: s.Token, Secret: s.Secret,
	}}
	if r.EventKinds != nil {
		in.EventKinds = []domain.EventKind{}
		for _, k := range *r.EventKinds {
			in.EventKinds = append(in.EventKinds, domain.EventKind(k))
		}
	}
	return in
}

func id(ctx contractshttp.Context) (uint64, bool) {
	v, err := strconv.ParseUint(ctx.Request().Route("id"), 10, 64)
	return v, err == nil
}

func notFound(ctx contractshttp.Context) contractshttp.Response {
	return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
}

func fail(ctx contractshttp.Context, err error) contractshttp.Response {
	var fe *domain.FieldError
	switch {
	case errors.As(err, &fe):
		return respond.Invalid(ctx, fe.Field, fe.Message)
	case errors.Is(err, app.ErrNotFound):
		return notFound(ctx)
	}
	return respond.ServerError(ctx, err)
}

func one(ctx contractshttp.Context, status int, c domain.Channel, err error) contractshttp.Response {
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Json(status, contractshttp.Json{"channel": toJSON(c)})
}

func (c *Controller) EventKinds(ctx contractshttp.Context) contractshttp.Response {
	type kindJSON struct {
		Kind    string `json:"kind"`
		Label   string `json:"label"`
		Default bool   `json:"default"`
	}
	out := make([]kindJSON, len(domain.EventKinds))
	for i, e := range domain.EventKinds {
		out[i] = kindJSON{string(e.Kind), e.Label, e.Default}
	}
	return ctx.Response().Success().Json(contractshttp.Json{"event_kinds": out})
}

func (c *Controller) List(ctx contractshttp.Context) contractshttp.Response {
	list, err := c.service.Channels(ctx.Context())
	if err != nil {
		return fail(ctx, err)
	}
	out := make([]channelJSON, len(list))
	for i, ch := range list {
		out[i] = toJSON(ch)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"channels": out})
}

func (c *Controller) Create(ctx contractshttp.Context) contractshttp.Response {
	var req channelRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	ch, err := c.service.AddChannel(ctx.Context(), req.input())
	return one(ctx, contractshttp.StatusCreated, ch, err)
}

func (c *Controller) Show(ctx contractshttp.Context) contractshttp.Response {
	cid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	ch, err := c.service.Channel(ctx.Context(), cid)
	return one(ctx, contractshttp.StatusOK, ch, err)
}

func (c *Controller) Update(ctx contractshttp.Context) contractshttp.Response {
	cid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req channelRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	ch, err := c.service.ChangeChannel(ctx.Context(), cid, req.input())
	return one(ctx, contractshttp.StatusOK, ch, err)
}

func (c *Controller) Delete(ctx contractshttp.Context) contractshttp.Response {
	cid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	if err := c.service.DeleteChannel(ctx.Context(), cid); err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().NoContent()
}

func (c *Controller) Test(ctx contractshttp.Context) contractshttp.Response {
	cid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	sendErr, err := c.service.TestChannel(ctx.Context(), cid)
	if err != nil {
		return fail(ctx, err)
	}
	if sendErr != nil {
		return ctx.Response().Json(contractshttp.StatusUnprocessableEntity, contractshttp.Json{"ok": false, "error": sendErr.Error()})
	}
	return ctx.Response().Success().Json(contractshttp.Json{"ok": true})
}
