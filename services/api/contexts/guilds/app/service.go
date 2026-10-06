// Package app holds the guilds use cases: creating Guilds and finding the
// Role a Member holds in one.
package app

import (
	"context"

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
}

// Memberships stores the Memberships.
type Memberships interface {
	// ListForMember lists the Member's Memberships by Guild id.
	ListForMember(ctx context.Context, memberID uint64) ([]domain.Membership, error)
	// RoleOf is the Member's Role in the Guild, false without a Membership.
	RoleOf(ctx context.Context, guildID, memberID uint64) (domain.Role, bool, error)
}

type Service struct {
	guilds      Guilds
	memberships Memberships
}

func NewService(guilds Guilds, memberships Memberships) *Service {
	return &Service{guilds: guilds, memberships: memberships}
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
