package infra

import (
	"context"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

// overrideRecord is a Permission override; exactly one of RoleID and
// MemberID is set.
type overrideRecord struct {
	ID        uint64 `gorm:"primaryKey"`
	GuildID   uint64
	ProjectID uint64
	RoleID    *uint64
	MemberID  *uint64
	Allow     int64
	Deny      int64
	orm.Timestamps
}

func (overrideRecord) TableName() string { return "permission_overrides" }

func (r overrideRecord) toDomain() domain.Override {
	o := domain.Override{ID: r.ID, GuildID: r.GuildID, ProjectID: r.ProjectID,
		Allow: domain.Permissions(r.Allow), Deny: domain.Permissions(r.Deny)}
	if r.RoleID != nil {
		o.RoleID = *r.RoleID
	}
	if r.MemberID != nil {
		o.MemberID = *r.MemberID
	}
	return o
}

type Overrides struct{}

func (Overrides) ForGuild(ctx context.Context, guildID uint64) ([]domain.Override, error) {
	return findOverrides(query(ctx).Where("guild_id", guildID))
}

func (Overrides) ForProject(ctx context.Context, guildID, projectID uint64) ([]domain.Override, error) {
	return findOverrides(query(ctx).Where("guild_id", guildID).Where("project_id", projectID))
}

func findOverrides(q contractsorm.Query) ([]domain.Override, error) {
	var recs []overrideRecord
	if err := q.OrderBy("id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Override, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// setOverride stores o in place of the Project's Override for the same
// Role or Member, or deletes that one when o is empty.
func setOverride(tx contractsorm.Query, o domain.Override) error {
	who, id := "role_id", o.RoleID
	if o.MemberID != 0 {
		who, id = "member_id", o.MemberID
	}
	if o.Empty() {
		_, err := tx.Exec("DELETE FROM permission_overrides WHERE project_id = ? AND "+who+" = ?", o.ProjectID, id)
		return err
	}
	_, err := tx.Exec(
		"INSERT INTO permission_overrides (guild_id, project_id, "+who+", allow, deny, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, now(), now()) "+
			"ON CONFLICT (project_id, "+who+") WHERE "+who+" IS NOT NULL "+
			"DO UPDATE SET allow = excluded.allow, deny = excluded.deny, updated_at = now()",
		o.GuildID, o.ProjectID, id, int64(o.Allow), int64(o.Deny))
	return err
}

func (Overrides) ForgetProject(ctx context.Context, projectID uint64) error {
	_, err := query(ctx).Exec("DELETE FROM permission_overrides WHERE project_id = ?", projectID)
	return err
}
