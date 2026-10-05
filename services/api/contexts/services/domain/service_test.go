package domain

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

const twoComponents = `services:
  web:
    image: docker.io/traefik/whoami:v1.10
    environment:
      SERVICE_FQDN_WEB_80:
      DB_URL: postgres://app:${SERVICE_PASSWORD_DB}@db/app
      GREETING: ${GREETING:-hello}
      TOKEN: ${TOKEN}
  db:
    image: docker.io/library/postgres:18-alpine
    environment:
      POSTGRES_PASSWORD: ${SERVICE_PASSWORD_DB}
    volumes:
      - data:/var/lib/postgresql/data
`

func counter() Generate {
	n := 0
	return func(k MagicKind) string {
		n++
		return string(k) + "-" + strings.Repeat("x", n)
	}
}

func TestNewService(t *testing.T) {
	s, err := NewService(1, 2, " Shop ", "shop", twoComponents, "localhost", counter())
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Shop" || s.DesiredState != Running || len(s.Components) != 2 {
		t.Fatalf("%+v", s)
	}
	web, _ := s.Component("web")
	db, _ := s.Component("db")
	if !web.Public || web.Port != 80 || !slices.Equal(web.Domains, []string{"shop.localhost"}) || db.Public || db.Domains != nil {
		t.Errorf("components %+v %+v", web, db)
	}
	var names []string
	for _, v := range s.Variables {
		names = append(names, v.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"GREETING", "SERVICE_PASSWORD_DB", "TOKEN"}) {
		t.Errorf("variables %v", names)
	}
	r, err := s.Resolved()
	if err != nil {
		t.Fatal(err)
	}
	rw, _ := r.Component("web")
	rd, _ := r.Component("db")
	if rw.Environment["SERVICE_FQDN_WEB_80"] != "shop.localhost" || rw.Environment["GREETING"] != "hello" || rw.Environment["TOKEN"] != "" {
		t.Errorf("web env %v", rw.Environment)
	}
	if rd.Environment["POSTGRES_PASSWORD"] != "password-x" || rw.Environment["DB_URL"] != "postgres://app:password-x@db/app" {
		t.Errorf("password not shared: %v %v", rd.Environment, rw.Environment)
	}

	if err := s.SetVariable("GREETING", "hi"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetVariable("SERVICE_PASSWORD_DB", "mine"); err == nil {
		t.Error("set a magic variable")
	}
	if err := s.SetVariable("NOPE", "x"); err == nil {
		t.Error("set an unknown variable")
	}
	r, _ = s.Resolved()
	if rw, _ := r.Component("web"); rw.Environment["GREETING"] != "hi" {
		t.Errorf("GREETING %q", rw.Environment["GREETING"])
	}
}

func TestChangeComposeKeepsValuesAndDomains(t *testing.T) {
	gen := counter()
	s, err := NewService(1, 2, "Shop", "shop", twoComponents, "localhost", gen)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetDomains("web", []string{"Shop.Example.com", "www.shop.example.com"}); err != nil {
		t.Fatal(err)
	}
	s.SetVariable("TOKEN", "t1")
	password := s.Values()["SERVICE_PASSWORD_DB"]

	changed := strings.Replace(twoComponents, "      TOKEN: ${TOKEN}\n", "      KEY: ${SERVICE_BASE64_KEY}\n", 1) + `  admin:
    image: docker.io/library/adminer:5
    environment:
      - SERVICE_URL_ADMIN_8080
`
	if err := s.ChangeCompose(changed, "localhost", gen); err != nil {
		t.Fatal(err)
	}
	v := s.Values()
	if v["SERVICE_PASSWORD_DB"] != password {
		t.Error("the generated password changed")
	}
	if _, ok := v["TOKEN"]; ok {
		t.Error("TOKEN kept though no longer used")
	}
	if !strings.HasPrefix(v["SERVICE_BASE64_KEY"], "base64-") {
		t.Errorf("new magic variable %q", v["SERVICE_BASE64_KEY"])
	}
	web, _ := s.Component("web")
	admin, _ := s.Component("admin")
	if !slices.Equal(web.Domains, []string{"shop.example.com", "www.shop.example.com"}) {
		t.Errorf("web domains %v", web.Domains)
	}
	if !slices.Equal(admin.Domains, []string{"shop-admin.localhost"}) || v["SERVICE_URL_ADMIN"] != "https://shop-admin.localhost" {
		t.Errorf("admin %+v %q", admin, v["SERVICE_URL_ADMIN"])
	}
	if !slices.Equal(s.Domains(), []string{"shop.example.com", "www.shop.example.com", "shop-admin.localhost"}) {
		t.Errorf("domains %v", s.Domains())
	}
}

func TestServiceRefusals(t *testing.T) {
	gen := counter()
	if _, err := NewService(1, 2, "", "x", twoComponents, "localhost", gen); err == nil {
		t.Error("empty name")
	}
	_, err := NewService(1, 2, "x", "x", "services:\n  app:\n    build: .\n", "localhost", gen)
	var cf *ComposeFileError
	if !errors.As(err, &cf) || len(cf.Errors) != 1 {
		t.Errorf("compose error %v", err)
	}
	_, err = NewService(1, 2, "x", "x", "services:\n  app:\n    image: x\n    environment:\n      U: ${SERVICE_URL_OTHER}\n", "localhost", gen)
	var fe *FieldError
	if !errors.As(err, &fe) || fe.Field != "compose" || !strings.Contains(fe.Message, "names no Public Component") {
		t.Errorf("fqdn of nothing: %v", err)
	}

	s, _ := NewService(1, 2, "Shop", "shop", twoComponents, "localhost", gen)
	for _, tt := range []struct {
		component string
		domains   []string
		want      string
	}{
		{"db", []string{"db.example.com"}, "is not public"},
		{"nope", []string{"a.example.com"}, "is not a Component"},
		{"web", nil, "at least one domain"},
		{"web", []string{"localhost"}, "is not a hostname"},
		{"web", []string{"a.example.com", "A.example.com"}, "listed twice"},
		{"web", slices.Repeat([]string{"a.example.com"}, 1), ""},
		{"web", []string{"1.example.com", "2.example.com", "3.example.com", "4.example.com", "5.example.com", "6.example.com", "7.example.com", "8.example.com", "9.example.com", "10.example.com", "11.example.com"}, "more than 10"},
	} {
		err := s.SetDomains(tt.component, tt.domains)
		if tt.want == "" {
			if err != nil {
				t.Errorf("%v: %v", tt.domains, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s %v: %v, want %q", tt.component, tt.domains, err, tt.want)
		}
	}
	if web, _ := s.Component("web"); !slices.Equal(web.Domains, []string{"a.example.com"}) {
		t.Errorf("a refused change was kept: %v", web.Domains)
	}
}

func TestSummarize(t *testing.T) {
	s := Service{DesiredState: Running, Components: []Component{{Name: "a"}, {Name: "b"}}}
	tests := []struct {
		statuses map[string]Status
		busy     bool
		lastErr  string
		desired  DesiredState
		want     ServiceStatus
	}{
		{map[string]Status{"a": StatusRunning, "b": StatusRunning}, false, "", Running, ServiceRunning},
		{map[string]Status{"a": StatusRunning, "b": StatusStarting}, false, "", Running, ServiceDeploying},
		{map[string]Status{"a": StatusRunning, "b": StatusExited}, false, "", Running, ServiceDegraded},
		{map[string]Status{"a": StatusStopped, "b": StatusMissing}, false, "", Running, ServiceStopped},
		{map[string]Status{"a": StatusExited, "b": StatusStopped}, false, "", Stopped, ServiceStopped},
		{map[string]Status{}, true, "", Running, ServiceDeploying},
		{map[string]Status{}, false, "pull failed", Running, ServiceFailed},
	}
	for _, tt := range tests {
		s.LastError, s.DesiredState = tt.lastErr, tt.desired
		if got := s.Summarize(tt.statuses, tt.busy); got != tt.want {
			t.Errorf("%v busy=%v err=%q: %s, want %s", tt.statuses, tt.busy, tt.lastErr, got, tt.want)
		}
	}
}

func TestNames(t *testing.T) {
	if ContainerName(3, "web") != "bakery-svc-3-web" || NetworkName(3) != "bakery-svc-3" || VolumeName(3, "data") != "bakery-svc-3-data" {
		t.Error("names")
	}
	if Slugify("  Uptime Kuma! ") != "uptime-kuma" || Slugify("***") != "service" {
		t.Error("slugify")
	}
	if DefaultDomain("s", "my_web", false, "localhost") != "s-my-web.localhost" {
		t.Error("default domain")
	}
}

func TestGeneratedName(t *testing.T) {
	if got := GeneratedName("ab12cd34"); got != "docker-compose-ab12cd34" {
		t.Errorf("got %q", got)
	}
}

func TestDescribe(t *testing.T) {
	s := Service{Name: "x"}
	if err := s.Describe("  my stack  "); err != nil || s.Description != "my stack" {
		t.Fatalf("describe: %v, %q", err, s.Description)
	}
	// 255 characters is the limit, counted in runes, not bytes.
	if err := s.Describe(strings.Repeat("é", 255)); err != nil {
		t.Fatalf("255 runes: %v", err)
	}
	var fe *FieldError
	if err := s.Describe(strings.Repeat("é", 256)); !errors.As(err, &fe) || fe.Field != "description" {
		t.Fatalf("256 runes: %v", err)
	}
	if s.Description != strings.Repeat("é", 255) {
		t.Fatal("a refused description changed the Service")
	}
	if err := s.Describe(""); err != nil || s.Description != "" {
		t.Fatalf("clear: %v", err)
	}
}

func TestVariableComponents(t *testing.T) {
	c, err := ParseCompose(twoComponents)
	if err != nil {
		t.Fatal(err)
	}
	got := VariableComponents(c)
	if !slices.Equal(got["SERVICE_PASSWORD_DB"], []string{"web", "db"}) {
		t.Fatalf("SERVICE_PASSWORD_DB: %v", got["SERVICE_PASSWORD_DB"])
	}
	if !slices.Equal(got["GREETING"], []string{"web"}) || !slices.Equal(got["TOKEN"], []string{"web"}) {
		t.Fatalf("web only: %v", got)
	}
}
