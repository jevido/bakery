package domain

import (
	"errors"
	"testing"
)

func TestDeriveIssuePrefix(t *testing.T) {
	for name, want := range map[string]string{
		"Default":      "DEF",
		"bakers guild": "BAK",
		"A1 b2":        "AB",
		"x":            "X",
		"42 — 🍞":       "GLD",
		"":             "GLD",
		"Crème brûlée": "CRM",
	} {
		if got := DeriveIssuePrefix(name); got != want {
			t.Errorf("DeriveIssuePrefix(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestParseIssuePrefix(t *testing.T) {
	for in, want := range map[string]string{"bak": "BAK", " DEF ": "DEF", "A": "A", "abcde": "ABCDE"} {
		if got, err := ParseIssuePrefix(in); err != nil || got != want {
			t.Errorf("ParseIssuePrefix(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "  ", "TOOLONG", "DE-F", "D3F", "DÉF"} {
		if _, err := ParseIssuePrefix(in); !errors.Is(err, ErrInvalidIssuePrefix) {
			t.Errorf("ParseIssuePrefix(%q) = %v, want ErrInvalidIssuePrefix", in, err)
		}
	}
}

func TestFreeIssuePrefix(t *testing.T) {
	in := func(taken ...string) func(string) bool {
		return func(p string) bool {
			for _, t := range taken {
				if t == p {
					return true
				}
			}
			return false
		}
	}
	cases := []struct {
		base  string
		taken []string
		want  string
	}{
		{"DEF", nil, "DEF"},
		{"DEF", []string{"DEF"}, "DEFA"},
		{"DEF", []string{"DEF", "DEFA", "DEFB"}, "DEFC"},
		{"DEF", append([]string{"DEF"}, letters("DEF")...), "DEFAA"},
		{"DEF", append(append([]string{"DEF"}, letters("DEF")...), "DEFAA"), "DEFAB"},
	}
	for _, c := range cases {
		if got := FreeIssuePrefix(c.base, in(c.taken...)); got != c.want {
			t.Errorf("FreeIssuePrefix(%q, %v) = %q, want %q", c.base, c.taken, got, c.want)
		}
	}
}

// letters is base followed by each of A to Z.
func letters(base string) []string {
	out := make([]string, 26)
	for i := range out {
		out[i] = base + string(rune('A'+i))
	}
	return out
}

func TestNewGuildDerivesItsIssuePrefix(t *testing.T) {
	g, err := NewGuild("  Default ", "", 1)
	if err != nil || g.IssuePrefix != "DEF" {
		t.Fatalf("got %+v, %v", g, err)
	}
	if err := g.ChangeIssuePrefix("bak"); err != nil || g.IssuePrefix != "BAK" {
		t.Errorf("ChangeIssuePrefix: %q, %v", g.IssuePrefix, err)
	}
	if err := g.ChangeIssuePrefix("TOOLONG"); !errors.Is(err, ErrInvalidIssuePrefix) || g.IssuePrefix != "BAK" {
		t.Errorf("bad prefix: %q, %v", g.IssuePrefix, err)
	}
	if err := g.Rename("Pastry"); err != nil || g.IssuePrefix != "BAK" {
		t.Errorf("rename changed the prefix: %q", g.IssuePrefix)
	}
}
