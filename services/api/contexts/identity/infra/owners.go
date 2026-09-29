// Package infra stores the Owner with the Goravel ORM and hashes passwords
// with Goravel's hasher.
package infra

import (
	"context"
	"errors"

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
	orm.Timestamps
}

func (userRecord) TableName() string { return "users" }

func (r userRecord) toDomain() domain.Owner {
	return domain.Owner{ID: r.ID, Name: r.Name, Email: r.Email, PasswordHash: r.Password}
}

type Owners struct{}

func (Owners) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func (o Owners) Exists(ctx context.Context) (bool, error) {
	n, err := o.query(ctx).Model(&userRecord{}).Count()
	return n > 0, err
}

func (o Owners) AddIfNone(ctx context.Context, owner domain.Owner) (domain.Owner, error) {
	rec := userRecord{Name: owner.Name, Email: owner.Email, Password: owner.PasswordHash}
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		// Serialises racing Setups: the second waits here, then sees the
		// first one's row.
		if _, err := tx.Exec("LOCK TABLE users IN EXCLUSIVE MODE"); err != nil {
			return err
		}
		n, err := tx.Model(&userRecord{}).Count()
		if err != nil {
			return err
		}
		if n > 0 {
			return app.ErrOwnerExists
		}
		return tx.Create(&rec)
	})
	if err != nil {
		return domain.Owner{}, err
	}
	return rec.toDomain(), nil
}

func (o Owners) ByEmail(ctx context.Context, email string) (domain.Owner, bool, error) {
	return first(o.query(ctx).Where("email", email))
}

func (o Owners) ByID(ctx context.Context, id uint64) (domain.Owner, bool, error) {
	return first(o.query(ctx).Where("id", id))
}

func first(q contractsorm.Query) (domain.Owner, bool, error) {
	var rec userRecord
	if err := q.FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Owner{}, false, nil
		}
		return domain.Owner{}, false, err
	}
	return rec.toDomain(), true, nil
}

type Hasher struct{}

func (Hasher) Make(password string) (string, error) { return facades.Hash().Make(password) }
func (Hasher) Check(password, hash string) bool     { return facades.Hash().Check(password, hash) }
