package app

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// The ports the fakes do not need panic when used.
type (
	approvalsPort = Approvals
	activityPort  = Activity
)

// memApprovals keeps Approvals in memory, guarding saves by status as the
// store does.
type memApprovals struct {
	approvalsPort
	byID map[uint64]domain.Approval
}

func (m *memApprovals) CreateApproval(_ context.Context, a domain.Approval) (domain.Approval, error) {
	a.ID = uint64(len(m.byID) + 1)
	m.byID[a.ID] = a
	return a, nil
}

func (m *memApprovals) Approval(_ context.Context, id uint64) (domain.Approval, bool, error) {
	a, ok := m.byID[id]
	return a, ok, nil
}

func (m *memApprovals) SaveApproval(_ context.Context, a domain.Approval, from []domain.ApprovalStatus) (bool, error) {
	if !slices.Contains(from, m.byID[a.ID].Status) {
		return false, nil
	}
	m.byID[a.ID] = a
	return true, nil
}

// memActivity keeps the Actions recorded.
type memActivity struct {
	activityPort
	actions []string
}

func (m *memActivity) Record(_ context.Context, e domain.ActivityEvent) error {
	m.actions = append(m.actions, e.Action)
	return nil
}

func hireService(t *testing.T) (*Service, *memActivity, *[]domain.Approval) {
	t.Helper()
	act := &memActivity{}
	s := NewService(nil, nil, nil, nil, nil, nil, act, nil, &memApprovals{byID: map[uint64]domain.Approval{}}, nil)
	s.Logf = t.Logf
	var heard []domain.Approval
	s.Decided = func(_ context.Context, a domain.Approval) error {
		heard = append(heard, a)
		return nil
	}
	return s, act, &heard
}

func TestHireApprovalDecided(t *testing.T) {
	ctx := context.Background()
	s, act, heard := hireService(t)
	a, err := s.RequestHireApproval(ctx, 1, 7, domain.HireAgentPayload{AgentID: 3, Name: "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Requester != domain.ByMember(7) || a.Type != domain.HireAgent || a.Status != domain.StatusPending {
		t.Fatalf("hire approval %+v", a)
	}
	if _, err := s.ApproveApproval(ctx, 1, 9, a.ID, ""); err != nil {
		t.Fatal(err)
	}
	if len(*heard) != 1 || (*heard)[0].Status != domain.StatusApproved || (*heard)[0].DeciderID != 9 {
		t.Fatalf("after approve, heard %+v", *heard)
	}
	// Approving again answers it unchanged and is heard again, so a failed
	// callback is healed by retrying the Decision.
	if _, err := s.ApproveApproval(ctx, 1, 9, a.ID, ""); err != nil {
		t.Fatal(err)
	}
	if len(*heard) != 2 {
		t.Errorf("after approving again, heard %d", len(*heard))
	}
	if want := []string{"approval.created", "approval.approved"}; !slices.Equal(act.actions, want) {
		t.Errorf("actions %v, want %v", act.actions, want)
	}
	// Another Guild's Approval is not found and not heard.
	if _, err := s.RejectApproval(ctx, 2, 9, a.ID, ""); !errors.Is(err, ErrNotFound) || len(*heard) != 2 {
		t.Errorf("reject in another guild = %v, heard %d", err, len(*heard))
	}
}

func TestDecidedErrorIsAnswered(t *testing.T) {
	ctx := context.Background()
	s, _, _ := hireService(t)
	boom := errors.New("boom")
	s.Decided = func(context.Context, domain.Approval) error { return boom }
	a, _ := s.RequestHireApproval(ctx, 1, 7, domain.HireAgentPayload{AgentID: 3, Name: "Ada"})
	if _, err := s.RejectApproval(ctx, 1, 9, a.ID, ""); !errors.Is(err, boom) {
		t.Errorf("reject = %v, want boom", err)
	}
	stored, _ := s.Approval(ctx, 1, a.ID)
	if stored.Status != domain.StatusRejected {
		t.Errorf("the Decision was not kept: %s", stored.Status)
	}
}

func TestCancelledHireCannotBeDecided(t *testing.T) {
	ctx := context.Background()
	s, act, heard := hireService(t)
	a, _ := s.RequestHireApproval(ctx, 1, 7, domain.HireAgentPayload{AgentID: 3, Name: "Ada"})
	if c, err := s.CancelApproval(ctx, 1, 7, a.ID); err != nil || c.Status != domain.StatusCancelled {
		t.Fatalf("cancel = %v, %+v", err, c)
	}
	// Cancelling again changes nothing and records nothing.
	if _, err := s.CancelApproval(ctx, 1, 7, a.ID); err != nil {
		t.Fatal(err)
	}
	var refused *domain.ApprovalRefusedError
	if _, err := s.ApproveApproval(ctx, 1, 9, a.ID, ""); !errors.As(err, &refused) {
		t.Errorf("approve a cancelled hire = %v, want refused", err)
	}
	if len(*heard) != 0 {
		t.Errorf("heard %d decisions", len(*heard))
	}
	if want := []string{"approval.created", "approval.cancelled"}; !slices.Equal(act.actions, want) {
		t.Errorf("actions %v, want %v", act.actions, want)
	}
}

func TestHireNotThroughRequestApproval(t *testing.T) {
	s, _, _ := hireService(t)
	_, err := s.RequestApproval(context.Background(), 1, domain.ByMember(7), "hire_agent", domain.HireAgentPayload{AgentID: 3, Name: "Ada"}, nil, nil)
	var fe *domain.FieldError
	if !errors.As(err, &fe) || fe.Field != "type" {
		t.Errorf("RequestApproval(hire_agent) = %v", err)
	}
}

func TestRecordAgentActivity(t *testing.T) {
	s, act, _ := hireService(t)
	ctx := context.Background()
	if err := s.RecordAgentActivity(ctx, domain.AgentEvent{GuildID: 1, AgentID: 3, AgentName: "Ada", Action: "agent.hired"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordAgentActivity(ctx, domain.AgentEvent{GuildID: 1, AgentID: 3, AgentName: "Ada", Action: "issue.created"}); err == nil {
		t.Error("a non-agent Action was recorded")
	}
	if want := []string{"agent.hired"}; !slices.Equal(act.actions, want) {
		t.Errorf("actions %v, want %v", act.actions, want)
	}
}
