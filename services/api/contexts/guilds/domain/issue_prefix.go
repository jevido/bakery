package domain

import (
	"errors"
	"strings"
)

var ErrInvalidIssuePrefix = errors.New("issue prefix is 1 to 5 letters A to Z")

// MaxIssuePrefixLength is in letters.
const MaxIssuePrefixLength = 5

// fallbackIssuePrefix is the prefix of a Guild whose name has no letter A
// to Z.
const fallbackIssuePrefix = "GLD"

// DeriveIssuePrefix is the Issue prefix a new Guild called name starts
// with: the first three of its letters A to Z after upper-casing, GLD when
// it has none. "Default" becomes DEF.
func DeriveIssuePrefix(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
			if b.Len() == 3 {
				break
			}
		}
	}
	if b.Len() == 0 {
		return fallbackIssuePrefix
	}
	return b.String()
}

// ParseIssuePrefix reads an Issue prefix a person typed: 1 to 5 letters A
// to Z, upper-cased; ErrInvalidIssuePrefix otherwise.
func ParseIssuePrefix(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" || len(s) > MaxIssuePrefixLength {
		return "", ErrInvalidIssuePrefix
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return "", ErrInvalidIssuePrefix
		}
	}
	return s, nil
}

// FreeIssuePrefix is base, or base with the first suffix A, B, … Z, AA,
// AB, … that taken does not report, cut to MaxIssuePrefixLength letters.
// Issue prefixes are unique across the installation, so a second "Default"
// gets DEFA.
func FreeIssuePrefix(base string, taken func(string) bool) string {
	for n := 0; ; n++ {
		p := base + issuePrefixSuffix(n)
		if len(p) > MaxIssuePrefixLength {
			p = p[len(p)-MaxIssuePrefixLength:]
		}
		if !taken(p) {
			return p
		}
	}
}

// issuePrefixSuffix is the n-th suffix: "" for 0, then A to Z, AA, AB, …
// (bijective base 26).
func issuePrefixSuffix(n int) string {
	var out []byte
	for n > 0 {
		n--
		out = append([]byte{byte('A' + n%26)}, out...)
		n /= 26
	}
	return string(out)
}

// ChangeIssuePrefix sets the Guild's Issue prefix from what a person typed.
// Whether another Guild has it already is the store's to refuse.
func (g *Guild) ChangeIssuePrefix(s string) error {
	p, err := ParseIssuePrefix(s)
	if err != nil {
		return err
	}
	g.IssuePrefix = p
	return nil
}
