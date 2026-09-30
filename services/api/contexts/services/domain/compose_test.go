package domain

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

const umami = `services:
  umami:
    image: ghcr.io/umami-software/umami:postgresql-latest
    environment:
      - SERVICE_URL_UMAMI_3000
      - DATABASE_URL=postgres://${SERVICE_USER_POSTGRES}:${SERVICE_PASSWORD_POSTGRES}@postgresql:5432/${POSTGRES_DB:-umami}
      - APP_SECRET=${SERVICE_PASSWORD_64_UMAMI}
    depends_on:
      postgresql:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-f", "http://127.0.0.1:3000/api/heartbeat"]
  postgresql:
    image: docker.io/library/postgres:16-alpine
    volumes:
      - postgresql-data:/var/lib/postgresql/data
    environment:
      POSTGRES_USER: $SERVICE_USER_POSTGRES
      POSTGRES_PASSWORD: ${SERVICE_PASSWORD_POSTGRES}
      POSTGRES_DB: ${POSTGRES_DB:-umami}
    restart: unless-stopped
volumes:
  postgresql-data:
`

func TestParseComposeUmami(t *testing.T) {
	c, err := ParseCompose(umami)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Components) != 2 || c.Components[0].Name != "umami" || c.Components[1].Name != "postgresql" {
		t.Fatalf("components %+v", c.Components)
	}
	u, _ := c.Component("umami")
	if u.Environment["SERVICE_URL_UMAMI_3000"] != "${SERVICE_URL_UMAMI_3000}" {
		t.Errorf("bare key: %q", u.Environment["SERVICE_URL_UMAMI_3000"])
	}
	if !slices.Equal(u.DependsOn, []string{"postgresql"}) {
		t.Errorf("depends_on %v", u.DependsOn)
	}
	pg, _ := c.Component("postgresql")
	if len(pg.Volumes) != 1 || pg.Volumes[0] != (VolumeMount{Volume: "postgresql-data", Path: "/var/lib/postgresql/data"}) {
		t.Errorf("volumes %+v", pg.Volumes)
	}
	if !slices.Equal(c.VolumeNames(), []string{"postgresql-data"}) {
		t.Errorf("volume names %v", c.VolumeNames())
	}
	order, err := StartOrder(c)
	if err != nil || !slices.Equal(order, []string{"postgresql", "umami"}) {
		t.Errorf("start order %v %v", order, err)
	}
	public, err := PublicComponents(c)
	if err != nil || len(public) != 1 || public["umami"] != 3000 {
		t.Errorf("public %v %v", public, err)
	}

	var names []string
	for _, v := range Variables(c) {
		names = append(names, v.Name)
		if v.Name == "POSTGRES_DB" && (!v.HasDefault || v.Default != "umami") {
			t.Errorf("POSTGRES_DB default %+v", v)
		}
	}
	want := []string{"SERVICE_USER_POSTGRES", "SERVICE_PASSWORD_POSTGRES", "POSTGRES_DB", "SERVICE_URL_UMAMI_3000", "SERVICE_PASSWORD_64_UMAMI"}
	slices.Sort(names)
	slices.Sort(want)
	if !slices.Equal(names, want) {
		t.Errorf("variables %v, want %v", names, want)
	}

	r, err := Interpolate(c, map[string]string{
		"SERVICE_USER_POSTGRES": "u", "SERVICE_PASSWORD_POSTGRES": "p",
		"SERVICE_PASSWORD_64_UMAMI": "s", "SERVICE_URL_UMAMI_3000": "https://umami.localhost",
	})
	if err != nil {
		t.Fatal(err)
	}
	ru, _ := r.Component("umami")
	if ru.Environment["DATABASE_URL"] != "postgres://u:p@postgresql:5432/umami" || ru.Environment["SERVICE_URL_UMAMI_3000"] != "https://umami.localhost" {
		t.Errorf("resolved %v", ru.Environment)
	}
	if u.Environment["DATABASE_URL"] == ru.Environment["DATABASE_URL"] {
		t.Error("Interpolate changed its input")
	}
}

func TestParseComposeRefusals(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"build", "services:\n  app:\n    build: .\n", "line 3: services.app.build: building images is not supported"},
		{"ports", "services:\n  app:\n    image: x\n    ports: [\"80:80\"]\n", "line 4: services.app.ports: host ports"},
		{"bind mount", "services:\n  app:\n    image: x\n    volumes:\n      - ./data:/data\n", "line 5: services.app.volumes: \"./data\" is a bind mount"},
		{"absolute bind", "services:\n  app:\n    image: x\n    volumes: [\"/srv:/data\"]\n", "is a bind mount"},
		{"long bind", "services:\n  app:\n    image: x\n    volumes:\n      - type: bind\n        source: ./x\n        target: /x\n", "only type: volume"},
		{"anonymous", "services:\n  app:\n    image: x\n    volumes: [\"/data\"]\n", "anonymous volumes"},
		{"relative path", "services:\n  app:\n    image: x\n    volumes: [\"d:data\"]\n", "must be absolute"},
		{"privileged", "services:\n  app:\n    image: x\n    privileged: true\n", "services.app.privileged: is not allowed"},
		{"network mode", "services:\n  app:\n    image: x\n    network_mode: host\n", "only the Service's own network"},
		{"other network", "services:\n  app:\n    image: x\n    networks: [front]\n", "only the Service's own network"},
		{"unknown key", "services:\n  app:\n    image: x\n    colour: blue\n", "services.app.colour: unknown key"},
		{"no image", "services:\n  app:\n    command: sleep 1\n", "services.app.image: is required"},
		{"bad name", "services:\n  App:\n    image: x\n", "a Component name is lowercase"},
		{"no services", "volumes:\n  x:\n", "services: is required"},
		{"empty services", "services: {}\n", "at least one Component"},
		{"top networks", "services:\n  app:\n    image: x\nnetworks:\n  front:\n", "line 4: networks:"},
		{"unknown dependency", "services:\n  app:\n    image: x\n    depends_on: [db]\n", "\"db\" is not a Component"},
		{"cycle", "services:\n  a:\n    image: x\n    depends_on: [b]\n  b:\n    image: x\n    depends_on: [a]\n", "goes round in a circle between a, b"},
		{"yaml", "services:\n  app: [\n", "not valid YAML"},
		{"not a mapping", "- a\n", "must be a mapping"},
		{"bad env name", "services:\n  app:\n    image: x\n    environment: [\"1X=y\"]\n", "not a valid variable name"},
		{"bad expose", "services:\n  app:\n    image: x\n    expose: [http]\n", "is not a port"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCompose(tt.src)
			var errs ComposeErrors
			if !errors.As(err, &errs) {
				t.Fatalf("err %v is not ComposeErrors", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("got %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestParseComposeTooMany(t *testing.T) {
	var b strings.Builder
	b.WriteString("services:\n")
	for i := range MaxComponents + 1 {
		b.WriteString("  c" + string(rune('a'+i)) + ":\n    image: x\n")
	}
	if _, err := ParseCompose(b.String()); err == nil || !strings.Contains(err.Error(), "at most 20") {
		t.Errorf("err %v", err)
	}
}

func TestParseComposeShapes(t *testing.T) {
	c, err := ParseCompose(`version: "3.8"
name: kuma
x-common: &common
  restart: always
services:
  web:
    image: docker.io/traefik/whoami:v1.10
    command: --port 8080 --name "my web"
    entrypoint: ["/whoami"]
    working_dir: /srv
    user: "1000:1000"
    expose: [8080, "9090/tcp"]
    networks: [default]
    labels:
      - com.example=yes
    environment:
      A: 1
      B:
      C: "x=y"
    volumes:
      - type: volume
        source: data
        target: /data
        read_only: true
      - cache:/cache:ro,z
`)
	if err != nil {
		t.Fatal(err)
	}
	w := c.Components[0]
	if !slices.Equal(w.Command, []string{"--port", "8080", "--name", "my web"}) || !slices.Equal(w.Entrypoint, []string{"/whoami"}) {
		t.Errorf("command %q entrypoint %q", w.Command, w.Entrypoint)
	}
	if w.WorkingDir != "/srv" || w.User != "1000:1000" || !slices.Equal(w.Expose, []int{8080, 9090}) {
		t.Errorf("fields %+v", w)
	}
	if w.Labels["com.example"] != "yes" {
		t.Errorf("labels %v", w.Labels)
	}
	if w.Environment["A"] != "1" || w.Environment["B"] != "${B}" || w.Environment["C"] != "x=y" {
		t.Errorf("env %v", w.Environment)
	}
	if !slices.Equal(w.Volumes, []VolumeMount{{"data", "/data", true}, {"cache", "/cache", true}}) {
		t.Errorf("volumes %+v", w.Volumes)
	}
}

func TestInterpolate(t *testing.T) {
	c := Compose{Components: []ComponentSpec{{
		Name: "app", Image: "img:${TAG:-1}",
		Command:     []string{"echo", "$$HOME", "${A}", "$B", "${C-dflt}", "${D:-${A}x}", "cost $5"},
		Environment: map[string]string{"E": "${E:?need it}"},
	}}}
	got, err := Interpolate(c, map[string]string{"A": "a", "B": "b", "E": "e"})
	if err != nil {
		t.Fatal(err)
	}
	g := got.Components[0]
	if g.Image != "img:1" || !slices.Equal(g.Command, []string{"echo", "$HOME", "a", "b", "dflt", "ax", "cost $5"}) || g.Environment["E"] != "e" {
		t.Errorf("got %q %q %v", g.Image, g.Command, g.Environment)
	}
	_, err = Interpolate(c, map[string]string{"A": "a"})
	var unset *UnsetVariableError
	if !errors.As(err, &unset) || !slices.Equal(unset.Names, []string{"B", "E"}) {
		t.Errorf("err %v", err)
	}
}

func TestClassifyVariable(t *testing.T) {
	tests := []struct {
		name    string
		kind    MagicKind
		subject string
		port    int
	}{
		{"SERVICE_PASSWORD_DB", MagicPassword, "DB", 0},
		{"SERVICE_PASSWORD_64_APP", MagicPassword64, "APP", 0},
		{"SERVICE_USER_POSTGRES", MagicUser, "POSTGRES", 0},
		{"SERVICE_BASE64_KEY", MagicBase64, "KEY", 0},
		{"SERVICE_BASE64_64_KEY", MagicBase64_64, "KEY", 0},
		{"SERVICE_FQDN_WEB_80", MagicFQDN, "WEB", 80},
		{"SERVICE_FQDN_UPTIME_KUMA_3001", MagicFQDN, "UPTIME_KUMA", 3001},
		{"SERVICE_URL_N8N", MagicURL, "N8N", 0},
		{"SERVICE_URL_WEB_99999", MagicURL, "WEB_99999", 0},
		{"SERVICE_PASSWORD_", MagicNone, "", 0},
		{"POSTGRES_DB", MagicNone, "", 0},
	}
	for _, tt := range tests {
		kind, subject, port := ClassifyVariable(tt.name)
		if kind != tt.kind || subject != tt.subject || port != tt.port {
			t.Errorf("%s: %q %q %d", tt.name, kind, subject, port)
		}
	}
}

func TestPublicComponentsRefusals(t *testing.T) {
	c := Compose{Components: []ComponentSpec{{Name: "web", Environment: map[string]string{"X": "${SERVICE_FQDN_DB_80}"}}}}
	if _, err := PublicComponents(c); err == nil || !strings.Contains(err.Error(), "names another Component") {
		t.Errorf("err %v", err)
	}
	c = Compose{Components: []ComponentSpec{{Name: "my-web", Environment: map[string]string{
		"SERVICE_FQDN_MY_WEB_80": "${SERVICE_FQDN_MY_WEB_80}", "U": "${SERVICE_URL_MY_WEB_81}",
	}}}}
	if _, err := PublicComponents(c); err == nil || !strings.Contains(err.Error(), "both port 80 and 81") {
		t.Errorf("err %v", err)
	}
}
