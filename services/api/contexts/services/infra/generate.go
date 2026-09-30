package infra

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/jevido/bakery/services/api/contexts/services/domain"
)

const alphanumeric = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// Generate makes the value of a generated Magic variable from crypto/rand.
func Generate(kind domain.MagicKind) string {
	switch kind {
	case domain.MagicPassword:
		return randomText(32)
	case domain.MagicPassword64:
		return randomText(64)
	case domain.MagicUser:
		return randomText(16)
	case domain.MagicBase64:
		return base64.StdEncoding.EncodeToString(randomBytes(32))
	case domain.MagicBase64_64:
		return base64.StdEncoding.EncodeToString(randomBytes(64))
	}
	return ""
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b) // never fails (crypto/rand panics instead)
	return b
}

// randomText is n characters from alphanumeric, without modulo bias.
func randomText(n int) string {
	out := make([]byte, 0, n)
	for len(out) < n {
		for _, b := range randomBytes(n) {
			if b < 248 && len(out) < n { // 248 = 4 * 62
				out = append(out, alphanumeric[int(b)%len(alphanumeric)])
			}
		}
	}
	return string(out)
}
