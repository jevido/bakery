package app

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/jevido/bakery/services/api/app/secret"
	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

var (
	ErrAlreadyMember      = errors.New("someone with this email is already a member of this guild")
	ErrAlreadyInvited     = errors.New("this email already has an open invitation to this guild")
	ErrInvitationNotFound = errors.New("invitation not found")
	// ErrSignInToAccept is an Invitation for the email of an existing
	// Member, accepted without their Session.
	ErrSignInToAccept = errors.New("sign in to accept this invitation")
	// ErrNotInvited is an Invitation accepted with the Session of a Member
	// whose email it is not for.
	ErrNotInvited = errors.New("this invitation is for someone else")
	// ErrMemberExists is identity refusing a new Member because the email
	// is taken.
	ErrMemberExists = errors.New("someone with this email is already a member")
)

// Invitations stores the Invitations. Only the hash of a link's token is
// stored.
type Invitations interface {
	// Add stores a new Invitation. An expired open one for the same Guild
	// and email is revoked first, in the same transaction; an open one that
	// has not expired is ErrAlreadyInvited.
	Add(ctx context.Context, inv domain.Invitation, tokenHash string) (domain.Invitation, error)
	// Open lists the Guild's Invitations that can still be accepted.
	Open(ctx context.Context, guildID uint64, now time.Time) ([]domain.Invitation, error)
	ByID(ctx context.Context, id uint64) (domain.Invitation, bool, error)
	ByTokenHash(ctx context.Context, tokenHash string) (domain.Invitation, bool, error)
	Revoke(ctx context.Context, id uint64, now time.Time) error
	// Accept takes the Invitation in turn (racing accepts of one link wait
	// for each other), refuses it with the domain's error if it can no
	// longer be accepted, asks join for the Member who accepts it, and then
	// stores their Membership holding the Invitation's Roles that still
	// exist (leaving one they hold already as it is) and marks the
	// Invitation accepted in one transaction.
	Accept(ctx context.Context, tokenHash string, now time.Time, join func(domain.Invitation) (uint64, error)) (domain.Invitation, uint64, error)
}

// Invite makes an Invitation into the Guild for email that gives these
// Roles (none: the Base role only), each one the actor may assign, and
// returns it with the token of its link. The token is only ever known here.
func (s *Service) Invite(ctx context.Context, guildID, actorID uint64, perms domain.Permissions, email string, roleIDs []uint64) (domain.Invitation, string, error) {
	if !perms.Has(domain.PermissionManageMembers) {
		return domain.Invitation{}, "", domain.ErrMissing{Permission: domain.PermissionManageMembers}
	}
	inv, err := domain.NewInvitation(guildID, email, uniqueIDs(roleIDs), actorID, s.Now())
	if err != nil {
		return domain.Invitation{}, "", err
	}
	if err := s.mayInviteWith(ctx, guildID, actorID, perms, inv.RoleIDs); err != nil {
		return domain.Invitation{}, "", err
	}
	if id, found, err := s.members.MemberByEmail(ctx, inv.Email); err != nil {
		return domain.Invitation{}, "", err
	} else if found {
		if _, in, err := s.memberships.Of(ctx, guildID, id); err != nil {
			return domain.Invitation{}, "", err
		} else if in {
			return domain.Invitation{}, "", ErrAlreadyMember
		}
	}
	token, err := secret.New()
	if err != nil {
		return domain.Invitation{}, "", err
	}
	inv, err = s.invitations.Add(ctx, inv, secret.Hash(token))
	return inv, token, err
}

// OpenInvitations lists the Guild's Invitations that can still be accepted.
func (s *Service) OpenInvitations(ctx context.Context, guildID uint64) ([]domain.Invitation, error) {
	return s.invitations.Open(ctx, guildID, s.Now())
}

// RevokeInvitation makes an Invitation of the Guild stop working; another
// Guild's is ErrInvitationNotFound.
func (s *Service) RevokeInvitation(ctx context.Context, guildID, id uint64) error {
	inv, found, err := s.invitations.ByID(ctx, id)
	if err != nil {
		return err
	}
	if !found || inv.GuildID != guildID || !inv.Open(s.Now()) {
		return ErrInvitationNotFound
	}
	return s.invitations.Revoke(ctx, id, s.Now())
}

// Invited is an Invitation a link points at, its Guild, and whether its
// email already belongs to a Member (who accepts while signed in).
type Invited struct {
	Invitation     domain.Invitation
	Guild          domain.Guild
	ExistingMember bool
}

// InvitationByToken is the Invitation a link points at, if it can still be
// accepted: ErrInvitationNotFound, or the domain's reason it cannot.
func (s *Service) InvitationByToken(ctx context.Context, token string) (Invited, error) {
	inv, found, err := s.invitations.ByTokenHash(ctx, secret.Hash(token))
	if err != nil {
		return Invited{}, err
	}
	if !found {
		return Invited{}, ErrInvitationNotFound
	}
	if err := inv.Refusal(s.Now()); err != nil {
		return Invited{}, err
	}
	g, found, err := s.guilds.ByID(ctx, inv.GuildID)
	if err != nil {
		return Invited{}, err
	}
	if !found {
		return Invited{}, ErrInvitationNotFound
	}
	_, existing, err := s.members.MemberByEmail(ctx, inv.Email)
	if err != nil {
		return Invited{}, err
	}
	return Invited{Invitation: inv, Guild: g, ExistingMember: existing}, nil
}

// AcceptAsNewMember accepts an Invitation for an email that has no Member
// yet: identity creates them with the name and password they picked, with
// the Invitation's email, and they get its Membership. It answers the new
// Member's id.
func (s *Service) AcceptAsNewMember(ctx context.Context, token, name, password string) (domain.Invitation, uint64, error) {
	return s.invitations.Accept(ctx, secret.Hash(token), s.Now(), func(inv domain.Invitation) (uint64, error) {
		id, err := s.members.CreateMember(ctx, name, inv.Email, password)
		if errors.Is(err, ErrMemberExists) {
			return 0, ErrSignInToAccept
		}
		return id, err
	})
}

// AcceptAsMember accepts an Invitation for the email of an existing Member,
// who is signed in as memberID with that email: they get its Membership.
func (s *Service) AcceptAsMember(ctx context.Context, token string, memberID uint64, email string) (domain.Invitation, error) {
	inv, _, err := s.invitations.Accept(ctx, secret.Hash(token), s.Now(), func(inv domain.Invitation) (uint64, error) {
		if domain.NormalizeEmail(email) != inv.Email {
			return 0, ErrNotInvited
		}
		return memberID, nil
	})
	return inv, err
}

// mayInviteWith checks that the actor may assign every one of these Roles
// of the Guild; ErrRoleNotFound for one that is not the Guild's.
func (s *Service) mayInviteWith(ctx context.Context, guildID, actorID uint64, perms domain.Permissions, roleIDs []uint64) error {
	g, err := s.Guild(ctx, guildID)
	if err != nil {
		return err
	}
	roles, err := s.roles.ForGuild(ctx, guildID)
	if err != nil {
		return err
	}
	ms, err := s.memberships.ListForGuild(ctx, guildID)
	if err != nil {
		return err
	}
	a, err := s.actor(ctx, Hierarchy{Guild: g, Roles: roles, Memberships: ms}, actorID, perms)
	if err != nil {
		return err
	}
	for _, id := range roleIDs {
		r, ok := findRole(roles, id)
		if !ok {
			return ErrRoleNotFound
		}
		if err := domain.CanAssign(a.Rank, r); err != nil {
			return err
		}
	}
	return nil
}

// uniqueIDs is ids without repeats, in their first order; never nil.
func uniqueIDs(ids []uint64) []uint64 {
	out := []uint64{}
	for _, id := range ids {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}
