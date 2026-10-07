// Package infra stores Guilds, their Roles, Memberships and Invitations
// with the Goravel ORM.
package infra

import (
	"context"
	"errors"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/guilds/app"
	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

type guildRecord struct {
	ID          uint64 `gorm:"primaryKey"`
	Name        string
	Description string
	MasterID    uint64
	orm.Timestamps
}

func (guildRecord) TableName() string { return "guilds" }

func (r guildRecord) toDomain() domain.Guild {
	return domain.Guild{ID: r.ID, Name: r.Name, Description: r.Description, MasterID: r.MasterID}
}

type membershipRecord struct {
	ID      uint64 `gorm:"primaryKey"`
	GuildID uint64
	UserID  uint64
	orm.Timestamps
}

func (membershipRecord) TableName() string { return "memberships" }

// membershipRoleRecord is one Role a Membership holds; the Base role is
// never stored.
type membershipRoleRecord struct {
	MembershipID uint64
	RoleID       uint64
}

func (membershipRoleRecord) TableName() string { return "membership_roles" }

type roleRecord struct {
	ID          uint64 `gorm:"primaryKey"`
	GuildID     uint64
	Name        string
	Color       string
	Position    int
	Permissions int64
	Base        bool
	orm.Timestamps
}

func (roleRecord) TableName() string { return "roles" }

func (r roleRecord) toDomain() domain.Role {
	return domain.Role{
		ID: r.ID, GuildID: r.GuildID, Name: r.Name, Color: r.Color, Position: r.Position,
		Permissions: domain.Permissions(r.Permissions), Base: r.Base,
	}
}

func toRoleRecord(r domain.Role) roleRecord {
	return roleRecord{
		ID: r.ID, GuildID: r.GuildID, Name: r.Name, Color: r.Color, Position: r.Position,
		Permissions: int64(r.Permissions), Base: r.Base,
	}
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

func (Guilds) Create(ctx context.Context, g domain.Guild) (domain.Guild, error) {
	var rec guildRecord
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		var err error
		rec, err = create(tx, g)
		return err
	})
	if err != nil {
		return domain.Guild{}, err
	}
	return rec.toDomain(), nil
}

func (Guilds) CreateFirstIfNone(ctx context.Context, g domain.Guild) (domain.Guild, bool, error) {
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
		rec, err = create(tx, g)
		created = err == nil
		return err
	})
	if err != nil || !created {
		return domain.Guild{}, false, err
	}
	return rec.toDomain(), true, nil
}

func (Guilds) MasteredBy(ctx context.Context, memberID uint64) (bool, error) {
	n, err := query(ctx).Model(&guildRecord{}).Where("master_id", memberID).Count()
	return n > 0, err
}

func (Guilds) Update(ctx context.Context, g domain.Guild) error {
	_, err := query(ctx).Model(&guildRecord{}).Where("id", g.ID).Update(map[string]any{"name": g.Name, "description": g.Description})
	return err
}

// Delete relies on the foreign keys: Memberships, Invitations, API tokens
// and Known hosts cascade with the Guild, while a Project, Server, S3
// storage or Notification channel still in it refuses the delete. That
// closes the gap between DeleteGuild's checks and the delete.
func (Guilds) Delete(ctx context.Context, id uint64) error {
	_, err := query(ctx).Where("id", id).Delete(&guildRecord{})
	if isForeignKeyViolation(err) {
		return app.ErrGuildInUse{}
	}
	return err
}

// create stores the Guild, its seeded Roles and its Guild Master's
// Membership holding Admin.
func create(tx contractsorm.Query, g domain.Guild) (guildRecord, error) {
	rec := guildRecord{Name: g.Name, Description: g.Description, MasterID: g.MasterID}
	if err := tx.Create(&rec); err != nil {
		return guildRecord{}, err
	}
	var admin uint64
	for _, r := range domain.SeedRoles(rec.ID) {
		rr := toRoleRecord(r)
		if err := tx.Create(&rr); err != nil {
			return guildRecord{}, err
		}
		if r.Name == domain.AdminRole {
			admin = rr.ID
		}
	}
	m := membershipRecord{GuildID: rec.ID, UserID: g.MasterID}
	if err := tx.Create(&m); err != nil {
		return guildRecord{}, err
	}
	return rec, tx.Create(&membershipRoleRecord{MembershipID: m.ID, RoleID: admin})
}

type Memberships struct{}

func (Memberships) ListForMember(ctx context.Context, memberID uint64) ([]domain.Membership, error) {
	q := query(ctx)
	return list(q, q.Where("user_id", memberID).OrderBy("guild_id"))
}

func (Memberships) ListForGuild(ctx context.Context, guildID uint64) ([]domain.Membership, error) {
	q := query(ctx)
	return list(q, q.Where("guild_id", guildID).OrderBy("id"))
}

func (Memberships) Of(ctx context.Context, guildID, memberID uint64) (domain.Membership, bool, error) {
	q := query(ctx)
	ms, err := list(q, q.Where("guild_id", guildID).Where("user_id", memberID))
	if err != nil || len(ms) == 0 {
		return domain.Membership{}, false, err
	}
	return ms[0], true, nil
}

// list finds the Memberships q selects and, through tx, the Roles each
// holds.
func list(tx, q contractsorm.Query) ([]domain.Membership, error) {
	var recs []membershipRecord
	if err := q.Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Membership, len(recs))
	if len(recs) == 0 {
		return out, nil
	}
	ids := make([]any, len(recs))
	at := make(map[uint64]int, len(recs))
	for i, r := range recs {
		out[i] = domain.Membership{ID: r.ID, GuildID: r.GuildID, MemberID: r.UserID}
		ids[i], at[r.ID] = r.ID, i
	}
	var held []membershipRoleRecord
	if err := tx.Model(&membershipRoleRecord{}).WhereIn("membership_id", ids).OrderBy("role_id").Find(&held); err != nil {
		return nil, err
	}
	for _, h := range held {
		i := at[h.MembershipID]
		out[i].RoleIDs = append(out[i].RoleIDs, h.RoleID)
	}
	return out, nil
}

// lockGuild reads the Guild and locks its row until tx ends.
func lockGuild(tx contractsorm.Query, id uint64) (domain.Guild, error) {
	var rec guildRecord
	if err := tx.Where("id", id).LockForUpdate().FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Guild{}, app.ErrGuildNotFound
		}
		return domain.Guild{}, err
	}
	return rec.toDomain(), nil
}

type Roles struct{}

func (Roles) ForGuild(ctx context.Context, guildID uint64) ([]domain.Role, error) {
	var recs []roleRecord
	if err := query(ctx).Where("guild_id", guildID).OrderBy("position").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Role, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

func (Roles) Change(ctx context.Context, guildID uint64, decide func(app.Hierarchy) (app.Change, error)) (app.Change, error) {
	var c app.Change
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		// Locks the Guild first, then its Roles and Memberships, so a
		// concurrent change or accepted Transfer offer waits and then
		// decides on what this one left.
		g, err := lockGuild(tx, guildID)
		if err != nil {
			return err
		}
		var recs []roleRecord
		if err := tx.Where("guild_id", guildID).OrderBy("position").LockForUpdate().Find(&recs); err != nil {
			return err
		}
		h := app.Hierarchy{Guild: g, Roles: make([]domain.Role, len(recs))}
		for i, r := range recs {
			h.Roles[i] = r.toDomain()
		}
		if h.Memberships, err = list(tx, tx.Where("guild_id", guildID).OrderBy("id").LockForUpdate()); err != nil {
			return err
		}
		if c, err = decide(h); err != nil {
			return err
		}
		return store(tx, guildID, &c)
	})
	if err != nil {
		return app.Change{}, err
	}
	return c, nil
}

// store writes c for the Guild in tx, giving created Roles their ids.
// Positions are unique only at commit, so Roles can swap them here.
func store(tx contractsorm.Query, guildID uint64, c *app.Change) error {
	if c.DeletedRole != 0 {
		// The Memberships and Invitations holding it lose it by cascade.
		if _, err := tx.Exec("DELETE FROM roles WHERE id = ? AND guild_id = ? AND NOT base", c.DeletedRole, guildID); err != nil {
			return err
		}
	}
	for i, r := range c.Roles {
		r.GuildID = guildID
		rec := toRoleRecord(r)
		if r.ID == 0 {
			if err := tx.Create(&rec); err != nil {
				return err
			}
			c.Roles[i] = rec.toDomain()
			continue
		}
		if _, err := tx.Model(&roleRecord{}).Where("id", r.ID).Where("guild_id", guildID).Update(map[string]any{
			"name": rec.Name, "color": rec.Color, "position": rec.Position, "permissions": rec.Permissions,
		}); err != nil {
			return err
		}
	}
	if m := c.Membership; m != nil {
		if _, err := tx.Exec("DELETE FROM membership_roles WHERE membership_id = ?", m.ID); err != nil {
			return err
		}
		for _, id := range m.RoleIDs {
			if err := tx.Create(&membershipRoleRecord{MembershipID: m.ID, RoleID: id}); err != nil {
				return err
			}
		}
	}
	if c.Override != nil {
		c.Override.GuildID = guildID
		if err := setOverride(tx, *c.Override); err != nil {
			return err
		}
	}
	if c.RemovedMembership != 0 {
		if _, err := tx.Exec("DELETE FROM permission_overrides WHERE guild_id = ? AND member_id = (SELECT user_id FROM memberships WHERE id = ?)", guildID, c.RemovedMembership); err != nil {
			return err
		}
		if _, err := tx.Where("id", c.RemovedMembership).Where("guild_id", guildID).Delete(&membershipRecord{}); err != nil {
			return err
		}
	}
	return nil
}
