package app

import (
	"context"

	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
)

// Sender sends one Notification to one Notification channel, by its
// Channel kind. Its errors never contain the channel's secrets.
type Sender interface {
	Send(ctx context.Context, c domain.Channel, n domain.Notification) error
}

// Mailer sends one plain-text email through an email channel's SMTP
// server, to the given recipients rather than the channel's.
type Mailer interface {
	Mail(ctx context.Context, c domain.Channel, to []string, subject, body string) error
}
