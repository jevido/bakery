package domain

import (
	"errors"
	"testing"
	"time"
)

var offerMade = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func TestNewOffer(t *testing.T) {
	g := Guild{ID: 1, MasterID: 2}
	cases := []struct {
		name     string
		from, to uint64
		isMember bool
		want     error
	}{
		{"the Guild Master to a Member", 2, 3, true, nil},
		{"someone else", 3, 4, true, ErrNotGuildMaster},
		{"to themselves", 2, 2, true, ErrOfferToSelf},
		{"to someone not in the Guild", 2, 5, false, ErrOfferNotMember},
	}
	for _, c := range cases {
		o, err := NewOffer(g, c.from, c.to, c.isMember, offerMade)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
		if err == nil && (o.Status != OfferOpen || !o.ExpiresAt.Equal(offerMade.Add(7*24*time.Hour)) || o.GuildID != 1) {
			t.Errorf("%s: %+v", c.name, o)
		}
	}
}

func TestOfferTransitions(t *testing.T) {
	open := func() Offer {
		o, err := NewOffer(Guild{ID: 1, MasterID: 2}, 2, 3, true, offerMade)
		if err != nil {
			t.Fatal(err)
		}
		return o
	}
	now := offerMade.Add(time.Hour)
	cases := []struct {
		name   string
		status OfferStatus
		act    func(o *Offer, g *Guild) error
		want   error
		after  OfferStatus
	}{
		{"accepted by the Member", OfferOpen, func(o *Offer, g *Guild) error { return o.Accept(g, now, 3) }, nil, OfferAccepted},
		{"accepted by the Guild Master", OfferOpen, func(o *Offer, g *Guild) error { return o.Accept(g, now, 2) }, ErrNotOffered, OfferOpen},
		{"accepted by someone else", OfferOpen, func(o *Offer, g *Guild) error { return o.Accept(g, now, 4) }, ErrNotOffered, OfferOpen},
		{"declined by the Member", OfferOpen, func(o *Offer, _ *Guild) error { return o.Decline(now, 3) }, nil, OfferDeclined},
		{"declined by the Guild Master", OfferOpen, func(o *Offer, _ *Guild) error { return o.Decline(now, 2) }, ErrNotOffered, OfferOpen},
		{"withdrawn by the Guild Master", OfferOpen, func(o *Offer, _ *Guild) error { return o.Withdraw(now, 2) }, nil, OfferWithdrawn},
		{"withdrawn by the Member", OfferOpen, func(o *Offer, _ *Guild) error { return o.Withdraw(now, 3) }, ErrNotGuildMaster, OfferOpen},
		{"accepted after a decline", OfferDeclined, func(o *Offer, g *Guild) error { return o.Accept(g, now, 3) }, ErrOfferClosed, OfferDeclined},
		{"declined after a withdrawal", OfferWithdrawn, func(o *Offer, _ *Guild) error { return o.Decline(now, 3) }, ErrOfferClosed, OfferWithdrawn},
		{"withdrawn after an accept", OfferAccepted, func(o *Offer, _ *Guild) error { return o.Withdraw(now, 2) }, ErrOfferClosed, OfferAccepted},
		{"accepted after expiry", OfferExpired, func(o *Offer, g *Guild) error { return o.Accept(g, now, 3) }, ErrOfferExpired, OfferExpired},
	}
	for _, c := range cases {
		o, g := open(), Guild{ID: 1, MasterID: 2}
		o.Status = c.status
		if err := c.act(&o, &g); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
		if o.Status != c.after {
			t.Errorf("%s: status %s, want %s", c.name, o.Status, c.after)
		}
		wantMaster := uint64(2)
		if c.after == OfferAccepted && c.status == OfferOpen {
			wantMaster = 3
		}
		if g.MasterID != wantMaster {
			t.Errorf("%s: Guild Master %d, want %d", c.name, g.MasterID, wantMaster)
		}
	}
}

func TestOfferExpiresAtExactlySevenDays(t *testing.T) {
	o, _ := NewOffer(Guild{ID: 1, MasterID: 2}, 2, 3, true, offerMade)
	justBefore := offerMade.Add(7*24*time.Hour - time.Nanosecond)
	exactly := offerMade.Add(7 * 24 * time.Hour)
	if o.Expired(justBefore) {
		t.Error("expired before 7 days")
	}
	if !o.Expired(exactly) {
		t.Error("not expired at exactly 7 days")
	}
	g := Guild{ID: 1, MasterID: 2}
	for name, act := range map[string]func(o *Offer) error{
		"accept":   func(o *Offer) error { return o.Accept(&g, exactly, 3) },
		"decline":  func(o *Offer) error { return o.Decline(exactly, 3) },
		"withdraw": func(o *Offer) error { return o.Withdraw(exactly, 2) },
	} {
		c := o
		if err := act(&c); !errors.Is(err, ErrOfferExpired) {
			t.Errorf("%s at 7 days: %v", name, err)
		}
	}
	if g.MasterID != 2 {
		t.Error("an expired offer moved the Guild Master")
	}
	if !o.Expire(exactly) || o.Status != OfferExpired || o.Expire(exactly) {
		t.Errorf("Expire: %s", o.Status)
	}
}

func TestAcceptNeedsTheGuildTheOfferWasMadeIn(t *testing.T) {
	o, _ := NewOffer(Guild{ID: 1, MasterID: 2}, 2, 3, true, offerMade)
	other := Guild{ID: 9, MasterID: 2}
	if err := o.Accept(&other, offerMade, 3); !errors.Is(err, ErrOfferClosed) || other.MasterID != 2 {
		t.Errorf("another Guild: %v, master %d", err, other.MasterID)
	}
}
