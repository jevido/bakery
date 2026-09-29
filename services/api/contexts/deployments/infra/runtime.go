package infra

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/deployments/app"
)

// Runtime builds and runs Application Containers with Podman.
type Runtime struct {
	Podman  *podman.Client
	Network string
	// StartTimeout is how long a Container gets to reach running.
	StartTimeout time.Duration
	// Settle is how long it must then stay running; an app that crashes on
	// boot fails the Deployment instead of taking the Route.
	Settle time.Duration
}

func (r Runtime) Build(ctx context.Context, dir, dockerfile, tag string, labels map[string]string, out func(string)) error {
	tar := podman.TarDir(dir)
	defer tar.Close()
	_, err := r.Podman.Build(ctx, tar, podman.BuildOptions{Tag: tag, Dockerfile: dockerfile, Labels: labels}, out)
	return err
}

func (r Runtime) Start(ctx context.Context, spec app.ContainerSpec) error {
	_, err := r.Podman.CreateContainer(ctx, podman.ContainerSpec{
		Name:  spec.Name,
		Image: spec.Image,
		Env:   spec.Env,
		Labels: map[string]string{
			"bakery.managed":     "true",
			"bakery.application": strconv.FormatUint(spec.ApplicationID, 10),
			"bakery.deployment":  strconv.FormatUint(spec.DeploymentID, 10),
		},
		Networks:      podman.OnNetwork(r.Network),
		RestartPolicy: "always",
	})
	if err != nil {
		return err
	}
	if err := r.Podman.StartContainer(ctx, spec.Name); err != nil {
		r.remove(spec.Name)
		return err
	}
	if err := r.waitRunning(ctx, spec.Name); err != nil {
		r.remove(spec.Name)
		return err
	}
	return nil
}

func (r Runtime) waitRunning(ctx context.Context, name string) error {
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
		case info.State.Running && time.Since(runningSince) >= r.Settle:
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

func (r Runtime) RemoveOthers(ctx context.Context, applicationID uint64, keep string) ([]string, error) {
	list, err := r.containers(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, c := range list {
		name := strings.TrimPrefix(firstName(c.Names), "/")
		if name == keep {
			continue
		}
		// Stop gracefully first so the app can finish in-flight requests.
		_ = r.Podman.StopContainer(ctx, c.ID, 10)
		if err := r.Remove(ctx, c.ID); err != nil {
			return removed, err
		}
		removed = append(removed, name)
	}
	return removed, nil
}

func (r Runtime) RemoveAll(ctx context.Context, applicationID uint64) error {
	_, err := r.RemoveOthers(ctx, applicationID, "")
	return err
}

func firstName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return names[0]
}
