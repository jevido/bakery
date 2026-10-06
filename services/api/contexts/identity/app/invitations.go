package app

import (
	"context"
	"errors"
	"time"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

var (
	ErrAlreadyMember      = errors.New("someone with this email is already a member")
	ErrAlreadyInvited     = errors.New("this email already has an open invitation")
	ErrInvitationNotFound = errors.New("invitation not found")
)

// Invitations stores the Invitations. Only the hash of a link's token is
// stored.
type Invitations interface {
	// Add stores a new Invitation. An expired open one for the same email
	// is revoked first, in the same transaction; an open one that has not
	// expired is ErrAlreadyInvited.
	Add(ctx context.Context, inv domain.Invitation, tokenHash string) (domain.Invitation, error)
	// Open lists the Invitations that can still be accepted.
	Open(ctx context.Context, now time.Time) ([]domain.Invitation, error)
	ByID(ctx context.Context, id uint64) (domain.Invitation, bool, error)
	ByTokenHash(ctx context.Context, tokenHash string) (domain.Invitation, bool, error)
	Revoke(ctx context.Context, id uint64, now time.Time) error
	// Accept stores the Member and marks the Invitation accepted in one
	// transaction, refusing it (with the domain's error) if it was
	// accepted, revoked or expired meanwhile, and ErrAlreadyMember when the
	// email is taken.
	Accept(ctx context.Context, tokenHash string, m domain.Member, now time.Time) (domain.Member, error)
}

// Invite makes an Invitation for email with role, and returns it with the
// token of its link. The token is only ever known here. Only an admin of the
// Current guild gets here (guilds.Admin guards the route).
func (s *Service) Invite(ctx context.Context, actorID uint64, email string, role domain.Role) (domain.Invitation, string, error) {
	actor, err := s.CurrentMember(ctx, actorID)
	if err != nil {
		return domain.Invitation{}, "", err
	}
	inv, err := domain.NewInvitation(email, role, actor.ID, s.now())
	if err != nil {
		return domain.Invitation{}, "", err
	}
	if _, found, err := s.members.ByEmail(ctx, inv.Email); err != nil {
		return domain.Invitation{}, "", err
	} else if found {
		return domain.Invitation{}, "", ErrAlreadyMember
	}
	token, err := newSecret()
	if err != nil {
		return domain.Invitation{}, "", err
	}
	inv, err = s.invitations.Add(ctx, inv, hashSecret(token))
	return inv, token, err
}

// OpenInvitations lists the Invitations that can still be accepted.
func (s *Service) OpenInvitations(ctx context.Context) ([]domain.Invitation, error) {
	return s.invitations.Open(ctx, s.now())
}

// RevokeInvitation makes an Invitation's link stop working.
func (s *Service) RevokeInvitation(ctx context.Context, id uint64) error {
	inv, found, err := s.invitations.ByID(ctx, id)
	if err != nil {
		return err
	}
	if !found || !inv.Open(s.now()) {
		return ErrInvitationNotFound
	}
	return s.invitations.Revoke(ctx, id, s.now())
}

// InvitationByToken is the Invitation a link points at, if it can still be
// accepted: ErrInvitationNotFound, or the domain's reason it cannot.
func (s *Service) InvitationByToken(ctx context.Context, token string) (domain.Invitation, error) {
	inv, found, err := s.invitations.ByTokenHash(ctx, hashSecret(token))
	if err != nil {
		return domain.Invitation{}, err
	}
	if !found {
		return domain.Invitation{}, ErrInvitationNotFound
	}
	if err := inv.Refusal(s.now()); err != nil {
		return domain.Invitation{}, err
	}
	return inv, nil
}

// AcceptInvitation creates the invited Member, who picks their name and
// password, with the email of the Invitation, and lets MemberAdded give
// them its Role.
func (s *Service) AcceptInvitation(ctx context.Context, token, name, password string) (domain.Member, error) {
	inv, err := s.InvitationByToken(ctx, token)
	if err != nil {
		return domain.Member{}, err
	}
	m, err := domain.NewMember(name, inv.Email, password)
	if err != nil {
		return domain.Member{}, err
	}
	if m.PasswordHash, err = s.hasher.Make(password); err != nil {
		return domain.Member{}, err
	}
	if m, err = s.invitations.Accept(ctx, hashSecret(token), m, s.now()); err != nil {
		return domain.Member{}, err
	}
	return m, s.memberAdded(ctx, MemberAdded{MemberID: m.ID, Role: inv.Role})
}
