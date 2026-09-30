package infra

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/routing/domain"
)

func TestRenderInternalTLS(t *testing.T) {
	raw, err := Render([]domain.Route{
		{ApplicationID: 2, Domains: []string{"zeta.localhost"}, Container: "bakery-app-2-9", Port: 8080},
		{ApplicationID: 1, Domains: []string{"whoami.localhost"}, Container: "bakery-app-1-4", Port: 80},
	}, RenderOptions{InternalTLS: true})
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "admin": {
    "listen": "0.0.0.0:2019"
  },
  "apps": {
    "http": {
      "http_port": 80,
      "https_port": 443,
      "servers": {
        "http": {
          "listen": [
            ":80"
          ],
          "routes": [
            {
              "handle": [
                {
                  "handler": "reverse_proxy",
                  "upstreams": [
                    {
                      "dial": "bakery-app-1-4:80"
                    }
                  ]
                }
              ],
              "match": [
                {
                  "host": [
                    "whoami.localhost"
                  ]
                }
              ],
              "terminal": true
            },
            {
              "handle": [
                {
                  "handler": "reverse_proxy",
                  "upstreams": [
                    {
                      "dial": "bakery-app-2-9:8080"
                    }
                  ]
                }
              ],
              "match": [
                {
                  "host": [
                    "zeta.localhost"
                  ]
                }
              ],
              "terminal": true
            }
          ]
        },
        "https": {
          "automatic_https": {
            "disable_redirects": true
          },
          "listen": [
            ":443"
          ],
          "routes": [
            {
              "handle": [
                {
                  "handler": "reverse_proxy",
                  "upstreams": [
                    {
                      "dial": "bakery-app-1-4:80"
                    }
                  ]
                }
              ],
              "match": [
                {
                  "host": [
                    "whoami.localhost"
                  ]
                }
              ],
              "terminal": true
            },
            {
              "handle": [
                {
                  "handler": "reverse_proxy",
                  "upstreams": [
                    {
                      "dial": "bakery-app-2-9:8080"
                    }
                  ]
                }
              ],
              "match": [
                {
                  "host": [
                    "zeta.localhost"
                  ]
                }
              ],
              "terminal": true
            }
          ]
        }
      }
    },
    "pki": {
      "certificate_authorities": {
        "local": {
          "install_trust": false
        }
      }
    },
    "tls": {
      "automation": {
        "policies": [
          {
            "issuers": [
              {
                "module": "internal"
              }
            ],
            "subjects": [
              "whoami.localhost",
              "zeta.localhost"
            ]
          }
        ]
      }
    }
  }
}`
	if string(raw) != want {
		t.Fatalf("rendered config differs:\n%s", raw)
	}
}

func TestRenderACMEAndEmpty(t *testing.T) {
	raw, err := Render(nil, RenderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg["apps"]["tls"]; ok {
		t.Error("ACME mode must not set a tls automation policy")
	}
	if _, ok := cfg["apps"]["pki"]; ok {
		t.Error("ACME mode must not configure the internal CA")
	}
	if cfg["admin"]["listen"] != CaddyAdminListen {
		t.Error("admin listen address missing")
	}
}

func TestRenderDashboardRoute(t *testing.T) {
	raw, err := Render([]domain.Route{
		{ApplicationID: 1, Domains: []string{"a.example.com"}, Container: "bakery-app-1-4", Port: 80},
	}, RenderOptions{InternalTLS: true, Dashboard: &domain.DashboardRoute{
		Domain: "bakery.example.com", API: "bakery-api:4910", Web: "bakery-web:80",
	}})
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Apps struct {
			HTTP struct {
				Servers map[string]struct {
					Routes []struct {
						Match  []struct{ Host []string } `json:"match"`
						Handle []struct {
							Handler string `json:"handler"`
							Routes  []struct {
								Match  []struct{ Path []string } `json:"match"`
								Handle []struct {
									Handler       string `json:"handler"`
									FlushInterval *int   `json:"flush_interval"`
									Upstreams     []struct{ Dial string }
								} `json:"handle"`
							} `json:"routes"`
						} `json:"handle"`
					} `json:"routes"`
				} `json:"servers"`
			} `json:"http"`
			TLS struct {
				Automation struct {
					Policies []struct{ Subjects []string }
				}
			} `json:"tls"`
		} `json:"apps"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	routes := cfg.Apps.HTTP.Servers["https"].Routes
	if len(routes) != 2 || routes[0].Match[0].Host[0] != "bakery.example.com" {
		t.Fatalf("dashboard route must come first: %s", raw)
	}
	sub := routes[0].Handle[0]
	if sub.Handler != "subroute" || len(sub.Routes) != 2 {
		t.Fatalf("want a subroute with two routes: %s", raw)
	}
	api, web := sub.Routes[0], sub.Routes[1]
	if api.Match[0].Path[0] != "/api/*" || api.Handle[0].Upstreams[0].Dial != "bakery-api:4910" {
		t.Fatalf("api route wrong: %s", raw)
	}
	if fi := api.Handle[0].FlushInterval; fi == nil || *fi != -1 {
		t.Fatalf("api route must flush immediately (SSE): %s", raw)
	}
	if len(web.Match) != 0 || web.Handle[0].Upstreams[0].Dial != "bakery-web:80" {
		t.Fatalf("web route wrong: %s", raw)
	}
	subjects := cfg.Apps.TLS.Automation.Policies[0].Subjects
	if len(subjects) != 2 || subjects[0] != "bakery.example.com" {
		t.Fatalf("dashboard domain missing from TLS subjects: %v", subjects)
	}
}

func TestRenderACME(t *testing.T) {
	routes := []domain.Route{{ApplicationID: 1, Domains: []string{"a.example.com"}, Container: "bakery-app-1-4", Port: 80}}
	raw, err := Render(routes, RenderOptions{
		Dashboard: &domain.DashboardRoute{Domain: "bakery.example.com", API: "bakery-api:4910", Web: "bakery-web:80"},
		ACME:      ACME{CA: "https://pebble:14000/dir", Email: "me@example.com", TrustedRootsFile: "/data/bakery/acme-root.pem"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	apps := cfg["apps"].(map[string]any)
	servers := apps["http"].(map[string]any)["servers"].(map[string]any)
	if _, ok := servers["http"]; ok {
		t.Fatalf("ACME mode must leave :80 to Caddy (challenges, redirects): %s", raw)
	}
	if _, ok := servers["https"].(map[string]any)["automatic_https"]; ok {
		t.Fatalf("ACME mode keeps redirects on: %s", raw)
	}
	if _, ok := apps["pki"]; ok {
		t.Fatalf("no internal CA in ACME mode: %s", raw)
	}
	policy := apps["tls"].(map[string]any)["automation"].(map[string]any)["policies"].([]any)[0].(map[string]any)
	got, _ := json.Marshal(policy)
	want := `{"issuers":[{"ca":"https://pebble:14000/dir","email":"me@example.com","module":"acme","trusted_roots_pem_files":["/data/bakery/acme-root.pem"]}],"subjects":["bakery.example.com","a.example.com"]}`
	if string(got) != want {
		t.Fatalf("policy\n got %s\nwant %s", got, want)
	}

	// Nothing configured: Caddy's default issuers, no policy of ours.
	raw, _ = Render(routes, RenderOptions{})
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg["apps"].(map[string]any)["tls"]; ok {
		t.Fatalf("no ACME settings should render no TLS policy: %s", raw)
	}
}

func TestRenderSeveralDomains(t *testing.T) {
	raw, err := Render([]domain.Route{
		{ApplicationID: 1, Domains: []string{"b.example.com", "a.example.com"}, Container: "bakery-app-1-4", Port: 80},
		{ApplicationID: 2, Domains: nil, Container: "bakery-app-2-1", Port: 80},
	}, RenderOptions{InternalTLS: true})
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Apps struct {
			HTTP struct {
				Servers map[string]struct {
					Routes []struct {
						Match []struct{ Host []string } `json:"match"`
					} `json:"routes"`
				} `json:"servers"`
			} `json:"http"`
			TLS struct {
				Automation struct {
					Policies []struct{ Subjects []string }
				}
			} `json:"tls"`
		} `json:"apps"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	routes := cfg.Apps.HTTP.Servers["https"].Routes
	if len(routes) != 1 || strings.Join(routes[0].Match[0].Host, ",") != "b.example.com,a.example.com" {
		t.Fatalf("one route matching both domains, none for a route without domains: %s", raw)
	}
	if got := strings.Join(cfg.Apps.TLS.Automation.Policies[0].Subjects, ","); got != "b.example.com,a.example.com" {
		t.Fatalf("subjects %s", got)
	}
}

func TestRenderWwwRedirect(t *testing.T) {
	raw, err := Render([]domain.Route{
		{ApplicationID: 1, Domains: []string{"example.com"}, Container: "bakery-app-1-4", Port: 80,
			Settings: domain.RouteSettings{WwwRedirect: domain.ToApex}},
		// Another Application owns www.other.com explicitly: no counterpart.
		{ApplicationID: 2, Domains: []string{"other.com"}, Container: "bakery-app-2-1", Port: 80,
			Settings: domain.RouteSettings{WwwRedirect: domain.ToApex}},
		{ApplicationID: 3, Domains: []string{"www.other.com"}, Container: "bakery-app-3-1", Port: 80},
	}, RenderOptions{InternalTLS: true, HTTPSPort: 4943})
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Apps struct {
			HTTP struct {
				Servers map[string]struct {
					Routes []struct {
						Match  []struct{ Host []string } `json:"match"`
						Handle []struct {
							Handler    string              `json:"handler"`
							StatusCode int                 `json:"status_code"`
							Headers    map[string][]string `json:"headers"`
						} `json:"handle"`
					} `json:"routes"`
				} `json:"servers"`
			} `json:"http"`
			TLS struct {
				Automation struct {
					Policies []struct{ Subjects []string }
				}
			} `json:"tls"`
		} `json:"apps"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	routes := cfg.Apps.HTTP.Servers["https"].Routes
	if len(routes) != 4 {
		t.Fatalf("want three routes and one redirect: %s", raw)
	}
	r := routes[3]
	if r.Match[0].Host[0] != "www.example.com" || r.Handle[0].Handler != "static_response" || r.Handle[0].StatusCode != 308 ||
		r.Handle[0].Headers["Location"][0] != "https://example.com:4943{http.request.uri}" {
		t.Fatalf("redirect route wrong: %+v", r)
	}
	if got := strings.Join(cfg.Apps.TLS.Automation.Policies[0].Subjects, ","); got != "example.com,other.com,www.other.com,www.example.com" {
		t.Fatalf("subjects %s", got)
	}
}
