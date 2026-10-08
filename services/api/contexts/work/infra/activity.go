package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/work/app"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

type activityRecord struct {
	ID            uint64 `gorm:"primaryKey"`
	GuildID       uint64
	ActorMemberID *uint64
	ActorAgentID  *uint64
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
		GuildID: e.GuildID, ActorMemberID: nullable(e.Actor.MemberID), ActorAgentID: nullable(e.Actor.AgentID), Action: e.Action,
		EntityType: e.EntityType, EntityID: e.EntityID, ProjectID: nullable(e.ProjectID),
		Details: string(details), CreatedAt: e.CreatedAt,
	}
	return facades.Orm().WithContext(ctx).Query().Create(&rec)
}

// toDomain reads the details with numbers as json.Number, so an id stays
// exact.
func (r activityRecord) toDomain() (domain.ActivityEvent, error) {
	details := map[string]any{}
	dec := json.NewDecoder(bytes.NewReader([]byte(r.Details)))
	dec.UseNumber()
	if err := dec.Decode(&details); err != nil {
		return domain.ActivityEvent{}, err
	}
	return domain.ActivityEvent{
		ID: r.ID, GuildID: r.GuildID, Actor: actor(r.ActorMemberID, r.ActorAgentID), Action: r.Action,
		EntityType: r.EntityType, EntityID: r.EntityID, ProjectID: deref(r.ProjectID),
		Details: details, CreatedAt: r.CreatedAt.UTC(),
	}, nil
}

func (Activity) find(q contractsorm.Query) ([]domain.ActivityEvent, error) {
	var recs []activityRecord
	if err := q.Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.ActivityEvent, len(recs))
	for i, r := range recs {
		e, err := r.toDomain()
		if err != nil {
			return nil, err
		}
		out[i] = e
	}
	return out, nil
}

func (a Activity) Activity(ctx context.Context, guildID uint64, q app.ActivityQuery) ([]domain.ActivityEvent, error) {
	query := facades.Orm().WithContext(ctx).Query().Where("guild_id", guildID)
	if q.EntityType != "" {
		query = query.Where("entity_type", q.EntityType)
	}
	if q.Actor.MemberID != 0 {
		query = query.Where("actor_member_id", q.Actor.MemberID)
	}
	if q.Actor.AgentID != 0 {
		query = query.Where("actor_agent_id", q.Actor.AgentID)
	}
	if q.Before != 0 {
		query = query.Where("id < ?", q.Before)
	}
	return a.find(query.Order("id DESC").Limit(q.Limit))
}

func (a Activity) IssueActivity(ctx context.Context, issueID uint64) ([]domain.ActivityEvent, error) {
	return a.find(facades.Orm().WithContext(ctx).Query().
		Where("entity_type", domain.IssueEntity).Where("entity_id", issueID).Order("id"))
}
