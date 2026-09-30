package infra

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

// NewServerKey generates an ed25519 Server key: the private half as OpenSSH
// PEM, the public half as an authorized_keys line with the comment.
func NewServerKey(comment string) (domain.ServerKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return domain.ServerKey{}, err
	}
	block, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return domain.ServerKey{}, err
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return domain.ServerKey{}, err
	}
	public := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))) + " " + comment
	return domain.ServerKey{Public: public, Private: string(pem.EncodeToMemory(block))}, nil
}
