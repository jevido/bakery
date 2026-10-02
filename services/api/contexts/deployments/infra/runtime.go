package infra

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/deployments/app"
)

// Runtime builds and runs Application Containers with Podman.
type Runtime struct {
	// Server names the Server Podman runs on.
	Server  string
	Podman  *podman.Client
	Network string
	// StartTimeout is how long a Container gets to reach running.
	StartTimeout time.Duration
	// Settle is how long it must then stay running; an app that crashes on
	// boot fails the Deployment instead of taking the Route.
	Settle time.Duration
	// InsecureRegistries are registry hosts (host[:port]) pulled from
	// without TLS verification.
	InsecureRegistries []string
}

func (r Runtime) ServerName() string { return r.Server }

func (r Runtime) Build(ctx context.Context, req app.BuildRequest, out func(string)) error {
	tar := podman.TarDir(req.Dir)
	defer tar.Close()
	_, err := r.Podman.Build(ctx, tar, podman.BuildOptions{Tag: req.Tag, Dockerfile: req.Dockerfile, Labels: req.Labels, BuildArgs: req.BuildArgs}, out)
	return err
}

func (r Runtime) Pull(ctx context.Context, req app.PullRequest, out func(string)) (string, error) {
	opts := podman.PullOptions{Username: req.Username, Password: req.Password, TLSVerify: !slices.Contains(r.InsecureRegistries, RegistryHost(req.Reference))}
	id, err := r.Podman.PullImageWith(ctx, req.Reference, opts, out)
	if err != nil {
		return "", err
	}
	i := strings.LastIndex(req.Tag, ":")
	if err := r.Podman.TagImage(ctx, id, req.Tag[:i], req.Tag[i+1:]); err != nil {
		return "", fmt.Errorf("tagging %s: %w", req.Tag, err)
	}
	return r.Podman.ImageDigest(ctx, req.Reference)
}

// RegistryHost is the registry part of a full image reference.
func RegistryHost(ref string) string {
	host, _, _ := strings.Cut(ref, "/")
	return host
}

func (r Runtime) Start(ctx context.Context, spec app.ContainerSpec) error {
	application := strconv.FormatUint(spec.ApplicationID, 10)
	labels := map[string]string{"bakery.managed": "true", "bakery.application": application}
	if spec.Preview != 0 {
		labels["bakery.preview"] = strconv.Itoa(spec.Preview)
	}
	// A Server's first Container comes before its Proxy, which otherwise
	// makes the network.
	if err := r.Podman.EnsureNetwork(ctx, r.Network); err != nil {
		return fmt.Errorf("network %s: %w", r.Network, err)
	}
	volumes := make([]podman.NamedVolume, len(spec.Mounts))
	for i, m := range spec.Mounts {
		// Created here, with Bakery's labels, rather than implicitly by
		// Podman, so RemoveVolumes finds exactly the volumes Bakery made.
		if err := r.Podman.CreateVolume(ctx, m.Volume, labels); err != nil {
			return fmt.Errorf("creating volume %s: %w", m.Volume, err)
		}
		volumes[i] = podman.NamedVolume{Name: m.Volume, Dest: m.Path}
	}
	_, err := r.Podman.CreateContainer(ctx, podman.ContainerSpec{
		Name:           spec.Name,
		Image:          spec.Image,
		Env:            spec.Env,
		Labels:         containerLabels(labels, spec.DeploymentID),
		Networks:       podman.OnNetwork(r.Network),
		RestartPolicy:  "always",
		Volumes:        volumes,
		ResourceLimits: podman.Limits(spec.MemoryMB, spec.CPUs),
	})
	if err != nil {
		return err
	}
	if err := r.Podman.StartContainer(ctx, spec.Name); err != nil {
		r.remove(spec.Name)
		return err
	}
	settle := time.Duration(0)
	if spec.Settle {
		settle = r.Settle
	}
	if err := r.waitRunning(ctx, spec.Name, settle); err != nil {
		r.remove(spec.Name)
		return err
	}
	return nil
}

// containerLabels are a Container's labels: its Volumes' plus its
// Deployment.
func containerLabels(labels map[string]string, deploymentID uint64) map[string]string {
	out := map[string]string{"bakery.deployment": strconv.FormatUint(deploymentID, 10)}
	for k, v := range labels {
		out[k] = v
	}
	return out
}

func (r Runtime) waitRunning(ctx context.Context, name string, settle time.Duration) error {
	deadline := time.Now().Add(r.StartTimeout)
	var runningSince time.Time
	for {
		info, err := r.Podman.InspectContainer(ctx, name)
		if err != nil {
			return err
		}
		switch {
		case info.State.Running && runningSince.IsZero():
			runningSince = time.Now()
		case info.State.Running && time.Since(runningSince) >= settle:
			return nil
		case !info.State.Running && info.State.Status == "exited":
			return r.exited(ctx, name, info.State.ExitCode)
		case !info.State.Running:
			runningSince = time.Time{}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("not running after %s (state %s)", r.StartTimeout, info.State.Status)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
}

// probeScript requests $0 with curl, or else wget, within $1 seconds, and
// prints why it failed. Exit 127 means the image has neither.
const probeScript = `if command -v curl >/dev/null 2>&1; then exec curl -fsS -o /dev/null --max-time "$1" "$0"
elif command -v wget >/dev/null 2>&1; then exec wget -q -O /dev/null -T "$1" "$0"
else echo "no curl or wget in the image" >&2; exit 127; fi`

// errNoProbeTool is a Health check that cannot run in this image.
var errNoProbeTool = errors.New("the health check needs sh and curl or wget in the image")

func (r Runtime) Probe(ctx context.Context, container, url string, timeout time.Duration) (bool, string, error) {
	info, err := r.Podman.InspectContainer(ctx, container)
	if err != nil {
		return false, "", err
	}
	if !info.State.Running {
		if info.State.Status == "exited" {
			return false, "", r.exited(ctx, container, info.State.ExitCode)
		}
		return false, "", fmt.Errorf("container is %s", info.State.Status)
	}
	secs := int(timeout.Seconds())
	// A little longer than the tool's own limit, so its message wins.
	pctx, cancel := context.WithTimeout(ctx, timeout+2*time.Second)
	defer cancel()
	code, out, err := r.Podman.Exec(pctx, container, []string{"sh", "-c", probeScript, url, strconv.Itoa(secs)})
	switch {
	case err != nil && ctx.Err() != nil:
		return false, "", ctx.Err()
	case err != nil && pctx.Err() != nil:
		return false, fmt.Sprintf("no answer within %ds", secs), nil
	case err != nil:
		return false, "", fmt.Errorf("%w (%v)", errNoProbeTool, err)
	case code == 127 || code == 126:
		return false, "", errNoProbeTool
	case code != 0:
		if out == "" {
			out = fmt.Sprintf("exit code %d", code)
		}
		return false, strings.TrimSpace(out), nil
	}
	return true, "", nil
}

// exited explains a Container that stopped on its own, with its last output.
func (r Runtime) exited(ctx context.Context, name string, code int) error {
	var tail []string
	_ = r.Podman.Logs(ctx, name, false, 5, func(_, line string) { tail = append(tail, line) })
	msg := fmt.Sprintf("exited with code %d", code)
	if len(tail) > 0 {
		msg += "; last output: " + strings.Join(tail, " | ")
	}
	return fmt.Errorf("%s", msg)
}

func (r Runtime) remove(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = r.Podman.RemoveContainer(ctx, name)
}

func (r Runtime) ImageExists(ctx context.Context, image string) (bool, error) {
	return r.Podman.ImageExists(ctx, image)
}

func (r Runtime) RemoveImage(ctx context.Context, image string) (int64, error) {
	reclaimed, _, err := r.Podman.RemoveUnusedImage(ctx, image)
	return reclaimed, err
}

func (r Runtime) Remove(ctx context.Context, name string) error {
	err := r.Podman.RemoveContainer(ctx, name)
	if podman.IsNotFound(err) {
		return nil
	}
	return err
}

func (r Runtime) containers(ctx context.Context, applicationID uint64) ([]podman.ContainerSummary, error) {
	return r.Podman.ListContainers(ctx, map[string]string{
		"bakery.managed":     "true",
		"bakery.application": strconv.FormatUint(applicationID, 10),
	})
}

func (r Runtime) RemoveOthers(ctx context.Context, applicationID uint64, preview int, keep string) ([]string, error) {
	return r.removeWhere(ctx, applicationID, true, func(c podman.ContainerSummary) bool {
		return PreviewOf(c.Labels) == preview && strings.TrimPrefix(firstName(c.Names), "/") != keep
	})
}

// PreviewOf is the Preview number a Container's labels name, 0 for the
// Application's own.
func PreviewOf(labels map[string]string) int {
	n, _ := strconv.Atoi(labels["bakery.preview"])
	return n
}

// removeWhere removes the Application's Containers that match, stopping
// each gracefully first when graceful.
func (r Runtime) removeWhere(ctx context.Context, applicationID uint64, graceful bool, match func(podman.ContainerSummary) bool) ([]string, error) {
	list, err := r.containers(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, c := range list {
		if !match(c) {
			continue
		}
		name := strings.TrimPrefix(firstName(c.Names), "/")
		// Stop gracefully first so the app can finish in-flight requests.
		// Otherwise kill it at once: a forced remove would still wait for
		// the stop timeout of an app that ignores SIGTERM.
		timeout := 0
		if graceful {
			timeout = 10
		}
		_ = r.Podman.StopContainer(ctx, c.ID, timeout)
		if err := r.Remove(ctx, c.ID); err != nil {
			return removed, err
		}
		removed = append(removed, name)
	}
	return removed, nil
}

// RemoveVolumes removes every Volume of the Application; call it after its
// Containers are gone.
func (r Runtime) RemoveVolumes(ctx context.Context, applicationID uint64) error {
	list, err := r.Podman.ListVolumes(ctx, map[string]string{
		"bakery.managed":     "true",
		"bakery.application": strconv.FormatUint(applicationID, 10),
	})
	if err != nil {
		return err
	}
	for _, v := range list {
		if err := r.Podman.RemoveVolume(ctx, v.Name); err != nil {
			return err
		}
	}
	return nil
}

// RemoveAll removes every Container of the Application, its Previews'
// included, at once: nobody is served by them any more, and a graceful stop
// of each would hold up deleting the Application.
func (r Runtime) RemoveAll(ctx context.Context, applicationID uint64) error {
	_, err := r.removeWhere(ctx, applicationID, false, func(podman.ContainerSummary) bool { return true })
	return err
}

// RemovePreview removes the Preview's Containers at once, then its
// Volumes.
func (r Runtime) RemovePreview(ctx context.Context, applicationID uint64, preview int) ([]string, error) {
	removed, err := r.removeWhere(ctx, applicationID, false, func(c podman.ContainerSummary) bool { return PreviewOf(c.Labels) == preview })
	if err != nil {
		return removed, err
	}
	list, err := r.Podman.ListVolumes(ctx, map[string]string{
		"bakery.managed":     "true",
		"bakery.application": strconv.FormatUint(applicationID, 10),
		"bakery.preview":     strconv.Itoa(preview),
	})
	if err != nil {
		return removed, err
	}
	for _, v := range list {
		if err := r.Podman.RemoveVolume(ctx, v.Name); err != nil {
			return removed, err
		}
		removed = append(removed, v.Name)
	}
	return removed, nil
}

// Running returns the name of the Application's own running Container (not
// a Preview's).
func (r Runtime) Running(ctx context.Context, applicationID uint64) (string, bool, error) {
	list, err := r.containers(ctx, applicationID)
	if err != nil {
		return "", false, err
	}
	for _, c := range list {
		if c.State == "running" && PreviewOf(c.Labels) == 0 {
			return strings.TrimPrefix(firstName(c.Names), "/"), true, nil
		}
	}
	return "", false, nil
}

func firstName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return names[0]
}
