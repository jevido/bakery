package app

import (
	"context"
	"errors"
	"strings"

	"github.com/jevido/bakery/services/api/app/secret"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

// ErrInvalidRunKey answers an unknown Run key and one whose Run is no
// longer running.
var ErrInvalidRunKey = errors.New("invalid run key")

// RunKeyHolder is who a Run key speaks for, as agents tells it: the Agent,
// in its Run's Guild, during that Run, and the Member who hired it.
type RunKeyHolder struct {
	AgentID       uint64
	GuildID       uint64
	RunID         uint64
	HirerMemberID uint64
}

// RunKeyHolders finds the holder of the Run key with a hash; agents
// answers it, so identity never reads a Run.
type RunKeyHolders func(ctx context.Context, keyHash string) (RunKeyHolder, bool, error)

// AuthenticateRun finds the holder of a Run key; ErrInvalidRunKey when
// there is none or nobody registered RunKeyHolders.
func (s *Service) AuthenticateRun(ctx context.Context, key string) (RunKeyHolder, error) {
	if !strings.HasPrefix(key, domain.RunKeyPrefix) || s.RunKeyHolders == nil {
		return RunKeyHolder{}, ErrInvalidRunKey
	}
	h, found, err := s.RunKeyHolders(ctx, secret.Hash(key))
	if err != nil {
		return RunKeyHolder{}, err
	}
	if !found {
		return RunKeyHolder{}, ErrInvalidRunKey
	}
	return h, nil
}
