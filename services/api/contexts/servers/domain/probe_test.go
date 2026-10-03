package domain

import (
	"slices"
	"testing"
)

func validated() Server {
	s := Server{Kind: Remote}
	s.RecordValidation(Validation{Checks: []Check{{Name: CheckSSH, OK: true, Required: true}}}, "key")
	return s
}

func TestProbeUnreachableAfterTwoFailures(t *testing.T) {
	s := validated()
	if got := s.RecordProbe(false, 0, 0); len(got) != 0 || s.Status != Reachable {
		t.Fatalf("one failure: %v %s", got, s.Status)
	}
	if got := s.RecordProbe(false, 0, 0); !slices.Equal(got, []HealthChange{BecameUnreachable}) || s.Status != Unreachable {
		t.Fatalf("two failures: %v %s", got, s.Status)
	}
	if got := s.RecordProbe(false, 0, 0); len(got) != 0 {
		t.Fatalf("still down repeated: %v", got)
	}
	if got := s.RecordProbe(true, 10, 100); !slices.Equal(got, []HealthChange{BecameReachable}) || s.Status != Reachable || s.FailedProbes != 0 {
		t.Fatalf("back: %v %+v", got, s)
	}
	if got := s.RecordProbe(true, 10, 100); len(got) != 0 {
		t.Fatalf("still up repeated: %v", got)
	}
	// A failure between successes does not count up to two.
	s.RecordProbe(false, 0, 0)
	s.RecordProbe(true, 10, 100)
	if got := s.RecordProbe(false, 0, 0); len(got) != 0 || s.Status != Reachable {
		t.Fatalf("not in a row: %v %s", got, s.Status)
	}
}

func TestProbeHighDiskUsage(t *testing.T) {
	s := validated()
	if got := s.RecordProbe(true, 89, 100); len(got) != 0 {
		t.Fatalf("89 %%: %v", got)
	}
	if got := s.RecordProbe(true, 90, 100); !slices.Equal(got, []HealthChange{BecameHighDiskUsage}) || !s.HighDiskUsage {
		t.Fatalf("90 %%: %v", got)
	}
	if got := s.RecordProbe(true, 95, 100); len(got) != 0 {
		t.Fatalf("still full repeated: %v", got)
	}
	if got := s.RecordProbe(true, 86, 100); len(got) != 0 || !s.HighDiskUsage {
		t.Fatalf("86 %% cleared it: %v", got)
	}
	s.RecordProbe(true, 84, 100)
	if s.HighDiskUsage {
		t.Fatal("84 % did not clear it")
	}
	if got := s.RecordProbe(true, 91, 100); !slices.Equal(got, []HealthChange{BecameHighDiskUsage}) {
		t.Fatalf("full again: %v", got)
	}
}

func TestProbeLeavesFailedValidationAlone(t *testing.T) {
	s := Server{Kind: Remote}
	s.RecordValidation(Validation{Checks: []Check{{Name: CheckLinger, OK: false, Required: true}}}, "key")
	if s.Probed() {
		t.Fatal("a Server whose Validation failed is probed")
	}
	if got := s.RecordProbe(true, 99, 100); len(got) != 0 || s.Status != Unreachable || s.HighDiskUsage {
		t.Fatalf("got %v %+v", got, s)
	}
	u := Server{Kind: Remote, Status: Unvalidated}
	if u.Probed() {
		t.Fatal("an unvalidated Server is probed")
	}
}
