package infra

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/servers/app"
)

// cpuSampleInterval is how long CPU use is measured over.
const cpuSampleInterval = 500 * time.Millisecond

// observer reads metrics through a Server's Podman API; what libpod does
// not report (current CPU use, filesystem size) comes from shell.
type observer struct {
	client *podman.Client
	// readProc returns a /proc file of the Server (not of a container).
	readProc func(ctx context.Context, path string) (string, error)
	// filesystem returns the size and free space of path's filesystem.
	filesystem func(ctx context.Context, path string) (int64, int64, error)
}

func (o observer) HostInfo(ctx context.Context) (app.HostInfo, error) {
	info, err := o.client.Info(ctx)
	if err != nil {
		return app.HostInfo{}, err
	}
	h := app.HostInfo{CPUs: info.CPUs, MemTotal: info.MemTotal, MemFree: info.MemFree, GraphRoot: info.GraphRoot}
	// Podman's memFree leaves out the page cache the kernel gives back on
	// demand, so a busy server would always look full; MemAvailable does not.
	if raw, err := o.readProc(ctx, "/proc/meminfo"); err == nil {
		if avail, ok := memAvailable(raw); ok {
			h.MemFree = avail
		}
	}
	return h, nil
}

func (o observer) procStat(ctx context.Context) (string, error) {
	raw, err := o.readProc(ctx, "/proc/stat")
	line, _, _ := strings.Cut(raw, "\n")
	return line, err
}

// memAvailable reads MemAvailable (in kB) from /proc/meminfo, in bytes.
func memAvailable(meminfo string) (int64, bool) {
	for _, line := range strings.Split(meminfo, "\n") {
		if rest, ok := strings.CutPrefix(line, "MemAvailable:"); ok {
			f := strings.Fields(rest)
			if len(f) == 0 {
				return 0, false
			}
			kb, err := strconv.ParseInt(f[0], 10, 64)
			return kb * 1024, err == nil
		}
	}
	return 0, false
}

func (o observer) CPUPercent(ctx context.Context) (float64, error) {
	first, err := o.procStat(ctx)
	if err != nil {
		return 0, err
	}
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-time.After(cpuSampleInterval):
	}
	second, err := o.procStat(ctx)
	if err != nil {
		return 0, err
	}
	return cpuBetween(first, second)
}

func (o observer) Filesystem(ctx context.Context, path string) (int64, int64, error) {
	return o.filesystem(ctx, path)
}

func (o observer) PodmanDiskUsage(ctx context.Context) (app.PodmanDiskUsage, error) {
	du, err := o.client.DiskUsage(ctx)
	return app.PodmanDiskUsage{Images: du.Images, Containers: du.Containers, Volumes: du.Volumes}, err
}

func (o observer) Containers(ctx context.Context) ([]app.ContainerSample, error) {
	list, err := o.client.ListContainers(ctx, map[string]string{"bakery.managed": "true"})
	if err != nil {
		return nil, err
	}
	var ids []string
	labels := map[string]map[string]string{}
	for _, c := range list {
		if c.State == "running" {
			ids = append(ids, c.ID)
			labels[c.ID] = c.Labels
		}
	}
	stats, err := o.client.Stats(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]app.ContainerSample, len(stats))
	for i, s := range stats {
		out[i] = app.ContainerSample{Name: s.Name, Labels: labels[s.ID], CPU: s.CPU, MemUsage: s.MemUsage, MemLimit: s.MemLimit}
	}
	return out, nil
}

// cpuTimes parses the aggregate "cpu" line of /proc/stat into busy and
// total jiffies (idle and iowait count as not busy).
func cpuTimes(line string) (busy, total uint64, err error) {
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, fmt.Errorf("servers: unexpected /proc/stat line %q", line)
	}
	for i, f := range fields[1:] {
		// guest and guest_nice (fields 9 and 10) are already in user and nice.
		if i >= 8 {
			break
		}
		n, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("servers: /proc/stat: %w", err)
		}
		total += n
		if i != 3 && i != 4 {
			busy += n
		}
	}
	return busy, total, nil
}

func cpuBetween(first, second string) (float64, error) {
	b1, t1, err := cpuTimes(first)
	if err != nil {
		return 0, err
	}
	b2, t2, err := cpuTimes(second)
	if err != nil {
		return 0, err
	}
	if t2 <= t1 || b2 < b1 {
		return 0, nil
	}
	return float64(b2-b1) / float64(t2-t1) * 100, nil
}

// localProc reads a /proc file; /proc/stat and /proc/meminfo are the
// host's even inside the API container.
func localProc(_ context.Context, path string) (string, error) {
	raw, err := os.ReadFile(path)
	return string(raw), err
}

// localFilesystem statfs's path. Inside the API container the host's graph
// root is not mounted, but the container's own root filesystem lives in
// it, so "/" is the same filesystem.
func localFilesystem(_ context.Context, path string) (int64, int64, error) {
	var st syscall.Statfs_t
	err := syscall.Statfs(path, &st)
	if errors.Is(err, syscall.ENOENT) && inContainer() {
		err = syscall.Statfs("/", &st)
	}
	if err != nil {
		return 0, 0, err
	}
	return int64(st.Blocks) * st.Bsize, int64(st.Bavail) * st.Bsize, nil
}

// parseDF reads the size and available bytes from `df -P -B1 <path>`.
func parseDF(out string) (int64, int64, error) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return 0, 0, fmt.Errorf("servers: unexpected df output %q", out)
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 4 {
		return 0, 0, fmt.Errorf("servers: unexpected df output %q", out)
	}
	total, err1 := strconv.ParseInt(fields[1], 10, 64)
	avail, err2 := strconv.ParseInt(fields[3], 10, 64)
	if err := errors.Join(err1, err2); err != nil {
		return 0, 0, fmt.Errorf("servers: df: %w", err)
	}
	return total, avail, nil
}

// shellQuote quotes s for a POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
