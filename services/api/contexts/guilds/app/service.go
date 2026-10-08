// Package app holds the guilds use cases: creating Guilds, finding the
// Guild a request acts in and the Permissions a Member holds there, and managing
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
	// Create stores g with its seeded Roles (domain.SeedRoles) and a
	// Membership holding Admin for its Guild Master, in one transaction,
	// with the first free Issue prefix (domain.FreeIssuePrefix) from
	// g's.
	Create(ctx context.Context, g domain.Guild) (domain.Guild, error)
	// CreateFirstIfNone is Create unless a Guild exists already, in which
	// case it stores nothing and reports false. The check and the insert are
	// one step, so racing calls cannot both create one.
	CreateFirstIfNone(ctx context.Context, g domain.Guild) (domain.Guild, bool, error)
	// MasteredBy reports whether the Member is the Guild Master of any
	// Guild.
	MasteredBy(ctx context.Context, memberID uint64) (bool, error)
	// Update stores g's name, description and Issue prefix;
	// ErrIssuePrefixTaken when another Guild has that prefix.
	Update(ctx context.Context, g domain.Guild) error
	// Delete removes the Guild with its Memberships, Invitations and API
	// tokens. ErrGuildInUse when something of another context still
	// belongs to it.
	Delete(ctx context.Context, id uint64) error
}

// Memberships stores the Memberships with the Roles each holds.
type Memberships interface {
	// ListForMember lists the Member's Memberships by Guild id.
	ListForMember(ctx context.Context, memberID uint64) ([]domain.Membership, error)
	// ListForGuild lists the Guild's Memberships of people, never an
	// Agent membership.
	ListForGuild(ctx context.Context, guildID uint64) ([]domain.Membership, error)
	// Of is the Member's Membership in the Guild, false without one.
	Of(ctx context.Context, guildID, memberID uint64) (domain.Membership, bool, error)
	// OfAgent is the Agent's Agent membership in the Guild, false without
	// one.
	OfAgent(ctx context.Context, guildID, agentID uint64) (domain.Membership, bool, error)
}

// Roles stores the Roles of every Guild and which of them each Membership
// holds.
type Roles interface {
	// ForGuild lists the Guild's Roles by Position, the Base role first.
	ForGuild(ctx context.Context, guildID uint64) ([]domain.Role, error)
	// Change locks the Guild, then its Roles, then its Memberships (the
	// order Offers.Accept locks in), hands them to decide and stores the
	// Change it answers, in one transaction, so two changes to one Guild's
	// hierarchy, or a change and an accepted Transfer offer, take turns.
	// It answers the Change as stored, created Roles with their ids.
	Change(ctx context.Context, guildID uint64, decide func(Hierarchy) (Change, error)) (Change, error)
}

// Hierarchy is a Guild with its Roles by Position and its Memberships,
// Agent memberships included, as Roles.Change hands them over.
type Hierarchy struct {
	Guild       domain.Guild
	Roles       []domain.Role
	Memberships []domain.Membership
}

// Change is what Roles.Change stores.
type Change struct {
	// Roles are stored as they are; one without an ID is created.
	Roles []domain.Role
	// DeletedRole is the id of a Role to delete, 0 for none.
	DeletedRole uint64
	// Membership, when set, holds exactly its RoleIDs afterwards.
	Membership *domain.Membership
	// RemovedMembership is the id of a Membership to delete, 0 for none;
	// its Member's Overrides in the Guild go with it.
	RemovedMembership uint64
	// Override, when set, replaces the Project's Override for its Role or
	// Member; an empty one deletes it.
	Override *domain.Override
	// Agents are Agent memberships that hold exactly their RoleIDs
	// afterwards; one without an ID is created.
	Agents []domain.Membership
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
	roles       Roles
	invitations Invitations
	offers      Offers
	overrides   Overrides
	members     Members
	// Now is the clock; time.Now unless a test sets it.
	Now func() time.Time

	onDeleting []deletingCheck
	onLeaving  []func(ctx context.Context, guildID, memberID, actorID uint64) error
}

type deletingCheck struct {
	kind  string
	inUse func(ctx context.Context, guildID uint64) (bool, error)
}

var ErrGuildNotFound = errors.New("guild not found")

// ErrIssuePrefixTaken refuses an Issue prefix another Guild has.
var ErrIssuePrefixTaken = errors.New("another guild has this issue prefix")

// ErrGuildInUse refuses to delete a Guild that still owns something;
// Blocking names what, in the words of the contexts that own it.
type ErrGuildInUse struct{ Blocking []string }

func (e ErrGuildInUse) Error() string {
	return "the guild still owns resources; delete them first"
}

func NewService(guilds Guilds, memberships Memberships, roles Roles, invitations Invitations, offers Offers, overrides Overrides, members Members) *Service {
	return &Service{guilds: guilds, memberships: memberships, roles: roles, invitations: invitations, offers: offers, overrides: overrides, members: members, Now: time.Now}
}

// CreateGuild makes a Guild with the seeded Roles and creatorID its Guild
// Master, also holding Admin so the Role shows on the Members page.
func (s *Service) CreateGuild(ctx context.Context, name, description string, creatorID uint64) (domain.Guild, error) {
	g, err := domain.NewGuild(name, description, creatorID)
	if err != nil {
		return domain.Guild{}, err
	}
	return s.guilds.Create(ctx, g)
}

// MakeFirstGuild makes the installation's first Guild, "Default", with
// memberID (the Instance admin Setup just created) its Guild Master holding
// Admin; nothing when a Guild exists already, as after the migration from
// before Guilds.
func (s *Service) MakeFirstGuild(ctx context.Context, memberID uint64) error {
	g, err := domain.NewGuild(domain.FirstGuildName, "", memberID)
	if err != nil {
		return err
	}
	_, _, err = s.guilds.CreateFirstIfNone(ctx, g)
	return err
}

// IsGuildMaster reports whether the Member is the Guild Master of any
// Guild: such a Member cannot leave it or have their account deleted.
func (s *Service) IsGuildMaster(ctx context.Context, memberID uint64) (bool, error) {
	return s.guilds.MasteredBy(ctx, memberID)
}

// GuildsOf lists the Guilds the Member holds a Membership in.
func (s *Service) GuildsOf(ctx context.Context, memberID uint64) ([]domain.Membership, error) {
	return s.memberships.ListForMember(ctx, memberID)
}

// PermissionsIn is what the Member's Membership in the Guild allows, every
// Permission for its Guild Master; false without one.
func (s *Service) PermissionsIn(ctx context.Context, guildID, memberID uint64) (domain.Permissions, bool, error) {
	m, ok, err := s.memberships.Of(ctx, guildID, memberID)
	if err != nil || !ok {
		return 0, false, err
	}
	if g, found, err := s.guilds.ByID(ctx, guildID); err != nil {
		return 0, false, err
	} else if found && g.MasterID == memberID {
		return domain.AllPermissions, true, nil
	}
	roles, err := s.roles.ForGuild(ctx, guildID)
	if err != nil {
		return 0, false, err
	}
	return domain.PermissionsOf(roles, m), true, nil
}

// RolesIn lists the Guild's Roles by Position, the Base role first.
func (s *Service) RolesIn(ctx context.Context, guildID uint64) ([]domain.Role, error) {
	return s.roles.ForGuild(ctx, guildID)
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

// UpdateGuild renames and describes the Guild, and changes its Issue prefix
// unless issuePrefix is nil. Only a Member with manage_guild gets here.
func (s *Service) UpdateGuild(ctx context.Context, id uint64, name, description string, issuePrefix *string) (domain.Guild, error) {
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
	if issuePrefix != nil {
		if err := g.ChangeIssuePrefix(*issuePrefix); err != nil {
			return domain.Guild{}, err
		}
	}
	if err := s.guilds.Update(ctx, g); err != nil {
		return domain.Guild{}, err
	}
	return g, nil
}

// IssuePrefix is the Guild's Issue prefix.
func (s *Service) IssuePrefix(ctx context.Context, guildID uint64) (string, error) {
	g, err := s.Guild(ctx, guildID)
	return g.IssuePrefix, err
}

// IsMember reports whether the Member holds a Membership in the Guild.
func (s *Service) IsMember(ctx context.Context, guildID, memberID uint64) (bool, error) {
	_, ok, err := s.memberships.Of(ctx, guildID, memberID)
	return ok, err
}

// DeleteGuild deletes the Guild once it owns nothing: ErrGuildInUse names
// what still blocks it. Its Memberships, Invitations and API tokens go with
// it. Only a Member with administrator gets here (Admin guards the route).
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
