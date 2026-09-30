package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
)

const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// newSecret is 32 random bytes written in base62, for Invitation links and
// API tokens.
func newSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	n := new(big.Int).SetBytes(b)
	base, mod := big.NewInt(62), new(big.Int)
	var out []byte
	for n.Sign() > 0 {
		n.DivMod(n, base, mod)
		out = append(out, base62[mod.Int64()])
	}
	return string(out), nil
}

// hashSecret is what is stored of a secret: its SHA-256, in hex. The
// secrets are random, so a fast hash is enough.
func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
