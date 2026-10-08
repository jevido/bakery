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
	for _, s := range []string{"", "hire_agent", "Request_Board_Approval"} {
		if _, err := ParseApprovalType(s); err == nil {
			t.Errorf("ParseApprovalType(%q) accepted", s)
		}
	}
}

func TestApprovalStatusActionable(t *testing.T) {
	for st, want := range map[ApprovalStatus]bool{StatusPending: true, StatusRevisionRequested: true, StatusApproved: false, StatusRejected: false} {
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
	if a.Status != StatusPending || !a.Actionable() || a.RequesterID != 7 || a.DeciderID != 0 || a.DecidedAt != nil {
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

func approvalIn(st ApprovalStatus) Approval {
	return Approval{ID: 5, GuildID: 1, Type: RequestBoardApproval, Status: st, RequesterID: 7, Payload: BoardApprovalPayload{Title: "Ship it", Risks: []string{}}}
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
	if err := a.Resubmit(9, nil, at); !errors.Is(err, ErrNotRequester) {
		t.Errorf("Resubmit by another = %v", err)
	}
	if err := a.Resubmit(7, &BoardApprovalPayload{Title: " "}, at); err == nil || a.Status != StatusRevisionRequested {
		t.Errorf("Resubmit with a bad payload = %v, %s", err, a.Status)
	}
	if err := a.Resubmit(7, &BoardApprovalPayload{Title: " Ship it now ", Risks: []string{"", "late"}}, at); err != nil {
		t.Fatal(err)
	}
	if a.Status != StatusPending || a.DeciderID != 0 || a.DecisionNote != "" || a.DecidedAt != nil || a.Payload.Title != "Ship it now" || len(a.Payload.Risks) != 1 {
		t.Errorf("after Resubmit: %+v", a)
	}
	for _, from := range []ApprovalStatus{StatusPending, StatusApproved, StatusRejected} {
		a := approvalIn(from)
		refused(t, a.Resubmit(7, nil, at), "Only revision requested approvals can be resubmitted")
	}
	// Without a new payload the old one stays.
	a = approvalIn(StatusRevisionRequested)
	if err := a.Resubmit(7, nil, at); err != nil || a.Payload.Title != "Ship it" {
		t.Errorf("Resubmit without payload = %v, %+v", err, a.Payload)
	}
}

func TestNewApprovalComment(t *testing.T) {
	c, err := NewApprovalComment(5, 7, "  looks good ")
	if err != nil || c.Body != "looks good" || c.ApprovalID != 5 || c.AuthorID != 7 {
		t.Errorf("NewApprovalComment = %+v, %v", c, err)
	}
	for _, b := range []string{"", "   ", strings.Repeat("x", MaxComment+1)} {
		if _, err := NewApprovalComment(5, 7, b); err == nil {
			t.Errorf("NewApprovalComment(%d characters) accepted", len(b))
		}
	}
}

func TestApprovalDecisionEvents(t *testing.T) {
	h := Happened{ActorID: 9, At: time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)}
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
