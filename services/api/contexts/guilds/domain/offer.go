package domain

import (
	"errors"
	"time"
)

// OfferLifetime is how long a Transfer offer stays open.
const OfferLifetime = 7 * 24 * time.Hour

// OfferStatus is where a Transfer offer stands; only an open one can
// change.
type OfferStatus string

const (
	OfferOpen      OfferStatus = "open"
	OfferAccepted  OfferStatus = "accepted"
	OfferDeclined  OfferStatus = "declined"
	OfferWithdrawn OfferStatus = "withdrawn"
	OfferExpired   OfferStatus = "expired"
)

var (
	ErrNotGuildMaster = errors.New("only the guild master can do this")
	ErrGuildMaster    = errors.New("the guild master cannot be removed or re-roled; they transfer it first")
	ErrOfferToSelf    = errors.New("you are the guild master already")
	ErrOfferNotMember = errors.New("the guild master can only be offered to a member of the guild")
	ErrOfferOpen      = errors.New("an offer is open already; withdraw it first")
	ErrNotOffered     = errors.New("this offer is not yours")
	ErrOfferClosed    = errors.New("this offer is no longer open")
	ErrOfferExpired   = errors.New("this offer has expired")
)

// Offer is a Transfer offer: the Guild Master (FromID) offers the Guild
// Master of one Guild to another Member (ToID). Nothing changes until that
// Member accepts.
type Offer struct {
	ID        uint64
	GuildID   uint64
	FromID    uint64
	ToID      uint64
	CreatedAt time.Time
	ExpiresAt time.Time
	Status    OfferStatus
}

// NewOffer is g's Guild Master (from) offering it to another Member of g
// (to; toIsMember says whether they hold a Membership there), open for
// OfferLifetime from now.
func NewOffer(g Guild, from, to uint64, toIsMember bool, now time.Time) (Offer, error) {
	switch {
	case from != g.MasterID:
		return Offer{}, ErrNotGuildMaster
	case to == from:
		return Offer{}, ErrOfferToSelf
	case !toIsMember:
		return Offer{}, ErrOfferNotMember
	}
	return Offer{GuildID: g.ID, FromID: from, ToID: to, CreatedAt: now, ExpiresAt: now.Add(OfferLifetime), Status: OfferOpen}, nil
}

// Expired reports whether o is open but past its expiry: from ExpiresAt
// on, it counts as expired.
func (o Offer) Expired(now time.Time) bool {
	return o.Status == OfferOpen && !now.Before(o.ExpiresAt)
}

// Expire marks an open offer past its expiry as expired, and reports
// whether it did, so the change can be stored.
func (o *Offer) Expire(now time.Time) bool {
	if !o.Expired(now) {
		return false
	}
	o.Status = OfferExpired
	return true
}

// Accept is the offered Member (by) taking the Guild Master of g: g's
// Guild Master becomes them, in the same step as the offer closes. Their
// Roles and the former Guild Master's do not change.
func (o *Offer) Accept(g *Guild, now time.Time, by uint64) error {
	if err := o.closeBy(o.ToID, now, by, ErrNotOffered); err != nil {
		return err
	}
	if g.ID != o.GuildID || g.MasterID != o.FromID {
		return ErrOfferClosed
	}
	g.MasterID = o.ToID
	o.Status = OfferAccepted
	return nil
}

// Decline is the offered Member (by) turning the offer down.
func (o *Offer) Decline(now time.Time, by uint64) error {
	if err := o.closeBy(o.ToID, now, by, ErrNotOffered); err != nil {
		return err
	}
	o.Status = OfferDeclined
	return nil
}

// Withdraw is the Guild Master who made the offer (by) taking it back.
func (o *Offer) Withdraw(now time.Time, by uint64) error {
	if err := o.closeBy(o.FromID, now, by, ErrNotGuildMaster); err != nil {
		return err
	}
	o.Status = OfferWithdrawn
	return nil
}

// closeBy checks that who may close o is by and that o is still open.
func (o *Offer) closeBy(who uint64, now time.Time, by uint64, notWho error) error {
	switch {
	case by != who:
		return notWho
	case o.Expired(now), o.Status == OfferExpired:
		return ErrOfferExpired
	case o.Status != OfferOpen:
		return ErrOfferClosed
	}
	return nil
}
