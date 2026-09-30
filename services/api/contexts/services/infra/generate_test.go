package infra

import (
	"encoding/base64"
	"regexp"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/services/domain"
)

func TestGenerate(t *testing.T) {
	text := regexp.MustCompile(`^[A-Za-z0-9]+$`)
	for kind, n := range map[domain.MagicKind]int{domain.MagicPassword: 32, domain.MagicPassword64: 64, domain.MagicUser: 16} {
		v := Generate(kind)
		if len(v) != n || !text.MatchString(v) {
			t.Errorf("%s: %q", kind, v)
		}
		if Generate(kind) == v {
			t.Errorf("%s: the same value twice", kind)
		}
	}
	for kind, n := range map[domain.MagicKind]int{domain.MagicBase64: 32, domain.MagicBase64_64: 64} {
		raw, err := base64.StdEncoding.DecodeString(Generate(kind))
		if err != nil || len(raw) != n {
			t.Errorf("%s: %d bytes, %v", kind, len(raw), err)
		}
	}
	if Generate(domain.MagicFQDN) != "" {
		t.Error("fqdn is not generated")
	}
}
