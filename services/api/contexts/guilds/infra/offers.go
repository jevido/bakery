package infra

import (
	"context"
	"errors"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/guilds/app"
	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

type offerRecord struct {
	ID           uint64 `gorm:"primaryKey"`
	GuildID      uint64
	FromMemberID uint64
	ToMemberID   uint64
	Status       string
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

func (offerRecord) TableName() string { return "guild_master_offers" }

func (r offerRecord) toDomain() domain.Offer {
	return domain.Offer{
		ID: r.ID, GuildID: r.GuildID, FromID: r.FromMemberID, ToID: r.ToMemberID,
		CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, Status: domain.OfferStatus(r.Status),
	}
}

type Offers struct{}

func firstOffer(q contractsorm.Query) (domain.Offer, bool, error) {
	var rec offerRecord
	if err := q.FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Offer{}, false, nil
		}
		return domain.Offer{}, false, err
	}
	return rec.toDomain(), true, nil
}

func (Offers) OpenIn(ctx context.Context, guildID uint64) (domain.Offer, bool, error) {
	return firstOffer(query(ctx).Where("guild_id", guildID).Where("status", string(domain.OfferOpen)))
}

func (Offers) OpenTo(ctx context.Context, memberID uint64) ([]domain.Offer, error) {
	var recs []offerRecord
	if err := query(ctx).Where("to_member_id", memberID).Where("status", string(domain.OfferOpen)).OrderBy("id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Offer, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

func (Offers) ByID(ctx context.Context, id uint64) (domain.Offer, bool, error) {
	return firstOffer(query(ctx).Where("id", id))
}

func (Offers) Add(ctx context.Context, o domain.Offer) (domain.Offer, error) {
	rec := offerRecord{
		GuildID: o.GuildID, FromMemberID: o.FromID, ToMemberID: o.ToID,
		Status: string(o.Status), CreatedAt: o.CreatedAt, ExpiresAt: o.ExpiresAt,
	}
	if err := query(ctx).Create(&rec); err != nil {
		// guild_master_offers_one_open: another offer was made meanwhile.
		if isUniqueViolation(err) {
			return domain.Offer{}, domain.ErrOfferOpen
		}
		return domain.Offer{}, err
	}
	return rec.toDomain(), nil
}

func (Offers) Close(ctx context.Context, o domain.Offer) error {
	res, err := query(ctx).Model(&offerRecord{}).Where("id", o.ID).Where("status", string(domain.OfferOpen)).
		Update("status", string(o.Status))
	if err != nil {
		return err
	}
	if res.RowsAffected == 0 {
		return domain.ErrOfferClosed
	}
	return nil
}

func (Offers) Accept(ctx context.Context, id uint64, accept func(g *domain.Guild, o *domain.Offer, isMember bool) error) error {
	return facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		o, found, err := firstOffer(tx.Where("id", id))
		if err != nil {
			return err
		}
		if !found {
			return app.ErrOfferNotFound
		}
		// The Guild first, as Memberships.Change locks it, so a removal of
		// the offered Member cannot pass between the check and the swap.
		g, err := lockGuild(tx, o.GuildID)
		if err != nil {
			return err
		}
		if o, _, err = firstOffer(tx.Where("id", id).LockForUpdate()); err != nil {
			return err
		}
		n, err := tx.Model(&membershipRecord{}).Where("guild_id", g.ID).Where("user_id", o.ToID).Count()
		if err != nil {
			return err
		}
		if err := accept(&g, &o, n > 0); err != nil {
			return err
		}
		if _, err := tx.Model(&guildRecord{}).Where("id", g.ID).Update("master_id", g.MasterID); err != nil {
			return err
		}
		_, err = tx.Model(&offerRecord{}).Where("id", o.ID).Update("status", string(o.Status))
		return err
	})
}

func (Offers) WithdrawTo(ctx context.Context, guildID, memberID uint64) error {
	_, err := query(ctx).Model(&offerRecord{}).Where("guild_id", guildID).Where("to_member_id", memberID).
		Where("status", string(domain.OfferOpen)).Update("status", string(domain.OfferWithdrawn))
	return err
}
