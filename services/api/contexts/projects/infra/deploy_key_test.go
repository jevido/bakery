package infra

import (
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestNewDeployKey(t *testing.T) {
	k, err := NewDeployKey("bakery-web")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(k.Public, "ssh-ed25519 ") || !strings.HasSuffix(k.Public, " bakery-web") {
		t.Fatalf("public: %q", k.Public)
	}
	signer, err := ssh.ParsePrivateKey([]byte(k.Private))
	if err != nil {
		t.Fatal(err)
	}
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(k.Public))
	if err != nil || string(pub.Marshal()) != string(signer.PublicKey().Marshal()) {
		t.Fatalf("halves do not match: %v", err)
	}
}
