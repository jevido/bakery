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
		// Registry hosts (host[:port], comma-separated) image Applications
		// are pulled from without TLS verification. Empty on a server.
		"insecure_registries": config.Env("BAKERY_INSECURE_REGISTRIES", ""),
		// The nixpacks binary the nixpacks Build pack writes Dockerfiles
		// with: a path, or a name on PATH (the API image has it there).
		"nixpacks": config.Env("BAKERY_NIXPACKS", "nixpacks"),
		// The Dashboard Route: the domain Bakery's own dashboard is served on
		// through the proxy (empty in development, where Vite serves it), and
		// the containers it sends /api/* and everything else to.
		"dashboard": map[string]any{
			"domain":       config.Env("BAKERY_DASHBOARD_DOMAIN", ""),
			"api_upstream": config.Env("BAKERY_DASHBOARD_API_UPSTREAM", "bakery-api:4910"),
			"web_upstream": config.Env("BAKERY_DASHBOARD_WEB_UPSTREAM", "bakery-web:80"),
		},
		// Where certificates come from when internal_tls is off. All empty
		// means Caddy's defaults (Let's Encrypt, then ZeroSSL). ca_root is a
		// PEM file the CA's own HTTPS certificate is signed by (Pebble in
		// tests), copied into the proxy.
		"acme": map[string]any{
			"ca":      config.Env("BAKERY_ACME_CA", ""),
			"email":   config.Env("BAKERY_ACME_EMAIL", ""),
			"ca_root": config.Env("BAKERY_ACME_CA_ROOT", ""),
		},
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
			// Host address the admin API is published on; empty (the default)
			// means not published. It has no authentication: 127.0.0.1 at
			// most. Development sets 127.0.0.1:4949 in .env. The default is
			// "not published" because an empty environment variable counts as
			// unset and could never switch publishing off.
			"admin_publish": config.Env("BAKERY_PROXY_ADMIN_PUBLISH", ""),
			"internal_tls":  config.Env("BAKERY_PROXY_INTERNAL_TLS", true),
		},
	})
}
