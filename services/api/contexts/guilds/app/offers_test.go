package app

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

// memOffers keeps the Transfer offers; Accept changes the Guild Master in
// store.
type memOffers struct {
	store *memStore
	all   []domain.Offer
}

func (m *memOffers) OpenIn(_ context.Context, guildID uint64) (domain.Offer, bool, error) {
	for _, o := range m.all {
		if o.GuildID == guildID && o.Status == domain.OfferOpen {
			return o, true, nil
		}
	}
	return domain.Offer{}, false, nil
}

func (m *memOffers) OpenTo(_ context.Context, memberID uint64) ([]domain.Offer, error) {
	var out []domain.Offer
	for _, o := range m.all {
		if o.ToID == memberID && o.Status == domain.OfferOpen {
			out = append(out, o)
		}
	}
	return out, nil
}

func (m *memOffers) ByID(_ context.Context, id uint64) (domain.Offer, bool, error) {
	for _, o := range m.all {
		if o.ID == id {
			return o, true, nil
		}
	}
	return domain.Offer{}, false, nil
}

func (m *memOffers) Add(ctx context.Context, o domain.Offer) (domain.Offer, error) {
	if _, open, _ := m.OpenIn(ctx, o.GuildID); open {
		return domain.Offer{}, domain.ErrOfferOpen
	}
	o.ID = uint64(len(m.all) + 1)
	m.all = append(m.all, o)
	return o, nil
}

func (m *memOffers) Close(_ context.Context, o domain.Offer) error {
	i := slices.IndexFunc(m.all, func(x domain.Offer) bool { return x.ID == o.ID })
	if i < 0 || m.all[i].Status != domain.OfferOpen {
		return domain.ErrOfferClosed
	}
	m.all[i].Status = o.Status
	return nil
}

func (m *memOffers) Accept(ctx context.Context, id uint64, accept func(g *domain.Guild, o *domain.Offer, isMember bool) error) error {
	i := slices.IndexFunc(m.all, func(x domain.Offer) bool { return x.ID == id })
	if i < 0 {
		return ErrOfferNotFound
	}
	o := m.all[i]
	g, _, _ := m.store.ByID(ctx, o.GuildID)
	_, isMember, _ := m.store.Of(ctx, o.GuildID, o.ToID)
	if err := accept(&g, &o, isMember); err != nil {
		return err
	}
	m.all[i] = o
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	for j := range m.store.guilds {
		if m.store.guilds[j].ID == g.ID {
			m.store.guilds[j].MasterID = g.MasterID
		}
	}
	return nil
}

func (m *memOffers) WithdrawTo(_ context.Context, guildID, memberID uint64) error {
	for i, o := range m.all {
		if o.GuildID == guildID && o.ToID == memberID && o.Status == domain.OfferOpen {
			m.all[i].Status = domain.OfferWithdrawn
		}
	}
	return nil
}

func TestTheCreatorIsGuildMaster(t *testing.T) {
	ctx := context.Background()
	s, m, _ := twoGuilds(t)
	if m.guilds[0].MasterID != 1 || m.guilds[1].MasterID != 2 {
		t.Fatalf("Guild Masters: %d, %d; want the Instance admin and Ann", m.guilds[0].MasterID, m.guilds[1].MasterID)
	}
	if r, _ := m.roleOf(2, 2); r != "admin" {
		t.Errorf("Ann holds %q in Bakers, want Admin too", r)
	}
	p, ok, err := s.Place(ctx, 2, false, 0, 2)
	if err != nil || !ok || !p.GuildMaster || p.Permissions != domain.AllPermissions {
		t.Errorf("Ann's Place in Bakers: %+v %v %v", p, ok, err)
	}
	if yes, _ := s.IsGuildMaster(ctx, 2); !yes {
		t.Error("Ann is a Guild Master")
	}
	if yes, _ := s.IsGuildMaster(ctx, 3); yes {
		t.Error("Dev is no Guild Master")
	}
}

func TestAcceptingAnOfferSwapsTheGuildMaster(t *testing.T) {
	ctx := context.Background()
	s, m, _ := twoGuilds(t)
	viewer := m.seeded(2, "viewer")
	if _, err := s.OfferGuildMaster(ctx, 2, 3, 2); !errors.Is(err, domain.ErrNotGuildMaster) {
		t.Errorf("Dev offers: %v", err)
	}
	if _, err := s.OfferGuildMaster(ctx, 2, 2, 1); !errors.Is(err, domain.ErrOfferNotMember) {
		t.Errorf("offer to the Instance admin, not in Bakers: %v", err)
	}
	o, err := s.OfferGuildMaster(ctx, 2, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.OfferGuildMaster(ctx, 2, 2, 3); !errors.Is(err, domain.ErrOfferOpen) {
		t.Errorf("a second offer: %v", err)
	}
	if m.guilds[1].MasterID != 2 {
		t.Fatal("the offer moved the Guild Master")
	}
	if err := s.AcceptOffer(ctx, o.ID, 1); !errors.Is(err, ErrOfferNotFound) {
		t.Errorf("someone else accepts: %v", err)
	}
	if err := s.AcceptOffer(ctx, o.ID, 2); !errors.Is(err, domain.ErrNotOffered) {
		t.Errorf("the Guild Master accepts: %v", err)
	}
	if offers, _ := s.OffersTo(ctx, 3); len(offers) != 1 {
		t.Errorf("Dev's offers: %+v", offers)
	}
	if err := s.AcceptOffer(ctx, o.ID, 3); err != nil {
		t.Fatal(err)
	}
	if m.guilds[1].MasterID != 3 {
		t.Fatalf("Guild Master %d, want Dev", m.guilds[1].MasterID)
	}
	if r, _ := m.roleOf(2, 2); r != "admin" {
		t.Errorf("Ann's Roles changed: %s", r)
	}
	if ms, _, _ := m.Of(ctx, 2, 3); !slices.Equal(ms.RoleIDs, []uint64{viewer}) {
		t.Errorf("Dev's Roles changed: %v", ms.RoleIDs)
	}
	if err := s.AcceptOffer(ctx, o.ID, 3); !errors.Is(err, domain.ErrOfferClosed) {
		t.Errorf("accepted twice: %v", err)
	}
	// Dev, Guild Master now, removes Ann; Ann, still Admin, cannot remove
	// Dev.
	if err := s.RemoveMembership(ctx, 2, 2, adminPerms, 3); !errors.Is(err, domain.ErrGuildMaster) {
		t.Errorf("the old Guild Master removes the new one: %v", err)
	}
	if err := s.RemoveMembership(ctx, 2, 3, domain.AllPermissions, 2); err != nil {
		t.Errorf("the new Guild Master removes the old one: %v", err)
	}
}

func TestOfferDeclineWithdrawAndExpiry(t *testing.T) {
	ctx := context.Background()
	s, m, _ := twoGuilds(t)
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	o, _ := s.OfferGuildMaster(ctx, 2, 2, 3)
	if err := s.DeclineOffer(ctx, o.ID, 3); err != nil {
		t.Fatal(err)
	}
	if _, open, _ := s.OpenOffer(ctx, 2); open {
		t.Error("a declined offer is open")
	}
	o, _ = s.OfferGuildMaster(ctx, 2, 2, 3)
	if err := s.WithdrawOffer(ctx, 2, 3); !errors.Is(err, domain.ErrNotGuildMaster) {
		t.Errorf("Dev withdraws: %v", err)
	}
	if err := s.WithdrawOffer(ctx, 2, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.WithdrawOffer(ctx, 2, 2); !errors.Is(err, ErrOfferNotFound) {
		t.Errorf("withdrawn twice: %v", err)
	}
	o, _ = s.OfferGuildMaster(ctx, 2, 2, 3)
	now = now.Add(domain.OfferLifetime)
	if err := s.AcceptOffer(ctx, o.ID, 3); !errors.Is(err, domain.ErrOfferExpired) {
		t.Errorf("accepted at 7 days: %v", err)
	}
	if got, _, _ := (&memOffers{all: s.offers.(*memOffers).all}).ByID(ctx, o.ID); got.Status != domain.OfferExpired {
		t.Errorf("status %s, want written expired", got.Status)
	}
	if m.guilds[1].MasterID != 2 {
		t.Error("an expired offer moved the Guild Master")
	}
	// An expired offer no longer holds the Guild's one open place.
	if _, err := s.OfferGuildMaster(ctx, 2, 2, 3); err != nil {
		t.Errorf("offer after expiry: %v", err)
	}
	if err := s.RemoveMembership(ctx, 2, 2, adminPerms, 3); err != nil {
		t.Fatal(err)
	}
	if _, open, _ := s.OpenOffer(ctx, 2); open {
		t.Error("the offer to a removed Member stays open")
	}
}
