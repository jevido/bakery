package infra

import (
	"context"
	"errors"
	"strings"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/identity/app"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

type apiTokenRecord struct {
	ID        uint64 `gorm:"primaryKey"`
	UserID    uint64
	GuildID   uint64
	Name      string
	TokenHash string
	// Permissions is comma-separated.
	Permissions string
	ExpiresAt   *time.Time
	LastUsedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (apiTokenRecord) TableName() string { return "api_tokens" }

func (r apiTokenRecord) toDomain() domain.APIToken {
	var permissions []domain.Permission
	for _, p := range strings.Split(r.Permissions, ",") {
		if p != "" {
			permissions = append(permissions, domain.Permission(p))
		}
	}
	return domain.APIToken{ID: r.ID, MemberID: r.UserID, GuildID: r.GuildID, Name: r.Name, Permissions: permissions, ExpiresAt: r.ExpiresAt, LastUsedAt: r.LastUsedAt, CreatedAt: r.CreatedAt}
}

type APITokens struct{}

func (APITokens) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func (a APITokens) Add(ctx context.Context, t domain.APIToken, hash string) (domain.APIToken, error) {
	permissions := make([]string, len(t.Permissions))
	for i, p := range t.Permissions {
		permissions[i] = string(p)
	}
	rec := apiTokenRecord{UserID: t.MemberID, GuildID: t.GuildID, Name: t.Name, TokenHash: hash, Permissions: strings.Join(permissions, ","), ExpiresAt: t.ExpiresAt, CreatedAt: t.CreatedAt, UpdatedAt: t.CreatedAt}
	if err := a.query(ctx).Create(&rec); err != nil {
		if isUniqueViolation(err) {
			return domain.APIToken{}, app.ErrTokenNameTaken
		}
		return domain.APIToken{}, err
	}
	return rec.toDomain(), nil
}

func (a APITokens) ForMember(ctx context.Context, memberID, guildID uint64) ([]domain.APIToken, error) {
	var recs []apiTokenRecord
	if err := a.query(ctx).Where("user_id", memberID).Where("guild_id", guildID).Order("created_at desc").Order("id desc").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.APIToken, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

func (a APITokens) ByHash(ctx context.Context, hash string) (domain.APIToken, bool, error) {
	var rec apiTokenRecord
	if err := a.query(ctx).Where("token_hash", hash).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.APIToken{}, false, nil
		}
		return domain.APIToken{}, false, err
	}
	return rec.toDomain(), true, nil
}

func (a APITokens) Revoke(ctx context.Context, memberID, guildID, id uint64) (bool, error) {
	res, err := a.query(ctx).Where("id", id).Where("user_id", memberID).Where("guild_id", guildID).Delete(&apiTokenRecord{})
	if err != nil {
		return false, err
	}
	return res.RowsAffected > 0, nil
}

func (a APITokens) RevokeAll(ctx context.Context, memberID, guildID uint64) error {
	_, err := a.query(ctx).Where("user_id", memberID).Where("guild_id", guildID).Delete(&apiTokenRecord{})
	return err
}

func (a APITokens) Touch(ctx context.Context, id uint64, now time.Time) error {
	_, err := a.query(ctx).Model(&apiTokenRecord{}).Where("id", id).Update("last_used_at", now)
	return err
}
