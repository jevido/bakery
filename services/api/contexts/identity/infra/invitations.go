package infra

import (
	"context"
	"errors"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/identity/app"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

type invitationRecord struct {
	ID         uint64 `gorm:"primaryKey"`
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
		ID: r.ID, Email: r.Email, Role: domain.Role(r.Role), CreatedAt: r.CreatedAt,
		ExpiresAt: r.ExpiresAt, AcceptedAt: r.AcceptedAt, RevokedAt: r.RevokedAt,
	}
	if r.InvitedBy != nil {
		inv.InvitedBy = *r.InvitedBy
	}
	return inv
}

type Invitations struct{}

func (Invitations) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func (i Invitations) Add(ctx context.Context, inv domain.Invitation, tokenHash string) (domain.Invitation, error) {
	rec := invitationRecord{
		Email: inv.Email, Role: string(inv.Role), TokenHash: tokenHash,
		ExpiresAt: inv.ExpiresAt, CreatedAt: inv.CreatedAt, UpdatedAt: inv.CreatedAt,
	}
	if inv.InvitedBy != 0 {
		by := inv.InvitedBy
		rec.InvitedBy = &by
	}
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		// An expired Invitation still holds the email's slot in the
		// invitations_one_open index; it is revoked to free it.
		if _, err := tx.Model(&invitationRecord{}).
			Where("email", inv.Email).Where("accepted_at IS NULL").Where("revoked_at IS NULL").
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

func (i Invitations) Open(ctx context.Context, now time.Time) ([]domain.Invitation, error) {
	var recs []invitationRecord
	err := i.query(ctx).Where("accepted_at IS NULL").Where("revoked_at IS NULL").Where("expires_at > ?", now).
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

func (i Invitations) ByID(ctx context.Context, id uint64) (domain.Invitation, bool, error) {
	return firstInvitation(i.query(ctx).Where("id", id))
}

func (i Invitations) ByTokenHash(ctx context.Context, tokenHash string) (domain.Invitation, bool, error) {
	return firstInvitation(i.query(ctx).Where("token_hash", tokenHash))
}

func (i Invitations) Revoke(ctx context.Context, id uint64, now time.Time) error {
	_, err := i.query(ctx).Model(&invitationRecord{}).Where("id", id).Where("revoked_at IS NULL").Update("revoked_at", now)
	return err
}

func (i Invitations) Accept(ctx context.Context, tokenHash string, m domain.Member, now time.Time) (domain.Member, error) {
	user := userRecord{Name: m.Name, Email: m.Email, Password: m.PasswordHash, Role: string(m.Role)}
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
		if err := tx.Create(&user); err != nil {
			if isUniqueViolation(err) {
				return app.ErrAlreadyMember
			}
			return err
		}
		_, err = tx.Model(&invitationRecord{}).Where("id", inv.ID).Update("accepted_at", now)
		return err
	})
	if err != nil {
		return domain.Member{}, err
	}
	return user.toDomain(), nil
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
