package podman

import (
	"context"
	"net/http"
	"net/url"
)

// Info is what Bakery reads from the service's /info: the host's CPUs and
// memory, and where Podman keeps its storage.
type Info struct {
	CPUs int
	// CPUIdlePercent is the host's idle CPU since boot, as Podman reports it.
	CPUIdlePercent float64
	MemTotal       int64
	MemFree        int64
	GraphRoot      string
}

func (c *Client) Info(ctx context.Context) (Info, error) {
	var out struct {
		Host struct {
			CPUs           int   `json:"cpus"`
			MemTotal       int64 `json:"memTotal"`
			MemFree        int64 `json:"memFree"`
			CPUUtilization struct {
				IdlePercent float64 `json:"idlePercent"`
			} `json:"cpuUtilization"`
		} `json:"host"`
		Store struct {
			GraphRoot string `json:"graphRoot"`
		} `json:"store"`
	}
	if err := c.call(ctx, http.MethodGet, "/info", nil, nil, &out); err != nil {
		return Info{}, err
	}
	return Info{
		CPUs: out.Host.CPUs, CPUIdlePercent: out.Host.CPUUtilization.IdlePercent,
		MemTotal: out.Host.MemTotal, MemFree: out.Host.MemFree, GraphRoot: out.Store.GraphRoot,
	}, nil
}

// DiskUsage is the space Podman's images, containers and volumes take, in
// bytes, from /system/df.
type DiskUsage struct {
	Images     int64
	Containers int64
	Volumes    int64
}

func (c *Client) DiskUsage(ctx context.Context) (DiskUsage, error) {
	var out struct {
		ImagesSize int64 `json:"ImagesSize"`
		Containers []struct {
			RWSize int64 `json:"RWSize"`
		} `json:"Containers"`
		Volumes []struct {
			Size int64 `json:"Size"`
		} `json:"Volumes"`
	}
	if err := c.call(ctx, http.MethodGet, "/system/df", nil, nil, &out); err != nil {
		return DiskUsage{}, err
	}
	d := DiskUsage{Images: out.ImagesSize}
	for _, ct := range out.Containers {
		d.Containers += ct.RWSize
	}
	for _, v := range out.Volumes {
		d.Volumes += v.Size
	}
	return d, nil
}

// ContainerStats is one container's CPU and memory use.
type ContainerStats struct {
	ID   string
	Name string
	// CPU is the percentage of one CPU, as `podman stats` shows it.
	CPU      float64
	MemUsage int64
	MemLimit int64
}

// Stats reads one sample for each of the running containers named by id.
func (c *Client) Stats(ctx context.Context, ids []string) ([]ContainerStats, error) {
	if len(ids) == 0 {
		// Without containers the API answers for every running one.
		return nil, nil
	}
	q := url.Values{"stream": {"false"}, "containers": ids}
	var out struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"Error"`
		Stats []struct {
			ContainerID string  `json:"ContainerID"`
			Name        string  `json:"Name"`
			CPU         float64 `json:"CPU"`
			MemUsage    int64   `json:"MemUsage"`
			MemLimit    int64   `json:"MemLimit"`
		} `json:"Stats"`
	}
	if err := c.call(ctx, http.MethodGet, "/containers/stats", q, nil, &out); err != nil {
		return nil, err
	}
	if out.Error != nil && out.Error.Message != "" {
		return nil, &Error{Status: http.StatusInternalServerError, Message: out.Error.Message}
	}
	stats := make([]ContainerStats, len(out.Stats))
	for i, s := range out.Stats {
		stats[i] = ContainerStats{ID: s.ContainerID, Name: s.Name, CPU: s.CPU, MemUsage: s.MemUsage, MemLimit: s.MemLimit}
	}
	return stats, nil
}
