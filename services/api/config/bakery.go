package config

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// Bakery's own settings. Every one has a default that works for local
// development; see .env.example.
func init() {
	config := facades.Config()
	config.Add("bakery", map[string]any{
		// Applications without a Domain get <slug>.<domain_suffix>.
		"domain_suffix": config.Env("BAKERY_DOMAIN_SUFFIX", "localhost"),
		// The Podman API socket; empty means the rootless socket of the
		// user running Bakery ($XDG_RUNTIME_DIR/podman/podman.sock).
		"podman_socket": config.Env("BAKERY_PODMAN_SOCKET", ""),
		// The network Bakery's containers share; Caddy reaches Applications
		// on it by container name.
		"network": config.Env("BAKERY_NETWORK", "bakery"),
		"proxy": map[string]any{
			"image": config.Env("BAKERY_PROXY_IMAGE", "docker.io/library/caddy:2"),
			// Host address the proxy's HTTP(S) ports are published on; empty
			// means every interface, as a server needs.
			"bind":       config.Env("BAKERY_PROXY_BIND", ""),
			"http_port":  config.Env("BAKERY_PROXY_HTTP_PORT", 4940),
			"https_port": config.Env("BAKERY_PROXY_HTTPS_PORT", 4943),
			// How the API reaches Caddy's admin API. On a server the API runs
			// on the bakery network and uses http://bakery-proxy:2019.
			"admin_url": config.Env("BAKERY_PROXY_ADMIN_URL", "http://127.0.0.1:4949"),
			// Host address the admin API is published on; empty means not
			// published. It has no authentication: 127.0.0.1 at most.
			"admin_publish": config.Env("BAKERY_PROXY_ADMIN_PUBLISH", "127.0.0.1:4949"),
			"internal_tls":  config.Env("BAKERY_PROXY_INTERNAL_TLS", true),
		},
	})
}
