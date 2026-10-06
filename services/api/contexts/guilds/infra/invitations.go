package infra

import (
	"context"
	"errors"
	"strings"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/guilds/app"
	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

type invitationRecord struct {
	ID         uint64 `gorm:"primaryKey"`
	GuildID    uint64
	Email      string
	Role       string
	TokenHash  string
	InvitedBy  *uint64
	ExpiresAt  time.Time
	AcceptedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (invitationRecord) TableName() string { return "invitations" }

func (r invitationRecord) toDomain() domain.Invitation {
	inv := domain.Invitation{
		ID: r.ID, GuildID: r.GuildID, Email: r.Email, Role: r.Role, CreatedAt: r.CreatedAt,
		ExpiresAt: r.ExpiresAt, AcceptedAt: r.AcceptedAt, RevokedAt: r.RevokedAt,
	}
	if r.InvitedBy != nil {
		inv.InvitedBy = *r.InvitedBy
	}
	return inv
}

type Invitations struct{}

func (Invitations) Add(ctx context.Context, inv domain.Invitation, tokenHash string) (domain.Invitation, error) {
	rec := invitationRecord{
		GuildID: inv.GuildID, Email: inv.Email, Role: inv.Role, TokenHash: tokenHash,
		ExpiresAt: inv.ExpiresAt, CreatedAt: inv.CreatedAt, UpdatedAt: inv.CreatedAt,
	}
	if inv.InvitedBy != 0 {
		by := inv.InvitedBy
		rec.InvitedBy = &by
	}
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		// An expired Invitation still holds the email's slot in the Guild's
		// invitations_one_open index; it is revoked to free it.
		if _, err := tx.Model(&invitationRecord{}).
			Where("guild_id", inv.GuildID).Where("email", inv.Email).
			Where("accepted_at IS NULL").Where("revoked_at IS NULL").
			Where("expires_at <= ?", inv.CreatedAt).
			Update("revoked_at", inv.CreatedAt); err != nil {
			return err
		}
		err := tx.Create(&rec)
		if isUniqueViolation(err) {
			return app.ErrAlreadyInvited
		}
		return err
	})
	if err != nil {
		return domain.Invitation{}, err
	}
	return rec.toDomain(), nil
}

func (Invitations) Open(ctx context.Context, guildID uint64, now time.Time) ([]domain.Invitation, error) {
	var recs []invitationRecord
	err := query(ctx).Where("guild_id", guildID).
		Where("accepted_at IS NULL").Where("revoked_at IS NULL").Where("expires_at > ?", now).
		Order("created_at desc").Find(&recs)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Invitation, len(recs))
	for n, r := range recs {
		out[n] = r.toDomain()
	}
	return out, nil
}

func (Invitations) ByID(ctx context.Context, id uint64) (domain.Invitation, bool, error) {
	return firstInvitation(query(ctx).Where("id", id))
}

func (Invitations) ByTokenHash(ctx context.Context, tokenHash string) (domain.Invitation, bool, error) {
	return firstInvitation(query(ctx).Where("token_hash", tokenHash))
}

func (Invitations) Revoke(ctx context.Context, id uint64, now time.Time) error {
	_, err := query(ctx).Model(&invitationRecord{}).Where("id", id).Where("revoked_at IS NULL").Update("revoked_at", now)
	return err
}

func (Invitations) Accept(ctx context.Context, tokenHash string, now time.Time, join func(domain.Invitation) (uint64, error)) (domain.Invitation, uint64, error) {
	var (
		accepted domain.Invitation
		memberID uint64
	)
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		// The row lock makes two racing accepts of one link take turns; the
		// second then sees it accepted.
		inv, found, err := firstInvitation(tx.Where("token_hash", tokenHash).LockForUpdate())
		if err != nil {
			return err
		}
		if !found {
			return app.ErrInvitationNotFound
		}
		if err := inv.Accept(now); err != nil {
			return err
		}
		if memberID, err = join(inv); err != nil {
			return err
		}
		if err := addMembership(tx, inv, memberID, now); err != nil {
			return err
		}
		accepted = inv
		_, err = tx.Model(&invitationRecord{}).Where("id", inv.ID).Update("accepted_at", now)
		return err
	})
	if err != nil {
		return domain.Invitation{}, 0, err
	}
	return accepted, memberID, nil
}

func firstInvitation(q contractsorm.Query) (domain.Invitation, bool, error) {
	var rec invitationRecord
	if err := q.FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Invitation{}, false, nil
		}
		return domain.Invitation{}, false, err
	}
	return rec.toDomain(), true, nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key")
}

func isForeignKeyViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23503")
}

// addMembership gives memberID a Membership in inv's Guild holding the seeded Role
// inv names, unless they hold one there already.
func addMembership(tx contractsorm.Query, inv domain.Invitation, memberID uint64, now time.Time) error {
	role, err := domain.SeededRoleName(inv.Role)
	if err != nil {
		return err
	}
	var added []uint64
	if err := tx.Raw(`INSERT INTO memberships (guild_id, user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?) ON CONFLICT (guild_id, user_id) DO NOTHING RETURNING id`, inv.GuildID, memberID, now, now).Scan(&added); err != nil {
		return err
	}
	if len(added) == 0 {
		return nil
	}
	_, err = tx.Exec(`INSERT INTO membership_roles (membership_id, role_id)
		SELECT ?, id FROM roles WHERE guild_id = ? AND name = ? AND NOT base`, added[0], inv.GuildID, role)
	return err
}
