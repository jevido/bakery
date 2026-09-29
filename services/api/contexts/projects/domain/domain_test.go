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
		{func(i *ApplicationInput) { i.GitURL = "ssh://git@github.com/x/y" }, "git_url"},
		{func(i *ApplicationInput) { i.GitURL = "file:///etc" }, "git_url"},
		{func(i *ApplicationInput) { i.GitURL = "/home/me/repo" }, "git_url"},
		{func(i *ApplicationInput) { i.GitURL = "git@github.com:x/y.git" }, "git_url"},
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
	if err := CheckEnvVars([]EnvVar{{"HELLO", "world"}, {"_x1", ""}}); err != nil {
		t.Fatal(err)
	}
	for _, vars := range [][]EnvVar{
		{{"1ABC", "x"}},
		{{"A-B", "x"}},
		{{"A", "x"}, {"A", "y"}},
	} {
		if field(CheckEnvVars(vars)) != "env" {
			t.Errorf("%v should be refused", vars)
		}
	}
}
