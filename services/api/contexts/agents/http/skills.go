package http

import (
	"context"
	"errors"
	"slices"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/agents/app"
	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// skillJSON is a Skill on the wire, as the library lists it.
type skillJSON struct {
	ID          uint64    `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	FileCount   int       `json:"file_count"`
	Size        int       `json:"size"`
	AgentsCount int       `json:"agents_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedBy   *Named    `json:"created_by"`
}

type skillFileJSON struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Size       int    `json:"size"`
	Encoding   string `json:"encoding"`
	Executable bool   `json:"executable"`
}

type skillAgentJSON struct {
	ID     uint64 `json:"id"`
	Name   string `json:"name"`
	Icon   string `json:"icon"`
	Status string `json:"status"`
}

// skillDetailJSON is one Skill with its files, by path, and the Agents
// that have it.
type skillDetailJSON struct {
	skillJSON
	Files  []skillFileJSON  `json:"files"`
	Agents []skillAgentJSON `json:"agents"`
}

// creators names the Members who created the Skills; a removed one is
// left out.
func (c *Controller) creators(ctx context.Context, ks []domain.Skill) (map[uint64]Named, error) {
	var ids []uint64
	for _, k := range ks {
		if k.CreatedBy != 0 && !slices.Contains(ids, k.CreatedBy) {
			ids = append(ids, k.CreatedBy)
		}
	}
	out := map[uint64]Named{}
	if len(ids) == 0 || c.Members == nil {
		return out, nil
	}
	ns, err := c.Members(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, n := range ns {
		out[n.ID] = n
	}
	return out, nil
}

func skillOf(s app.SkillSummary, creators map[uint64]Named) skillJSON {
	out := skillJSON{
		ID: s.ID, Slug: s.Slug, Name: s.Name, Description: s.Description, FileCount: s.FileCount, Size: s.Size,
		AgentsCount: s.AgentsCount, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
	if n, ok := creators[s.CreatedBy]; ok {
		out.CreatedBy = &n
	}
	return out
}

// skillDetail answers the Skill with its files and the Agents that are
// not terminated and have it.
func (c *Controller) skillDetail(ctx contractshttp.Context, k domain.Skill) (skillDetailJSON, error) {
	agents, err := c.service.SkillAgents(ctx.Context(), k.ID)
	if err != nil {
		return skillDetailJSON{}, err
	}
	creators, err := c.creators(ctx.Context(), []domain.Skill{k})
	if err != nil {
		return skillDetailJSON{}, err
	}
	out := skillDetailJSON{
		skillJSON: skillOf(app.SkillSummary{Skill: k, FileCount: len(k.Files), Size: k.Size(), AgentsCount: len(agents)}, creators),
		Files:     make([]skillFileJSON, 0, len(k.Files)),
		Agents:    make([]skillAgentJSON, len(agents)),
	}
	for p, f := range k.Files {
		_, encoding := domain.EncodeSkillContent(f.Content)
		out.Files = append(out.Files, skillFileJSON{Path: p, Kind: domain.SkillFileKind(p, f.Executable), Size: len(f.Content), Encoding: encoding, Executable: f.Executable})
	}
	// SKILL.md first, then the others by path, as Paperclip's file tree.
	slices.SortFunc(out.Files, func(a, b skillFileJSON) int {
		switch {
		case a.Path == domain.SkillMarkdown:
			return -1
		case b.Path == domain.SkillMarkdown:
			return 1
		case a.Path < b.Path:
			return -1
		case a.Path > b.Path:
			return 1
		}
		return 0
	})
	for i, a := range agents {
		out.Agents[i] = skillAgentJSON{ID: a.ID, Name: a.Name, Icon: string(a.Icon), Status: string(a.Status)}
	}
	return out, nil
}

func (c *Controller) skillResponse(ctx contractshttp.Context, status int, k domain.Skill) contractshttp.Response {
	d, err := c.skillDetail(ctx, k)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Json(status, contractshttp.Json{"skill": d})
}

// ListSkills answers the Current guild's Skills, by name.
func (c *Controller) ListSkills(ctx contractshttp.Context) contractshttp.Response {
	ss, err := c.service.Skills(ctx.Context(), c.Guild(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	ks := make([]domain.Skill, len(ss))
	for i, s := range ss {
		ks[i] = s.Skill
	}
	creators, err := c.creators(ctx.Context(), ks)
	if err != nil {
		return fail(ctx, err)
	}
	out := make([]skillJSON, len(ss))
	for i, s := range ss {
		out[i] = skillOf(s, creators)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"skills": out})
}

// ShowSkill answers one of the Current guild's Skills with its files and
// Agents.
func (c *Controller) ShowSkill(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	k, err := c.service.Skill(ctx.Context(), c.Guild(ctx), id)
	if err != nil {
		return fail(ctx, err)
	}
	return c.skillResponse(ctx, contractshttp.StatusOK, k)
}

// ShowSkillFile answers one Skill file's content, utf8 or base64.
func (c *Controller) ShowSkillFile(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	k, err := c.service.Skill(ctx.Context(), c.Guild(ctx), id)
	if err != nil {
		return fail(ctx, err)
	}
	p := ctx.Request().Query("path", "")
	f, ok := k.Files[p]
	if !ok {
		return notFound(ctx)
	}
	content, encoding := domain.EncodeSkillContent(f.Content)
	return ctx.Response().Success().Json(contractshttp.Json{"file": contractshttp.Json{
		"path": p, "content": content, "encoding": encoding, "executable": f.Executable, "size": len(f.Content),
	}})
}

type skillRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Markdown    string `json:"markdown"`
}

// CreateSkill makes a Skill in the Current guild, its SKILL.md the given
// Markdown or one generated for its name and description.
func (c *Controller) CreateSkill(ctx contractshttp.Context) contractshttp.Response {
	var req skillRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	k, err := c.service.CreateSkill(ctx.Context(), c.Guild(ctx), c.Member(ctx), app.SkillInput{
		Name: req.Name, Slug: req.Slug, Description: req.Description, Markdown: req.Markdown,
	})
	if err != nil {
		return fail(ctx, err)
	}
	return c.skillResponse(ctx, contractshttp.StatusCreated, k)
}

type skillFileRequest struct {
	Path       string  `json:"path"`
	Content    *string `json:"content"`
	Encoding   string  `json:"encoding"`
	Executable bool    `json:"executable"`
}

// WriteSkillFile creates or replaces one file of one of the Current
// guild's Skills.
func (c *Controller) WriteSkillFile(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req skillFileRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	if req.Content == nil {
		return respond.Invalid(ctx, "content", "is required")
	}
	k, err := c.service.WriteSkillFile(ctx.Context(), c.Guild(ctx), c.Member(ctx), id, app.SkillFileInput{
		Path: req.Path, Content: *req.Content, Encoding: req.Encoding, Executable: req.Executable,
	})
	if err != nil {
		return fail(ctx, err)
	}
	return c.skillResponse(ctx, contractshttp.StatusOK, k)
}

// DeleteSkillFile removes one file, never SKILL.md, of one of the Current
// guild's Skills.
func (c *Controller) DeleteSkillFile(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	k, err := c.service.DeleteSkillFile(ctx.Context(), c.Guild(ctx), c.Member(ctx), id, ctx.Request().Query("path", ""))
	if err != nil {
		return fail(ctx, err)
	}
	return c.skillResponse(ctx, contractshttp.StatusOK, k)
}

// DeleteSkill removes one of the Current guild's Skills; while Agents
// still have it, 422 names them.
func (c *Controller) DeleteSkill(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	err := c.service.DeleteSkill(ctx.Context(), c.Guild(ctx), c.Member(ctx), id)
	var inUse *app.SkillInUseError
	if errors.As(err, &inUse) {
		agents := make([]Named, len(inUse.Agents))
		for i, a := range inUse.Agents {
			agents[i] = Named{ID: a.ID, Name: a.Name}
		}
		return ctx.Response().Json(contractshttp.StatusUnprocessableEntity, contractshttp.Json{"message": err.Error(), "agents": agents})
	}
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().NoContent()
}
