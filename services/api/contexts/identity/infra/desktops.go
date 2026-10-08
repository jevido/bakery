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

type desktopRecord struct {
	ID         uint64 `gorm:"primaryKey"`
	MemberID   uint64
	Name       string
	KeyHash    string
	LastSeenAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (desktopRecord) TableName() string { return "desktops" }

type desktopSignInRecord struct {
	ID                 uint64 `gorm:"primaryKey"`
	SecretHash         string
	ClientName         string
	PendingKeyHash     string
	ApprovedByMemberID *uint64
	DesktopID          *uint64
	ApprovedAt         *time.Time
	CancelledAt        *time.Time
	ExpiresAt          time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (desktopSignInRecord) TableName() string { return "desktop_sign_ins" }

func (r desktopSignInRecord) toDomain() domain.DesktopSignIn {
	return domain.DesktopSignIn{ID: r.ID, ClientName: r.ClientName, KeyHash: r.PendingKeyHash, ApprovedByMemberID: r.ApprovedByMemberID, DesktopID: r.DesktopID, ApprovedAt: r.ApprovedAt, CancelledAt: r.CancelledAt, ExpiresAt: r.ExpiresAt, CreatedAt: r.CreatedAt}
}

// DesktopSignIns stores the Desktop sign-ins and the Desktops approving
// them makes.
type DesktopSignIns struct{}

func (DesktopSignIns) Add(ctx context.Context, s domain.DesktopSignIn, secretHash string) (domain.DesktopSignIn, error) {
	rec := desktopSignInRecord{SecretHash: secretHash, ClientName: s.ClientName, PendingKeyHash: s.KeyHash, ExpiresAt: s.ExpiresAt, CreatedAt: s.CreatedAt, UpdatedAt: s.CreatedAt}
	if err := facades.Orm().WithContext(ctx).Query().Create(&rec); err != nil {
		return domain.DesktopSignIn{}, err
	}
	return rec.toDomain(), nil
}

func firstDesktopSignIn(q contractsorm.Query) (desktopSignInRecord, bool, error) {
	var rec desktopSignInRecord
	if err := q.FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return desktopSignInRecord{}, false, nil
		}
		return desktopSignInRecord{}, false, err
	}
	return rec, true, nil
}

func (DesktopSignIns) ByID(ctx context.Context, id uint64) (domain.DesktopSignIn, string, bool, error) {
	rec, found, err := firstDesktopSignIn(facades.Orm().WithContext(ctx).Query().Where("id", id))
	return rec.toDomain(), rec.SecretHash, found, err
}

func (DesktopSignIns) Change(ctx context.Context, id uint64, change func(s *domain.DesktopSignIn, secretHash string) (*domain.Desktop, error)) (domain.DesktopSignIn, error) {
	var out domain.DesktopSignIn
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		// The row lock makes two racing approvals take turns; the second
		// then sees it approved and makes no second Desktop.
		rec, found, err := firstDesktopSignIn(tx.Where("id", id).LockForUpdate())
		if err != nil {
			return err
		}
		if !found {
			return app.ErrDesktopSignInNotFound
		}
		s := rec.toDomain()
		d, err := change(&s, rec.SecretHash)
		if err != nil {
			return err
		}
		now := time.Now()
		if d != nil {
			drec := desktopRecord{MemberID: d.MemberID, Name: d.Name, KeyHash: d.KeyHash, CreatedAt: d.CreatedAt, UpdatedAt: d.CreatedAt}
			if err := tx.Create(&drec); err != nil {
				return err
			}
			s.DesktopID = &drec.ID
		}
		if _, err := tx.Model(&desktopSignInRecord{}).Where("id", id).Update(map[string]any{
			"approved_by_member_id": s.ApprovedByMemberID,
			"desktop_id":            s.DesktopID,
			"approved_at":           s.ApprovedAt,
			"cancelled_at":          s.CancelledAt,
			"updated_at":            now,
		}); err != nil {
			return err
		}
		out = s
		return nil
	})
	return out, err
}
