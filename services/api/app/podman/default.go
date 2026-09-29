package podman

import (
	"sync"

	"github.com/jevido/bakery/services/api/app/facades"
)

var (
	defaultOnce   sync.Once
	defaultClient *Client
)

// Default is the client for the configured socket (bakery.podman_socket),
// shared by every context.
func Default() *Client {
	defaultOnce.Do(func() {
		sock := facades.Config().GetString("bakery.podman_socket")
		if sock == "" {
			sock = DefaultSocket()
		}
		defaultClient = New(sock)
	})
	return defaultClient
}
