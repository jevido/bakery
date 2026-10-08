package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseApprovalType(t *testing.T) {
	if _, err := ParseApprovalType("request_board_approval"); err != nil {
		t.Errorf("ParseApprovalType = %v", err)
	}
	if _, err := ParseApprovalType("hire_agent"); err != nil {
		t.Errorf("ParseApprovalType(hire_agent) = %v", err)
	}
	for _, s := range []string{"", "Hire_Agent", "Request_Board_Approval"} {
		if _, err := ParseApprovalType(s); err == nil {
			t.Errorf("ParseApprovalType(%q) accepted", s)
		}
	}
}

func TestApprovalStatusActionable(t *testing.T) {
	for st, want := range map[ApprovalStatus]bool{StatusPending: true, StatusRevisionRequested: true, StatusApproved: false, StatusRejected: false, StatusCancelled: false} {
		if got := st.Actionable(); got != want {
			t.Errorf("%s.Actionable() = %v", st, got)
		}
	}
	if _, err := ParseApprovalStatus("cancelled"); err != nil {
		t.Errorf("ParseApprovalStatus(cancelled) = %v", err)
	}
}

func TestRequestApproval(t *testing.T) {
	a, err := RequestApproval(1, ByMember(7), RequestBoardApproval, BoardApprovalPayload{
		Title: "  Approve hosting ", Summary: " Costs **42** ", Risks: []string{"May grow", "  ", "Lock-in"},
	}, []uint64{3, 4, 3})
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != StatusPending || !a.Actionable() || a.Requester != ByMember(7) || a.DeciderID != 0 || a.DecidedAt != nil {
		t.Errorf("starting state %+v", a)
	}
	if p := a.Payload.(BoardApprovalPayload); p.Title != "Approve hosting" || p.Summary != "Costs **42**" || len(p.Risks) != 2 {
		t.Errorf("payload %+v", a.Payload)
	}
	if len(a.IssueIDs) != 2 || a.IssueIDs[0] != 3 || a.IssueIDs[1] != 4 {
		t.Errorf("issue ids %v", a.IssueIDs)
	}
}

func TestBoardApprovalPayloadRules(t *testing.T) {
	long := strings.Repeat("x", MaxApprovalText+1)
	many := make([]string, MaxRisks+1)
	for i := range many {
		many[i] = "risk"
	}
	for name, p := range map[string]BoardApprovalPayload{
		"empty title":    {Title: "  "},
		"long title":     {Title: strings.Repeat("t", MaxTitle+1)},
		"long summary":   {Title: "t", Summary: long},
		"long action":    {Title: "t", RecommendedAction: long},
		"long next":      {Title: "t", NextActionOnApproval: long},
		"long risk":      {Title: "t", Risks: []string{strings.Repeat("r", MaxRisk+1)}},
		"too many risks": {Title: "t", Risks: many},
	} {
		if _, err := RequestApproval(1, ByMember(1), RequestBoardApproval, p, nil); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestApprovalRequestedActivity(t *testing.T) {
	a := Approval{ID: 5, GuildID: 1, Type: RequestBoardApproval, Payload: BoardApprovalPayload{Title: "Approve hosting"}, IssueIDs: []uint64{3}}
	e := ApprovalRequested{Happened: Happened{Actor: ByMember(7)}, Approval: a}.Activity()
	if e.Action != ApprovalCreatedAction || e.EntityType != ApprovalEntity || e.EntityID != 5 || e.ProjectID != 0 || e.Actor != ByMember(7) {
		t.Errorf("event %+v", e)
	}
	if e.Details["title"] != "Approve hosting" || e.Details["type"] != RequestBoardApproval {
		t.Errorf("details %v", e.Details)
	}
}

func approvalIn(st ApprovalStatus) Approval {
	return Approval{ID: 5, GuildID: 1, Type: RequestBoardApproval, Status: st, Requester: ByMember(7), Payload: BoardApprovalPayload{Title: "Ship it", Risks: []string{}}}
}

func refused(t *testing.T, err error, msg string) {
	t.Helper()
	var r *ApprovalRefusedError
	if !errors.As(err, &r) || r.Message != msg {
		t.Errorf("err = %v, want refused %q", err, msg)
	}
}

func TestApproveAndReject(t *testing.T) {
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	for _, from := range []ApprovalStatus{StatusPending, StatusRevisionRequested} {
		a := approvalIn(from)
		changed, err := a.Approve(9, "  fine  ", at)
		if err != nil || !changed || a.Status != StatusApproved || a.DeciderID != 9 || a.DecisionNote != "fine" || a.DecidedAt == nil || !a.DecidedAt.Equal(at) {
			t.Errorf("Approve from %s = %v %v, %+v", from, changed, err, a)
		}
		a = approvalIn(from)
		changed, err = a.Reject(9, "", at)
		if err != nil || !changed || a.Status != StatusRejected || a.DecisionNote != "" {
			t.Errorf("Reject from %s = %v %v, %+v", from, changed, err, a)
		}
	}
	// The same Decision again is unchanged, as Paperclip's applied: false.
	a := approvalIn(StatusApproved)
	if changed, err := a.Approve(9, "again", at); err != nil || changed || a.DecisionNote != "" {
		t.Errorf("Approve on approved = %v %v, %+v", changed, err, a)
	}
	a = approvalIn(StatusRejected)
	if changed, err := a.Reject(9, "", at); err != nil || changed {
		t.Errorf("Reject on rejected = %v %v", changed, err)
	}
	a = approvalIn(StatusApproved)
	_, err := a.Reject(9, "", at)
	refused(t, err, "Only pending or revision requested approvals can be rejected")
	a = approvalIn(StatusRejected)
	_, err = a.Approve(9, "", at)
	refused(t, err, "Only pending or revision requested approvals can be approved")
	a = approvalIn(StatusPending)
	if _, err := a.Approve(9, strings.Repeat("x", MaxDecisionNote+1), at); err == nil || a.Status != StatusPending {
		t.Errorf("Approve with a long note = %v, %s", err, a.Status)
	}
}

func TestRequestRevision(t *testing.T) {
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	a := approvalIn(StatusPending)
	if err := a.RequestRevision(9, "more detail", at); err != nil || a.Status != StatusRevisionRequested || a.DeciderID != 9 || a.DecisionNote != "more detail" {
		t.Errorf("RequestRevision = %v, %+v", err, a)
	}
	for _, from := range []ApprovalStatus{StatusRevisionRequested, StatusApproved, StatusRejected} {
		a := approvalIn(from)
		refused(t, a.RequestRevision(9, "", at), "Only pending approvals can request revision")
	}
}

func TestResubmit(t *testing.T) {
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	a := approvalIn(StatusPending)
	if err := a.RequestRevision(9, "more detail", at); err != nil {
		t.Fatal(err)
	}
	if err := a.Resubmit(ByMember(9), nil, at); !errors.Is(err, ErrNotRequester) {
		t.Errorf("Resubmit by another = %v", err)
	}
	if err := a.Resubmit(ByMember(7), &BoardApprovalPayload{Title: " "}, at); err == nil || a.Status != StatusRevisionRequested {
		t.Errorf("Resubmit with a bad payload = %v, %s", err, a.Status)
	}
	if err := a.Resubmit(ByMember(7), &BoardApprovalPayload{Title: " Ship it now ", Risks: []string{"", "late"}}, at); err != nil {
		t.Fatal(err)
	}
	if a.Status != StatusPending || a.DeciderID != 0 || a.DecisionNote != "" || a.DecidedAt != nil || a.Payload.Label() != "Ship it now" || len(a.Payload.(BoardApprovalPayload).Risks) != 1 {
		t.Errorf("after Resubmit: %+v", a)
	}
	for _, from := range []ApprovalStatus{StatusPending, StatusApproved, StatusRejected} {
		a := approvalIn(from)
		refused(t, a.Resubmit(ByMember(7), nil, at), "Only revision requested approvals can be resubmitted")
	}
	// Without a new payload the old one stays.
	a = approvalIn(StatusRevisionRequested)
	if err := a.Resubmit(ByMember(7), nil, at); err != nil || a.Payload.Label() != "Ship it" {
		t.Errorf("Resubmit without payload = %v, %+v", err, a.Payload)
	}
}

func TestNewApprovalComment(t *testing.T) {
	c, err := NewApprovalComment(5, ByMember(7), "  looks good ")
	if err != nil || c.Body != "looks good" || c.ApprovalID != 5 || c.Author != ByMember(7) {
		t.Errorf("NewApprovalComment = %+v, %v", c, err)
	}
	for _, b := range []string{"", "   ", strings.Repeat("x", MaxComment+1)} {
		if _, err := NewApprovalComment(5, ByMember(7), b); err == nil {
			t.Errorf("NewApprovalComment(%d characters) accepted", len(b))
		}
	}
}

func TestApprovalDecisionEvents(t *testing.T) {
	h := Happened{Actor: ByMember(9), At: time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)}
	a := approvalIn(StatusApproved)
	a.DecisionNote = "fine"
	for _, tc := range []struct {
		e      Event
		action string
		note   bool
	}{
		{ApprovalApproved{h, a}, "approval.approved", true},
		{ApprovalRejected{h, a}, "approval.rejected", true},
		{RevisionRequested{h, a}, "approval.revision_requested", true},
		{ApprovalResubmitted{h, approvalIn(StatusPending)}, "approval.resubmitted", false},
	} {
		e := tc.e.Activity()
		if e.Action != tc.action || e.EntityType != ApprovalEntity || e.EntityID != 5 || e.Details["title"] != "Ship it" || e.Details["type"] != RequestBoardApproval {
			t.Errorf("%s: %+v", tc.action, e)
		}
		if _, has := e.Details["decision_note"]; has != tc.note {
			t.Errorf("%s: decision_note present = %v", tc.action, has)
		}
	}
	e := ApprovalCommentWritten{h, a, ApprovalComment{ID: 3, Body: strings.Repeat("é", 200)}}.Activity()
	if e.Action != "approval.comment_added" || e.Details["comment_id"] != uint64(3) || len([]rune(e.Details["snippet"].(string))) != SnippetLength {
		t.Errorf("comment event: %+v", e)
	}
}

func hireIn(st ApprovalStatus) Approval {
	return Approval{ID: 6, GuildID: 1, Type: HireAgent, Status: st, Requester: ByMember(7), Payload: HireAgentPayload{AgentID: 3, Name: "Ada", Roles: []string{}}}
}

func TestHireAgentPayload(t *testing.T) {
	a, err := RequestApproval(1, ByMember(7), HireAgent, HireAgentPayload{
		AgentID: 3, Name: "  Ada ", Job: "engineer", Title: " Backend ", ManagerID: 0, ManagerName: "stale", Roles: []string{" Deployer "},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := a.Payload.(HireAgentPayload)
	if p.Name != "Ada" || p.Title != "Backend" || p.ManagerName != "" || len(p.Roles) != 1 || p.Roles[0] != "Deployer" {
		t.Errorf("payload %+v", p)
	}
	if a.Payload.Label() != "Hire Agent: Ada" {
		t.Errorf("Label = %q", a.Payload.Label())
	}
	many := make([]string, MaxHireRoles+1)
	for i := range many {
		many[i] = "r"
	}
	for name, p := range map[string]HireAgentPayload{
		"no agent":        {Name: "Ada"},
		"empty name":      {AgentID: 3, Name: " "},
		"long name":       {AgentID: 3, Name: strings.Repeat("n", MaxAgentName+1)},
		"long title":      {AgentID: 3, Name: "Ada", Title: strings.Repeat("t", MaxAgentTitle+1)},
		"long abilities":  {AgentID: 3, Name: "Ada", Capabilities: strings.Repeat("c", MaxAgentCapabilities+1)},
		"too many roles":  {AgentID: 3, Name: "Ada", Roles: many},
		"empty role name": {AgentID: 3, Name: "Ada", Roles: []string{" "}},
	} {
		if _, err := RequestApproval(1, ByMember(7), HireAgent, p, nil); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestPayloadMustFitType(t *testing.T) {
	if _, err := RequestApproval(1, ByMember(7), HireAgent, BoardApprovalPayload{Title: "t"}, nil); err == nil {
		t.Error("hire_agent with a board payload accepted")
	}
	if _, err := RequestApproval(1, ByMember(7), RequestBoardApproval, HireAgentPayload{AgentID: 3, Name: "Ada"}, nil); err == nil {
		t.Error("request_board_approval with a hire payload accepted")
	}
	if _, err := RequestApproval(1, ByMember(7), RequestBoardApproval, nil, nil); err == nil {
		t.Error("no payload accepted")
	}
}

func TestCancel(t *testing.T) {
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	for _, from := range []ApprovalStatus{StatusPending, StatusRevisionRequested} {
		a := hireIn(from)
		if changed, err := a.Cancel(at); err != nil || !changed || a.Status != StatusCancelled || a.Actionable() || !a.UpdatedAt.Equal(at) {
			t.Errorf("Cancel from %s = %v %v, %+v", from, changed, err, a)
		}
	}
	a := hireIn(StatusCancelled)
	if changed, err := a.Cancel(at); err != nil || changed {
		t.Errorf("Cancel on cancelled = %v %v", changed, err)
	}
	for _, from := range []ApprovalStatus{StatusApproved, StatusRejected} {
		a := hireIn(from)
		_, err := a.Cancel(at)
		refused(t, err, "Only pending or revision requested approvals can be cancelled")
	}
	b := approvalIn(StatusPending)
	_, err := b.Cancel(at)
	refused(t, err, "Only hire agent approvals can be cancelled")
	// Nothing leaves cancelled.
	a = hireIn(StatusCancelled)
	_, err = a.Approve(9, "", at)
	refused(t, err, "Only pending or revision requested approvals can be approved")
	_, err = a.Reject(9, "", at)
	refused(t, err, "Only pending or revision requested approvals can be rejected")
}

func TestHireNeverRevised(t *testing.T) {
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	a := hireIn(StatusPending)
	refused(t, a.RequestRevision(9, "", at), "Hire agent approvals cannot be sent back for revision")
	a = hireIn(StatusRevisionRequested)
	refused(t, a.Resubmit(ByMember(7), HireAgentPayload{AgentID: 3, Name: "Bob"}, at), "A hire agent approval's payload cannot change")
	if err := a.Resubmit(ByMember(7), nil, at); err != nil || a.Status != StatusPending {
		t.Errorf("plain Resubmit of a hire = %v, %s", err, a.Status)
	}
}

func TestHireActivity(t *testing.T) {
	h := Happened{Actor: ByMember(9)}
	e := ApprovalRequested{Happened: h, Approval: hireIn(StatusPending)}.Activity()
	if e.Details["title"] != "Hire Agent: Ada" || e.Details["type"] != HireAgent {
		t.Errorf("created details %v", e.Details)
	}
	e = ApprovalCancelled{Happened: h, Approval: hireIn(StatusCancelled)}.Activity()
	if e.Action != "approval.cancelled" || e.EntityType != ApprovalEntity || e.Details["title"] != "Hire Agent: Ada" {
		t.Errorf("cancelled event %+v", e)
	}
}

func TestAgentEvent(t *testing.T) {
	ev := AgentEvent{Happened: Happened{Actor: ByMember(9)}, GuildID: 1, AgentID: 3, AgentName: "Ada", Action: AgentPausedAction, Details: map[string]any{"x": 1}}
	v, err := ev.Validated()
	if err != nil {
		t.Fatal(err)
	}
	e := v.Activity()
	if e.EntityType != AgentEntity || e.EntityID != 3 || e.ProjectID != 0 || e.Details["name"] != "Ada" || e.Details["x"] != 1 {
		t.Errorf("agent event %+v", e)
	}
	for name, bad := range map[string]AgentEvent{
		"not an agent action":      {GuildID: 1, AgentID: 3, AgentName: "Ada", Action: "issue.created"},
		"no agent":                 {GuildID: 1, AgentName: "Ada", Action: AgentPausedAction},
		"no name":                  {GuildID: 1, AgentID: 3, Action: AgentPausedAction},
		"agent action on a budget": {GuildID: 1, BudgetID: 5, AgentName: "Shop", Action: AgentPausedAction},
	} {
		if _, err := bad.Validated(); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	// A Project's Budget is in its Project; a Budget incident is its own entity.
	b, err := AgentEvent{Happened: Happened{Actor: ByMember(9)}, GuildID: 1, BudgetID: 5, AgentName: "Shop", Action: BudgetUpdatedAction,
		Details: map[string]any{"scope_type": "project", "scope_id": uint64(12)}}.Validated()
	if e := b.Activity(); err != nil || e.EntityType != BudgetEntity || e.EntityID != 5 || e.ProjectID != 12 || e.Details["name"] != "Shop" {
		t.Errorf("budget event %+v %v", e, err)
	}
	i, err := AgentEvent{GuildID: 1, BudgetID: 6, AgentName: "Ada", Action: BudgetHardCrossedAction, Details: map[string]any{"scope_type": "agent", "scope_id": uint64(3)}}.Validated()
	if e := i.Activity(); err != nil || e.EntityType != IncidentEntity || e.EntityID != 6 || e.ProjectID != 0 {
		t.Errorf("incident event %+v %v", e, err)
	}
}
