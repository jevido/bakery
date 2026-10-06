// Package app holds the guilds use cases: creating Guilds, finding the
// Guild a request acts in and the Role a Member holds there, and managing
// the Memberships and Invitations of a Guild.
package app

import (
	"context"
	"errors"
	"time"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

// Guilds stores the Guilds.
type Guilds interface {
	ByID(ctx context.Context, id uint64) (domain.Guild, bool, error)
	// All lists every Guild by id.
	All(ctx context.Context) ([]domain.Guild, error)
	// Create stores g and its first admin Membership for adminID in one
	// transaction.
	Create(ctx context.Context, g domain.Guild, adminID uint64) (domain.Guild, error)
	// CreateFirstIfNone is Create unless a Guild exists already, in which
	// case it stores nothing and reports false. The check and the insert are
	// one step, so racing calls cannot both create one.
	CreateFirstIfNone(ctx context.Context, g domain.Guild, adminID uint64) (domain.Guild, bool, error)
	// Update stores g's name and description.
	Update(ctx context.Context, g domain.Guild) error
	// Delete removes the Guild with its Memberships, Invitations and API
	// tokens. ErrGuildInUse when something of another context still
	// belongs to it.
	Delete(ctx context.Context, id uint64) error
}

// Memberships stores the Memberships.
type Memberships interface {
	// ListForMember lists the Member's Memberships by Guild id.
	ListForMember(ctx context.Context, memberID uint64) ([]domain.Membership, error)
	// ListForGuild lists the Guild's Memberships.
	ListForGuild(ctx context.Context, guildID uint64) ([]domain.Membership, error)
	// RoleOf is the Member's Role in the Guild, false without a Membership.
	RoleOf(ctx context.Context, guildID, memberID uint64) (domain.Role, bool, error)
	// Add stores m unless the Member holds a Membership in that Guild
	// already, which it leaves as it is.
	Add(ctx context.Context, m domain.Membership) error
	// Change gives memberID's Membership in the Guild the Role to, or
	// deletes it when to is empty, once check accepts every Membership of
	// the Guild as they are. Check and change are one step, so two admins
	// demoting each other cannot leave the Guild without one.
	// ErrMembershipNotFound when memberID holds none there.
	Change(ctx context.Context, guildID, memberID uint64, to domain.Role, check func([]domain.Membership) error) (domain.Membership, error)
}

// Members is what guilds needs to know and ask of identity's Members.
type Members interface {
	IsInstanceAdmin(ctx context.Context, memberID uint64) (bool, error)
	// MemberByEmail is the id of the Member with this email.
	MemberByEmail(ctx context.Context, email string) (uint64, bool, error)
	// CreateMember stores a new Member for an accepted Invitation;
	// ErrMemberExists when the email is taken.
	CreateMember(ctx context.Context, name, email, password string) (uint64, error)
	// RevokeAPITokens deletes every API token the Member made in the Guild.
	RevokeAPITokens(ctx context.Context, memberID, guildID uint64) error
	ResetTwoFactor(ctx context.Context, memberID uint64) error
}

type Service struct {
	guilds      Guilds
	memberships Memberships
	invitations Invitations
	members     Members
	// Now is the clock; time.Now unless a test sets it.
	Now func() time.Time

	onDeleting []deletingCheck
}

type deletingCheck struct {
	kind  string
	inUse func(ctx context.Context, guildID uint64) (bool, error)
}

var ErrGuildNotFound = errors.New("guild not found")

// ErrGuildInUse refuses to delete a Guild that still owns something;
// Blocking names what, in the words of the contexts that own it.
type ErrGuildInUse struct{ Blocking []string }

func (e ErrGuildInUse) Error() string {
	return "the guild still owns resources; delete them first"
}

func NewService(guilds Guilds, memberships Memberships, invitations Invitations, members Members) *Service {
	return &Service{guilds: guilds, memberships: memberships, invitations: invitations, members: members, Now: time.Now}
}

// CreateGuild makes a Guild with creatorID as its admin.
func (s *Service) CreateGuild(ctx context.Context, name, description string, creatorID uint64) (domain.Guild, error) {
	g, err := domain.NewGuild(name, description)
	if err != nil {
		return domain.Guild{}, err
	}
	return s.guilds.Create(ctx, g, creatorID)
}

// MakeFirstGuild makes the installation's first Guild, "Default", with
// memberID (the Instance admin Setup just created) as its admin; nothing
// when a Guild exists already, as after the migration from before Guilds.
func (s *Service) MakeFirstGuild(ctx context.Context, memberID uint64) error {
	g, err := domain.NewGuild(domain.FirstGuildName, "")
	if err != nil {
		return err
	}
	_, _, err = s.guilds.CreateFirstIfNone(ctx, g, memberID)
	return err
}

// GuildsOf lists the Guilds the Member holds a Membership in.
func (s *Service) GuildsOf(ctx context.Context, memberID uint64) ([]domain.Membership, error) {
	return s.memberships.ListForMember(ctx, memberID)
}

// RoleOf is the Member's Role in the Guild, false without a Membership.
func (s *Service) RoleOf(ctx context.Context, guildID, memberID uint64) (domain.Role, bool, error) {
	return s.memberships.RoleOf(ctx, guildID, memberID)
}

// OnGuildDeleting registers a check DeleteGuild asks first: another context
// that keeps something of kind (e.g. "projects") in the Guild answers true
// while it does, and the deletion is refused naming kind.
func (s *Service) OnGuildDeleting(kind string, inUse func(ctx context.Context, guildID uint64) (bool, error)) {
	s.onDeleting = append(s.onDeleting, deletingCheck{kind: kind, inUse: inUse})
}

// Blocking names what still keeps the Guild from being deleted, in the
// order the checks were registered; empty when nothing does.
func (s *Service) Blocking(ctx context.Context, guildID uint64) ([]string, error) {
	out := []string{}
	for _, c := range s.onDeleting {
		inUse, err := c.inUse(ctx, guildID)
		if err != nil {
			return nil, err
		}
		if inUse {
			out = append(out, c.kind)
		}
	}
	return out, nil
}

// Guild is the Guild by id.
func (s *Service) Guild(ctx context.Context, id uint64) (domain.Guild, error) {
	g, found, err := s.guilds.ByID(ctx, id)
	if err == nil && !found {
		err = ErrGuildNotFound
	}
	return g, err
}

// UpdateGuild renames and describes the Guild. Only an admin of the Guild
// gets here (Admin guards the route).
func (s *Service) UpdateGuild(ctx context.Context, id uint64, name, description string) (domain.Guild, error) {
	g, err := s.Guild(ctx, id)
	if err != nil {
		return domain.Guild{}, err
	}
	if err := g.Rename(name); err != nil {
		return domain.Guild{}, err
	}
	if err := g.ChangeDescription(description); err != nil {
		return domain.Guild{}, err
	}
	return g, s.guilds.Update(ctx, g)
}

// DeleteGuild deletes the Guild once it owns nothing: ErrGuildInUse names
// what still blocks it. Its Memberships, Invitations and API tokens go with
// it. Only an admin of the Guild gets here (Admin guards the route).
func (s *Service) DeleteGuild(ctx context.Context, id uint64) error {
	if _, err := s.Guild(ctx, id); err != nil {
		return err
	}
	blocking, err := s.Blocking(ctx, id)
	if err != nil {
		return err
	}
	if len(blocking) > 0 {
		return ErrGuildInUse{Blocking: blocking}
	}
	return s.guilds.Delete(ctx, id)
}

// CanActIn reports whether the Member may act in the Guild: they hold a
// Membership there, or they are the Instance admin.
func (s *Service) CanActIn(ctx context.Context, guildID, memberID uint64, instanceAdmin bool) (bool, error) {
	_, ok, err := s.placeIn(ctx, guildID, memberID, instanceAdmin)
	return ok, err
}
