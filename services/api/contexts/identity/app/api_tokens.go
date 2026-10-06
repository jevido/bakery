package app

import (
	"context"
	"errors"
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
	// Add stores a new token; a name its Member already uses is
	// ErrTokenNameTaken.
	Add(ctx context.Context, t domain.APIToken, hash string) (domain.APIToken, error)
	ForMember(ctx context.Context, memberID uint64) ([]domain.APIToken, error)
	ByHash(ctx context.Context, hash string) (domain.APIToken, bool, error)
	// Revoke deletes the Member's token, reporting whether there was one.
	Revoke(ctx context.Context, memberID, id uint64) (bool, error)
	Touch(ctx context.Context, id uint64, now time.Time) error
}

// CreateAPIToken makes a token for the Member and returns its value, which
// is never shown again. The Member's current Role caps the Permissions;
// expiresInDays is nil for a token that never expires.
func (s *Service) CreateAPIToken(ctx context.Context, memberID uint64, name string, permissions []domain.Permission, expiresInDays *int) (domain.APIToken, string, error) {
	m, err := s.CurrentMember(ctx, memberID)
	if err != nil {
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
	t, err := domain.NewAPIToken(memberID, m.Role, name, permissions, expiresAt, now)
	if err != nil {
		return domain.APIToken{}, "", err
	}
	secret, err := newSecret()
	if err != nil {
		return domain.APIToken{}, "", err
	}
	value := domain.APITokenPrefix + secret
	t, err = s.apiTokens.Add(ctx, t, hashSecret(value))
	return t, value, err
}

// APITokensOf lists the Member's tokens, newest first.
func (s *Service) APITokensOf(ctx context.Context, memberID uint64) ([]domain.APIToken, error) {
	return s.apiTokens.ForMember(ctx, memberID)
}

// RevokeAPIToken deletes one of the Member's own tokens.
func (s *Service) RevokeAPIToken(ctx context.Context, memberID, id uint64) error {
	found, err := s.apiTokens.Revoke(ctx, memberID, id)
	if err == nil && !found {
		err = ErrTokenNotFound
	}
	return err
}

// Authenticate finds the Member an API token belongs to, with the token for
// its Permissions. A token of a removed Member is gone with them, and an
// expired one no longer counts.
func (s *Service) Authenticate(ctx context.Context, value string) (domain.Member, domain.APIToken, error) {
	if !strings.HasPrefix(value, domain.APITokenPrefix) {
		return domain.Member{}, domain.APIToken{}, ErrInvalidAPIToken
	}
	t, found, err := s.apiTokens.ByHash(ctx, hashSecret(value))
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
