// Package secret makes the random secrets behind links and tokens and the
// hashes they are stored as. Identity (API tokens) and guilds (Invitation
// links) both use it.
package secret

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
)

const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// New is 32 random bytes written in base62.
func New() (string, error) {
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

// Hash is how a secret is stored: only its SHA-256, in hex.
func Hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
