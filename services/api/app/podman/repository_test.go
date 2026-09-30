package podman

import "testing"

func TestRepository(t *testing.T) {
	for ref, want := range map[string]string{
		"docker.io/traefik/whoami:v1.10": "docker.io/traefik/whoami",
		"127.0.0.1:4950/me/app:1":        "127.0.0.1:4950/me/app",
		"127.0.0.1:4950/me/app":          "127.0.0.1:4950/me/app",
		"ghcr.io/me/app@sha256:abc":      "ghcr.io/me/app",
		"ghcr.io/me/app:1@sha256:abc":    "ghcr.io/me/app",
		"localhost/bakery/whoami:12":     "localhost/bakery/whoami",
	} {
		if got := Repository(ref); got != want {
			t.Errorf("Repository(%q) = %q, want %q", ref, got, want)
		}
	}
}
