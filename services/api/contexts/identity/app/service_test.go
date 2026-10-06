package app

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

type memMembers struct {
	mu            sync.Mutex
	members       []domain.Member
	recoveryCodes map[uint64]map[string]bool
}

func (m *memMembers) InstanceAdminExists(context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.members {
		if x.InstanceAdmin {
			return true, nil
		}
	}
	return false, nil
}

func (m *memMembers) AddInstanceAdminIfNone(ctx context.Context, o domain.Member) (domain.Member, error) {
	if exists, _ := m.InstanceAdminExists(ctx); exists {
		return domain.Member{}, ErrSetupDone
	}
	return m.add(o), nil
}

func (m *memMembers) add(x domain.Member) domain.Member {
	m.mu.Lock()
	defer m.mu.Unlock()
	x.ID = uint64(len(m.members) + 1)
	m.members = append(m.members, x)
	return x
}

func (m *memMembers) find(match func(domain.Member) bool) (domain.Member, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.members {
		if match(x) {
			return x, true, nil
		}
	}
	return domain.Member{}, false, nil
}

func (m *memMembers) ByEmail(_ context.Context, email string) (domain.Member, bool, error) {
	return m.find(func(x domain.Member) bool { return x.Email == email })
}

func (m *memMembers) ByID(_ context.Context, id uint64) (domain.Member, bool, error) {
	return m.find(func(x domain.Member) bool { return x.ID == id })
}

func (m *memMembers) ByIDs(_ context.Context, ids []uint64) ([]domain.Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Member
	for _, x := range m.members {
		if slices.Contains(ids, x.ID) {
			out = append(out, x)
		}
	}
	return out, nil
}

func newTestService() *Service {
	members := &memMembers{}
	return NewService(members, &memInvitations{members: members}, &memAPITokens{}, plainHasher{})
}

type plainHasher struct{}

func (plainHasher) Make(p string) (string, error) { return "h:" + p, nil }
func (plainHasher) Check(p, h string) bool        { return h == "h:"+p }

func TestSetupOnlyOnce(t *testing.T) {
	ctx := context.Background()
	s := newTestService()

	if needed, _ := s.SetupNeeded(ctx); !needed {
		t.Fatal("setup should be needed on a fresh install")
	}
	if _, err := s.SetUp(ctx, "Ada", "ada@example.com", "correct horse"); err != nil {
		t.Fatalf("first setup: %v", err)
	}
	if _, err := s.SetUp(ctx, "Bram", "bram@example.com", "correct horse"); !errors.Is(err, ErrSetupDone) {
		t.Fatalf("second setup: err = %v, want ErrSetupDone", err)
	}
	if needed, _ := s.SetupNeeded(ctx); needed {
		t.Fatal("setup should not be needed after it ran")
	}
}

func TestSetupMakesTheInstanceAdminAndTellsMemberAdded(t *testing.T) {
	ctx := context.Background()
	s := newTestService()
	var heard []MemberAdded
	s.MemberAdded = func(_ context.Context, e MemberAdded) error {
		heard = append(heard, e)
		return nil
	}
	m, err := s.SetUp(ctx, "Ada", "ada@example.com", "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !m.InstanceAdmin {
		t.Error("the Member Setup creates is the Instance admin")
	}
	if want := (MemberAdded{MemberID: m.ID, InstanceAdmin: true, Role: domain.RoleAdmin}); len(heard) != 1 || heard[0] != want {
		t.Errorf("MemberAdded heard %v, want [%v]", heard, want)
	}
	s.SetUp(ctx, "Bram", "bram@example.com", "correct horse")
	if len(heard) != 1 {
		t.Errorf("a refused Setup told MemberAdded: %v", heard)
	}
}

func TestLogin(t *testing.T) {
	ctx := context.Background()
	s := newTestService()
	if _, err := s.SetUp(ctx, "Ada", "ada@example.com", "correct horse"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Login(ctx, "ADA@example.com", "correct horse"); err != nil {
		t.Fatalf("right password: %v", err)
	}
	if _, err := s.Login(ctx, "ada@example.com", "wrong password"); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("wrong password: err = %v", err)
	}
	if _, err := s.Login(ctx, "nobody@example.com", "correct horse"); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("unknown email: err = %v", err)
	}
}

func (m *memMembers) change(id uint64, f func(*domain.Member)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.members {
		if m.members[i].ID == id {
			f(&m.members[i])
		}
	}
}

func (m *memMembers) SetName(_ context.Context, id uint64, name string) error {
	m.change(id, func(x *domain.Member) { x.Name = name })
	return nil
}

func (m *memMembers) SetPassword(_ context.Context, id uint64, hash string, from time.Time) error {
	m.change(id, func(x *domain.Member) { x.PasswordHash, x.SessionsValidFrom = hash, from })
	return nil
}

func (m *memMembers) SetSessionsValidFrom(_ context.Context, id uint64, t time.Time) error {
	m.change(id, func(x *domain.Member) { x.SessionsValidFrom = t })
	return nil
}

func (m *memMembers) SetPendingTwoFactor(_ context.Context, id uint64, secret []byte) error {
	m.change(id, func(x *domain.Member) {
		x.TwoFactor.Secret, x.TwoFactor.State = secret, domain.TwoFactorPending
	})
	return nil
}

func (m *memMembers) EnableTwoFactor(_ context.Context, id uint64, step int64, _ time.Time, hashes []string) error {
	m.change(id, func(x *domain.Member) { x.TwoFactor.State, x.TwoFactor.LastStep = domain.TwoFactorOn, step })
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setCodes(id, hashes)
	return nil
}

func (m *memMembers) setCodes(id uint64, hashes []string) {
	if m.recoveryCodes == nil {
		m.recoveryCodes = map[uint64]map[string]bool{}
	}
	m.recoveryCodes[id] = map[string]bool{}
	for _, h := range hashes {
		m.recoveryCodes[id][h] = true
	}
}

func (m *memMembers) ClearTwoFactor(_ context.Context, id uint64, from *time.Time) error {
	m.change(id, func(x *domain.Member) {
		x.TwoFactor = domain.TwoFactor{State: domain.TwoFactorOff}
		if from != nil {
			x.SessionsValidFrom = *from
		}
	})
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.recoveryCodes, id)
	return nil
}

func (m *memMembers) AdvanceTwoFactorStep(_ context.Context, id uint64, step int64) (bool, error) {
	advanced := false
	m.change(id, func(x *domain.Member) {
		if x.TwoFactor.LastStep < step {
			x.TwoFactor.LastStep, advanced = step, true
		}
	})
	return advanced, nil
}

func (m *memMembers) ReplaceRecoveryCodes(_ context.Context, id uint64, hashes []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setCodes(id, hashes)
	return nil
}

func (m *memMembers) UseRecoveryCode(_ context.Context, id uint64, hash string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.recoveryCodes[id][hash] {
		delete(m.recoveryCodes[id], hash)
		return true, nil
	}
	return false, nil
}

func (m *memMembers) RecoveryCodesLeft(_ context.Context, id uint64) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.recoveryCodes[id]), nil
}
