// Package infra stores the Members with the Goravel ORM and hashes passwords
// with Goravel's hasher.
package infra

import (
	"context"
	"errors"
	"strings"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/identity/app"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

type userRecord struct {
	ID       uint64 `gorm:"primaryKey"`
	Name     string
	Email    string
	Password string
	Role     string
	orm.Timestamps
}

func (userRecord) TableName() string { return "users" }

func (r userRecord) toDomain() domain.Member {
	return domain.Member{ID: r.ID, Name: r.Name, Email: r.Email, PasswordHash: r.Password, Role: domain.Role(r.Role)}
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
	rec := userRecord{Name: owner.Name, Email: owner.Email, Password: owner.PasswordHash, Role: string(domain.RoleOwner)}
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

// isUniqueViolation reports whether err is Postgres refusing a duplicate
// in a unique index.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key")
}
