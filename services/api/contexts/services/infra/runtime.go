package infra

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/services/domain"
)

// Runtime runs a Service's Components with Podman: one network per
// Service, one volume per named volume, one Container per Component. A
// stopped Service has no Containers (its volumes stay), so
// podman-restart.service cannot bring it back after a reboot by accident.
type Runtime struct {
	Podman *podman.Client
	// Network is the network the Proxy is on; Public Components join it.
	Network string
	// StartCheck is how long a started Container is watched for exiting
	// before the next one starts; 0 is 3 seconds.
	StartCheck time.Duration
}

func labels(serviceID uint64) map[string]string {
	return map[string]string{"bakery.managed": "true", "bakery.service": strconv.FormatUint(serviceID, 10)}
}

// Up makes the network and volumes, pulls the images (every one when pull
// is set, else only missing ones) and replaces every Component's Container
// in start order, each started before the next is created. Containers of
// Components the Compose file no longer has are removed.
func (r Runtime) Up(ctx context.Context, s domain.Service, resolved domain.Compose, pull bool) error {
	order, err := domain.StartOrder(resolved)
	if err != nil {
		return err
	}
	network := domain.NetworkName(s.ID)
	if err := r.Podman.EnsureNetworkLabeled(ctx, network, labels(s.ID)); err != nil {
		return fmt.Errorf("creating network %s: %w", network, err)
	}
	if err := r.Podman.EnsureNetwork(ctx, r.Network); err != nil {
		return fmt.Errorf("creating network %s: %w", r.Network, err)
	}
	for _, v := range resolved.VolumeNames() {
		name := domain.VolumeName(s.ID, v)
		if err := r.Podman.CreateVolume(ctx, name, labels(s.ID)); err != nil {
			return fmt.Errorf("creating volume %s: %w", name, err)
		}
	}
	pulled := map[string]bool{}
	for _, c := range resolved.Components {
		if pulled[c.Image] {
			continue
		}
		pulled[c.Image] = true
		exists, err := r.Podman.ImageExists(ctx, c.Image)
		if err != nil {
			return err
		}
		if pull || !exists {
			if err := r.pull(ctx, c.Image); err != nil {
				return fmt.Errorf("pulling %s for %s: %w", c.Image, c.Name, err)
			}
		}
	}
	if err := r.removeOthers(ctx, s.ID, order); err != nil {
		return err
	}
	for _, name := range order {
		spec, _ := resolved.Component(name)
		comp, _ := s.Component(name)
		if err := r.start(ctx, s.ID, spec, comp.Public); err != nil {
			return fmt.Errorf("starting %s: %w", name, err)
		}
	}
	return nil
}

func (r Runtime) start(ctx context.Context, serviceID uint64, c domain.ComponentSpec, public bool) error {
	name := domain.ContainerName(serviceID, c.Name)
	if err := r.removeContainer(ctx, name); err != nil {
		return err
	}
	l := maps.Clone(c.Labels)
	if l == nil {
		l = map[string]string{}
	}
	maps.Copy(l, labels(serviceID))
	l["bakery.component"] = c.Name
	network, opts := podman.NetworkWithAliases(domain.NetworkName(serviceID), c.Name)
	networks := map[string]map[string]any{network: opts}
	if public {
		networks[r.Network] = map[string]any{}
	}
	var volumes []podman.NamedVolume
	for _, m := range c.Volumes {
		v := podman.NamedVolume{Name: domain.VolumeName(serviceID, m.Volume), Dest: m.Path}
		if m.ReadOnly {
			v.Options = []string{"ro"}
		}
		volumes = append(volumes, v)
	}
	var expose map[uint16]string
	for _, p := range c.Expose {
		if expose == nil {
			expose = map[uint16]string{}
		}
		expose[uint16(p)] = "tcp"
	}
	if _, err := r.Podman.CreateContainer(ctx, podman.ContainerSpec{
		Name: name, Image: c.Image,
		Command: c.Command, Entrypoint: c.Entrypoint, WorkDir: c.WorkingDir, User: c.User,
		Env: c.Environment, Labels: l, Networks: networks, Volumes: volumes, Expose: expose,
		RestartPolicy: "always",
	}); err != nil {
		return err
	}
	if err := r.Podman.StartContainer(ctx, name); err != nil {
		return err
	}
	return r.watch(ctx, name)
}

// watch fails when the Container exits within StartCheck, with the end of
// its output, so a Component that cannot start is reported as the reason.
func (r Runtime) watch(ctx context.Context, name string) error {
	check := r.StartCheck
	if check == 0 {
		check = 3 * time.Second
	}
	deadline := time.Now().Add(check)
	for time.Now().Before(deadline) {
		info, err := r.Podman.InspectContainer(ctx, name)
		if err != nil {
			return err
		}
		if !info.State.Running {
			var lines []string
			_ = r.Podman.Logs(ctx, name, false, 10, func(_, line string) { lines = append(lines, line) })
			msg := fmt.Sprintf("it exited with code %d", info.State.ExitCode)
			if len(lines) > 0 {
				msg += ": " + strings.Join(lines, " | ")
			}
			return fmt.Errorf("%s", msg)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return nil
}

// pull tries three times: registries (Docker Hub in particular) sometimes
// refuse a token request that succeeds a moment later.
func (r Runtime) pull(ctx context.Context, image string) error {
	var err error
	for attempt := range 3 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}
		if err = r.Podman.PullImage(ctx, image, func(string) {}); err == nil {
			return nil
		}
	}
	return err
}

func (r Runtime) removeContainer(ctx context.Context, name string) error {
	if err := r.Podman.StopContainer(ctx, name, 10); err != nil && !podman.IsNotFound(err) {
		return err
	}
	err := r.Podman.RemoveContainer(ctx, name)
	if podman.IsNotFound(err) {
		return nil
	}
	return err
}

// removeOthers removes the Service's Containers whose Component is not in
// keep; nil keep removes every one.
func (r Runtime) removeOthers(ctx context.Context, serviceID uint64, keep []string) error {
	list, err := r.Podman.ListContainers(ctx, labels(serviceID))
	if err != nil {
		return err
	}
	for _, c := range list {
		if slices.Contains(keep, c.Labels["bakery.component"]) {
			continue
		}
		if err := r.removeContainer(ctx, c.ID); err != nil {
			return err
		}
	}
	return nil
}

// Down stops and removes every Container of the Service; the network and
// volumes stay.
func (r Runtime) Down(ctx context.Context, s domain.Service) error {
	return r.removeOthers(ctx, s.ID, nil)
}

// Remove removes the Containers, the network and, when withVolumes is true,
// every volume of the Service with all their data.
func (r Runtime) Remove(ctx context.Context, s domain.Service, withVolumes bool) error {
	if err := r.Down(ctx, s); err != nil {
		return err
	}
	if err := r.Podman.RemoveNetwork(ctx, domain.NetworkName(s.ID)); err != nil || !withVolumes {
		return err
	}
	volumes, err := r.Podman.ListVolumes(ctx, labels(s.ID))
	if err != nil {
		return err
	}
	for _, v := range volumes {
		if err := r.Podman.RemoveVolume(ctx, v.Name); err != nil {
			return err
		}
	}
	return nil
}

// Statuses reads what each Component's Container is doing; details explain
// an exited one.
func (r Runtime) Statuses(ctx context.Context, s domain.Service) (map[string]domain.Status, map[string]string, error) {
	statuses := map[string]domain.Status{}
	details := map[string]string{}
	for _, c := range s.Components {
		info, err := r.Podman.InspectContainer(ctx, domain.ContainerName(s.ID, c.Name))
		switch {
		case podman.IsNotFound(err) && s.DesiredState == domain.Stopped:
			statuses[c.Name] = domain.StatusStopped
		case podman.IsNotFound(err):
			statuses[c.Name] = domain.StatusMissing
		case err != nil:
			return nil, nil, err
		case !info.State.Running:
			statuses[c.Name] = domain.StatusExited
			details[c.Name] = fmt.Sprintf("exited with code %d", info.State.ExitCode)
		default:
			statuses[c.Name] = domain.StatusRunning
		}
	}
	return statuses, details, nil
}

// Logs hands a Component's output to out, each line prefixed with the time
// it was written; with follow until ctx ends or the Container stops. found
// is false when it has no Container.
func (r Runtime) Logs(ctx context.Context, s domain.Service, component string, follow bool, tail int, out func(stream, line string)) (bool, error) {
	err := r.Podman.TimestampedLogs(ctx, domain.ContainerName(s.ID, component), follow, tail, out)
	if podman.IsNotFound(err) {
		return false, nil
	}
	return true, err
}
