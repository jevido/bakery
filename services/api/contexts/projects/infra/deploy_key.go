package infra

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/jevido/bakery/services/api/contexts/projects/domain"
)

// NewDeployKey generates an ed25519 Deploy key: the private half as OpenSSH
// PEM (what ssh -i reads), the public half as an authorized_keys line with
// the comment.
func NewDeployKey(comment string) (domain.DeployKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return domain.DeployKey{}, err
	}
	block, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return domain.DeployKey{}, err
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return domain.DeployKey{}, err
	}
	public := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))) + " " + comment
	return domain.DeployKey{Public: public, Private: string(pem.EncodeToMemory(block))}, nil
}
