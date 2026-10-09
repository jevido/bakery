package infra

import (
	"context"
	"errors"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/agents/app"
	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

type skillRecord struct {
	ID                uint64 `gorm:"primaryKey"`
	GuildID           uint64
	Slug              string
	Name              string
	Description       string
	CreatedByMemberID *uint64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (skillRecord) TableName() string { return "skills" }

func (r skillRecord) toDomain() domain.Skill {
	return domain.Skill{
		ID: r.ID, GuildID: r.GuildID, Slug: r.Slug, Name: r.Name, Description: r.Description,
		CreatedBy: deref(r.CreatedByMemberID), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

type skillFileRecord struct {
	SkillID    uint64
	Path       string
	Content    []byte
	Executable bool
	UpdatedAt  time.Time
}

func (skillFileRecord) TableName() string { return "skill_files" }

// Skills keeps Skills, their Skill files and which Agents have them.
type Skills struct{}

func (Skills) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

// skillSummaryRecord repeats skillRecord's columns: a raw Scan does not
// fill an embedded struct.
type skillSummaryRecord struct {
	ID                uint64
	GuildID           uint64
	Slug              string
	Name              string
	Description       string
	CreatedByMemberID *uint64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	FileCount         int
	Size              int
	AgentsCount       int
}

func (s Skills) Skills(ctx context.Context, guildID uint64) ([]app.SkillSummary, error) {
	var recs []skillSummaryRecord
	err := s.query(ctx).Raw(`SELECT s.*,
			(SELECT count(*) FROM skill_files f WHERE f.skill_id = s.id) AS file_count,
			(SELECT COALESCE(sum(octet_length(f.content)), 0) FROM skill_files f WHERE f.skill_id = s.id) AS size,
			(SELECT count(*) FROM agent_skills a JOIN agents g ON g.id = a.agent_id
				WHERE a.skill_id = s.id AND g.status <> ?) AS agents_count
		FROM skills s WHERE s.guild_id = ? ORDER BY lower(s.name), s.id`, string(domain.Terminated), guildID).Scan(&recs)
	if err != nil {
		return nil, err
	}
	out := make([]app.SkillSummary, len(recs))
	for i, r := range recs {
		k := skillRecord{ID: r.ID, GuildID: r.GuildID, Slug: r.Slug, Name: r.Name, Description: r.Description,
			CreatedByMemberID: r.CreatedByMemberID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}.toDomain()
		out[i] = app.SkillSummary{Skill: k, FileCount: r.FileCount, Size: r.Size, AgentsCount: r.AgentsCount}
	}
	return out, nil
}

func (s Skills) Skill(ctx context.Context, id uint64) (domain.Skill, bool, error) {
	var rec skillRecord
	if err := s.query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Skill{}, false, nil
		}
		return domain.Skill{}, false, err
	}
	var files []skillFileRecord
	if err := s.query(ctx).Where("skill_id", id).Find(&files); err != nil {
		return domain.Skill{}, false, err
	}
	k := rec.toDomain()
	k.Files = make(map[string]domain.SkillFile, len(files))
	for _, f := range files {
		k.Files[f.Path] = domain.SkillFile{Content: f.Content, Executable: f.Executable, UpdatedAt: f.UpdatedAt}
	}
	return k, true, nil
}

func (s Skills) CreateSkill(ctx context.Context, k domain.Skill) (domain.Skill, error) {
	rec := skillRecord{
		GuildID: k.GuildID, Slug: k.Slug, Name: k.Name, Description: k.Description,
		CreatedByMemberID: nullable(k.CreatedBy), CreatedAt: k.CreatedAt, UpdatedAt: k.UpdatedAt,
	}
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		if err := tx.Create(&rec); err != nil {
			return err
		}
		for p, f := range k.Files {
			if _, err := tx.Exec(`INSERT INTO skill_files (skill_id, path, content, executable, updated_at) VALUES (?, ?, ?, ?, ?)`,
				rec.ID, p, f.Content, f.Executable, f.UpdatedAt); err != nil {
				return err
			}
		}
		return nil
	})
	if taken(err) {
		return domain.Skill{}, domain.ErrSkillSlugTaken
	}
	if err != nil {
		return domain.Skill{}, err
	}
	k.ID = rec.ID
	return k, nil
}

func (s Skills) SaveSkillFile(ctx context.Context, k domain.Skill, path string) error {
	f := k.Files[path]
	return facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		if _, err := tx.Exec(`INSERT INTO skill_files (skill_id, path, content, executable, updated_at) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (skill_id, path) DO UPDATE SET content = EXCLUDED.content, executable = EXCLUDED.executable,
				updated_at = EXCLUDED.updated_at`, k.ID, path, f.Content, f.Executable, f.UpdatedAt); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE skills SET name = ?, description = ?, updated_at = ? WHERE id = ?`, k.Name, k.Description, k.UpdatedAt, k.ID)
		return err
	})
}

func (s Skills) DeleteSkillFile(ctx context.Context, k domain.Skill, path string) error {
	return facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		if _, err := tx.Exec(`DELETE FROM skill_files WHERE skill_id = ? AND path = ?`, k.ID, path); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE skills SET updated_at = ? WHERE id = ?`, k.UpdatedAt, k.ID)
		return err
	})
}

type skillAgentRecord struct {
	ID     uint64
	Name   string
	Icon   string
	Status string
}

func (s Skills) SkillAgents(ctx context.Context, skillID uint64) ([]app.SkillAgent, error) {
	var recs []skillAgentRecord
	if err := s.query(ctx).Raw(`SELECT g.id, g.name, g.icon, g.status FROM agent_skills a JOIN agents g ON g.id = a.agent_id
		WHERE a.skill_id = ? ORDER BY lower(g.name), g.id`, skillID).Scan(&recs); err != nil {
		return nil, err
	}
	out := make([]app.SkillAgent, len(recs))
	for i, r := range recs {
		out[i] = app.SkillAgent{ID: r.ID, Name: r.Name, Icon: domain.Icon(r.Icon), Status: domain.Status(r.Status)}
	}
	return out, nil
}

// DeleteSkill takes the Skill from the given Agents first: agent_skills
// restricts deleting a Skill any Agent still has, so one that gained it
// meanwhile makes the delete fail instead of losing it silently.
func (s Skills) DeleteSkill(ctx context.Context, id uint64, agentIDs []uint64) error {
	return facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		if len(agentIDs) > 0 {
			if _, err := tx.Exec(`DELETE FROM agent_skills WHERE skill_id = ? AND agent_id IN ?`, id, agentIDs); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`DELETE FROM skills WHERE id = ?`, id)
		return err
	})
}

func (s Skills) SkillNames(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error) {
	out := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var recs []skillRecord
	if err := s.query(ctx).Where("guild_id", guildID).Where("id IN ?", ids).Find(&recs); err != nil {
		return nil, err
	}
	for _, r := range recs {
		out[r.ID] = r.Name
	}
	return out, nil
}
