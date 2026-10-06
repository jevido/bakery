package http

import (
	"errors"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/guilds/app"
	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

type guildRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type currentGuildJSON struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Blocking names what keeps the Guild from being deleted.
	Blocking []string `json:"blocking"`
}

// guildFailure answers the errors of Guilds.
func guildFailure(ctx contractshttp.Context, err error) contractshttp.Response {
	var inUse app.ErrGuildInUse
	switch {
	case errors.Is(err, domain.ErrInvalidName):
		return respond.Invalid(ctx, "name", err.Error())
	case errors.Is(err, domain.ErrDescriptionTooLong):
		return respond.Invalid(ctx, "description", err.Error())
	case errors.Is(err, app.ErrGuildNotFound):
		return respond.Error(ctx, contractshttp.StatusNotFound, "guild not found")
	case errors.As(err, &inUse):
		return ctx.Response().Json(contractshttp.StatusConflict, contractshttp.Json{"message": inUse.Error(), "blocking": inUse.Blocking})
	}
	return respond.ServerError(ctx, err)
}

// Guilds lists the Guilds the Member may switch to, with their Permissions
// and former role in each (wireRole): every Guild for the Instance admin.
func (c *Controller) Guilds(ctx contractshttp.Context) contractshttp.Response {
	p := placeOf(ctx)
	places, err := c.service.GuildsFor(ctx.Context(), p.principal.MemberID, p.principal.InstanceAdmin)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"guilds": guildsJSON(places)})
}

func guildsJSON(places []app.Place) []guildJSON {
	out := make([]guildJSON, len(places))
	for i, pl := range places {
		out[i] = guildJSON{ID: pl.Guild.ID, Name: pl.Guild.Name, Role: wireRole(pl.Permissions), Permissions: pl.Permissions.Keys()}
	}
	return out
}

// CreateGuild makes a Guild with the Member holding Admin and switches the
// Session to it.
func (c *Controller) CreateGuild(ctx contractshttp.Context) contractshttp.Response {
	var req guildRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	g, err := c.service.CreateGuild(ctx.Context(), req.Name, req.Description, MemberID(ctx))
	admin := domain.Of(domain.PermissionAdministrator)
	if err != nil {
		return guildFailure(ctx, err)
	}
	SetCurrent(ctx, g.ID)
	return ctx.Response().Json(contractshttp.StatusCreated, contractshttp.Json{
		"guild": guildJSON{ID: g.ID, Name: g.Name, Role: wireRole(admin), Permissions: admin.Keys()},
	})
}

// CurrentGuild is the Current guild with what keeps it from being deleted.
func (c *Controller) CurrentGuild(ctx contractshttp.Context) contractshttp.Response {
	g, err := c.service.Guild(ctx.Context(), Current(ctx))
	if err != nil {
		return guildFailure(ctx, err)
	}
	blocking, err := c.service.Blocking(ctx.Context(), g.ID)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{
		"guild": currentGuildJSON{ID: g.ID, Name: g.Name, Description: g.Description, Blocking: blocking},
	})
}

// UpdateCurrentGuild renames and describes the Current guild.
func (c *Controller) UpdateCurrentGuild(ctx contractshttp.Context) contractshttp.Response {
	var req guildRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	g, err := c.service.UpdateGuild(ctx.Context(), Current(ctx), req.Name, req.Description)
	if err != nil {
		return guildFailure(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{
		"guild": currentGuildJSON{ID: g.ID, Name: g.Name, Description: g.Description, Blocking: []string{}},
	})
}

// DeleteCurrentGuild deletes the Current guild once it owns nothing, and
// forgets it as the Session's Current guild.
func (c *Controller) DeleteCurrentGuild(ctx contractshttp.Context) contractshttp.Response {
	id := Current(ctx)
	if err := c.service.DeleteGuild(ctx.Context(), id); err != nil {
		return guildFailure(ctx, err)
	}
	if cookieGuild(ctx) == id {
		forgetCurrent(ctx)
	}
	return ctx.Response().NoContent()
}

// SwitchGuild makes the Guild the Session's Current guild; 404 for one the
// Member may not act in.
func (c *Controller) SwitchGuild(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "guild not found")
	}
	p := placeOf(ctx)
	can, err := c.service.CanActIn(ctx.Context(), id, p.principal.MemberID, p.principal.InstanceAdmin)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	if !can {
		return respond.Error(ctx, contractshttp.StatusNotFound, "guild not found")
	}
	SetCurrent(ctx, id)
	return ctx.Response().NoContent()
}
