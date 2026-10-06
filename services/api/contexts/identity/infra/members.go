// Package infra stores the Members with the Goravel ORM and hashes passwords
// with Goravel's hasher.
package infra

import (
	"context"
	"errors"
	"strings"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/identity/app"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

type userRecord struct {
	ID                       uint64 `gorm:"primaryKey"`
	Name                     string
	Email                    string
	Password                 string
	Role                     string
	InstanceAdmin            bool
	TwoFactorSecretEncrypted *string
	TwoFactorEnabledAt       *time.Time
	TwoFactorLastStep        int64
	SessionsValidFrom        *time.Time
	orm.Timestamps
}

func (userRecord) TableName() string { return "users" }

// toDomain decrypts the TOTP secret; a secret that no longer decrypts (the
// app key changed) reads as two-factor off rather than locking the Member
// out.
func (r userRecord) toDomain() domain.Member {
	m := domain.Member{ID: r.ID, Name: r.Name, Email: r.Email, PasswordHash: r.Password, Role: domain.Role(r.Role), InstanceAdmin: r.InstanceAdmin}
	m.TwoFactor.State = domain.TwoFactorOff
	if r.TwoFactorSecretEncrypted != nil {
		if plain, err := facades.Crypt().DecryptString(*r.TwoFactorSecretEncrypted); err == nil {
			m.TwoFactor.Secret = []byte(plain)
			m.TwoFactor.State = domain.TwoFactorPending
			if r.TwoFactorEnabledAt != nil {
				m.TwoFactor.State = domain.TwoFactorOn
			}
		}
	}
	m.TwoFactor.LastStep = r.TwoFactorLastStep
	if r.SessionsValidFrom != nil {
		m.SessionsValidFrom = *r.SessionsValidFrom
	}
	return m
}

type Members struct{}

func (Members) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func (o Members) OwnerExists(ctx context.Context) (bool, error) {
	n, err := o.query(ctx).Model(&userRecord{}).Where("role", string(domain.RoleOwner)).Count()
	return n > 0, err
}

func (o Members) AddOwnerIfNone(ctx context.Context, owner domain.Member) (domain.Member, error) {
	rec := userRecord{Name: owner.Name, Email: owner.Email, Password: owner.PasswordHash, Role: string(domain.RoleOwner), InstanceAdmin: true}
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		// Serialises racing Setups: the second waits here, then sees the
		// first one's row.
		if _, err := tx.Exec("LOCK TABLE users IN EXCLUSIVE MODE"); err != nil {
			return err
		}
		n, err := tx.Model(&userRecord{}).Where("role", string(domain.RoleOwner)).Count()
		if err != nil {
			return err
		}
		if n > 0 {
			return app.ErrOwnerExists
		}
		return tx.Create(&rec)
	})
	if err != nil {
		return domain.Member{}, err
	}
	return rec.toDomain(), nil
}

func (o Members) ByEmail(ctx context.Context, email string) (domain.Member, bool, error) {
	return first(o.query(ctx).Where("email", email))
}

func (o Members) ByID(ctx context.Context, id uint64) (domain.Member, bool, error) {
	return first(o.query(ctx).Where("id", id))
}

func first(q contractsorm.Query) (domain.Member, bool, error) {
	var rec userRecord
	if err := q.FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Member{}, false, nil
		}
		return domain.Member{}, false, err
	}
	return rec.toDomain(), true, nil
}

type Hasher struct{}

func (Hasher) Make(password string) (string, error) { return facades.Hash().Make(password) }
func (Hasher) Check(password, hash string) bool     { return facades.Hash().Check(password, hash) }

func (o Members) All(ctx context.Context) ([]domain.Member, error) {
	var recs []userRecord
	// The Owner first, then by name.
	if err := o.query(ctx).OrderByRaw("role = 'owner' DESC, lower(name), id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Member, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

func (o Members) SetRole(ctx context.Context, id uint64, role domain.Role) error {
	_, err := o.query(ctx).Model(&userRecord{}).Where("id", id).Update("role", string(role))
	return err
}

func (o Members) Remove(ctx context.Context, id uint64) error {
	_, err := o.query(ctx).Where("id", id).Delete(&userRecord{})
	return err
}

func (o Members) update(ctx context.Context, id uint64, values map[string]any) error {
	_, err := o.query(ctx).Model(&userRecord{}).Where("id", id).Update(values)
	return err
}

func (o Members) SetName(ctx context.Context, id uint64, name string) error {
	return o.update(ctx, id, map[string]any{"name": name})
}

func (o Members) SetPassword(ctx context.Context, id uint64, hash string, sessionsValidFrom time.Time) error {
	return o.update(ctx, id, map[string]any{"password": hash, "sessions_valid_from": sessionsValidFrom})
}

func (o Members) SetSessionsValidFrom(ctx context.Context, id uint64, t time.Time) error {
	return o.update(ctx, id, map[string]any{"sessions_valid_from": t})
}

func (o Members) SetPendingTwoFactor(ctx context.Context, id uint64, secret []byte) error {
	enc, err := facades.Crypt().EncryptString(string(secret))
	if err != nil {
		return err
	}
	_, err = o.query(ctx).Exec(`UPDATE users SET two_factor_secret_encrypted = ?, two_factor_enabled_at = NULL, updated_at = now() WHERE id = ?`, enc, id)
	return err
}

func (o Members) EnableTwoFactor(ctx context.Context, id uint64, step int64, at time.Time, hashes []string) error {
	return facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		if _, err := tx.Exec(`UPDATE users SET two_factor_enabled_at = ?, two_factor_last_step = ?, updated_at = now() WHERE id = ?`, at, step, id); err != nil {
			return err
		}
		return replaceRecoveryCodes(tx, id, hashes)
	})
}

func (o Members) ClearTwoFactor(ctx context.Context, id uint64, sessionsValidFrom *time.Time) error {
	return facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		if _, err := tx.Exec(`UPDATE users SET two_factor_secret_encrypted = NULL, two_factor_enabled_at = NULL, two_factor_last_step = 0,
			sessions_valid_from = COALESCE(?, sessions_valid_from), updated_at = now() WHERE id = ?`, sessionsValidFrom, id); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM recovery_codes WHERE user_id = ?`, id)
		return err
	})
}

func (o Members) AdvanceTwoFactorStep(ctx context.Context, id uint64, step int64) (bool, error) {
	res, err := o.query(ctx).Exec(`UPDATE users SET two_factor_last_step = ? WHERE id = ? AND two_factor_last_step < ?`, step, id, step)
	if err != nil {
		return false, err
	}
	return res.RowsAffected > 0, nil
}

func (o Members) ReplaceRecoveryCodes(ctx context.Context, id uint64, hashes []string) error {
	return facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		return replaceRecoveryCodes(tx, id, hashes)
	})
}

func replaceRecoveryCodes(tx contractsorm.Query, id uint64, hashes []string) error {
	if _, err := tx.Exec(`DELETE FROM recovery_codes WHERE user_id = ?`, id); err != nil {
		return err
	}
	for _, h := range hashes {
		if _, err := tx.Exec(`INSERT INTO recovery_codes (user_id, code_hash) VALUES (?, ?)`, id, h); err != nil {
			return err
		}
	}
	return nil
}

func (o Members) UseRecoveryCode(ctx context.Context, id uint64, hash string) (bool, error) {
	res, err := o.query(ctx).Exec(`DELETE FROM recovery_codes WHERE user_id = ? AND code_hash = ?`, id, hash)
	if err != nil {
		return false, err
	}
	return res.RowsAffected > 0, nil
}

func (o Members) RecoveryCodesLeft(ctx context.Context, id uint64) (int, error) {
	var n int64
	err := o.query(ctx).Raw(`SELECT count(*) FROM recovery_codes WHERE user_id = ?`, id).Scan(&n)
	return int(n), err
}

// isUniqueViolation reports whether err is Postgres refusing a duplicate
// in a unique index.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key")
}
