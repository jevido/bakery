package infra

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeNixpacks is a script standing in for nixpacks: it records its
// arguments and environment, and answers plan with JSON.
func fakeNixpacks(t *testing.T, body string) (bin, record string) {
	t.Helper()
	dir := t.TempDir()
	bin, record = filepath.Join(dir, "nixpacks"), filepath.Join(dir, "record")
	script := "#!/bin/sh\necho \"$@\" >> " + record + "\necho \"SECRET=$SECRET API_KEY=$APP_KEY\" >> " + record + "\n" + body
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, record
}

func TestNixpacksPlan(t *testing.T) {
	t.Setenv("APP_KEY", "the-api-key")
	bin, record := fakeNixpacks(t, `case "$1" in
build) echo "║ setup │ nodejs_18 ║";;
plan) echo '{"variables":{"NODE_ENV":"production","SECRET":"from-plan"}}';;
esac`)
	var lines []string
	file, args, err := Nixpacks{Binary: bin}.Plan(context.Background(), "/src", map[string]string{"SECRET": "s3cret", "A": "1"},
		func(_, line string) { lines = append(lines, line) })
	if err != nil {
		t.Fatal(err)
	}
	if file != ".nixpacks/Dockerfile" {
		t.Errorf("dockerfile %q", file)
	}
	if args["NODE_ENV"] != "production" || args["SECRET"] != "s3cret" || args["A"] != "1" {
		t.Errorf("build args %v", args)
	}
	if len(lines) != 1 || !strings.Contains(lines[0], "nodejs_18") {
		t.Errorf("output %q", lines)
	}
	raw, _ := os.ReadFile(record)
	got := string(raw)
	for _, want := range []string{"build /src --out /src --env A --env SECRET", "plan /src --format json --env A --env SECRET", "SECRET=s3cret API_KEY=\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("record lacks %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "s3cret") != 2 || strings.Contains(got, "the-api-key") {
		t.Errorf("a value reached the arguments, or the API's environment leaked:\n%s", got)
	}
}

func TestNixpacksErrors(t *testing.T) {
	_, _, err := Nixpacks{Binary: filepath.Join(t.TempDir(), "missing")}.Plan(context.Background(), "/src", nil, func(string, string) {})
	if err == nil || !strings.Contains(err.Error(), "nixpacks is not installed") {
		t.Errorf("missing binary: %v", err)
	}
	bin, _ := fakeNixpacks(t, `echo "Nixpacks was unable to generate a build plan for this app."; exit 1`)
	_, _, err = Nixpacks{Binary: bin}.Plan(context.Background(), "/src", nil, func(string, string) {})
	if err == nil || !strings.Contains(err.Error(), "unable to generate a build plan for this repository") {
		t.Errorf("no plan: %v", err)
	}
}
