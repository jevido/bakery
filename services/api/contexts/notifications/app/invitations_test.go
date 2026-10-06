package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
)

type fakeMailer struct {
	channel string
	to      []string
	subject string
	body    string
	err     error
}

func (f *fakeMailer) Mail(_ context.Context, c domain.Channel, to []string, subject, body string) error {
	f.channel, f.to, f.subject, f.body = c.Name, to, subject, body
	return f.err
}

func TestSendInvitation(t *testing.T) {
	store := &fakeStore{}
	mailer := &fakeMailer{}
	s := NewService(store, &fakeSender{}, mailer)
	inv := Invitation{Email: "dev@example.com", Role: "member", GuildID: 1, Guild: "Default", InvitedBy: "Jeff", Link: "https://bakery.example.com/#/invite/tok", ExpiresAt: time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)}

	if sent, err := s.SendInvitation(context.Background(), inv); sent || err != nil {
		t.Fatalf("without an email channel: %v %v", sent, err)
	}

	for _, in := range []domain.Input{
		{Name: "hook", Kind: domain.Webhook, Settings: domain.Settings{URL: "http://a/"}},
		{Name: "ops", Kind: domain.Email, Settings: domain.Settings{Host: "smtp", Port: 25, Security: domain.SecurityNone, From: "b@example.com", To: []string{"ops@example.com"}}},
		{Name: "second", Kind: domain.Email, Settings: domain.Settings{Host: "smtp", Port: 25, Security: domain.SecurityNone, From: "b@example.com", To: []string{"x@example.com"}}},
	} {
		c, err := domain.NewChannel(in)
		if err != nil {
			t.Fatal(err)
		}
		c.GuildID = 1
		_, _ = store.CreateChannel(context.Background(), c)
	}
	// Another Guild's email channel never sends this Guild's Invitations.
	if sent, err := s.SendInvitation(context.Background(), Invitation{Email: "x@example.com", GuildID: 2}); sent || err != nil {
		t.Fatalf("through another guild's channel: %v %v", sent, err)
	}
	// A disabled email channel is passed over.
	store.channels[1].Enabled = false
	sent, err := s.SendInvitation(context.Background(), inv)
	if !sent || err != nil || mailer.channel != "second" {
		t.Fatalf("with ops disabled: sent %v via %q, %v", sent, mailer.channel, err)
	}
	store.channels[1].Enabled = true
	sent, err = s.SendInvitation(context.Background(), inv)
	if !sent || err != nil {
		t.Fatalf("sent %v, %v", sent, err)
	}
	if mailer.channel != "ops" || len(mailer.to) != 1 || mailer.to[0] != "dev@example.com" || mailer.subject != "[The Bakery] Jeff invited you to Default on The Bakery" {
		t.Fatalf("mailed %+v", mailer)
	}
	for _, want := range []string{"invited you to Default on The Bakery as member", inv.Link, "7 October 2026 12:00 UTC"} {
		if !strings.Contains(mailer.body, want) {
			t.Errorf("body lacks %q:\n%s", want, mailer.body)
		}
	}

	mailer.err = errors.New("smtp: auth: 535 denied")
	if sent, err := s.SendInvitation(context.Background(), inv); sent || err == nil {
		t.Fatalf("failed send: %v %v", sent, err)
	}
}
