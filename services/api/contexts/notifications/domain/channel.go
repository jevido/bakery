// Package domain is the notifications model: Notification channels, the
// Notifications sent to them and their Deliveries. It depends on nothing
// but the standard library.
package domain

import (
	"fmt"
	"net/mail"
	"net/url"
	"slices"
	"strings"
	"time"
)

// FieldError is a broken rule on one input field.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }

func invalid(field, format string, args ...any) error {
	return &FieldError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// Kind is a Channel kind: how a Notification channel is reached.
type Kind string

const (
	Email    Kind = "email"
	Discord  Kind = "discord"
	Slack    Kind = "slack"
	Telegram Kind = "telegram"
	Ntfy     Kind = "ntfy"
	Webhook  Kind = "webhook"
)

// Kinds are the Channel kinds, in the order the dashboard offers them.
var Kinds = []Kind{Email, Discord, Slack, Telegram, Ntfy, Webhook}

// EventKind is what a Notification is about.
type EventKind string

const (
	DeploymentFailed    EventKind = "deployment_failed"
	DeploymentSucceeded EventKind = "deployment_succeeded"
	BackupFailed        EventKind = "backup_failed"
	BackupSucceeded     EventKind = "backup_succeeded"
	ServerUnreachable   EventKind = "server_unreachable"
	ServerReachable     EventKind = "server_reachable"
	DiskAlmostFull      EventKind = "disk_almost_full"
)

// EventKindInfo describes an Event kind for the dashboard.
type EventKindInfo struct {
	Kind  EventKind
	Label string
	// Default says whether a new channel is subscribed to it.
	Default bool
}

// EventKinds are every Event kind, in the order the dashboard lists them.
var EventKinds = []EventKindInfo{
	{DeploymentFailed, "Deployment failed", true},
	{DeploymentSucceeded, "Deployment succeeded", false},
	{BackupFailed, "Backup failed", true},
	{BackupSucceeded, "Backup succeeded", false},
	{ServerUnreachable, "Server unreachable", true},
	{ServerReachable, "Server reachable again", true},
	{DiskAlmostFull, "Disk almost full", true},
}

// DefaultEventKinds are what a new channel is subscribed to when none are
// given.
func DefaultEventKinds() []EventKind {
	var out []EventKind
	for _, e := range EventKinds {
		if e.Default {
			out = append(out, e.Kind)
		}
	}
	return out
}

func knownEventKind(k EventKind) bool {
	return slices.ContainsFunc(EventKinds, func(e EventKindInfo) bool { return e.Kind == k })
}

// Security is how an email channel talks to its SMTP server.
type Security string

const (
	SecurityNone     Security = "none"
	SecurityStartTLS Security = "starttls"
	SecurityTLS      Security = "tls"
)

// Settings are a channel's settings; which fields matter depends on its
// Kind. Password, BotToken, Token, Secret and, for discord and slack, URL
// are secrets: never shown, and kept when a change leaves them empty.
type Settings struct {
	// email
	Host     string   `json:"host,omitempty"`
	Port     int      `json:"port,omitempty"`
	Security Security `json:"security,omitempty"`
	Username string   `json:"username,omitempty"`
	Password string   `json:"password,omitempty"`
	From     string   `json:"from,omitempty"`
	To       []string `json:"to,omitempty"`
	// discord, slack, ntfy (the server), webhook
	URL string `json:"url,omitempty"`
	// telegram
	BotToken string `json:"bot_token,omitempty"`
	ChatID   string `json:"chat_id,omitempty"`
	// ntfy
	Topic string `json:"topic,omitempty"`
	Token string `json:"token,omitempty"`
	// webhook
	Secret string `json:"secret,omitempty"`
}

// URLIsSecret says whether the Kind's URL is a credential (a Discord or
// Slack incoming webhook URL is all it takes to post).
func (k Kind) URLIsSecret() bool { return k == Discord || k == Slack }

// DefaultNtfyURL is the ntfy server used when none is given.
const DefaultNtfyURL = "https://ntfy.sh"

// Channel is a Notification channel, the aggregate root.
type Channel struct {
	ID         uint64
	Name       string
	Kind       Kind
	Settings   Settings
	EventKinds []EventKind
	CreatedAt  time.Time
}

// Input is a channel as typed. Nil EventKinds means the defaults on a new
// channel and no change on an existing one.
type Input struct {
	Name       string
	Kind       Kind
	Settings   Settings
	EventKinds []EventKind
}

// NewChannel validates the input into a new channel.
func NewChannel(in Input) (Channel, error) {
	if !slices.Contains(Kinds, in.Kind) {
		return Channel{}, invalid("kind", "kind is one of email, discord, slack, telegram, ntfy or webhook")
	}
	c := Channel{Kind: in.Kind}
	if in.EventKinds == nil {
		in.EventKinds = DefaultEventKinds()
	}
	return c, c.Change(in)
}

// Change validates and applies the input. The Kind never changes; an empty
// secret setting keeps the stored one.
func (c *Channel) Change(in Input) error {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return invalid("name", "name is required")
	}
	if len(name) > 63 {
		return invalid("name", "name is at most 63 characters")
	}
	if in.Kind != "" && in.Kind != c.Kind {
		return invalid("kind", "the kind of a channel cannot change")
	}
	events := c.EventKinds
	if in.EventKinds != nil {
		events = nil
		for _, k := range in.EventKinds {
			if !knownEventKind(k) {
				return invalid("event_kinds", "unknown event kind %q", k)
			}
			if !slices.Contains(events, k) {
				events = append(events, k)
			}
		}
	}
	if len(events) == 0 {
		return invalid("event_kinds", "pick at least one event")
	}
	s, err := c.merge(in.Settings)
	if err != nil {
		return err
	}
	c.Name, c.Settings, c.EventKinds = name, s, events
	return nil
}

// merge keeps the stored secrets the input leaves empty, trims and
// validates the result for the channel's Kind.
func (c *Channel) merge(in Settings) (Settings, error) {
	old := c.Settings
	keep := func(v *string, stored string) {
		*v = strings.TrimSpace(*v)
		if *v == "" {
			*v = stored
		}
	}
	trim := func(v *string) { *v = strings.TrimSpace(*v) }
	var s Settings
	switch c.Kind {
	case Email:
		s = Settings{Host: in.Host, Port: in.Port, Security: in.Security, Username: in.Username, Password: in.Password, From: in.From}
		trim(&s.Host)
		trim(&s.Username)
		trim(&s.From)
		keep(&s.Password, old.Password)
		if s.Username == "" {
			s.Password = ""
		}
		if s.Security == "" {
			s.Security = SecurityStartTLS
		}
		for _, to := range in.To {
			if to = strings.TrimSpace(to); to != "" && !slices.Contains(s.To, to) {
				s.To = append(s.To, to)
			}
		}
		switch {
		case s.Host == "" || strings.ContainsAny(s.Host, " /:"):
			return s, invalid("host", "host is the SMTP server's name or address, without a port")
		case s.Port < 1 || s.Port > 65535:
			return s, invalid("port", "port is 1–65535 (587 for STARTTLS, 465 for TLS)")
		case s.Security != SecurityNone && s.Security != SecurityStartTLS && s.Security != SecurityTLS:
			return s, invalid("security", "security is none, starttls or tls")
		case !isAddress(s.From):
			return s, invalid("from", "from is an email address")
		case len(s.To) == 0:
			return s, invalid("to", "add at least one recipient")
		}
		for _, to := range s.To {
			if !isAddress(to) {
				return s, invalid("to", "%q is not an email address", to)
			}
		}
	case Discord, Slack:
		s = Settings{URL: in.URL}
		keep(&s.URL, old.URL)
		if err := checkURL(s.URL, true); err != nil {
			return s, err
		}
	case Telegram:
		s = Settings{BotToken: in.BotToken, ChatID: in.ChatID}
		keep(&s.BotToken, old.BotToken)
		trim(&s.ChatID)
		switch {
		case s.BotToken == "" || strings.ContainsAny(s.BotToken, " /?#"):
			return s, invalid("bot_token", "bot token is the token @BotFather gave, like 123456:ABC-DEF…")
		case s.ChatID == "" || strings.ContainsAny(s.ChatID, " /?#"):
			return s, invalid("chat_id", "chat id is the numeric id of the chat, or @channelname")
		}
	case Ntfy:
		s = Settings{URL: strings.TrimRight(strings.TrimSpace(in.URL), "/"), Topic: in.Topic, Token: in.Token}
		if s.URL == "" {
			s.URL = DefaultNtfyURL
		}
		trim(&s.Topic)
		keep(&s.Token, old.Token)
		if err := checkURL(s.URL, false); err != nil {
			return s, err
		}
		if s.Topic == "" || len(s.Topic) > 64 || strings.ContainsAny(s.Topic, " /?#") {
			return s, invalid("topic", "topic is 1–64 characters without spaces or slashes")
		}
	case Webhook:
		s = Settings{URL: in.URL, Secret: in.Secret}
		trim(&s.URL)
		keep(&s.Secret, old.Secret)
		if err := checkURL(s.URL, true); err != nil {
			return s, err
		}
	}
	return s, nil
}

func isAddress(s string) bool {
	a, err := mail.ParseAddress(s)
	return err == nil && a.Name == "" && a.Address == s
}

func checkURL(raw string, pathAllowed bool) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return invalid("url", "url is an http:// or https:// URL")
	}
	if !pathAllowed && u.Path != "" {
		return invalid("url", "url is the server's address without a path, e.g. https://ntfy.sh")
	}
	return nil
}

// Subscribed says whether the channel gets Notifications of the kind.
func (c Channel) Subscribed(k EventKind) bool { return slices.Contains(c.EventKinds, k) }
