package app

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// fakeSkills keeps Skills in memory; holders are the Agents having each.
type fakeSkills struct {
	rows    map[uint64]domain.Skill
	holders map[uint64][]SkillAgent
	next    uint64
}

func (f *fakeSkills) init() {
	if f.rows == nil {
		f.rows, f.holders = map[uint64]domain.Skill{}, map[uint64][]SkillAgent{}
	}
}

func (f *fakeSkills) Skills(_ context.Context, guildID uint64) ([]SkillSummary, error) {
	f.init()
	var out []SkillSummary
	for _, k := range f.rows {
		if k.GuildID == guildID {
			out = append(out, SkillSummary{Skill: k, FileCount: len(k.Files), Size: k.Size()})
		}
	}
	return out, nil
}

func (f *fakeSkills) Skill(_ context.Context, id uint64) (domain.Skill, bool, error) {
	f.init()
	k, ok := f.rows[id]
	k.Files = maps.Clone(k.Files)
	return k, ok, nil
}

func (f *fakeSkills) CreateSkill(_ context.Context, k domain.Skill) (domain.Skill, error) {
	f.init()
	for _, o := range f.rows {
		if o.GuildID == k.GuildID && o.Slug == k.Slug {
			return domain.Skill{}, domain.ErrSkillSlugTaken
		}
	}
	f.next++
	k.ID = f.next
	f.rows[k.ID] = k
	return k, nil
}

func (f *fakeSkills) SaveSkillFile(_ context.Context, k domain.Skill, path string) error {
	o := f.rows[k.ID]
	o.Files = maps.Clone(o.Files)
	o.Files[path] = k.Files[path]
	o.Name, o.Description, o.UpdatedAt = k.Name, k.Description, k.UpdatedAt
	f.rows[k.ID] = o
	return nil
}

func (f *fakeSkills) DeleteSkillFile(_ context.Context, k domain.Skill, path string) error {
	o := f.rows[k.ID]
	o.Files = maps.Clone(o.Files)
	delete(o.Files, path)
	f.rows[k.ID] = o
	return nil
}

func (f *fakeSkills) SkillAgents(_ context.Context, skillID uint64) ([]SkillAgent, error) {
	f.init()
	return f.holders[skillID], nil
}

func (f *fakeSkills) DeleteSkill(_ context.Context, id uint64, agentIDs []uint64) error {
	left := slices.DeleteFunc(slices.Clone(f.holders[id]), func(a SkillAgent) bool { return slices.Contains(agentIDs, a.ID) })
	if len(left) > 0 {
		return errors.New("agent_skills restricts the delete")
	}
	delete(f.rows, id)
	delete(f.holders, id)
	return nil
}

func (f *fakeSkills) SkillNames(_ context.Context, guildID uint64, ids []uint64) (map[uint64]string, error) {
	out := map[uint64]string{}
	for _, id := range ids {
		if k, ok := f.rows[id]; ok && k.GuildID == guildID {
			out[id] = k.Name
		}
	}
	return out, nil
}

func newSkillTest() (*Service, *fakeSkills, *fakeWork) {
	s, _, _, w := newTest()
	k := &fakeSkills{}
	s.skills = k
	return s, k, w
}

func TestCreateSkill(t *testing.T) {
	ctx := context.Background()
	s, _, w := newSkillTest()
	k, err := s.CreateSkill(ctx, 1, 7, SkillInput{Name: "Release notes", Description: "Write release notes"})
	if err != nil || k.Slug != "release-notes" || k.Files[domain.SkillMarkdown].Content == nil || k.CreatedBy != 7 {
		t.Fatalf("create: %+v %v", k, err)
	}
	if w.last.Action != "skill.created" || w.last.Entity != "skill" || w.last.EntityID != k.ID || w.last.AgentName != "Release notes" || w.last.Details["slug"] != "release-notes" {
		t.Fatalf("activity: %+v", w.last)
	}
	var fe *domain.FieldError
	if _, err := s.CreateSkill(ctx, 1, 7, SkillInput{Name: "Release notes"}); !errors.As(err, &fe) || fe.Field != "slug" {
		t.Fatalf("a taken slug: %v", err)
	}
	if _, err := s.CreateSkill(ctx, 2, 7, SkillInput{Name: "Release notes"}); err != nil {
		t.Fatalf("the same slug in another guild: %v", err)
	}
	if _, err := s.Skill(ctx, 2, k.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another guild's skill: %v", err)
	}
}

func TestSkillFiles(t *testing.T) {
	ctx := context.Background()
	s, store, w := newSkillTest()
	k, _ := s.CreateSkill(ctx, 1, 7, SkillInput{Name: "Notes"})
	before := len(w.actions)
	got, err := s.WriteSkillFile(ctx, 1, 7, k.ID, SkillFileInput{Path: "templates/notes.md", Content: "# Notes"})
	if err != nil || len(got.Files) != 2 || len(store.rows[k.ID].Files) != 2 {
		t.Fatalf("write: %+v %v", got, err)
	}
	if w.last.Action != "skill.file_updated" || w.last.Details["path"] != "templates/notes.md" {
		t.Fatalf("activity: %+v", w.last)
	}
	if _, err := s.WriteSkillFile(ctx, 1, 7, k.ID, SkillFileInput{Path: "templates/notes.md", Content: "# Notes"}); err != nil || len(w.actions) != before+1 {
		t.Fatalf("the same file again recorded: %v %v", w.actions, err)
	}
	if _, err := s.WriteSkillFile(ctx, 1, 7, k.ID, SkillFileInput{Path: "bin", Content: "AAE=", Encoding: "base64", Executable: true}); err != nil || string(store.rows[k.ID].Files["bin"].Content) != "\x00\x01" {
		t.Fatalf("base64: %v", err)
	}
	if _, err := s.WriteSkillFile(ctx, 1, 7, k.ID, SkillFileInput{Path: "SKILL.md", Content: "---\nname: Renamed\n---\n"}); err != nil || store.rows[k.ID].Name != "Renamed" {
		t.Fatalf("renaming through SKILL.md: %+v %v", store.rows[k.ID], err)
	}
	if _, err := s.DeleteSkillFile(ctx, 1, 7, k.ID, "SKILL.md"); err == nil {
		t.Fatal("deleted SKILL.md")
	}
	if _, err := s.DeleteSkillFile(ctx, 1, 7, k.ID, "templates/notes.md"); err != nil || w.last.Action != "skill.file_deleted" || len(store.rows[k.ID].Files) != 2 {
		t.Fatalf("delete a file: %v %+v", err, w.last)
	}
	if _, err := s.WriteSkillFile(ctx, 2, 7, k.ID, SkillFileInput{Path: "x", Content: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("writing another guild's skill: %v", err)
	}
}

func TestDeleteSkillInUse(t *testing.T) {
	ctx := context.Background()
	s, store, w := newSkillTest()
	k, _ := s.CreateSkill(ctx, 1, 7, SkillInput{Name: "Notes"})
	store.holders[k.ID] = []SkillAgent{{ID: 3, Name: "Release Bot", Status: domain.Idle}, {ID: 4, Name: "Old Bot", Status: domain.Terminated}}
	var inUse *SkillInUseError
	if err := s.DeleteSkill(ctx, 1, 7, k.ID); !errors.As(err, &inUse) || len(inUse.Agents) != 1 || inUse.Agents[0].Name != "Release Bot" {
		t.Fatalf("delete in use: %v", err)
	}
	if as, _ := s.SkillAgents(ctx, k.ID); len(as) != 1 || as[0].ID != 3 {
		t.Fatalf("skill agents: %+v", as)
	}
	store.holders[k.ID] = store.holders[k.ID][1:]
	if err := s.DeleteSkill(ctx, 1, 7, k.ID); err != nil || w.last.Action != "skill.deleted" {
		t.Fatalf("delete held only by a terminated agent: %v", err)
	}
	if _, ok := store.rows[k.ID]; ok {
		t.Fatal("the skill is left")
	}
}
