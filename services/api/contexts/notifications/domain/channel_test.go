package domain

import (
	"errors"
	"slices"
	"testing"
)

func field(t *testing.T, err error) string {
	t.Helper()
	var fe *FieldError
	if !errors.As(err, &fe) {
		t.Fatalf("got %v, want a FieldError", err)
	}
	return fe.Field
}

func emailInput() Input {
	return Input{Name: "ops", Kind: Email, Settings: Settings{
		Host: "smtp.example.com", Port: 587, Username: "bakery", Password: "pw",
		From: "bakery@example.com", To: []string{"ops@example.com", " ops@example.com", ""},
	}}
}

func TestNewChannelDefaults(t *testing.T) {
	c, err := NewChannel(emailInput())
	if err != nil {
		t.Fatal(err)
	}
	if c.Settings.Security != SecurityStartTLS {
		t.Errorf("security %q, want starttls", c.Settings.Security)
	}
	if !slices.Equal(c.Settings.To, []string{"ops@example.com"}) {
		t.Errorf("to %v", c.Settings.To)
	}
	if !c.Subscribed(DeploymentFailure) || c.Subscribed(DeploymentSuccess) || c.Subscribed(BackupSuccess) || !c.Subscribed(ServerDiskUsage) {
		t.Errorf("event kinds %v", c.EventKinds)
	}
}

func TestChannelValidation(t *testing.T) {
	cases := []struct {
		name string
		in   func() Input
		want string
	}{
		{"no name", func() Input { in := emailInput(); in.Name = " "; return in }, "name"},
		{"unknown kind", func() Input { return Input{Name: "x", Kind: "pager"} }, "kind"},
		{"no recipient", func() Input { in := emailInput(); in.Settings.To = nil; return in }, "to"},
		{"bad recipient", func() Input { in := emailInput(); in.Settings.To = []string{"Ops <ops@example.com>"}; return in }, "to"},
		{"bad from", func() Input { in := emailInput(); in.Settings.From = "nope"; return in }, "from"},
		{"port", func() Input { in := emailInput(); in.Settings.Port = 0; return in }, "port"},
		{"host with port", func() Input { in := emailInput(); in.Settings.Host = "smtp:25"; return in }, "host"},
		{"security", func() Input { in := emailInput(); in.Settings.Security = "ssl"; return in }, "security"},
		{"discord url", func() Input { return Input{Name: "d", Kind: Discord, Settings: Settings{URL: "ftp://x"}} }, "url"},
		{"webhook url", func() Input { return Input{Name: "w", Kind: Webhook, Settings: Settings{URL: "https://u:p@x/hook"}} }, "url"},
		{"telegram token", func() Input { return Input{Name: "t", Kind: Telegram, Settings: Settings{ChatID: "1"}} }, "bot_token"},
		{"telegram chat", func() Input { return Input{Name: "t", Kind: Telegram, Settings: Settings{BotToken: "1:a"}} }, "chat_id"},
		{"ntfy topic", func() Input { return Input{Name: "n", Kind: Ntfy, Settings: Settings{Topic: "a/b"}} }, "topic"},
		{"no events", func() Input { in := emailInput(); in.EventKinds = []EventKind{}; return in }, "event_kinds"},
		{"unknown event", func() Input { in := emailInput(); in.EventKinds = []EventKind{"reboot"}; return in }, "event_kinds"},
		{"from name", func() Input { in := emailInput(); in.Settings.FromName = "Ops <x>"; return in }, "from_name"},
		{"timeout low", func() Input { in := emailInput(); in.Settings.Timeout = -1; return in }, "timeout"},
		{"timeout high", func() Input { in := emailInput(); in.Settings.Timeout = 301; return in }, "timeout"},
		{"ehlo domain", func() Input { in := emailInput(); in.Settings.EHLODomain = "mail example"; return in }, "ehlo_domain"},
		{"ehlo hyphen", func() Input { in := emailInput(); in.Settings.EHLODomain = "-mail.example.com"; return in }, "ehlo_domain"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewChannel(tc.in())
			if got := field(t, err); got != tc.want {
				t.Errorf("field %q, want %q (%v)", got, tc.want, err)
			}
		})
	}
}

func TestNtfyDefaultServer(t *testing.T) {
	c, err := NewChannel(Input{Name: "n", Kind: Ntfy, Settings: Settings{Topic: "bakery"}})
	if err != nil {
		t.Fatal(err)
	}
	if c.Settings.URL != DefaultNtfyURL {
		t.Errorf("url %q", c.Settings.URL)
	}
}

func TestChangeKeepsSecrets(t *testing.T) {
	c, err := NewChannel(Input{Name: "hook", Kind: Webhook, Settings: Settings{URL: "https://example.com/h", Secret: "s3cret"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Change(Input{Name: "hook 2"}); err != nil {
		t.Fatal(err)
	}
	if c.Name != "hook 2" || c.Settings.Secret != "s3cret" || c.Settings.URL != "https://example.com/h" {
		t.Errorf("got %+v", c)
	}
	if err := c.Change(Input{Name: "hook 2", Settings: Settings{URL: "https://example.com/h2"}}); err != nil || c.Settings.URL != "https://example.com/h2" {
		t.Errorf("url %q, %v", c.Settings.URL, err)
	}
	if !slices.Equal(c.EventKinds, DefaultEventKinds()) {
		t.Errorf("nil event kinds changed them: %v", c.EventKinds)
	}

	d, _ := NewChannel(Input{Name: "d", Kind: Discord, Settings: Settings{URL: "https://discord.com/api/webhooks/1/x"}})
	if err := d.Change(Input{Name: "d", EventKinds: []EventKind{BackupFailure, BackupFailure}}); err != nil {
		t.Fatal(err)
	}
	if d.Settings.URL != "https://discord.com/api/webhooks/1/x" || !slices.Equal(d.EventKinds, []EventKind{BackupFailure}) {
		t.Errorf("got %+v", d)
	}
	if err := d.Change(Input{Name: "d", Kind: Slack}); field(t, err) != "kind" {
		t.Error("kind changed")
	}

	e, _ := NewChannel(emailInput())
	in := emailInput()
	in.Settings.Password = ""
	if err := e.Change(in); err != nil || e.Settings.Password != "pw" {
		t.Errorf("password %q, %v", e.Settings.Password, err)
	}
	in.Settings.Username = ""
	if err := e.Change(in); err != nil || e.Settings.Password != "" {
		t.Errorf("no username kept password %q, %v", e.Settings.Password, err)
	}
}

func TestEmailOptionalSettings(t *testing.T) {
	c, err := NewChannel(emailInput())
	if err != nil {
		t.Fatal(err)
	}
	if c.Settings.DisplayFromName() != "The Bakery" || c.Settings.EmailTimeout() != DefaultEmailTimeout || c.Settings.EHLODomain != "" {
		t.Errorf("defaults %+v", c.Settings)
	}
	in := emailInput()
	in.Settings.FromName, in.Settings.Timeout, in.Settings.EHLODomain = " Ops team ", 5, " mail.example.com "
	if err := c.Change(in); err != nil {
		t.Fatal(err)
	}
	if c.Settings.DisplayFromName() != "Ops team" || c.Settings.EmailTimeout().Seconds() != 5 || c.Settings.EHLODomain != "mail.example.com" {
		t.Errorf("got %+v", c.Settings)
	}
}

func TestEnabled(t *testing.T) {
	c, err := NewChannel(emailInput())
	if err != nil {
		t.Fatal(err)
	}
	if !c.Enabled {
		t.Fatal("a new channel is disabled")
	}
	off := false
	in := emailInput()
	in.Enabled = &off
	if err := c.Change(in); err != nil || c.Enabled {
		t.Fatalf("enabled %v, %v", c.Enabled, err)
	}
	if err := c.Change(emailInput()); err != nil || c.Enabled {
		t.Errorf("nil enabled changed it: %v, %v", c.Enabled, err)
	}
	d, err := NewChannel(in)
	if err != nil || d.Enabled {
		t.Errorf("new channel with enabled false: %v, %v", d.Enabled, err)
	}
}
