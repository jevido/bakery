package infra

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

// NewPrivateKey generates an ed25519 Private key: the private half as OpenSSH
// PEM, the public half as an authorized_keys line with the comment.
func NewPrivateKey(comment string) (domain.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return domain.PrivateKey{}, err
	}
	block, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return domain.PrivateKey{}, err
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return domain.PrivateKey{}, err
	}
	public := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))) + " " + comment
	return domain.PrivateKey{Public: public, Private: string(pem.EncodeToMemory(block))}, nil
}
