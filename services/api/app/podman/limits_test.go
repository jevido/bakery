package podman

import "testing"

func TestLimits(t *testing.T) {
	if Limits(0, 0) != nil {
		t.Fatal("no limits should be nil")
	}
	l := Limits(0, 1.25)
	if l.Memory != nil || l.CPU.Quota != 125000 || l.CPU.Period != 100000 {
		t.Fatalf("%+v", l)
	}
}
