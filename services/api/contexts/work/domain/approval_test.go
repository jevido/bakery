package domain

import (
	"strings"
	"testing"
)

func TestParseApprovalType(t *testing.T) {
	if _, err := ParseApprovalType("request_board_approval"); err != nil {
		t.Errorf("ParseApprovalType = %v", err)
	}
	for _, s := range []string{"", "hire_agent", "Request_Board_Approval"} {
		if _, err := ParseApprovalType(s); err == nil {
			t.Errorf("ParseApprovalType(%q) accepted", s)
		}
	}
}

func TestApprovalStatusActionable(t *testing.T) {
	for st, want := range map[ApprovalStatus]bool{ApprovalPending: true, RevisionRequested: true, ApprovalApproved: false, ApprovalRejected: false} {
		if got := st.Actionable(); got != want {
			t.Errorf("%s.Actionable() = %v", st, got)
		}
	}
	if _, err := ParseApprovalStatus("cancelled"); err == nil {
		t.Error("ParseApprovalStatus(cancelled) accepted")
	}
}

func TestRequestApproval(t *testing.T) {
	a, err := RequestApproval(1, 7, RequestBoardApproval, BoardApprovalPayload{
		Title: "  Approve hosting ", Summary: " Costs **42** ", Risks: []string{"May grow", "  ", "Lock-in"},
	}, []uint64{3, 4, 3})
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != ApprovalPending || !a.Actionable() || a.RequesterID != 7 || a.DeciderID != 0 || a.DecidedAt != nil {
		t.Errorf("starting state %+v", a)
	}
	if a.Payload.Title != "Approve hosting" || a.Payload.Summary != "Costs **42**" || len(a.Payload.Risks) != 2 {
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
		if _, err := RequestApproval(1, 1, RequestBoardApproval, p, nil); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestApprovalRequestedActivity(t *testing.T) {
	a := Approval{ID: 5, GuildID: 1, Type: RequestBoardApproval, Payload: BoardApprovalPayload{Title: "Approve hosting"}, IssueIDs: []uint64{3}}
	e := ApprovalRequested{Happened: Happened{ActorID: 7}, Approval: a}.Activity()
	if e.Action != ApprovalCreatedAction || e.EntityType != ApprovalEntity || e.EntityID != 5 || e.ProjectID != 0 || e.ActorID != 7 {
		t.Errorf("event %+v", e)
	}
	if e.Details["title"] != "Approve hosting" || e.Details["type"] != RequestBoardApproval {
		t.Errorf("details %v", e.Details)
	}
}
