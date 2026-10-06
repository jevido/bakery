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
	ErrEmailTaken     = errors.New("someone with this email is already a member")
)

// Members stores the Members.
type Members interface {
	// InstanceAdminExists reports whether Setup has been done.
	InstanceAdminExists(ctx context.Context) (bool, error)
	// AddInstanceAdminIfNone stores the Instance admin unless one exists
	// already (ErrSetupDone). The check and the insert are one step, so
	// racing Setups cannot both win.
	AddInstanceAdminIfNone(ctx context.Context, m domain.Member) (domain.Member, error)
	// Add stores a Member who is not the Instance admin; ErrEmailTaken when
	// the email is.
	Add(ctx context.Context, m domain.Member) (domain.Member, error)
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
	members   Members
	apiTokens APITokens
	hasher    Hasher
	// Now is the clock; time.Now unless a test sets it.
	Now func() time.Time
	// Random is where secrets come from; crypto/rand unless a test sets it.
	Random io.Reader
	// SetUpDone, when set, hears of the Instance admin right after Setup
	// stored them (guilds makes the first Guild). An error fails the
	// request, though the Instance admin stays.
	SetUpDone func(ctx context.Context, instanceAdminID uint64) error
}

func NewService(members Members, apiTokens APITokens, hasher Hasher) *Service {
	return &Service{members: members, apiTokens: apiTokens, hasher: hasher, Now: time.Now, Random: rand.Reader}
}

func (s *Service) now() time.Time { return s.Now() }

// SetupNeeded reports whether no Instance admin exists yet.
func (s *Service) SetupNeeded(ctx context.Context) (bool, error) {
	exists, err := s.members.InstanceAdminExists(ctx)
	return !exists, err
}

// SetUp creates the Instance admin, once, and then lets SetUpDone make the
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
	if s.SetUpDone != nil {
		err = s.SetUpDone(ctx, admin.ID)
	}
	return admin, err
}

// CreateMember stores a new Member, who is not the Instance admin, with the
// name and password they picked: someone accepting an Invitation (guilds
// gives them the Membership).
func (s *Service) CreateMember(ctx context.Context, name, email, password string) (domain.Member, error) {
	m, err := domain.NewMember(name, email, password)
	if err != nil {
		return domain.Member{}, err
	}
	if m.PasswordHash, err = s.hasher.Make(password); err != nil {
		return domain.Member{}, err
	}
	return s.members.Add(ctx, m)
}

// MemberByEmail finds a Member by email, compared as stored.
func (s *Service) MemberByEmail(ctx context.Context, email string) (domain.Member, bool, error) {
	return s.members.ByEmail(ctx, domain.NormalizeEmail(email))
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
