package infra

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

type activityRecord struct {
	ID            uint64 `gorm:"primaryKey"`
	GuildID       uint64
	ActorMemberID *uint64
	Action        string
	EntityType    string
	EntityID      uint64
	ProjectID     *uint64
	Details       string `gorm:"type:jsonb"`
	CreatedAt     time.Time
}

func (activityRecord) TableName() string { return "activity_events" }

// Activity keeps the Guild's Activity events. They are only ever added.
type Activity struct{}

func (Activity) Record(ctx context.Context, e domain.ActivityEvent) error {
	details, err := json.Marshal(e.Details)
	if err != nil {
		return err
	}
	rec := activityRecord{
		GuildID: e.GuildID, ActorMemberID: nullable(e.ActorID), Action: e.Action,
		EntityType: e.EntityType, EntityID: e.EntityID, ProjectID: nullable(e.ProjectID),
		Details: string(details), CreatedAt: e.CreatedAt,
	}
	return facades.Orm().WithContext(ctx).Query().Create(&rec)
}
