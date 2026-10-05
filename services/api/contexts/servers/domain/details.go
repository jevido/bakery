package domain

import (
	"strings"
	"time"
)

// Details are a Server's operating system and hardware, read live from
// Podman: what Coolify's Server overview shows.
type Details struct {
	// OS is the distribution and its version, e.g. "debian 12".
	OS     string
	Arch   string
	Kernel string
	CPUs   int
	Memory int64
	// PodmanVersion is the version of the Podman service Bakery talks to.
	PodmanVersion string
	// UpSince is when the Server booted; zero when Podman did not say.
	UpSince time.Time
}

// NewDetails derives the boot time from how long the Server has been up,
// to the second, as of now.
func NewDetails(distribution, version, arch, kernel string, cpus int, memory int64, podmanVersion string, uptime time.Duration, now time.Time) Details {
	d := Details{
		OS:   strings.TrimSpace(strings.TrimSpace(distribution) + " " + strings.TrimSpace(version)),
		Arch: arch, Kernel: kernel, CPUs: cpus, Memory: memory, PodmanVersion: podmanVersion,
	}
	if uptime > 0 {
		d.UpSince = now.Add(-uptime).Truncate(time.Second)
	}
	return d
}
