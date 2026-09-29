package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/jevido/bakery/services/api/contexts/routing/domain"
)

// CaddyAdminListen is where Caddy's admin API listens inside the Proxy
// container. Every config Bakery loads sets it again: a config without it
// would move the admin API back to localhost and lock Bakery out.
const CaddyAdminListen = "0.0.0.0:2019"

// RenderOptions shape the Caddy config apart from the Routes.
type RenderOptions struct {
	// InternalTLS issues certificates from Caddy's own CA instead of ACME
	// (for *.localhost in development) and turns off HTTP→HTTPS redirects,
	// which would point at 443 while the host publishes another port.
	InternalTLS bool
	// Dashboard, when set, is rendered before every Application route.
	Dashboard *domain.DashboardRoute
	// ACME configures the certificate issuer when InternalTLS is off.
	ACME ACME
}

// ACME says where certificates come from on a Server. All fields empty means
// Caddy's defaults (Let's Encrypt, then ZeroSSL).
type ACME struct {
	CA    string // directory URL
	Email string
	// TrustedRootsFile is a PEM file inside the Proxy that the CA's own
	// HTTPS certificate is signed by (a private CA such as Pebble).
	TrustedRootsFile string
}

type obj = map[string]any

// Render returns the full Caddy JSON config for the Routes: one server on
// :443 (TLS) with a host-matched reverse_proxy route per Route, sorted by
// Domain so the output is stable.
//
// With Internal TLS a second server on :80 serves the same routes over plain
// HTTP. With ACME there is no :80 server of ours: Caddy then runs its own
// there, answering HTTP-01 challenges and redirecting everything to HTTPS.
func Render(routes []domain.Route, opts RenderOptions) ([]byte, error) {
	sorted := append([]domain.Route(nil), routes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Domain < sorted[j].Domain })

	caddyRoutes := make([]obj, 0, len(sorted)+1)
	domains := make([]string, 0, len(sorted)+1)
	if d := opts.Dashboard; d != nil {
		domains = append(domains, d.Domain)
		caddyRoutes = append(caddyRoutes, dashboardRoute(*d))
	}
	for _, r := range sorted {
		domains = append(domains, r.Domain)
		caddyRoutes = append(caddyRoutes, obj{
			"match": []obj{{"host": []string{r.Domain}}},
			"handle": []obj{{
				"handler":   "reverse_proxy",
				"upstreams": []obj{{"dial": r.Container + ":" + strconv.Itoa(r.Port)}},
			}},
			"terminal": true,
		})
	}

	https := obj{"listen": []string{":443"}, "routes": caddyRoutes}
	if opts.InternalTLS {
		https["automatic_https"] = obj{"disable_redirects": true}
	}
	servers := obj{"https": https}
	if opts.InternalTLS {
		servers["http"] = obj{"listen": []string{":80"}, "routes": caddyRoutes}
	}
	apps := obj{
		"http": obj{
			"http_port":  80,
			"https_port": 443,
			"servers":    servers,
		},
	}
	if issuer := acmeIssuer(opts.ACME); !opts.InternalTLS && issuer != nil && len(domains) > 0 {
		apps["tls"] = obj{"automation": obj{"policies": []obj{{
			"subjects": domains,
			"issuers":  []obj{issuer},
		}}}}
	}
	if opts.InternalTLS {
		apps["pki"] = obj{"certificate_authorities": obj{"local": obj{"install_trust": false}}}
		if len(domains) > 0 {
			apps["tls"] = obj{"automation": obj{"policies": []obj{{
				"subjects": domains,
				"issuers":  []obj{{"module": "internal"}},
			}}}}
		}
	}
	return json.MarshalIndent(obj{
		"admin": obj{"listen": CaddyAdminListen},
		"apps":  apps,
	}, "", "  ")
}

// acmeIssuer is the ACME issuer for a, or nil when nothing is configured and
// Caddy's default issuers apply.
func acmeIssuer(a ACME) obj {
	if a == (ACME{}) {
		return nil
	}
	issuer := obj{"module": "acme"}
	if a.CA != "" {
		issuer["ca"] = a.CA
	}
	if a.Email != "" {
		issuer["email"] = a.Email
	}
	if a.TrustedRootsFile != "" {
		issuer["trusted_roots_pem_files"] = []string{a.TrustedRootsFile}
	}
	return issuer
}

// dashboardRoute sends /api/* to the API and the rest to the dashboard, so
// both share one origin and the SameSite=Strict session cookie works.
func dashboardRoute(d domain.DashboardRoute) obj {
	return obj{
		"match": []obj{{"host": []string{d.Domain}}},
		"handle": []obj{{
			"handler": "subroute",
			"routes": []obj{
				{
					"match": []obj{{"path": []string{"/api/*"}}},
					"handle": []obj{{
						"handler":   "reverse_proxy",
						"upstreams": []obj{{"dial": d.API}},
						// Log streams are server-sent events: pass every
						// write through instead of buffering.
						"flush_interval": -1,
					}},
				},
				{
					"handle": []obj{{
						"handler":   "reverse_proxy",
						"upstreams": []obj{{"dial": d.Web}},
					}},
				},
			},
		}},
		"terminal": true,
	}
}

// Caddy talks to the Proxy's admin API.
type Caddy struct {
	AdminURL string // e.g. http://127.0.0.1:4949
	http     http.Client
}

// Load replaces Caddy's whole config.
func (c *Caddy) Load(ctx context.Context, config []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.AdminURL+"/load", bytes.NewReader(config))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("caddy: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 16<<10))
		return fmt.Errorf("caddy: load refused (%d): %s", res.StatusCode, bytes.TrimSpace(body))
	}
	return nil
}

// WaitReady polls the admin API until it answers or ctx ends.
func (c *Caddy) WaitReady(ctx context.Context) error {
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.AdminURL+"/config/", nil)
		if res, err := c.http.Do(req); err == nil {
			res.Body.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("caddy admin API at %s did not answer: %w", c.AdminURL, ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}
