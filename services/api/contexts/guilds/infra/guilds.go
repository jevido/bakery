// Package infra stores Guilds and Memberships with the Goravel ORM.
package infra

import (
	"context"
	"errors"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

type guildRecord struct {
	ID          uint64 `gorm:"primaryKey"`
	Name        string
	Description string
	orm.Timestamps
}

func (guildRecord) TableName() string { return "guilds" }

func (r guildRecord) toDomain() domain.Guild {
	return domain.Guild{ID: r.ID, Name: r.Name, Description: r.Description}
}

type membershipRecord struct {
	ID      uint64 `gorm:"primaryKey"`
	GuildID uint64
	UserID  uint64
	Role    string
	orm.Timestamps
}

func (membershipRecord) TableName() string { return "memberships" }

func (r membershipRecord) toDomain() domain.Membership {
	return domain.Membership{ID: r.ID, GuildID: r.GuildID, MemberID: r.UserID, Role: domain.Role(r.Role)}
}

func query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

type Guilds struct{}

func (Guilds) ByID(ctx context.Context, id uint64) (domain.Guild, bool, error) {
	var rec guildRecord
	if err := query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Guild{}, false, nil
		}
		return domain.Guild{}, false, err
	}
	return rec.toDomain(), true, nil
}

func (Guilds) All(ctx context.Context) ([]domain.Guild, error) {
	var recs []guildRecord
	if err := query(ctx).OrderBy("id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Guild, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

func (Guilds) Create(ctx context.Context, g domain.Guild, adminID uint64) (domain.Guild, error) {
	var rec guildRecord
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		var err error
		rec, err = create(tx, g, adminID)
		return err
	})
	if err != nil {
		return domain.Guild{}, err
	}
	return rec.toDomain(), nil
}

func (Guilds) CreateFirstIfNone(ctx context.Context, g domain.Guild, adminID uint64) (domain.Guild, bool, error) {
	var rec guildRecord
	created := false
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		// Serialises racing calls: the second waits here, then sees the
		// first one's row.
		if _, err := tx.Exec("LOCK TABLE guilds IN EXCLUSIVE MODE"); err != nil {
			return err
		}
		n, err := tx.Model(&guildRecord{}).Count()
		if err != nil || n > 0 {
			return err
		}
		rec, err = create(tx, g, adminID)
		created = err == nil
		return err
	})
	if err != nil || !created {
		return domain.Guild{}, false, err
	}
	return rec.toDomain(), true, nil
}

func create(tx contractsorm.Query, g domain.Guild, adminID uint64) (guildRecord, error) {
	rec := guildRecord{Name: g.Name, Description: g.Description}
	if err := tx.Create(&rec); err != nil {
		return guildRecord{}, err
	}
	m := membershipRecord{GuildID: rec.ID, UserID: adminID, Role: string(domain.RoleAdmin)}
	return rec, tx.Create(&m)
}

type Memberships struct{}

func (Memberships) ListForMember(ctx context.Context, memberID uint64) ([]domain.Membership, error) {
	var recs []membershipRecord
	if err := query(ctx).Where("user_id", memberID).OrderBy("guild_id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Membership, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

func (Memberships) RoleOf(ctx context.Context, guildID, memberID uint64) (domain.Role, bool, error) {
	var rec membershipRecord
	if err := query(ctx).Where("guild_id", guildID).Where("user_id", memberID).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return domain.Role(rec.Role), true, nil
}
