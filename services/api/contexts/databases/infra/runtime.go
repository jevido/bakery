package infra

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/databases/domain"
)

// Runtime runs Database Containers with Podman. A Database that should run
// has a Container; one a Member stopped has none (its data stays in the
// volume), so podman-restart.service, which starts every container with
// restart policy always after a reboot, cannot bring it back by accident.
type Runtime struct {
	Podman  *podman.Client
	Network string
	// PublicBind is the host address Public ports are published on; empty
	// is every interface.
	PublicBind string
}

func (r Runtime) labels(id uint64) map[string]string {
	return map[string]string{"bakery.managed": "true", "bakery.database": strconv.FormatUint(id, 10)}
}

// Start pulls the image if it is missing, creates the volume and starts the
// Container. It does not wait for the Database to answer; Status tells.
func (r Runtime) Start(ctx context.Context, d domain.Database) error {
	image := d.Image()
	exists, err := r.Podman.ImageExists(ctx, image)
	if err != nil {
		return err
	}
	if !exists {
		if err := r.pull(ctx, image); err != nil {
			return fmt.Errorf("pulling %s: %w", image, err)
		}
	}
	spec := d.Type.Spec()
	volume := domain.VolumeName(d.ID)
	if err := r.Podman.CreateVolume(ctx, volume, r.labels(d.ID)); err != nil {
		return fmt.Errorf("creating volume %s: %w", volume, err)
	}
	name := domain.ContainerName(d.Slug)
	var ports []podman.PortMapping
	if d.PublicPort != 0 {
		ports = []podman.PortMapping{{HostIP: r.PublicBind, HostPort: uint16(d.PublicPort), ContainerPort: uint16(spec.Port), Protocol: "tcp"}}
	}
	if _, err := r.Podman.CreateContainer(ctx, podman.ContainerSpec{
		Name:           name,
		Image:          image,
		Command:        d.Command(),
		Env:            d.Env(),
		Labels:         r.labels(d.ID),
		Networks:       podman.OnNetwork(r.Network),
		PortMappings:   ports,
		Volumes:        []podman.NamedVolume{{Name: volume, Dest: spec.DataPath}},
		RestartPolicy:  "always",
		ResourceLimits: podman.Limits(d.ResourceLimits.MemoryMB, d.ResourceLimits.CPUs),
	}); err != nil {
		return err
	}
	if err := r.Podman.StartContainer(ctx, name); err != nil {
		_ = r.removeContainer(name)
		return err
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

// Stop stops and removes the Container; the volume stays.
func (r Runtime) Stop(ctx context.Context, d domain.Database) error {
	name := domain.ContainerName(d.Slug)
	if err := r.Podman.StopContainer(ctx, name, 10); err != nil && !podman.IsNotFound(err) {
		return err
	}
	return r.removeContainer(name)
}

func (r Runtime) removeContainer(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := r.Podman.RemoveContainer(ctx, name)
	if podman.IsNotFound(err) {
		return nil
	}
	return err
}

// Recreate replaces the Container with one made from the current settings,
// on the same volume.
func (r Runtime) Recreate(ctx context.Context, d domain.Database) error {
	if err := r.Stop(ctx, d); err != nil {
		return err
	}
	return r.Start(ctx, d)
}

// Remove removes the Container and then, when volume is true, the volume
// with all its data.
func (r Runtime) Remove(ctx context.Context, d domain.Database, volume bool) error {
	if err := r.Stop(ctx, d); err != nil || !volume {
		return err
	}
	return r.Podman.RemoveVolume(ctx, domain.VolumeName(d.ID))
}

// probeTimeout bounds one readiness probe.
const probeTimeout = 5 * time.Second

// Status reads what the Database's Container is doing. detail explains an
// exited Container (its exit code).
func (r Runtime) Status(ctx context.Context, d domain.Database) (status domain.Status, detail string, err error) {
	info, err := r.Podman.InspectContainer(ctx, domain.ContainerName(d.Slug))
	switch {
	case podman.IsNotFound(err) && d.DesiredState == domain.Stopped:
		return domain.StatusStopped, "", nil
	case podman.IsNotFound(err):
		return domain.StatusMissing, "", nil
	case err != nil:
		return "", "", err
	case !info.State.Running:
		return domain.StatusExited, fmt.Sprintf("exited with code %d", info.State.ExitCode), nil
	}
	pctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	code, _, err := r.Podman.Exec(pctx, domain.ContainerName(d.Slug), d.ReadinessProbe())
	if err != nil && ctx.Err() != nil {
		return "", "", ctx.Err()
	}
	if err != nil || code != 0 {
		return domain.StatusStarting, "", nil
	}
	return domain.StatusRunning, "", nil
}

// Logs hands the Container's output to out, each line prefixed with the
// time it was written; with follow until ctx ends or the Container stops.
// found is false when it has no Container.
func (r Runtime) Logs(ctx context.Context, d domain.Database, follow bool, tail int, out func(stream, line string)) (bool, error) {
	err := r.Podman.TimestampedLogs(ctx, domain.ContainerName(d.Slug), follow, tail, out)
	if podman.IsNotFound(err) {
		return false, nil
	}
	return true, err
}

// Dump runs the Database's DumpCommand in its Container, streaming stdout
// to w.
func (r Runtime) Dump(ctx context.Context, d domain.Database, w io.Writer) (int, string, error) {
	return r.Podman.ExecStream(ctx, domain.ContainerName(d.Slug), d.DumpCommand(), w)
}

// CopyIn streams a file into the Container's RestoreDir.
func (r Runtime) CopyIn(ctx context.Context, d domain.Database, name string, size int64, rd io.Reader) error {
	return r.Podman.CopyFileInto(ctx, domain.ContainerName(d.Slug), d.RestoreDir(), name, size, rd)
}

// Exec runs cmd in the Container.
func (r Runtime) Exec(ctx context.Context, d domain.Database, cmd []string) (int, string, error) {
	return r.Podman.Exec(ctx, domain.ContainerName(d.Slug), cmd)
}
