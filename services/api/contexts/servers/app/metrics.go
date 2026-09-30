package app

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

// HostInfo is the Server's CPUs, memory and where Podman stores its data.
type HostInfo struct {
	CPUs      int
	MemTotal  int64
	MemFree   int64
	GraphRoot string
}

// PodmanDiskUsage is what Podman's images, containers and volumes take.
type PodmanDiskUsage struct {
	Images, Containers, Volumes int64
}

// ContainerSample is one Bakery Container's use.
type ContainerSample struct {
	Name     string
	Labels   map[string]string
	CPU      float64
	MemUsage int64
	MemLimit int64
}

// Observer is what a Connection gives for metrics.
type Observer interface {
	HostInfo(ctx context.Context) (HostInfo, error)
	// CPUPercent samples the use of all CPUs over a short interval.
	CPUPercent(ctx context.Context) (float64, error)
	// Filesystem is the size and free space of the filesystem path is on.
	Filesystem(ctx context.Context, path string) (total, free int64, err error)
	PodmanDiskUsage(ctx context.Context) (PodmanDiskUsage, error)
	// Containers samples every running Container labelled bakery.managed.
	Containers(ctx context.Context) ([]ContainerSample, error)
}

// ErrUnreachable is returned when a Server cannot be connected to.
type ErrUnreachable struct{ Reason string }

func (e *ErrUnreachable) Error() string { return "server unreachable: " + e.Reason }

const metricsTimeout = 20 * time.Second

// Metrics reads the Server's and its Bakery Containers' use, live.
func (s *Service) Metrics(ctx context.Context, id uint64) (domain.Metrics, error) {
	srv, err := s.Get(ctx, id)
	if err != nil {
		return domain.Metrics{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, metricsTimeout)
	defer cancel()
	conn, err := s.connector.Connect(ctx, srv)
	if err != nil {
		return domain.Metrics{}, &ErrUnreachable{Reason: err.Error()}
	}
	defer conn.Close()
	m, err := readMetrics(ctx, conn, s.now())
	var unreachable *ErrUnreachable
	if err != nil && !errors.As(err, &unreachable) {
		// A part that could not be read stays zero; the rest is still worth
		// showing.
		s.log("servers: metrics of %s: %v", srv.Name, err)
		err = nil
	}
	return m, err
}

func readMetrics(ctx context.Context, o Observer, now time.Time) (domain.Metrics, error) {
	info, err := o.HostInfo(ctx)
	if err != nil {
		return domain.Metrics{}, &ErrUnreachable{Reason: err.Error()}
	}
	m := domain.ServerMetrics{CPUs: info.CPUs, MemTotal: info.MemTotal, MemUsed: info.MemTotal - info.MemFree, ReadAt: now}
	var errs []error
	if m.CPUPercent, err = o.CPUPercent(ctx); err != nil {
		errs = append(errs, err)
	}
	total, free, err := o.Filesystem(ctx, info.GraphRoot)
	if err != nil {
		errs = append(errs, err)
	}
	m.DiskTotal, m.DiskUsed = total, total-free
	du, err := o.PodmanDiskUsage(ctx)
	if err != nil {
		errs = append(errs, err)
	}
	m.Images, m.Containers, m.Volumes = du.Images, du.Containers, du.Volumes
	samples, err := o.Containers(ctx)
	if err != nil {
		errs = append(errs, err)
	}
	out := domain.Metrics{Server: m, Containers: []domain.ContainerMetrics{}}
	for _, c := range samples {
		owner, ownerID := domain.OwnerFromLabels(c.Labels)
		out.Containers = append(out.Containers, domain.ContainerMetrics{
			Name: c.Name, Owner: owner, OwnerID: ownerID, CPUPercent: c.CPU, MemUsed: c.MemUsage, MemLimit: c.MemLimit,
		})
	}
	sort.Slice(out.Containers, func(i, j int) bool { return out.Containers[i].Name < out.Containers[j].Name })
	return out, errors.Join(errs...)
}
