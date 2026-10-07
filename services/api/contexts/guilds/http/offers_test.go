package http

import (
	"context"
	"errors"
	"testing"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/contexts/guilds/app"
	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

func TestOfferStatus(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		want  int
		field string
	}{
		{"no such offer", app.ErrOfferNotFound, contractshttp.StatusNotFound, ""},
		{"a Member the offer is not for accepts", domain.ErrNotOffered, contractshttp.StatusNotFound, ""},
		{"someone but the Guild Master offers", domain.ErrNotGuildMaster, contractshttp.StatusForbidden, ""},
		{"to someone outside the Guild", domain.ErrOfferNotMember, contractshttp.StatusUnprocessableEntity, "member_id"},
		{"to themselves", domain.ErrOfferToSelf, contractshttp.StatusUnprocessableEntity, "member_id"},
		{"a second open offer", domain.ErrOfferOpen, contractshttp.StatusConflict, ""},
		{"expired", domain.ErrOfferExpired, contractshttp.StatusConflict, ""},
		{"no longer open", domain.ErrOfferClosed, contractshttp.StatusConflict, ""},
		{"anything else", context.Canceled, 0, ""},
	}
	for _, c := range cases {
		if got, field := offerStatus(errors.Join(c.err)); got != c.want || field != c.field {
			t.Errorf("%s: %d %q, want %d %q", c.name, got, field, c.want, c.field)
		}
	}
}
