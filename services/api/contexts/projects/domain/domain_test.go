package domain

import (
	"errors"
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
		{func(i *ApplicationInput) { i.Domain = "localhost" }, "domain"},
		{func(i *ApplicationInput) { i.Domain = "a_b.example.com" }, "domain"},
	}
	for i, c := range bad {
		in := ok
		c.mutate(&in)
		if _, err := in.Normalize(); field(err) != c.field {
			t.Errorf("case %d: err = %v, want field %s", i, err, c.field)
		}
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

func TestCheckEnvVars(t *testing.T) {
	if err := CheckEnvVars([]EnvVar{{"HELLO", "world", false, true}, {"_x1", "", true, false}}); err != nil {
		t.Fatal(err)
	}
	for _, vars := range [][]EnvVar{
		{{"1ABC", "x", false, true}},
		{{"A-B", "x", false, true}},
		{{"A", "x", false, true}, {"A", "y", false, true}},
	} {
		if field(CheckEnvVars(vars)) != "env" {
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
			if IsSSHSource(raw) {
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
	project := []EnvVar{{Name: "A", Value: "p", Runtime: true}, {Name: "B", Value: "p", Runtime: true}, {Name: "C", Value: "p", Build: true, Runtime: true}}
	environment := []EnvVar{{Name: "B", Value: "e", Runtime: true}, {Name: "D", Value: "e", Build: true}}
	application := []EnvVar{{Name: "C", Value: "a", Runtime: true}}
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

func TestCheckEnvVarsScope(t *testing.T) {
	if err := CheckEnvVars([]EnvVar{{Name: "A", Value: "x"}}); field(err) != "env" {
		t.Fatalf("a variable with no scope: %v", err)
	}
	if err := CheckEnvVars([]EnvVar{{Name: "A", Build: true}}); err != nil {
		t.Fatal(err)
	}
}
