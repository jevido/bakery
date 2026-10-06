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
	Telegram Kind = "telegram"
	Slack    Kind = "slack"
	Pushover Kind = "pushover"
	Webhook  Kind = "webhook"
	Ntfy     Kind = "ntfy"
)

// Kinds are the Channel kinds, in the order the dashboard offers them:
// Coolify's, with ntfy, which Coolify lacks, last.
var Kinds = []Kind{Email, Discord, Telegram, Slack, Pushover, Webhook, Ntfy}

// EventKind is what a Notification is about.
type EventKind string

const (
	DeploymentFailure EventKind = "deployment_failure"
	DeploymentSuccess EventKind = "deployment_success"
	BackupFailure     EventKind = "backup_failure"
	BackupSuccess     EventKind = "backup_success"
	ServerUnreachable EventKind = "server_unreachable"
	ServerReachable   EventKind = "server_reachable"
	ServerDiskUsage   EventKind = "server_disk_usage"
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
	{DeploymentSuccess, "Deployment success", false},
	{DeploymentFailure, "Deployment failure", true},
	{BackupSuccess, "Backup success", false},
	{BackupFailure, "Backup failure", true},
	{ServerDiskUsage, "Disk usage warning", true},
	{ServerReachable, "Server reachable", false},
	{ServerUnreachable, "Server unreachable", true},
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
// Kind. Password, BotToken, Token, Secret, UserKey, APIToken and, for
// discord, slack and webhook, URL are secrets: never shown, and kept when a
// change leaves them empty.
type Settings struct {
	// email
	Host     string   `json:"host,omitempty"`
	Port     int      `json:"port,omitempty"`
	Security Security `json:"security,omitempty"`
	Username string   `json:"username,omitempty"`
	Password string   `json:"password,omitempty"`
	From     string   `json:"from,omitempty"`
	To       []string `json:"to,omitempty"`
	// FromName is the display name in From:; empty means "The Bakery".
	FromName string `json:"from_name,omitempty"`
	// Timeout bounds one SMTP conversation, in seconds; 0 means
	// DefaultEmailTimeout.
	Timeout int `json:"timeout,omitempty"`
	// EHLODomain is the host name sent in EHLO; empty keeps Go's default.
	EHLODomain string `json:"ehlo_domain,omitempty"`
	// discord, slack, ntfy (the server), webhook
	URL string `json:"url,omitempty"`
	// discord: Ping mentions @here on an alarming Event kind.
	Ping bool `json:"ping,omitempty"`
	// telegram
	BotToken string `json:"bot_token,omitempty"`
	ChatID   string `json:"chat_id,omitempty"`
	// ThreadIDs sends an Event kind to a forum topic, by message thread id;
	// a kind without one goes to the main chat.
	ThreadIDs map[EventKind]string `json:"thread_ids,omitempty"`
	// pushover
	UserKey  string `json:"user_key,omitempty"`
	APIToken string `json:"api_token,omitempty"`
	// ntfy
	Topic string `json:"topic,omitempty"`
	Token string `json:"token,omitempty"`
	// webhook
	Secret string `json:"secret,omitempty"`
}

// URLIsSecret says whether the Kind's URL is a credential: a Discord or
// Slack incoming webhook URL is all it takes to post, and a webhook's often
// carries a token too.
func (k Kind) URLIsSecret() bool { return k == Discord || k == Slack || k == Webhook }

// DefaultNtfyURL is the ntfy server used when none is given.
const DefaultNtfyURL = "https://ntfy.sh"

// DefaultFromName is the display name of an email channel without one.
const DefaultFromName = "The Bakery"

// DefaultEmailTimeout bounds an SMTP conversation when the channel sets no
// timeout.
const DefaultEmailTimeout = 30 * time.Second

// EmailTimeout is how long one SMTP conversation of the channel may take.
func (s Settings) EmailTimeout() time.Duration {
	if s.Timeout <= 0 {
		return DefaultEmailTimeout
	}
	return time.Duration(s.Timeout) * time.Second
}

// DisplayFromName is the name shown in From:.
func (s Settings) DisplayFromName() string {
	if s.FromName == "" {
		return DefaultFromName
	}
	return s.FromName
}

// Channel is a Notification channel, the aggregate root.
type Channel struct {
	ID uint64
	// GuildID is the Guild the channel belongs to: it hears only of what
	// concerns that Guild.
	GuildID    uint64
	Name       string
	Kind       Kind
	Settings   Settings
	EventKinds []EventKind
	// Enabled is false for a channel that gets no Notifications.
	Enabled   bool
	CreatedAt time.Time
}

// Input is a channel as typed. Nil EventKinds means the defaults on a new
// channel and no change on an existing one; nil Enabled means enabled on a
// new channel and no change on an existing one.
type Input struct {
	Name       string
	Kind       Kind
	Settings   Settings
	EventKinds []EventKind
	Enabled    *bool
}

// NewChannel validates the input into a new channel.
func NewChannel(in Input) (Channel, error) {
	if !slices.Contains(Kinds, in.Kind) {
		return Channel{}, invalid("kind", "kind is one of email, discord, telegram, slack, pushover, webhook or ntfy")
	}
	c := Channel{Kind: in.Kind, Enabled: true}
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
	s, err := c.merge(in.Settings)
	if err != nil {
		return err
	}
	c.Name, c.Settings, c.EventKinds = name, s, events
	if in.Enabled != nil {
		c.Enabled = *in.Enabled
	}
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
		s = Settings{
			Host: in.Host, Port: in.Port, Security: in.Security, Username: in.Username, Password: in.Password,
			From: in.From, FromName: in.FromName, Timeout: in.Timeout, EHLODomain: in.EHLODomain,
		}
		trim(&s.Host)
		trim(&s.Username)
		trim(&s.From)
		trim(&s.FromName)
		trim(&s.EHLODomain)
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
		case len(s.FromName) > 63 || strings.ContainsAny(s.FromName, "\r\n<>\""):
			return s, invalid("from_name", "from name is at most 63 characters, without quotes or angle brackets")
		case s.Timeout != 0 && (s.Timeout < 1 || s.Timeout > 300):
			return s, invalid("timeout", "timeout is 1–300 seconds")
		case s.EHLODomain != "" && !isHostName(s.EHLODomain):
			return s, invalid("ehlo_domain", "EHLO domain is a host name, like mail.example.com")
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
		if c.Kind == Discord {
			s.Ping = in.Ping
		}
		keep(&s.URL, old.URL)
		if err := checkURL(s.URL); err != nil {
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
		for k, id := range in.ThreadIDs {
			id = strings.TrimSpace(id)
			switch {
			case id == "":
				continue
			case !knownEventKind(k):
				return s, invalid("thread_ids", "unknown event kind %q", k)
			case !isDigits(id) || len(id) > 19:
				return s, invalid("thread_ids", "a topic is the message thread id, digits only")
			}
			if s.ThreadIDs == nil {
				s.ThreadIDs = map[EventKind]string{}
			}
			s.ThreadIDs[k] = id
		}
	case Pushover:
		s = Settings{UserKey: in.UserKey, APIToken: in.APIToken}
		keep(&s.UserKey, old.UserKey)
		keep(&s.APIToken, old.APIToken)
		switch {
		case s.UserKey == "" || !isPushoverKey(s.UserKey):
			return s, invalid("user_key", "user key is the 30 letters and digits on your Pushover dashboard")
		case s.APIToken == "" || !isPushoverKey(s.APIToken):
			return s, invalid("api_token", "API token is the 30 letters and digits of your Pushover application")
		}
	case Ntfy:
		s = Settings{URL: strings.TrimRight(strings.TrimSpace(in.URL), "/"), Topic: in.Topic, Token: in.Token}
		if s.URL == "" {
			s.URL = DefaultNtfyURL
		}
		trim(&s.Topic)
		keep(&s.Token, old.Token)
		// A path is fine: a self-hosted ntfy may sit below one.
		if err := checkURL(s.URL); err != nil {
			return s, err
		}
		if s.Topic == "" || len(s.Topic) > 64 || strings.ContainsAny(s.Topic, " /?#") {
			return s, invalid("topic", "topic is 1–64 characters without spaces or slashes")
		}
	case Webhook:
		s = Settings{URL: in.URL, Secret: in.Secret}
		keep(&s.URL, old.URL)
		keep(&s.Secret, old.Secret)
		if err := checkURL(s.URL); err != nil {
			return s, err
		}
	}
	return s, nil
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// isPushoverKey says whether s looks like a Pushover user key or API
// token: letters and digits only. Pushover's are 30 long; the length is not
// checked, so a test stand-in can use its own.
func isPushoverKey(s string) bool {
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return len(s) <= 64
}

func isAddress(s string) bool {
	a, err := mail.ParseAddress(s)
	return err == nil && a.Name == "" && a.Address == s
}

// isHostName says whether s is a DNS host name: dot-separated labels of
// letters, digits and hyphens, none starting or ending with a hyphen.
func isHostName(s string) bool {
	if len(s) > 253 {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}

// IsAddress says whether s is a bare email address, like a@example.com.
func IsAddress(s string) bool { return isAddress(s) }

func checkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return invalid("url", "url is an http:// or https:// URL")
	}
	return nil
}

// Subscribed says whether the channel gets Notifications of the kind.
func (c Channel) Subscribed(k EventKind) bool { return slices.Contains(c.EventKinds, k) }
