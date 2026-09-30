package infra

import (
	"math"
	"testing"
)

func TestCPUBetween(t *testing.T) {
	first := "cpu  100 0 100 700 100 0 0 0 0 0"
	second := "cpu  150 0 150 800 100 0 0 0 0 0"
	got, err := cpuBetween(first, second)
	if err != nil || math.Abs(got-50) > 0.001 {
		t.Fatalf("%v %v", got, err)
	}
	if _, err := cpuBetween("intr 1 2", second); err == nil {
		t.Fatal("bad line accepted")
	}
}

func TestParseDF(t *testing.T) {
	out := "Filesystem        1-blocks        Used   Available Capacity Mounted on\n/dev/nvme0n1p2 1000000000000 400000000000 600000000000      40% /\n"
	total, avail, err := parseDF(out)
	if err != nil || total != 1000000000000 || avail != 600000000000 {
		t.Fatalf("%d %d %v", total, avail, err)
	}
	if _, _, err := parseDF("nope"); err == nil {
		t.Fatal("bad output accepted")
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("/a b/it's"); got != `'/a b/it'\''s'` {
		t.Fatal(got)
	}
}

func TestMemAvailable(t *testing.T) {
	got, ok := memAvailable("MemTotal:       65547308 kB\nMemFree:         5255188 kB\nMemAvailable:   40000000 kB\n")
	if !ok || got != 40000000*1024 {
		t.Fatal(got, ok)
	}
	if _, ok := memAvailable("MemTotal: 1 kB\n"); ok {
		t.Fatal("found MemAvailable in nothing")
	}
}
