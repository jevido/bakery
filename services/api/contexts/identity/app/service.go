// Package app holds the identity use cases: Setup, Login and finding the
// Member a request comes from.
package app

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"time"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

var (
	ErrSetupDone      = errors.New("setup is already done")
	ErrBadCredentials = errors.New("email or password is wrong")
	ErrMemberNotFound = errors.New("member not found")
)

// Members stores the Members.
type Members interface {
	// InstanceAdminExists reports whether Setup has been done.
	InstanceAdminExists(ctx context.Context) (bool, error)
	// AddInstanceAdminIfNone stores the Instance admin unless one exists
	// already (ErrSetupDone). The check and the insert are one step, so
	// racing Setups cannot both win.
	AddInstanceAdminIfNone(ctx context.Context, m domain.Member) (domain.Member, error)
	ByEmail(ctx context.Context, email string) (domain.Member, bool, error)
	ByID(ctx context.Context, id uint64) (domain.Member, bool, error)
	// ByIDs lists the Members with these ids that exist, in no order.
	ByIDs(ctx context.Context, ids []uint64) ([]domain.Member, error)

	SetName(ctx context.Context, id uint64, name string) error
	// SetPassword stores a new password hash and ends every Session issued
	// before sessionsValidFrom, in one step.
	SetPassword(ctx context.Context, id uint64, hash string, sessionsValidFrom time.Time) error
	SetSessionsValidFrom(ctx context.Context, id uint64, t time.Time) error

	// SetPendingTwoFactor stores a new, unconfirmed secret.
	SetPendingTwoFactor(ctx context.Context, id uint64, secret []byte) error
	// EnableTwoFactor switches the pending secret on, remembering step as
	// used and replacing the Recovery codes, in one transaction.
	EnableTwoFactor(ctx context.Context, id uint64, step int64, at time.Time, recoveryCodeHashes []string) error
	// ClearTwoFactor switches two-factor off and drops the secret and the
	// Recovery codes; with sessionsValidFrom set it also ends the Sessions
	// issued before it.
	ClearTwoFactor(ctx context.Context, id uint64, sessionsValidFrom *time.Time) error
	// AdvanceTwoFactorStep remembers step as the last accepted one, only if
	// it is later than the stored one, and reports whether it was: two
	// sign-ins racing with one code cannot both win.
	AdvanceTwoFactorStep(ctx context.Context, id uint64, step int64) (bool, error)
	ReplaceRecoveryCodes(ctx context.Context, id uint64, hashes []string) error
	// UseRecoveryCode deletes the Member's Recovery code with this hash and
	// reports whether there was one.
	UseRecoveryCode(ctx context.Context, id uint64, hash string) (bool, error)
	RecoveryCodesLeft(ctx context.Context, id uint64) (int, error)
}

// Hasher hashes and checks passwords.
type Hasher interface {
	Make(password string) (string, error)
	Check(password, hash string) bool
}

type Service struct {
	members     Members
	invitations Invitations
	apiTokens   APITokens
	hasher      Hasher
	// Now is the clock; time.Now unless a test sets it.
	Now func() time.Time
	// Random is where secrets come from; crypto/rand unless a test sets it.
	Random io.Reader
	// MemberAdded, when set, hears of each new Member right after they were
	// stored: the Instance admin after Setup (guilds makes the first Guild),
	// or someone who accepted an Invitation, with its Role (guilds gives
	// them a Membership). An error fails the request, though the Member
	// stays.
	MemberAdded func(ctx context.Context, e MemberAdded) error
}

// MemberAdded is a Member just stored.
type MemberAdded struct {
	MemberID      uint64
	InstanceAdmin bool
	// Role is the one an Invitation offered; admin for the Instance admin.
	Role domain.Role
}

func (s *Service) memberAdded(ctx context.Context, e MemberAdded) error {
	if s.MemberAdded == nil {
		return nil
	}
	return s.MemberAdded(ctx, e)
}

func NewService(members Members, invitations Invitations, apiTokens APITokens, hasher Hasher) *Service {
	return &Service{members: members, invitations: invitations, apiTokens: apiTokens, hasher: hasher, Now: time.Now, Random: rand.Reader}
}

func (s *Service) now() time.Time { return s.Now() }

// SetupNeeded reports whether no Instance admin exists yet.
func (s *Service) SetupNeeded(ctx context.Context) (bool, error) {
	exists, err := s.members.InstanceAdminExists(ctx)
	return !exists, err
}

// SetUp creates the Instance admin, once, and then lets MemberAdded make the
// first Guild.
func (s *Service) SetUp(ctx context.Context, name, email, password string) (domain.Member, error) {
	admin, err := domain.NewMember(name, email, password)
	if err != nil {
		return domain.Member{}, err
	}
	if exists, err := s.members.InstanceAdminExists(ctx); err != nil {
		return domain.Member{}, err
	} else if exists {
		return domain.Member{}, ErrSetupDone
	}
	admin.InstanceAdmin = true
	admin.PasswordHash, err = s.hasher.Make(password)
	if err != nil {
		return domain.Member{}, err
	}
	admin, err = s.members.AddInstanceAdminIfNone(ctx, admin)
	if err != nil {
		return domain.Member{}, err
	}
	return admin, s.memberAdded(ctx, MemberAdded{MemberID: admin.ID, InstanceAdmin: true, Role: domain.RoleAdmin})
}

// Login checks the credentials. A wrong email and a wrong password give the
// same error.
func (s *Service) Login(ctx context.Context, email, password string) (domain.Member, error) {
	m, found, err := s.members.ByEmail(ctx, domain.NormalizeEmail(email))
	if err != nil {
		return domain.Member{}, err
	}
	if !found || !s.hasher.Check(password, m.PasswordHash) {
		return domain.Member{}, ErrBadCredentials
	}
	return m, nil
}

// CurrentMember returns the Member a Session or API token points at; a
// removed Member is ErrMemberNotFound.
func (s *Service) CurrentMember(ctx context.Context, id uint64) (domain.Member, error) {
	m, found, err := s.members.ByID(ctx, id)
	if err != nil {
		return domain.Member{}, err
	}
	if !found {
		return domain.Member{}, ErrMemberNotFound
	}
	return m, nil
}
