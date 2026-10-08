package domain

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestActivityActions(t *testing.T) {
	h := Happened{Actor: ByMember(4), At: time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)}
	g := Goal{ID: 2, GuildID: 1, Title: "Ship", Level: TaskLevel, Status: Planned}
	i := Issue{ID: 9, GuildID: 1, Number: 3, Title: "Fix", ProjectID: 5, Status: Todo, Priority: Medium}
	d := IssueDocument{Key: "plan", Title: "Plan", Latest: 2}
	for _, tc := range []struct {
		e      Event
		action string
		entity string
	}{
		{GoalCreated{h, g}, "goal.created", GoalEntity},
		{GoalChanged{h, g, g}, "goal.updated", GoalEntity},
		{GoalDeleted{h, g}, "goal.deleted", GoalEntity},
		{IssueCreated{h, i}, "issue.created", IssueEntity},
		{IssueChanged{Happened: h, Before: i, After: i}, "issue.updated", IssueEntity},
		{IssueDeleted{h, i}, "issue.deleted", IssueEntity},
		{CommentWritten{h, i, Comment{ID: 7}}, "issue.comment_added", IssueEntity},
		{CommentDeleted{h, i, Comment{ID: 7}}, "issue.comment_deleted", IssueEntity},
		{DocumentSaved{Happened: h, Issue: i, Document: d, First: true}, "issue.document_created", IssueEntity},
		{DocumentSaved{Happened: h, Issue: i, Document: d}, "issue.document_updated", IssueEntity},
		{DocumentDeleted{h, i, d}, "issue.document_deleted", IssueEntity},
	} {
		a := tc.e.Activity()
		if a.Action != tc.action || a.EntityType != tc.entity || a.GuildID != 1 || a.Actor != ByMember(4) || !a.CreatedAt.Equal(h.At) {
			t.Errorf("%T: %+v", tc.e, a)
		}
		if tc.entity == IssueEntity && (a.EntityID != 9 || a.ProjectID != 5 || a.Details["issue_number"] != 3 || a.Details["issue_title"] != "Fix") {
			t.Errorf("%T: Issue event %+v", tc.e, a)
		}
		if tc.entity == GoalEntity && (a.EntityID != 2 || a.ProjectID != 0 || a.Details["title"] != "Ship") {
			t.Errorf("%T: Goal event %+v", tc.e, a)
		}
	}
}

func TestIssueChangedStatusAndBlocker(t *testing.T) {
	before := Issue{ID: 9, Status: Todo, Priority: Medium, AssigneeID: 3}
	after := before
	after.Status, after.AssigneeID = InProgress, 0
	e := IssueChanged{Before: before, After: after, BlockersBefore: []uint64{11}, BlockersAfter: []uint64{12, 11}}
	want := map[string]any{
		"status":   map[string]any{"from": Todo, "to": InProgress},
		"assignee": map[string]any{"from": map[string]any{"id": uint64(3), "kind": "member"}, "to": nil},
		"blockers": map[string]any{"added": []uint64{12}, "removed": []uint64{}},
	}
	if got := e.Changes(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changes() = %#v, want %#v", got, want)
	}
}

func TestIssueChangedToAnAgent(t *testing.T) {
	before := Issue{ID: 9, Status: Todo, Priority: Medium, AssigneeID: 3}
	after := before
	after.AssignAgent(5)
	want := map[string]any{"assignee": map[string]any{
		"from": map[string]any{"id": uint64(3), "kind": "member"},
		"to":   map[string]any{"id": uint64(5), "kind": "agent"},
	}}
	if got := (IssueChanged{Before: before, After: after}).Changes(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changes() = %#v, want %#v", got, want)
	}
}

func TestNothingChanged(t *testing.T) {
	i := Issue{ID: 9, Status: Todo, Priority: Medium}
	if c := (IssueChanged{Before: i, After: i, BlockersBefore: []uint64{1}, BlockersAfter: []uint64{1}}).Changes(); len(c) != 0 {
		t.Errorf("Issue Changes() = %v, want none", c)
	}
	g := Goal{ID: 2, Title: "Ship"}
	if c := (GoalChanged{Before: g, After: g}).Changes(); len(c) != 0 {
		t.Errorf("Goal Changes() = %v, want none", c)
	}
}

func TestGoalChangedDescriptionAndOwner(t *testing.T) {
	before := Goal{Description: "a"}
	after := Goal{Description: "b", OwnerID: 6}
	want := map[string]any{"description": true, "owner": map[string]any{"from": nil, "to": uint64(6)}}
	if got := (GoalChanged{Before: before, After: after}).Changes(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changes() = %#v, want %#v", got, want)
	}
}

func TestCommentSnippet(t *testing.T) {
	body := strings.Repeat("é", 200)
	a := CommentWritten{Comment: Comment{ID: 7, Body: body}}.Activity()
	if s := a.Details["snippet"].(string); len([]rune(s)) != SnippetLength {
		t.Errorf("snippet has %d characters", len([]rune(s)))
	}
}

func TestDocumentRestored(t *testing.T) {
	a := DocumentSaved{Document: IssueDocument{Key: "plan", Latest: 3}, RestoredFrom: 1}.Activity()
	if a.Action != DocumentUpdatedAction || a.Details["restored_from"] != 1 || a.Details["revision_number"] != 3 {
		t.Errorf("restore = %+v", a)
	}
}
