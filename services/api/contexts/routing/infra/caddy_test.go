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
