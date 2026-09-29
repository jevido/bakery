package infra

import (
	"encoding/json"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/routing/domain"
)

func TestRenderInternalTLS(t *testing.T) {
	raw, err := Render([]domain.Route{
		{ApplicationID: 2, Domain: "zeta.localhost", Container: "bakery-app-2-9", Port: 8080},
		{ApplicationID: 1, Domain: "whoami.localhost", Container: "bakery-app-1-4", Port: 80},
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
		{ApplicationID: 1, Domain: "a.example.com", Container: "bakery-app-1-4", Port: 80},
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
