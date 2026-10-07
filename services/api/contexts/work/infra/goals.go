// Package infra is the work context's persistence.
package infra

import (
	"context"
	"errors"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

type goalRecord struct {
	ID            uint64 `gorm:"primaryKey"`
	GuildID       uint64
	ParentID      *uint64
	OwnerMemberID *uint64
	Title         string
	Description   string
	Level         string
	Status        string
	orm.Timestamps
}

func (goalRecord) TableName() string { return "goals" }

// nullable stores 0 as NULL.
func nullable(id uint64) *uint64 {
	if id == 0 {
		return nil
	}
	return &id
}

func deref(id *uint64) uint64 {
	if id == nil {
		return 0
	}
	return *id
}

func stamp(t *orm.Timestamps) (created, updated time.Time) {
	if t.CreatedAt != nil {
		created = t.CreatedAt.StdTime().UTC()
	}
	if t.UpdatedAt != nil {
		updated = t.UpdatedAt.StdTime().UTC()
	}
	return created, updated
}

func (r goalRecord) toDomain() domain.Goal {
	g := domain.Goal{
		ID: r.ID, GuildID: r.GuildID, ParentID: deref(r.ParentID), OwnerID: deref(r.OwnerMemberID),
		Title: r.Title, Description: r.Description, Level: domain.GoalLevel(r.Level), Status: domain.GoalStatus(r.Status),
	}
	g.CreatedAt, g.UpdatedAt = stamp(&r.Timestamps)
	return g
}

// Goals keeps Goals.
type Goals struct{}

func (Goals) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func (s Goals) Goals(ctx context.Context, guildID uint64) ([]domain.Goal, error) {
	var recs []goalRecord
	if err := s.query(ctx).Where("guild_id", guildID).Order("id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Goal, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// Goal returns the Goal; found is false when there is none.
func (s Goals) Goal(ctx context.Context, id uint64) (domain.Goal, bool, error) {
	var rec goalRecord
	if err := s.query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Goal{}, false, nil
		}
		return domain.Goal{}, false, err
	}
	return rec.toDomain(), true, nil
}

func (s Goals) CreateGoal(ctx context.Context, g domain.Goal) (domain.Goal, error) {
	rec := goalRecord{
		GuildID: g.GuildID, ParentID: nullable(g.ParentID), OwnerMemberID: nullable(g.OwnerID),
		Title: g.Title, Description: g.Description, Level: string(g.Level), Status: string(g.Status),
	}
	if err := s.query(ctx).Create(&rec); err != nil {
		return domain.Goal{}, err
	}
	return rec.toDomain(), nil
}

func (s Goals) SaveGoal(ctx context.Context, g domain.Goal) error {
	_, err := s.query(ctx).Exec(`UPDATE goals SET parent_id = ?, owner_member_id = ?, title = ?, description = ?, level = ?, status = ?, updated_at = now() WHERE id = ?`,
		nullable(g.ParentID), nullable(g.OwnerID), g.Title, g.Description, string(g.Level), string(g.Status), g.ID)
	return err
}

// DeleteGoal deletes the Goal and moves its Sub-goals under its parent.
func (Goals) DeleteGoal(ctx context.Context, g domain.Goal) error {
	return facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		if _, err := tx.Exec(`UPDATE goals SET parent_id = ?, updated_at = now() WHERE parent_id = ?`, nullable(g.ParentID), g.ID); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM goals WHERE id = ?`, g.ID)
		return err
	})
}
