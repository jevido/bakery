package app

import (
	"context"
	"errors"
	"github.com/jevido/bakery/services/api/app/secret"
	"slices"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

var (
	ErrTokenNameTaken  = errors.New("you already have an API token with this name")
	ErrTokenNotFound   = errors.New("API token not found")
	ErrInvalidAPIToken = errors.New("invalid API token")
	ErrInvalidExpiry   = errors.New("expires in must be 7, 30, 60, 90 or 365 days, or never")
)

// ExpiryDays are the expiries a new API token may pick, in days, as
// Coolify's "Expires in" offers them; no expiry is Never.
var ExpiryDays = []int{7, 30, 60, 90, 365}

// touchEvery is how stale an API token's last use may get before a request
// writes it again, so a busy script does not write on every request.
const touchEvery = time.Minute

// APITokens stores the API tokens, by the hash of their value.
type APITokens interface {
	// Add stores a new token; a name its Member already uses in the Guild
	// is ErrTokenNameTaken.
	Add(ctx context.Context, t domain.APIToken, hash string) (domain.APIToken, error)
	// ForMember lists the Member's tokens in the Guild, newest first.
	ForMember(ctx context.Context, memberID, guildID uint64) ([]domain.APIToken, error)
	ByHash(ctx context.Context, hash string) (domain.APIToken, bool, error)
	// Revoke deletes the Member's token in the Guild, reporting whether
	// there was one.
	Revoke(ctx context.Context, memberID, guildID, id uint64) (bool, error)
	// RevokeAll deletes every token the Member made in the Guild.
	RevokeAll(ctx context.Context, memberID, guildID uint64) error
	Touch(ctx context.Context, id uint64, now time.Time) error
}

// CreateAPIToken makes a token for the Member in the Guild and returns its
// value, which is never shown again. member, the Member's current
// Permissions in that Guild, caps the Token permissions; expiresInDays is
// nil for a token that never expires.
func (s *Service) CreateAPIToken(ctx context.Context, memberID, guildID uint64, member domain.MemberPermissions, name string, permissions []domain.Permission, expiresInDays *int) (domain.APIToken, string, error) {
	if _, err := s.CurrentMember(ctx, memberID); err != nil {
		return domain.APIToken{}, "", err
	}
	now := s.now()
	var expiresAt *time.Time
	if expiresInDays != nil {
		if !slices.Contains(ExpiryDays, *expiresInDays) {
			return domain.APIToken{}, "", ErrInvalidExpiry
		}
		at := now.AddDate(0, 0, *expiresInDays)
		expiresAt = &at
	}
	t, err := domain.NewAPIToken(memberID, guildID, member, name, permissions, expiresAt, now)
	if err != nil {
		return domain.APIToken{}, "", err
	}
	raw, err := secret.New()
	if err != nil {
		return domain.APIToken{}, "", err
	}
	value := domain.APITokenPrefix + raw
	t, err = s.apiTokens.Add(ctx, t, secret.Hash(value))
	return t, value, err
}

// APITokensOf lists the Member's tokens in the Guild, newest first.
func (s *Service) APITokensOf(ctx context.Context, memberID, guildID uint64) ([]domain.APIToken, error) {
	return s.apiTokens.ForMember(ctx, memberID, guildID)
}

// RevokeAPIToken deletes one of the Member's own tokens in the Guild.
func (s *Service) RevokeAPIToken(ctx context.Context, memberID, guildID, id uint64) error {
	found, err := s.apiTokens.Revoke(ctx, memberID, guildID, id)
	if err == nil && !found {
		err = ErrTokenNotFound
	}
	return err
}

// RevokeAPITokensIn deletes every token the Member made in the Guild, for
// when they leave it.
func (s *Service) RevokeAPITokensIn(ctx context.Context, memberID, guildID uint64) error {
	return s.apiTokens.RevokeAll(ctx, memberID, guildID)
}

// Authenticate finds the Member an API token belongs to, with the token for
// its Permissions. A token of a removed Member is gone with them, and an
// expired one no longer counts.
func (s *Service) Authenticate(ctx context.Context, value string) (domain.Member, domain.APIToken, error) {
	if !strings.HasPrefix(value, domain.APITokenPrefix) {
		return domain.Member{}, domain.APIToken{}, ErrInvalidAPIToken
	}
	t, found, err := s.apiTokens.ByHash(ctx, secret.Hash(value))
	if err != nil {
		return domain.Member{}, domain.APIToken{}, err
	}
	now := s.now()
	if !found || t.Expired(now) {
		return domain.Member{}, domain.APIToken{}, ErrInvalidAPIToken
	}
	m, err := s.CurrentMember(ctx, t.MemberID)
	if errors.Is(err, ErrMemberNotFound) {
		return domain.Member{}, domain.APIToken{}, ErrInvalidAPIToken
	}
	if err != nil {
		return domain.Member{}, domain.APIToken{}, err
	}
	if t.LastUsedAt == nil || now.Sub(*t.LastUsedAt) >= touchEvery {
		if err := s.apiTokens.Touch(ctx, t.ID, now); err != nil {
			return domain.Member{}, domain.APIToken{}, err
		}
	}
	return m, t, nil
}
