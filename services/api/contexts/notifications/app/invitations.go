package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
)

// Invitation is what guilds tells about an Invitation into a Guild it just
// made.
type Invitation struct {
	Email string
	// Roles are the names of the Roles it gives besides the Base role.
	Roles     []string
	GuildID   uint64
	Guild     string
	InvitedBy string
	Link      string
	ExpiresAt time.Time
}

// mailTimeout bounds an Invitation email, which the inviting admin waits
// for.
const mailTimeout = 10 * time.Second

// InvitationEmail is the subject and body of the email of inv.
func InvitationEmail(inv Invitation) (subject, body string) {
	by := inv.InvitedBy
	if by == "" {
		by = "Someone"
	}
	subject = "[The Bakery] " + by + " invited you to " + inv.Guild + " on The Bakery"
	as := ""
	if len(inv.Roles) > 0 {
		as = " as " + strings.Join(inv.Roles, ", ")
	}
	body = fmt.Sprintf(`%s invited you to %s on The Bakery%s.

Open this link to accept:

%s

The link works once, until %s.
If you did not expect this, ignore this email.`, by, inv.Guild, as, inv.Link, inv.ExpiresAt.UTC().Format("2 January 2006 15:04 UTC"))
	return subject, body
}

// SendInvitation emails the Invitation's link to the invited person
// through the inviting Guild's enabled email channel with the lowest id.
// Without one it answers
// false and no error. It is no Delivery: it goes to a person, not to the
// channel's recipients.
func (s *Service) SendInvitation(ctx context.Context, inv Invitation) (bool, error) {
	channels, err := s.store.Channels(ctx, inv.GuildID)
	if err != nil {
		return false, err
	}
	for _, c := range channels {
		if c.Kind != domain.Email || !c.Enabled {
			continue
		}
		subject, body := InvitationEmail(inv)
		mctx, cancel := context.WithTimeout(ctx, mailTimeout)
		defer cancel()
		if err := s.mailer.Mail(mctx, c, []string{inv.Email}, subject, body); err != nil {
			s.Log("notifications: emailing the invitation of %s through %q: %v", inv.Email, c.Name, err)
			return false, err
		}
		return true, nil
	}
	return false, nil
}
