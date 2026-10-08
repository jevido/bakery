package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

func TestHire(t *testing.T) {
	a, err := Hire(1, 7, Profile{Name: "  Ada ", Job: "cto", Title: "Head of Platform", Icon: "rocket", Capabilities: "Go"}, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "Ada" || a.Job != "cto" || a.Status != PendingApproval || a.HirerID != 7 || a.GuildID != 1 {
		t.Fatalf("got %+v", a)
	}
	if JobLabel(a.Job) != "CTO" {
		t.Fatalf("label %q", JobLabel(a.Job))
	}
	b, err := Hire(1, 7, Profile{Name: "Bob"}, 0, now)
	if err != nil || b.Job != DefaultJob || b.Icon != "" {
		t.Fatalf("defaults: %+v %v", b, err)
	}
}

func TestHireRefuses(t *testing.T) {
	for field, p := range map[string]Profile{
		"name":         {Name: "   "},
		"job":          {Name: "Ada", Job: "wizard"},
		"title":        {Name: "Ada", Title: strings.Repeat("x", MaxTitle+1)},
		"icon":         {Name: "Ada", Icon: "unicorn"},
		"capabilities": {Name: "Ada", Capabilities: strings.Repeat("x", MaxCapabilities+1)},
	} {
		_, err := Hire(1, 7, p, 0, now)
		var fe *FieldError
		if !errors.As(err, &fe) || fe.Field != field {
			t.Errorf("%s: got %v", field, err)
		}
	}
	if _, err := Hire(1, 7, Profile{Name: strings.Repeat("é", MaxName+1)}, 0, now); err == nil {
		t.Error("long name accepted")
	}
}

func chain() []Agent {
	// 1 <- 2 <- 3 <- 4, and 5 terminated.
	return []Agent{
		{ID: 1, Name: "Ceo", Status: Idle},
		{ID: 2, Name: "Cto", Status: Idle, ManagerID: 1},
		{ID: 3, Name: "Eng", Status: Paused, ManagerID: 2},
		{ID: 4, Name: "Qa", Status: Idle, ManagerID: 3},
		{ID: 5, Name: "Gone", Status: Terminated},
	}
}

func TestCheckManager(t *testing.T) {
	as := chain()
	if err := CheckManager(as, 4, 1); err != nil {
		t.Errorf("valid manager: %v", err)
	}
	if err := CheckManager(as, 0, 4); err != nil {
		t.Errorf("new agent under a leaf: %v", err)
	}
	if err := CheckManager(as, 4, 0); err != nil {
		t.Errorf("no manager: %v", err)
	}
	if err := CheckManager(as, 2, 2); !errors.Is(err, ErrManagerCycle) {
		t.Errorf("self: %v", err)
	}
	// Depth > 2: the CEO under its report's report's report.
	if err := CheckManager(as, 1, 4); !errors.Is(err, ErrManagerCycle) {
		t.Errorf("cycle at depth 3: %v", err)
	}
	var fe *FieldError
	if err := CheckManager(as, 4, 5); !errors.As(err, &fe) || fe.Field != "reports_to" {
		t.Errorf("terminated manager: %v", err)
	}
	if err := CheckManager(as, 4, 99); !errors.As(err, &fe) || fe.Field != "reports_to" {
		t.Errorf("other guild's manager: %v", err)
	}
}

func TestApproveAndReject(t *testing.T) {
	a := Agent{Status: PendingApproval}
	if ok, err := a.Approve(now); !ok || err != nil || a.Status != Idle {
		t.Fatalf("approve: %v %v %s", ok, err, a.Status)
	}
	if ok, err := a.Approve(now); ok || err != nil {
		t.Fatalf("approve again: %v %v", ok, err)
	}
	var se *StatusError
	if _, err := a.Reject(now); !errors.As(err, &se) {
		t.Fatalf("reject idle: %v", err)
	}
	b := Agent{Status: PendingApproval}
	if ok, err := b.Reject(now); !ok || err != nil || b.Status != Terminated || b.TerminatedAt == nil {
		t.Fatalf("reject: %v %v %+v", ok, err, b)
	}
	if ok, err := b.Reject(now); ok || err != nil {
		t.Fatalf("reject again: %v %v", ok, err)
	}
	if _, err := b.Approve(now); !errors.As(err, &se) {
		t.Fatalf("approve terminated: %v", err)
	}
}

func TestOrg(t *testing.T) {
	as := append(chain(), Agent{ID: 6, Name: "orphan", Status: Idle, ManagerID: 5}, Agent{ID: 7, Name: "Analyst", Status: Idle, ManagerID: 1})
	org := Org(as)
	if len(org) != 2 || org[0].Agent.ID != 1 || org[1].Agent.ID != 6 {
		t.Fatalf("roots: %+v", org)
	}
	if r := org[0].Reports; len(r) != 2 || r[0].Agent.Name != "Analyst" || r[1].Agent.Name != "Cto" {
		t.Fatalf("reports by name: %+v", r)
	}
	if len(org[0].Reports[1].Reports[0].Reports) != 1 {
		t.Fatalf("depth: %+v", org[0].Reports[1])
	}
}
