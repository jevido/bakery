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
		ID: r.ID, GuildID: r.GuildID, Email: r.Email, RoleIDs: []uint64{}, CreatedAt: r.CreatedAt,
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
		GuildID: inv.GuildID, Email: inv.Email, TokenHash: tokenHash,
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
		if err != nil {
			return err
		}
		for _, id := range inv.RoleIDs {
			if err := tx.Create(&invitationRoleRecord{InvitationID: rec.ID, RoleID: id}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.Invitation{}, err
	}
	out := rec.toDomain()
	out.RoleIDs = inv.RoleIDs
	return out, nil
}

func (Invitations) Open(ctx context.Context, guildID uint64, now time.Time) ([]domain.Invitation, error) {
	var recs []invitationRecord
	err := query(ctx).Where("guild_id", guildID).
		Where("accepted_at IS NULL").Where("revoked_at IS NULL").Where("expires_at > ?", now).
		Order("created_at desc").Find(&recs)
	if err != nil {
		return nil, err
	}
	return withRoles(query(ctx), recs)
}

func (Invitations) ByID(ctx context.Context, id uint64) (domain.Invitation, bool, error) {
	q := query(ctx)
	return firstInvitation(q, q.Where("id", id))
}

func (Invitations) ByTokenHash(ctx context.Context, tokenHash string) (domain.Invitation, bool, error) {
	q := query(ctx)
	return firstInvitation(q, q.Where("token_hash", tokenHash))
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
		inv, found, err := firstInvitation(tx, tx.Where("token_hash", tokenHash).LockForUpdate())
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

// invitationRoleRecord is one Role an Invitation gives.
type invitationRoleRecord struct {
	InvitationID uint64
	RoleID       uint64
}

func (invitationRoleRecord) TableName() string { return "invitation_roles" }

// firstInvitation finds the Invitation q selects and, through tx, its
// Roles.
func firstInvitation(tx, q contractsorm.Query) (domain.Invitation, bool, error) {
	var rec invitationRecord
	if err := q.FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Invitation{}, false, nil
		}
		return domain.Invitation{}, false, err
	}
	out, err := withRoles(tx, []invitationRecord{rec})
	if err != nil {
		return domain.Invitation{}, false, err
	}
	return out[0], true, nil
}

// withRoles is recs with the Roles each gives, read through tx.
func withRoles(tx contractsorm.Query, recs []invitationRecord) ([]domain.Invitation, error) {
	out := make([]domain.Invitation, len(recs))
	if len(recs) == 0 {
		return out, nil
	}
	ids := make([]any, len(recs))
	at := make(map[uint64]int, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
		ids[i], at[r.ID] = r.ID, i
	}
	var held []invitationRoleRecord
	if err := tx.Model(&invitationRoleRecord{}).WhereIn("invitation_id", ids).OrderBy("role_id").Find(&held); err != nil {
		return nil, err
	}
	for _, h := range held {
		i := at[h.InvitationID]
		out[i].RoleIDs = append(out[i].RoleIDs, h.RoleID)
	}
	return out, nil
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

// addMembership gives memberID a Membership in inv's Guild holding the
// Roles inv gives that still exist, unless they hold one there already.
func addMembership(tx contractsorm.Query, inv domain.Invitation, memberID uint64, now time.Time) error {
	var added []uint64
	if err := tx.Raw(`INSERT INTO memberships (guild_id, user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?) ON CONFLICT (guild_id, user_id) DO NOTHING RETURNING id`, inv.GuildID, memberID, now, now).Scan(&added); err != nil {
		return err
	}
	if len(added) == 0 {
		return nil
	}
	// A deleted Role went from invitation_roles by cascade.
	_, err := tx.Exec(`INSERT INTO membership_roles (membership_id, role_id)
		SELECT ?, r.id FROM invitation_roles ir JOIN roles r ON r.id = ir.role_id
		WHERE ir.invitation_id = ? AND r.guild_id = ? AND NOT r.base`, added[0], inv.ID, inv.GuildID)
	return err
}
