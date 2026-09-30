package domain

import "time"

// ServerMetrics is what a Server uses, read live.
type ServerMetrics struct {
	CPUs int
	// CPUPercent is the share of all CPUs in use, 0–100.
	CPUPercent float64
	MemUsed    int64
	MemTotal   int64
	// DiskUsed and DiskTotal are of the filesystem Podman's storage is on.
	DiskUsed  int64
	DiskTotal int64
	// What Podman's images, containers and volumes take of it.
	Images     int64
	Containers int64
	Volumes    int64
	ReadAt     time.Time
}

// ContainerMetrics is what one Bakery Container uses. Owner says whose it
// is ("application", "database", "service", "proxy" or ""), OwnerID which
// one, both from its labels.
type ContainerMetrics struct {
	Name       string
	Owner      string
	OwnerID    string
	CPUPercent float64
	MemUsed    int64
	MemLimit   int64
}

// Metrics is a Server's metrics with those of its Bakery Containers.
type Metrics struct {
	Server     ServerMetrics
	Containers []ContainerMetrics
}

// OwnerFromLabels names whose a Container is from its bakery.* labels.
func OwnerFromLabels(labels map[string]string) (owner, id string) {
	for _, o := range []string{"application", "database", "service"} {
		if v := labels["bakery."+o]; v != "" {
			return o, v
		}
	}
	if labels["bakery.role"] == "proxy" {
		return "proxy", ""
	}
	return "", ""
}
