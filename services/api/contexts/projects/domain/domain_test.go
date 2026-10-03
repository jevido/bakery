package domain

import (
	"errors"
	"strings"
	"testing"
)

func field(err error) string {
	var fe *FieldError
	if errors.As(err, &fe) {
		return fe.Field
	}
	return ""
}

func TestApplicationInputNormalize(t *testing.T) {
	ok := ApplicationInput{Name: "Who Am I", GitURL: "https://github.com/traefik/whoami", Port: 80}
	in, err := ok.Normalize()
	if err != nil {
		t.Fatalf("valid input: %v", err)
	}
	if in.GitBranch != "main" || in.DockerfilePath != "Dockerfile" {
		t.Errorf("defaults not filled: %+v", in)
	}

	bad := []struct {
		mutate func(*ApplicationInput)
		field  string
	}{
		{func(i *ApplicationInput) { i.Name = " " }, "name"},
		{func(i *ApplicationInput) { i.Name = "!!!" }, "name"},
		{func(i *ApplicationInput) { i.GitURL = "file:///etc" }, "git_url"},
		{func(i *ApplicationInput) { i.GitURL = "/home/me/repo" }, "git_url"},
		{func(i *ApplicationInput) { i.GitURL = "https://user:pw@github.com/x/y" }, "git_url"},
		{func(i *ApplicationInput) { i.GitBranch = "--upload-pack=x" }, "git_branch"},
		{func(i *ApplicationInput) { i.DockerfilePath = "../Dockerfile" }, "dockerfile_path"},
		{func(i *ApplicationInput) { i.DockerfilePath = "/etc/passwd" }, "dockerfile_path"},
		{func(i *ApplicationInput) { i.Port = 0 }, "port"},
		{func(i *ApplicationInput) { i.Port = 70000 }, "port"},
		{func(i *ApplicationInput) { i.Domains = []string{"localhost"} }, "domains"},
		{func(i *ApplicationInput) { i.Domains = []string{"a_b.example.com"} }, "domains"},
		{func(i *ApplicationInput) { i.Domains = []string{"a.example.com", "A.example.com "} }, "domains"},
		{func(i *ApplicationInput) {
			for n := range MaxDomains + 1 {
				i.Domains = append(i.Domains, string(rune('a'+n))+".example.com")
			}
		}, "domains"},
	}
	for i, c := range bad {
		in := ok
		c.mutate(&in)
		if _, err := in.Normalize(); field(err) != c.field {
			t.Errorf("case %d: err = %v, want field %s", i, err, c.field)
		}
	}
}

func TestDomainsNormalize(t *testing.T) {
	in := ApplicationInput{Name: "web", GitURL: "https://example.com/r.git", Port: 80, Domains: []string{" B.Example.com", "", "a.example.com"}}
	out, err := in.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Domains) != 2 || out.Domains[0] != "b.example.com" || out.Domains[1] != "a.example.com" {
		t.Fatalf("domains: %v", out.Domains)
	}
	if (Application{Domains: out.Domains}).PrimaryDomain() != "b.example.com" {
		t.Fatal("primary is the first domain")
	}
}

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"Who Am I":   "who-am-i",
		"  API v2 ":  "api-v2",
		"ümlaut app": "mlaut-app",
	} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCheckEnvironmentVariables(t *testing.T) {
	if err := CheckEnvironmentVariables([]EnvironmentVariable{{"HELLO", "world", false, true}, {"_x1", "", true, false}}); err != nil {
		t.Fatal(err)
	}
	for _, vars := range [][]EnvironmentVariable{
		{{"1ABC", "x", false, true}},
		{{"A-B", "x", false, true}},
		{{"A", "x", false, true}, {"A", "y", false, true}},
	} {
		if field(CheckEnvironmentVariables(vars)) != "environment_variables" {
			t.Errorf("%v should be refused", vars)
		}
	}
}

func TestGitURLKinds(t *testing.T) {
	for raw, want := range map[string]string{
		"https://github.com/x/y":                "https",
		"ssh://git@github.com/x/y.git":          "ssh",
		"ssh://git@127.0.0.1:4952/me/app.git":   "ssh",
		"git@github.com:x/y.git":                "ssh",
		"git@gitlab.com:group/sub/repo.git":     "ssh",
		"ssh://github.com/x/y":                  "", // no user
		"ssh://git:pw@github.com/x/y":           "", // password
		"ssh://-oProxyCommand=x@github.com/x/y": "",
		"ssh://git@-oProxyCommand=x/y":          "",
		"ssh://git@github.com/":                 "", // no path
		"git@github.com:-oProxyCommand=x":       "",
		"-oProxyCommand=x@github.com:y":         "",
		"git@github.com":                        "",
		"ext::sh -c touch% /tmp/pwned":          "",
		"file:///etc":                           "",
		"git://github.com/x/y":                  "",
		"git@github.com:x/y with space":         "",
		"http://github.com/x/y":                 "",
	} {
		in := ApplicationInput{Name: "a", GitURL: raw, Port: 80}
		_, err := in.Normalize()
		got := ""
		if err == nil {
			got = "https"
			if IsSSHRepository(raw) {
				got = "ssh"
			}
		}
		if got != want {
			t.Errorf("%q: got %q (err %v), want %q", raw, got, err, want)
		}
	}
}

func TestHealthCheckNormalize(t *testing.T) {
	h, err := HealthCheck{Enabled: true}.Normalize()
	if err != nil {
		t.Fatalf("empty check: %v", err)
	}
	if h != (HealthCheck{Enabled: true, Path: "/", Interval: 5, Timeout: 5, Retries: 10}) {
		t.Errorf("defaults not filled: %+v", h)
	}
	if _, err := (HealthCheck{Path: "/health?full=1", Interval: 300, Timeout: 60, Retries: 100, StartPeriod: 600}).Normalize(); err != nil {
		t.Errorf("maximums refused: %v", err)
	}
	bad := []struct {
		h     HealthCheck
		field string
	}{
		{HealthCheck{Path: "health"}, "health_check.path"},
		{HealthCheck{Path: "/a b"}, "health_check.path"},
		{HealthCheck{Path: "/'; rm -rf /"}, "health_check.path"},
		{HealthCheck{Interval: 301}, "health_check.interval"},
		{HealthCheck{Timeout: -1}, "health_check.timeout"},
		{HealthCheck{Retries: 101}, "health_check.retries"},
		{HealthCheck{StartPeriod: -1}, "health_check.start_period"},
	}
	for _, c := range bad {
		if _, err := c.h.Normalize(); field(err) != c.field {
			t.Errorf("%+v: got %v, want an error on %s", c.h, err, c.field)
		}
	}
	in := ApplicationInput{Name: "x", GitURL: "https://github.com/a/b", Port: 80, HealthCheck: &HealthCheck{Path: "nope"}}
	if _, err := in.Normalize(); field(err) != "health_check.path" {
		t.Errorf("input with a bad check: %v", err)
	}
}

func TestMergeVariables(t *testing.T) {
	project := []EnvironmentVariable{{Name: "A", Value: "p", Runtime: true}, {Name: "B", Value: "p", Runtime: true}, {Name: "C", Value: "p", Build: true, Runtime: true}}
	environment := []EnvironmentVariable{{Name: "B", Value: "e", Runtime: true}, {Name: "D", Value: "e", Build: true}}
	application := []EnvironmentVariable{{Name: "C", Value: "a", Runtime: true}}
	build, runtime := Merge(project, environment, application)
	if len(build) != 1 || build["D"] != "e" {
		t.Errorf("build %v", build)
	}
	if len(runtime) != 3 || runtime["A"] != "p" || runtime["B"] != "e" || runtime["C"] != "a" {
		t.Errorf("runtime %v", runtime)
	}
	got := map[string]bool{}
	for _, v := range Inherited(project, environment, application) {
		got[v.From+":"+v.Name] = v.Overridden
	}
	want := map[string]bool{"environment:B": false, "environment:D": false, "project:A": false, "project:B": true, "project:C": true}
	if len(got) != len(want) {
		t.Fatalf("inherited %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s overridden %v, want %v", k, got[k], v)
		}
	}
}

func TestCheckEnvironmentVariablesScope(t *testing.T) {
	if err := CheckEnvironmentVariables([]EnvironmentVariable{{Name: "A", Value: "x"}}); field(err) != "environment_variables" {
		t.Fatalf("a variable with no scope: %v", err)
	}
	if err := CheckEnvironmentVariables([]EnvironmentVariable{{Name: "A", Build: true}}); err != nil {
		t.Fatal(err)
	}
}

func TestBuildPacks(t *testing.T) {
	img, err := ApplicationInput{Name: "who", BuildPack: DockerImage, DockerImage: " docker.io/traefik/whoami:v1.10 ",
		GitURL: "https://github.com/x/y", Port: 80}.Normalize()
	if err != nil {
		t.Fatalf("image: %v", err)
	}
	if img.DockerImage != "docker.io/traefik/whoami:v1.10" || img.GitURL != "" || img.GitBranch != "" || img.DockerfilePath != "" {
		t.Errorf("image input not cleaned: %+v", img)
	}
	for _, ref := range []string{"localhost/app:1", "127.0.0.1:4950/me/app:1", "ghcr.io/me/app@sha256:abc"} {
		if _, err := (ApplicationInput{Name: "x", BuildPack: DockerImage, DockerImage: ref, Port: 80}).Normalize(); err != nil {
			t.Errorf("%s: %v", ref, err)
		}
	}
	for _, ref := range []string{"", "nginx", "nginx:1.27", "library/nginx", "-x/y", "docker.io/a b"} {
		if _, err := (ApplicationInput{Name: "x", BuildPack: DockerImage, DockerImage: ref, Port: 80}).Normalize(); field(err) != "docker_image" {
			t.Errorf("%q: err = %v, want docker_image", ref, err)
		}
	}

	st, err := ApplicationInput{Name: "site", BuildPack: Static, GitURL: "https://github.com/x/y", Port: 3000, DockerImage: "docker.io/x/y"}.Normalize()
	if err != nil {
		t.Fatalf("static: %v", err)
	}
	if st.Port != StaticPort || st.PublishDirectory != "." || st.DockerImage != "" {
		t.Errorf("static defaults: %+v", st)
	}
	for _, dir := range []string{"../x", "/srv", "a/../../b"} {
		if _, err := (ApplicationInput{Name: "x", BuildPack: Static, GitURL: "https://github.com/x/y", PublishDirectory: dir}).Normalize(); field(err) != "publish_directory" {
			t.Errorf("%q: err = %v, want publish_directory", dir, err)
		}
	}

	if _, err := (ApplicationInput{Name: "x", BuildPack: "heroku", GitURL: "https://github.com/x/y", Port: 80}).Normalize(); field(err) != "build_pack" {
		t.Errorf("unknown pack: err = %v", err)
	}
	if _, err := (ApplicationInput{Name: "x", BuildPack: Nixpacks, Port: 80}).Normalize(); field(err) != "git_url" {
		t.Errorf("nixpacks without source: err = %v", err)
	}
	d, _ := ApplicationInput{Name: "x", GitURL: "https://github.com/x/y", Port: 80}.Normalize()
	if d.BuildPack != Dockerfile {
		t.Errorf("default pack = %q", d.BuildPack)
	}
}

func TestRegistryCredentials(t *testing.T) {
	base := ApplicationInput{Name: "x", BuildPack: DockerImage, DockerImage: "ghcr.io/me/app:1", Port: 80}
	with := func(u, p string) ApplicationInput {
		in := base
		in.RegistryCredentials = &RegistryCredentials{Username: u, Password: p}
		return in
	}
	if in, err := with(" me ", "token").Normalize(); err != nil || in.RegistryCredentials.Username != "me" {
		t.Errorf("valid: %+v, %v", in.RegistryCredentials, err)
	}
	if in, err := with("", "left over").Normalize(); err != nil || *in.RegistryCredentials != (RegistryCredentials{}) {
		t.Errorf("clearing: %+v, %v", in.RegistryCredentials, err)
	}
	for _, c := range [][2]string{{"me", ""}, {"a b", "x"}} {
		if _, err := with(c[0], c[1]).Normalize(); field(err) != "registry_credentials" {
			t.Errorf("%q: err = %v", c, err)
		}
	}
	git := ApplicationInput{Name: "x", GitURL: "https://github.com/x/y", Port: 80, RegistryCredentials: &RegistryCredentials{Username: "me", Password: "pw"}}
	if in, err := git.Normalize(); err != nil || *in.RegistryCredentials != (RegistryCredentials{}) {
		t.Errorf("git pack keeps credentials: %+v, %v", in.RegistryCredentials, err)
	}
}

func TestStorages(t *testing.T) {
	base := ApplicationInput{Name: "web", GitURL: "https://example.com/r.git", Port: 80}
	ok := []Storage{{" data ", "/data"}, {"cache", "/var/cache/app"}}
	in := base
	in.Storages = &ok
	out, err := in.Normalize()
	if err != nil || (*out.Storages)[0].Name != "data" {
		t.Fatalf("valid: %v %+v", err, out.Storages)
	}
	for i, bad := range [][]Storage{
		{{"Data", "/data"}},
		{{"data", "data"}},
		{{"data", "/data/"}},
		{{"data", "/data/../etc"}},
		{{"data", "/"}},
		{{"data", "/a:b"}},
		{{"data", "/a"}, {"data", "/b"}},
		{{"a", "/data"}, {"b", "/data"}},
		{{"a", "/1"}, {"b", "/2"}, {"c", "/3"}, {"d", "/4"}, {"e", "/5"}, {"f", "/6"}, {"g", "/7"}, {"h", "/8"}, {"i", "/9"}, {"j", "/10"}, {"k", "/11"}},
	} {
		in := base
		in.Storages = &bad
		if _, err := in.Normalize(); field(err) != "storages" {
			t.Errorf("case %d: %v", i, err)
		}
	}
}

func TestResourceLimits(t *testing.T) {
	for _, ok := range []ResourceLimits{{}, {MemoryMB: 16}, {CPUs: 0.1}, {MemoryMB: 65536, CPUs: 64}, {CPUs: 1.25}} {
		if err := ok.Check(); err != nil {
			t.Errorf("%+v: %v", ok, err)
		}
	}
	for l, f := range map[ResourceLimits]string{
		{MemoryMB: 8}:     "resource_limits.memory_mb",
		{MemoryMB: 70000}: "resource_limits.memory_mb",
		{CPUs: 0.05}:      "resource_limits.cpus",
		{CPUs: 65}:        "resource_limits.cpus",
		{CPUs: 1.255}:     "resource_limits.cpus",
	} {
		if field(l.Check()) != f {
			t.Errorf("%+v: want %s", l, f)
		}
	}
}

func TestNewEnvironment(t *testing.T) {
	e, err := NewEnvironment("  staging ", " Pre-production ")
	if err != nil || e.Name != "staging" || e.Description != "Pre-production" {
		t.Fatalf("valid input: %+v, %v", e, err)
	}
	if _, err := NewEnvironment("Prüfung (eu) #2", ""); err != nil {
		t.Errorf("unicode and allowed punctuation refused: %v", err)
	}
	long := strings.Repeat("a", 256)
	for _, c := range []struct{ name, description, field string }{
		{"", "", "name"},
		{"  ", "", "name"},
		{"qa", "", "name"},
		{long, "", "name"},
		{"stag;ing", "", "name"},
		{"$(rm)", "", "name"},
		{"staging", long, "description"},
	} {
		if _, err := NewEnvironment(c.name, c.description); field(err) != c.field {
			t.Errorf("NewEnvironment(%q, %d chars) = %v, want a %s error", c.name, len(c.description), err, c.field)
		}
	}
}

func TestGeneratedApplicationName(t *testing.T) {
	for _, c := range []struct {
		pack                BuildPack
		url, branch, random string
		want                string
	}{
		{Dockerfile, "https://github.com/traefik/whoami", "master", "ab12cd34", "whoami:master-ab12cd34"},
		{Nixpacks, "https://github.com/me/My Repo.git/", "", "ab12cd34", "my-repo:main-ab12cd34"},
		{Dockerfile, "git@github.com:me/WebApp.git", "feature/x", "ab12cd34", "web-app:feature/x-ab12cd34"},
		{Dockerfile, "git@host:repo.git", "main", "ab12cd34", "repo:main-ab12cd34"},
		{DockerImage, "", "", "ab12cd34", "docker-image-ab12cd34"},
	} {
		if got := GeneratedApplicationName(c.pack, c.url, c.branch, c.random); got != c.want {
			t.Errorf("GeneratedApplicationName(%s, %q, %q) = %q, want %q", c.pack, c.url, c.branch, got, c.want)
		}
	}
	long := GeneratedApplicationName(Dockerfile, "https://x/"+strings.Repeat("a", 120), "main", "ab12cd34")
	if len(long) != 100 || !strings.HasSuffix(long, ":main-ab12cd34") {
		t.Errorf("long repository: %q (%d)", long, len(long))
	}
	in := ApplicationInput{Name: GeneratedApplicationName(Dockerfile, "https://github.com/traefik/whoami", "master", "ab12cd34"), GitURL: "https://github.com/traefik/whoami", Port: 80}
	if _, err := in.Normalize(); err != nil {
		t.Errorf("a generated name must pass Normalize: %v", err)
	}
	if got := Slugify(in.Name); got != "whoami-master-ab12cd34" {
		t.Errorf("slug %q", got)
	}
}
