package app

import (
	"context"
	"errors"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

var ErrOfferNotFound = errors.New("offer not found")

// Offers stores the Transfer offers. An open offer past its expiry stays
// open in storage until it is next read, and is then written expired.
type Offers interface {
	// OpenIn is the Guild's open offer, expired or not; false when none.
	OpenIn(ctx context.Context, guildID uint64) (domain.Offer, bool, error)
	// OpenTo lists the open offers to the Member in every Guild, expired
	// or not, by id.
	OpenTo(ctx context.Context, memberID uint64) ([]domain.Offer, error)
	ByID(ctx context.Context, id uint64) (domain.Offer, bool, error)
	// Add stores a new open offer; domain.ErrOfferOpen when the Guild has
	// one already.
	Add(ctx context.Context, o domain.Offer) (domain.Offer, error)
	// Close stores o's status, if the stored offer is still open;
	// domain.ErrOfferClosed when it was not.
	Close(ctx context.Context, o domain.Offer) error
	// Accept takes the offer's Guild and the offer in turn (a Membership
	// change of that Guild waits meanwhile), calls accept with both and
	// whether the offered Member still holds a Membership there, and stores
	// the Guild Master and the offer's status in one transaction.
	Accept(ctx context.Context, id uint64, accept func(g *domain.Guild, o *domain.Offer, isMember bool) error) error
	// WithdrawTo withdraws the Guild's open offer to the Member, if any.
	WithdrawTo(ctx context.Context, guildID, memberID uint64) error
}

// expire writes o expired when it is open past its expiry; Expiry is read,
// not swept.
func (s *Service) expire(ctx context.Context, o *domain.Offer) error {
	if !o.Expire(s.Now()) {
		return nil
	}
	if err := s.offers.Close(ctx, *o); err != nil && !errors.Is(err, domain.ErrOfferClosed) {
		return err
	}
	return nil
}

// OpenOffer is the Guild's open Transfer offer, false when there is none
// or it has expired.
func (s *Service) OpenOffer(ctx context.Context, guildID uint64) (domain.Offer, bool, error) {
	o, found, err := s.offers.OpenIn(ctx, guildID)
	if err != nil || !found {
		return domain.Offer{}, false, err
	}
	if err := s.expire(ctx, &o); err != nil {
		return domain.Offer{}, false, err
	}
	return o, o.Status == domain.OfferOpen, nil
}

// OffersTo lists the open Transfer offers to the Member, in every Guild.
func (s *Service) OffersTo(ctx context.Context, memberID uint64) ([]domain.Offer, error) {
	all, err := s.offers.OpenTo(ctx, memberID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Offer, 0, len(all))
	for _, o := range all {
		if err := s.expire(ctx, &o); err != nil {
			return nil, err
		}
		if o.Status == domain.OfferOpen {
			out = append(out, o)
		}
	}
	return out, nil
}

// OfferGuildMaster is the Guild Master (from) offering it to another
// Member of the Guild (to). Nothing changes until they accept.
func (s *Service) OfferGuildMaster(ctx context.Context, guildID, from, to uint64) (domain.Offer, error) {
	g, err := s.Guild(ctx, guildID)
	if err != nil {
		return domain.Offer{}, err
	}
	_, isMember, err := s.memberships.Of(ctx, guildID, to)
	if err != nil {
		return domain.Offer{}, err
	}
	o, err := domain.NewOffer(g, from, to, isMember, s.Now())
	if err != nil {
		return domain.Offer{}, err
	}
	// Writes an expired open offer expired, so it no longer holds the
	// Guild's one open place.
	if _, open, err := s.OpenOffer(ctx, guildID); err != nil {
		return domain.Offer{}, err
	} else if open {
		return domain.Offer{}, domain.ErrOfferOpen
	}
	return s.offers.Add(ctx, o)
}

// WithdrawOffer is the Guild Master (by) taking the Guild's open offer
// back; ErrOfferNotFound when there is none.
func (s *Service) WithdrawOffer(ctx context.Context, guildID, by uint64) error {
	o, open, err := s.OpenOffer(ctx, guildID)
	if err != nil {
		return err
	}
	if !open {
		return ErrOfferNotFound
	}
	if err := o.Withdraw(s.Now(), by); err != nil {
		return err
	}
	return s.offers.Close(ctx, o)
}

// offer is the offer by id with its expiry written; ErrOfferNotFound when
// there is none, or when by is neither of its two Members, so nobody else
// learns it exists.
func (s *Service) offer(ctx context.Context, id, by uint64) (domain.Offer, error) {
	o, found, err := s.offers.ByID(ctx, id)
	if err != nil {
		return domain.Offer{}, err
	}
	if !found || (by != o.ToID && by != o.FromID) {
		return domain.Offer{}, ErrOfferNotFound
	}
	return o, s.expire(ctx, &o)
}

// AcceptOffer is the offered Member (by) taking the Guild Master: the
// Guild's Guild Master becomes them in one step, and both keep their
// Roles.
func (s *Service) AcceptOffer(ctx context.Context, id, by uint64) error {
	if _, err := s.offer(ctx, id, by); err != nil {
		return err
	}
	return s.offers.Accept(ctx, id, func(g *domain.Guild, o *domain.Offer, isMember bool) error {
		if err := o.Accept(g, s.Now(), by); err != nil {
			return err
		}
		if !isMember {
			return domain.ErrOfferNotMember
		}
		return nil
	})
}

// DeclineOffer is the offered Member (by) turning the offer down.
func (s *Service) DeclineOffer(ctx context.Context, id, by uint64) error {
	o, err := s.offer(ctx, id, by)
	if err != nil {
		return err
	}
	if err := o.Decline(s.Now(), by); err != nil {
		return err
	}
	return s.offers.Close(ctx, o)
}
