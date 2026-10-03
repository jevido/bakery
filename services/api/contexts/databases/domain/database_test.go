package domain

import (
	"errors"
	"strings"
	"testing"
)

func pw() string { return "p@ss/word" }

func TestNewDatabase(t *testing.T) {
	d, err := NewDatabase(2, 1, Input{Name: " Main DB ", Type: PostgreSQL}, "main-db", pw)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Main DB" || d.Version != "18-alpine" || d.DesiredState != Running || d.EnvironmentID != 2 || d.ProjectID != 1 {
		t.Fatalf("got %+v", d)
	}
	if d.Credentials != (Credentials{Username: "bakery", Password: "p@ss/word", DatabaseName: "main_db"}) {
		t.Fatalf("credentials %+v", d.Credentials)
	}
	m, _ := NewDatabase(2, 1, Input{Name: "m", Type: MySQL}, "m", pw)
	if m.Credentials.RootPassword == "" {
		t.Fatal("mysql has no root password")
	}
}

func TestNewDatabaseRules(t *testing.T) {
	for _, tc := range []struct {
		in    Input
		field string
	}{
		{Input{Name: "x", Type: "oracle"}, "type"},
		{Input{Name: " ", Type: Redis}, "name"},
		{Input{Name: strings.Repeat("x", 101), Type: Redis}, "name"},
		{Input{Name: "x", Type: Redis, Version: "8 alpine"}, "version"},
		{Input{Name: "x", Type: Redis, Version: "-8"}, "version"},
		{Input{Name: "x", Type: Redis, PublicPort: 80}, "public_port"},
		{Input{Name: "x", Type: Redis, PublicPort: 70000}, "public_port"},
		{Input{Name: "x", Type: Redis, ResourceLimits: ResourceLimits{MemoryMB: 8}}, "resource_limits.memory_mb"},
		{Input{Name: "x", Type: Redis, ResourceLimits: ResourceLimits{CPUs: 0.125}}, "resource_limits.cpus"},
	} {
		_, err := NewDatabase(1, 1, tc.in, "x", pw)
		var fe *FieldError
		if !errors.As(err, &fe) || fe.Field != tc.field {
			t.Errorf("%+v: want %s error, got %v", tc.in, tc.field, err)
		}
	}
	if _, err := NewDatabase(1, 1, Input{Name: "x", Type: Valkey, Version: "8.1.2-alpine", PublicPort: 6380, ResourceLimits: ResourceLimits{MemoryMB: 256, CPUs: 0.5}}, "x", pw); err != nil {
		t.Fatal(err)
	}
}

func TestUpdate(t *testing.T) {
	d, _ := NewDatabase(1, 1, Input{Name: "x", Type: Redis}, "x", pw)
	creds := d.Credentials
	if recreate, err := d.Update(Input{Name: "renamed", Type: MongoDB}); err != nil || recreate {
		t.Fatalf("rename: recreate %v, %v", recreate, err)
	}
	if d.Name != "renamed" || d.Type != Redis || d.Version != "8-alpine" || d.Credentials != creds {
		t.Fatalf("after rename %+v", d)
	}
	for _, in := range []Input{
		{Name: "renamed", Version: "7"},
		{Name: "renamed", Version: "7", PublicPort: 6390},
		{Name: "renamed", Version: "7", PublicPort: 6390, ResourceLimits: ResourceLimits{MemoryMB: 64}},
	} {
		if recreate, err := d.Update(in); err != nil || !recreate {
			t.Fatalf("%+v: recreate %v, %v", in, recreate, err)
		}
	}
	if _, err := d.Update(Input{Name: ""}); err == nil {
		t.Fatal("empty name accepted")
	}
}

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{"Main DB": "main-db", "!!!": "redis", strings.Repeat("ab-", 20): "ab-ab-ab-ab-ab-ab-ab-ab-ab-ab-ab-ab-ab-a"} {
		if got := Slugify(in, Redis); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
	if ContainerName("main") != "bakery-db-main" || VolumeName(7) != "bakery-db-7-data" {
		t.Fatal("names")
	}
}

func TestDatabaseTypes(t *testing.T) {
	for _, e := range DatabaseTypes {
		d, err := NewDatabase(1, 1, Input{Name: "app", Type: e}, "app", pw)
		if err != nil {
			t.Fatal(err)
		}
		s := e.Spec()
		if s.Repository == "" || s.Port == 0 || s.DataPath == "" || len(d.Env()) == 0 || len(d.ReadinessProbe()) == 0 {
			t.Errorf("%s: incomplete spec %+v", e, s)
		}
		if !strings.HasPrefix(d.Image(), "docker.io/") || !strings.HasSuffix(d.Image(), ":"+s.DefaultVersion) {
			t.Errorf("%s: image %s", e, d.Image())
		}
		for _, arg := range d.Command() {
			if strings.Contains(arg, "p@ss") {
				t.Errorf("%s: password on the command line", e)
			}
		}
	}
}

func TestURLs(t *testing.T) {
	for e, want := range map[DatabaseType]string{
		PostgreSQL: "postgres://bakery:p%40ss%2Fword@bakery-db-app:5432/app",
		MySQL:      "mysql://bakery:p%40ss%2Fword@bakery-db-app:3306/app",
		MariaDB:    "mysql://bakery:p%40ss%2Fword@bakery-db-app:3306/app",
		Redis:      "redis://default:p%40ss%2Fword@bakery-db-app:6379/0",
		Valkey:     "redis://default:p%40ss%2Fword@bakery-db-app:6379/0",
		MongoDB:    "mongodb://bakery:p%40ss%2Fword@bakery-db-app:27017/app?authSource=admin",
	} {
		d, _ := NewDatabase(1, 1, Input{Name: "app", Type: e}, "app", pw)
		if got := d.InternalURL(); got != want {
			t.Errorf("%s: %s, want %s", e, got, want)
		}
		if d.PublicURL("example.com") != "" {
			t.Errorf("%s: public URL without a public port", e)
		}
	}
	d, _ := NewDatabase(1, 1, Input{Name: "app", Type: PostgreSQL, PublicPort: 54321}, "app", pw)
	if got := d.PublicURL("db.example.com"); got != "postgres://bakery:p%40ss%2Fword@db.example.com:54321/app" {
		t.Fatal(got)
	}
}

func TestGeneratedName(t *testing.T) {
	if got := GeneratedName(PostgreSQL, "ab12cd34"); got != "postgresql-database-ab12cd34" {
		t.Errorf("got %q", got)
	}
	if _, err := NewDatabase(2, 1, Input{Name: GeneratedName(Valkey, "ab12cd34"), Type: Valkey}, "x", pw); err != nil {
		t.Errorf("a generated name must be valid: %v", err)
	}
}
