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
	if !c.Subscribed(DeploymentFailed) || c.Subscribed(DeploymentSucceeded) || c.Subscribed(BackupSucceeded) || !c.Subscribed(DiskAlmostFull) {
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
		{"ntfy path", func() Input {
			return Input{Name: "n", Kind: Ntfy, Settings: Settings{URL: "https://ntfy.sh/x", Topic: "a"}}
		}, "url"},
		{"no events", func() Input { in := emailInput(); in.EventKinds = []EventKind{}; return in }, "event_kinds"},
		{"unknown event", func() Input { in := emailInput(); in.EventKinds = []EventKind{"reboot"}; return in }, "event_kinds"},
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
	if err := d.Change(Input{Name: "d", EventKinds: []EventKind{BackupFailed, BackupFailed}}); err != nil {
		t.Fatal(err)
	}
	if d.Settings.URL != "https://discord.com/api/webhooks/1/x" || !slices.Equal(d.EventKinds, []EventKind{BackupFailed}) {
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
