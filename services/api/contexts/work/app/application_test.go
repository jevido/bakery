package app

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// memProjects has Projects 1 and 2; Application 10 is in Project 1 and
// Application 20 in Project 2.
type memProjects struct{}

func (memProjects) ProjectNames(_ context.Context, _ uint64, ids []uint64) (map[uint64]string, error) {
	out := map[uint64]string{}
	for _, id := range ids {
		if id == 1 || id == 2 {
			out[id] = "P"
		}
	}
	return out, nil
}

func (memProjects) ApplicationNames(_ context.Context, _ uint64, ids []uint64) (map[uint64]string, error) {
	out := map[uint64]string{}
	for _, id := range ids {
		if id == 10 || id == 20 {
			out[id] = "A"
		}
	}
	return out, nil
}

func (memProjects) ApplicationInProject(_ context.Context, _, projectID, applicationID uint64) (bool, error) {
	return applicationID == projectID*10, nil
}

func TestIssueApplication(t *testing.T) {
	ctx := context.Background()
	act := &memActivity{}
	s := NewService(nil, &memIssues{byID: map[uint64]domain.Issue{}}, nil, nil, memberGuild{}, memProjects{}, act, nil, nil, nil)
	s.Logf = t.Logf
	all := func(ids []uint64) ([]uint64, error) { return ids, nil }
	var fe *domain.FieldError

	if _, err := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: "No project", ApplicationID: 10}, all); !errors.As(err, &fe) || fe.Field != "application_id" {
		t.Fatalf("an Issue without a Project named an Application: %v", err)
	}
	if _, err := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: "Other project", ProjectID: 1, ApplicationID: 20}, all); !errors.As(err, &fe) || fe.Field != "application_id" {
		t.Fatalf("an Issue named another Project's Application: %v", err)
	}
	i, err := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: "Fix", ProjectID: 1, ApplicationID: 10}, all)
	if err != nil || i.ApplicationID != 10 {
		t.Fatalf("CreateIssue = %+v, %v", i, err)
	}
	ref := "1"
	if _, err := s.ChangeIssue(ctx, 1, domain.ByMember(7), ref, IssuePatch{ApplicationID: ptr(uint64(20))}, all); !errors.As(err, &fe) || fe.Field != "application_id" {
		t.Fatalf("changed to another Project's Application: %v", err)
	}
	act.actions = nil
	if i, err = s.ChangeIssue(ctx, 1, domain.ByMember(7), ref, IssuePatch{ApplicationID: ptr(uint64(0))}, all); err != nil || i.ApplicationID != 0 {
		t.Fatalf("clearing the Application = %+v, %v", i, err)
	}
	if !slices.Equal(act.actions, []string{domain.IssueApplicationChangedAction}) {
		t.Fatalf("clearing the Application recorded %v", act.actions)
	}
	if i, err = s.ChangeIssue(ctx, 1, domain.ByMember(7), ref, IssuePatch{ApplicationID: ptr(uint64(10))}, all); err != nil || i.ApplicationID != 10 {
		t.Fatalf("naming the Application = %+v, %v", i, err)
	}
	// Moving to Project 2 and naming its Application in one change.
	if i, err = s.ChangeIssue(ctx, 1, domain.ByMember(7), ref, IssuePatch{ProjectID: ptr(uint64(2)), ApplicationID: ptr(uint64(20))}, all); err != nil || i.ApplicationID != 20 {
		t.Fatalf("moving with the new Project's Application = %+v, %v", i, err)
	}
	if i, err = s.ChangeIssue(ctx, 1, domain.ByMember(7), ref, IssuePatch{ProjectID: ptr(uint64(1))}, all); err != nil || i.ApplicationID != 0 {
		t.Fatalf("moving to another Project kept the Application: %+v, %v", i, err)
	}
}

func TestIssueApplicationChangedKeepsNames(t *testing.T) {
	e := domain.IssueApplicationChanged{Issue: domain.Issue{ID: 1, ProjectID: 1}, From: &domain.NamedApplication{ID: 10, Name: "web"}}
	a := e.Activity()
	from, _ := a.Details["from"].(map[string]any)
	if a.Action != domain.IssueApplicationChangedAction || from["name"] != "web" || a.Details["to"] != nil {
		t.Fatalf("activity = %+v", a)
	}
}
