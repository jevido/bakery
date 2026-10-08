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

func TestStatusMovesAndEdit(t *testing.T) {
	at := time.Now()
	a, _ := Hire(1, 7, Profile{Name: "Ada"}, 0, at)
	name := "Bea"
	if _, err := a.Edit(Patch{Name: &name}, at); err == nil {
		t.Error("a pending agent was edited")
	}
	if err := a.Pause(at); err == nil {
		t.Error("a pending agent was paused")
	}
	a.Approve(at)
	bad := "nope"
	if _, err := a.Edit(Patch{Job: &bad}, at); err == nil || a.Job != DefaultJob {
		t.Errorf("unknown job: %v, %s", err, a.Job)
	}
	var m uint64 = 3
	ch, err := a.Edit(Patch{Name: &name, ManagerID: &m}, at)
	if err != nil || len(ch) != 2 || ch["name"] != (Change{"Ada", "Bea"}) || ch["reports_to"] != (Change{uint64(0), uint64(3)}) {
		t.Errorf("edit: %v %v", ch, err)
	}
	if err := a.Resume(at); err == nil {
		t.Error("an idle agent was resumed")
	}
	if a.Pause(at) != nil || a.Resume(at) != nil || a.Terminate(at) != nil {
		t.Fatal("pause, resume, terminate")
	}
	if a.Terminate(at) == nil || a.Resume(at) == nil || a.Rolable() == nil {
		t.Error("a terminated agent moved")
	}
}

func TestEditHeartbeat(t *testing.T) {
	a, _ := Hire(1, 7, Profile{Name: "Ada"}, 0, now)
	if a.Heartbeat != DefaultHeartbeatPolicy() || a.Heartbeat.Enabled || a.Heartbeat.IntervalSec != 300 || !a.Heartbeat.WakeOnDemand {
		t.Fatalf("default policy %+v", a.Heartbeat)
	}
	a.Status = Idle
	on := true
	changes, err := a.Edit(Patch{Heartbeat: &HeartbeatPatch{Enabled: &on}}, now)
	if err != nil || !a.Heartbeat.Enabled || a.Heartbeat.IntervalSec != 300 || len(changes) != 1 || changes["heartbeat.enabled"] != (Change{From: false, To: true}) {
		t.Fatalf("partial patch: %+v %v %v", a.Heartbeat, changes, err)
	}
	for _, sec := range []int{59, 86401, 0} {
		var fe *FieldError
		if _, err := a.Edit(Patch{Heartbeat: &HeartbeatPatch{IntervalSec: &sec}}, now); !errors.As(err, &fe) || fe.Field != "heartbeat.interval_sec" {
			t.Errorf("interval %d: %v", sec, err)
		}
	}
	for _, sec := range []int{60, 86400} {
		if _, err := a.Edit(Patch{Heartbeat: &HeartbeatPatch{IntervalSec: &sec}}, now); err != nil || a.Heartbeat.IntervalSec != sec {
			t.Errorf("interval %d: %+v %v", sec, a.Heartbeat, err)
		}
	}
	later := now.Add(time.Hour)
	if changes, err := a.Edit(Patch{Heartbeat: &HeartbeatPatch{Enabled: &on}}, later); err != nil || len(changes) != 0 || a.UpdatedAt.Equal(later) {
		t.Errorf("no-op: %v %v", changes, err)
	}
}

func TestEditHeartbeatWhileRunning(t *testing.T) {
	a, _ := Hire(1, 7, Profile{Name: "Ada"}, 0, now)
	a.Status = Running
	off := false
	if _, err := a.Edit(Patch{Heartbeat: &HeartbeatPatch{WakeOnDemand: &off}}, now); err != nil || a.Heartbeat.WakeOnDemand {
		t.Fatalf("policy while running: %+v %v", a.Heartbeat, err)
	}
	title := "Head"
	var se *StatusError
	if _, err := a.Edit(Patch{Title: &title, Heartbeat: &HeartbeatPatch{WakeOnDemand: &off}}, now); !errors.As(err, &se) {
		t.Errorf("title while running: %v", err)
	}
}

func TestHeartbeatDue(t *testing.T) {
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	last := at.Add(time.Minute)
	a := Agent{Status: Idle, CreatedAt: at, Heartbeat: HeartbeatPolicy{Enabled: true, IntervalSec: 60}}
	for _, c := range []struct {
		name string
		edit func(*Agent)
		now  time.Duration
		want bool
	}{
		{"before the first interval", func(*Agent) {}, 59 * time.Second, false},
		{"one interval after hire", func(*Agent) {}, time.Minute, true},
		{"timer off", func(a *Agent) { a.Heartbeat.Enabled = false }, time.Hour, false},
		{"running", func(a *Agent) { a.Status = Running }, time.Hour, true},
		{"error", func(a *Agent) { a.Status = Error }, time.Hour, true},
		{"paused", func(a *Agent) { a.Status = Paused }, time.Hour, false},
		{"since the last heartbeat", func(a *Agent) { a.LastHeartbeatAt = &last }, 119 * time.Second, false},
		{"an interval after the last", func(a *Agent) { a.LastHeartbeatAt = &last }, 2 * time.Minute, true},
	} {
		b := a
		c.edit(&b)
		if got := b.HeartbeatDue(at.Add(c.now)); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

func TestChainOfCommand(t *testing.T) {
	ceo := Agent{ID: 1, Name: "Ceo"}
	cto := Agent{ID: 2, Name: "Cto", ManagerID: 1}
	dev := Agent{ID: 3, Name: "Dev", ManagerID: 2}
	all := []Agent{ceo, cto, dev}
	chain := ChainOfCommand(all, dev)
	if len(chain) != 2 || chain[0].ID != 2 || chain[1].ID != 1 {
		t.Fatalf("chain of dev: %+v", chain)
	}
	if chain := ChainOfCommand(all, ceo); len(chain) != 0 {
		t.Fatalf("chain of the top: %+v", chain)
	}
	// A cycle in stored rows ends the walk instead of looping.
	loop := []Agent{{ID: 1, ManagerID: 2}, {ID: 2, ManagerID: 1}}
	if chain := ChainOfCommand(loop, loop[0]); len(chain) != 1 || chain[0].ID != 2 {
		t.Fatalf("chain through a cycle: %+v", chain)
	}
}
