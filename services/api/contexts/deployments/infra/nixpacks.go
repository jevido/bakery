package infra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// NixpacksDockerfile is where `nixpacks build --out` writes the Dockerfile,
// relative to the clone.
const NixpacksDockerfile = ".nixpacks/Dockerfile"

// Nixpacks plans builds with the nixpacks binary where the API runs, as Git
// clones with git. It only reads the clone and writes a Dockerfile into it;
// Podman builds that Dockerfile.
type Nixpacks struct {
	// Binary is the nixpacks executable, a path or a name on PATH.
	Binary string
}

// Plan writes .nixpacks/Dockerfile into dir and returns its path and the
// build args it needs: Nixpacks' own (NODE_ENV, CI, …) and the build
// variables. The variables reach nixpacks through its environment and are
// named with --env NAME, so no value is ever on a command line.
func (n Nixpacks) Plan(ctx context.Context, dir string, buildEnv map[string]string, out func(stream, line string)) (string, map[string]string, error) {
	names := slices.Sorted(maps.Keys(buildEnv))
	var envFlags []string
	for _, name := range names {
		envFlags = append(envFlags, "--env", name)
	}
	// Only what nixpacks needs, not the API's own environment.
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.TempDir()}
	for _, name := range names {
		env = append(env, name+"="+buildEnv[name])
	}

	var lines []string
	build := exec.CommandContext(ctx, n.Binary, append([]string{"build", dir, "--out", dir}, envFlags...)...)
	build.Env = env
	err := runStreaming(build, func(stream, line string) {
		lines = append(lines, line)
		out(stream, line)
	})
	if err != nil {
		return "", nil, n.explain(err, lines)
	}

	plan := exec.CommandContext(ctx, n.Binary, append([]string{"plan", dir, "--format", "json"}, envFlags...)...)
	plan.Env = env
	raw, err := plan.Output()
	if err != nil {
		return "", nil, n.explain(err, nil)
	}
	var p struct {
		Variables map[string]string `json:"variables"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", nil, fmt.Errorf("reading the nixpacks plan: %w", err)
	}
	args := p.Variables
	if args == nil {
		args = map[string]string{}
	}
	maps.Copy(args, buildEnv)
	return NixpacksDockerfile, args, nil
}

func (n Nixpacks) explain(err error, lines []string) error {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("nixpacks is not installed (set BAKERY_NIXPACKS): %s not found", n.Binary)
	}
	for _, line := range lines {
		if strings.Contains(line, "unable to generate a build plan") {
			return errors.New("Nixpacks was unable to generate a build plan for this repository; add a Dockerfile or use a language it supports")
		}
	}
	return fmt.Errorf("nixpacks: %w", err)
}
