package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// SkillSummary is a Skill as the library lists it: without its files'
// bytes, with how many files it has, their size and how many Agents that
// are not terminated have it.
type SkillSummary struct {
	domain.Skill
	FileCount   int
	Size        int
	AgentsCount int
}

// SkillAgent is an Agent that has a Skill in its Agent skills.
type SkillAgent struct {
	ID     uint64
	Name   string
	Icon   domain.Icon
	Status domain.Status
}

// Skills keeps a Guild's Skills and their Skill files.
type Skills interface {
	// Skills lists the Guild's Skills, by name.
	Skills(ctx context.Context, guildID uint64) ([]SkillSummary, error)
	// Skill returns the Skill with every Skill file; found is false when
	// there is none.
	Skill(ctx context.Context, id uint64) (s domain.Skill, found bool, err error)
	// CreateSkill stores a new Skill with its files;
	// domain.ErrSkillSlugTaken when the Guild has its Skill slug.
	CreateSkill(ctx context.Context, s domain.Skill) (domain.Skill, error)
	// SaveSkillFile stores the Skill file at path, and the Skill's name,
	// description and updated time.
	SaveSkillFile(ctx context.Context, s domain.Skill, path string) error
	// DeleteSkillFile removes the Skill file at path, and stores the
	// Skill's updated time.
	DeleteSkillFile(ctx context.Context, s domain.Skill, path string) error
	// SkillAgents lists every Agent having the Skill, terminated ones
	// included, by name.
	SkillAgents(ctx context.Context, skillID uint64) ([]SkillAgent, error)
	// DeleteSkill removes the Skill after taking it from the Agents among
	// agentIDs, in one transaction.
	DeleteSkill(ctx context.Context, id uint64, agentIDs []uint64) error
	// SkillNames names the Guild's Skills among ids.
	SkillNames(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error)
}

// SkillInUseError refuses deleting a Skill Agents still have.
type SkillInUseError struct {
	Agents []SkillAgent
}

func (e *SkillInUseError) Error() string {
	names := make([]string, len(e.Agents))
	for i, a := range e.Agents {
		names[i] = a.Name
	}
	return fmt.Sprintf("the skill is still used by %s; take it off them first", strings.Join(names, ", "))
}

// Skills lists the Guild's Skills.
func (s *Service) Skills(ctx context.Context, guildID uint64) ([]SkillSummary, error) {
	return s.skills.Skills(ctx, guildID)
}

// Skill returns the Guild's Skill with its files; ErrNotFound for another
// Guild's or none.
func (s *Service) Skill(ctx context.Context, guildID, id uint64) (domain.Skill, error) {
	k, found, err := s.skills.Skill(ctx, id)
	if err != nil {
		return domain.Skill{}, err
	}
	if !found || k.GuildID != guildID {
		return domain.Skill{}, ErrNotFound
	}
	return k, nil
}

// SkillInGuild reports whether the Skill is the Guild's.
func (s *Service) SkillInGuild(ctx context.Context, id, guildID uint64) (bool, error) {
	k, found, err := s.skills.Skill(ctx, id)
	return found && k.GuildID == guildID, err
}

// SkillAgents lists the Agents that are not terminated and have the
// Skill.
func (s *Service) SkillAgents(ctx context.Context, skillID uint64) ([]SkillAgent, error) {
	all, err := s.skills.SkillAgents(ctx, skillID)
	if err != nil {
		return nil, err
	}
	out := make([]SkillAgent, 0, len(all))
	for _, a := range all {
		if a.Status != domain.Terminated {
			out = append(out, a)
		}
	}
	return out, nil
}

// SkillNames names the Guild's Skills among ids that still exist, for
// work's Activity.
func (s *Service) SkillNames(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error) {
	return s.skills.SkillNames(ctx, guildID, ids)
}

// SkillInput is a new Skill as typed: Slug and Markdown may be blank.
type SkillInput struct {
	Name, Slug, Description, Markdown string
}

// CreateSkill makes a Skill in the Guild by the Member.
func (s *Service) CreateSkill(ctx context.Context, guildID, memberID uint64, in SkillInput) (domain.Skill, error) {
	k, err := domain.NewSkill(guildID, in.Name, in.Slug, in.Description, in.Markdown, memberID, s.now())
	if err != nil {
		return domain.Skill{}, err
	}
	created, err := s.skills.CreateSkill(ctx, k)
	if err == domain.ErrSkillSlugTaken {
		return domain.Skill{}, &domain.FieldError{Field: "slug", Message: "another skill of the guild has it"}
	}
	if err != nil {
		return domain.Skill{}, err
	}
	s.recordSkill(ctx, domain.SkillCreated{Skill: created, ActorID: memberID})
	return created, nil
}

// SkillFileInput is one Skill file as sent: its content in its encoding.
type SkillFileInput struct {
	Path, Content, Encoding string
	Executable              bool
}

// WriteSkillFile creates or replaces one of the Guild's Skill's files by
// the Member; the same file again changes nothing and records nothing.
func (s *Service) WriteSkillFile(ctx context.Context, guildID, memberID, skillID uint64, in SkillFileInput) (domain.Skill, error) {
	k, err := s.Skill(ctx, guildID, skillID)
	if err != nil {
		return domain.Skill{}, err
	}
	content, err := domain.DecodeSkillContent(in.Content, in.Encoding)
	if err != nil {
		return domain.Skill{}, err
	}
	changed, err := k.WriteFile(in.Path, content, in.Executable, s.now())
	if err != nil || !changed {
		return k, err
	}
	if err := s.skills.SaveSkillFile(ctx, k, in.Path); err != nil {
		return domain.Skill{}, err
	}
	s.recordSkill(ctx, domain.SkillFileUpdated{Skill: k, ActorID: memberID, Path: in.Path})
	return k, nil
}

// DeleteSkillFile removes one of the Guild's Skill's files, never its
// SKILL.md, by the Member.
func (s *Service) DeleteSkillFile(ctx context.Context, guildID, memberID, skillID uint64, path string) (domain.Skill, error) {
	k, err := s.Skill(ctx, guildID, skillID)
	if err != nil {
		return domain.Skill{}, err
	}
	if err := k.DeleteFile(path, s.now()); err != nil {
		return domain.Skill{}, err
	}
	if err := s.skills.DeleteSkillFile(ctx, k, path); err != nil {
		return domain.Skill{}, err
	}
	s.recordSkill(ctx, domain.SkillFileDeleted{Skill: k, ActorID: memberID, Path: path})
	return k, nil
}

// DeleteSkill removes one of the Guild's Skills by the Member: a
// *SkillInUseError while an Agent that is not terminated has it. A
// terminated Agent's hold on it goes with it.
func (s *Service) DeleteSkill(ctx context.Context, guildID, memberID, skillID uint64) error {
	k, err := s.Skill(ctx, guildID, skillID)
	if err != nil {
		return err
	}
	all, err := s.skills.SkillAgents(ctx, skillID)
	if err != nil {
		return err
	}
	var inUse []SkillAgent
	var terminated []uint64
	for _, a := range all {
		if a.Status == domain.Terminated {
			terminated = append(terminated, a.ID)
		} else {
			inUse = append(inUse, a)
		}
	}
	if len(inUse) > 0 {
		return &SkillInUseError{Agents: inUse}
	}
	if err := s.skills.DeleteSkill(ctx, skillID, terminated); err != nil {
		return err
	}
	s.recordSkill(ctx, domain.SkillDeleted{Skill: k, ActorID: memberID})
	return nil
}

// recordSkill adds a Skill's domain event to work's Activity; a failure is
// logged, as the change it tells of is stored already.
func (s *Service) recordSkill(ctx context.Context, e any) {
	var k domain.Skill
	var actorID uint64
	var action, path string
	switch e := e.(type) {
	case domain.SkillCreated:
		k, actorID, action = e.Skill, e.ActorID, "skill.created"
	case domain.SkillFileUpdated:
		k, actorID, action, path = e.Skill, e.ActorID, "skill.file_updated", e.Path
	case domain.SkillFileDeleted:
		k, actorID, action, path = e.Skill, e.ActorID, "skill.file_deleted", e.Path
	case domain.SkillDeleted:
		k, actorID, action = e.Skill, e.ActorID, "skill.deleted"
	default:
		return
	}
	act := Activity{GuildID: k.GuildID, ActorID: actorID, Entity: "skill", EntityID: k.ID, Action: action, AgentName: k.Name, Details: map[string]any{"slug": k.Slug}}
	if path != "" {
		act.Details["path"] = path
	}
	if err := s.work.RecordActivity(ctx, act); err != nil {
		s.Logf("agents: recording %s of skill %d: %v", action, k.ID, err)
	}
}
