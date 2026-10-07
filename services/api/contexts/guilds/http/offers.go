package http

import (
	"context"
	"errors"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/guilds/app"
	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
	"github.com/jevido/bakery/services/api/contexts/identity"
)

// personJSON names a Member on a Guild Master or a Transfer offer.
type personJSON struct {
	ID    uint64 `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type offerGuildJSON struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

type offerJSON struct {
	ID        uint64          `json:"id"`
	Guild     *offerGuildJSON `json:"guild,omitempty"`
	From      *personJSON     `json:"from"`
	To        *personJSON     `json:"to"`
	CreatedAt time.Time       `json:"created_at"`
	ExpiresAt time.Time       `json:"expires_at"`
}

// person is the Member by id, nil when they are gone.
func person(ctx contractshttp.Context, id uint64) (*personJSON, error) {
	m, found, err := identity.MemberByID(ctx.Context(), id)
	if err != nil || !found {
		return nil, err
	}
	return &personJSON{ID: m.ID, Name: m.Name, Email: m.Email}, nil
}

func toOfferJSON(ctx contractshttp.Context, o domain.Offer) (*offerJSON, error) {
	from, err := person(ctx, o.FromID)
	if err != nil {
		return nil, err
	}
	to, err := person(ctx, o.ToID)
	if err != nil {
		return nil, err
	}
	return &offerJSON{ID: o.ID, From: from, To: to, CreatedAt: o.CreatedAt, ExpiresAt: o.ExpiresAt}, nil
}

// openOffer is the Guild's open Transfer offer as the wire shows it, nil
// without one.
func (c *Controller) openOffer(ctx contractshttp.Context, guildID uint64) (*offerJSON, error) {
	o, open, err := c.service.OpenOffer(ctx.Context(), guildID)
	if err != nil || !open {
		return nil, err
	}
	return toOfferJSON(ctx, o)
}

// offersTo lists the open Transfer offers to the Member with the Guild of
// each, for `GET /api/me`.
func (c *Controller) offersTo(ctx contractshttp.Context, memberID uint64) ([]offerJSON, error) {
	os, err := c.service.OffersTo(ctx.Context(), memberID)
	if err != nil {
		return nil, err
	}
	out := make([]offerJSON, 0, len(os))
	for _, o := range os {
		g, err := c.service.Guild(ctx.Context(), o.GuildID)
		if err != nil {
			return nil, err
		}
		j, err := toOfferJSON(ctx, o)
		if err != nil {
			return nil, err
		}
		j.Guild = &offerGuildJSON{ID: g.ID, Name: g.Name}
		out = append(out, *j)
	}
	return out, nil
}

type offerRequest struct {
	MemberID uint64 `json:"member_id"`
}

// OfferGuildMaster is the Current guild's Guild Master offering it to
// another Member of the Guild.
func (c *Controller) OfferGuildMaster(ctx contractshttp.Context) contractshttp.Response {
	var req offerRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	o, err := c.service.OfferGuildMaster(ctx.Context(), Current(ctx), MemberID(ctx), req.MemberID)
	if err != nil {
		return offerFailure(ctx, err)
	}
	j, err := toOfferJSON(ctx, o)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusCreated, contractshttp.Json{"offer": j})
}

// WithdrawOffer is the Guild Master taking the Current guild's open offer
// back.
func (c *Controller) WithdrawOffer(ctx contractshttp.Context) contractshttp.Response {
	if err := c.service.WithdrawOffer(ctx.Context(), Current(ctx), MemberID(ctx)); err != nil {
		return offerFailure(ctx, err)
	}
	return ctx.Response().NoContent()
}

// AcceptOffer is the offered Member taking the Guild Master; the offer may
// be in a Guild other than the Current one.
func (c *Controller) AcceptOffer(ctx contractshttp.Context) contractshttp.Response {
	return c.closeOffer(ctx, c.service.AcceptOffer)
}

// DeclineOffer is the offered Member turning the offer down.
func (c *Controller) DeclineOffer(ctx contractshttp.Context) contractshttp.Response {
	return c.closeOffer(ctx, c.service.DeclineOffer)
}

func (c *Controller) closeOffer(ctx contractshttp.Context, close func(ctx context.Context, id, by uint64) error) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "offer not found")
	}
	if err := close(ctx.Context(), id, MemberID(ctx)); err != nil {
		return offerFailure(ctx, err)
	}
	return ctx.Response().NoContent()
}

// offerFailure answers the errors of Transfer offers.
func offerFailure(ctx contractshttp.Context, err error) contractshttp.Response {
	status, field := offerStatus(err)
	switch {
	case status == contractshttp.StatusNotFound:
		return respond.Error(ctx, status, "offer not found")
	case field != "":
		return respond.Invalid(ctx, field, err.Error())
	case status != 0:
		return respond.Error(ctx, status, err.Error())
	}
	return guildFailure(ctx, err)
}

// offerStatus is the status a Transfer offer's error answers with, and the
// field it is about when it is a 422; 0 for an error that is not an
// offer's. Someone the offer is not for learns nothing about it: 404, as
// for no offer at all.
func offerStatus(err error) (int, string) {
	switch {
	case errors.Is(err, app.ErrOfferNotFound), errors.Is(err, domain.ErrNotOffered):
		return contractshttp.StatusNotFound, ""
	case errors.Is(err, domain.ErrNotGuildMaster):
		return contractshttp.StatusForbidden, ""
	case errors.Is(err, domain.ErrOfferToSelf), errors.Is(err, domain.ErrOfferNotMember):
		return contractshttp.StatusUnprocessableEntity, "member_id"
	case errors.Is(err, domain.ErrOfferOpen), errors.Is(err, domain.ErrOfferClosed), errors.Is(err, domain.ErrOfferExpired):
		return contractshttp.StatusConflict, ""
	}
	return 0, ""
}
