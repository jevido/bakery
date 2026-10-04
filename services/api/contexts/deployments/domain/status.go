package domain

// ContainerState is the state Podman reports for one of an Application's
// own Containers: running, restarting, created (not started yet), exited
// and so on.
type ContainerState string

// Health is what the Healthcheck says of a running Application.
type Health string

const (
	Healthy   Health = "healthy"
	Unhealthy Health = "unhealthy"
	// UnknownHealth is a running Application without a Healthcheck.
	UnknownHealth Health = "unknown"
)

// ApplicationStatus is Coolify's status string for an Application:
// running:healthy, running:unhealthy, running:unknown, restarting, degraded
// (some of its Containers run, others stopped) or exited (none run, or it
// has none).
type ApplicationStatus string

const (
	StatusExited     ApplicationStatus = "exited"
	StatusRestarting ApplicationStatus = "restarting"
	StatusDegraded   ApplicationStatus = "degraded:unhealthy"
)

// StatusOf is the Application's status from the states of its own
// Containers and, when one runs, the health of the newest running one.
// A Container that is still being created counts for nothing: the next
// Deployment's Container is only one while it starts.
func StatusOf(states []ContainerState, health Health) ApplicationStatus {
	var running, restarting, stopped int
	for _, s := range states {
		switch s {
		case "running":
			running++
		case "restarting":
			restarting++
		case "created", "configured", "initialized":
		default:
			stopped++
		}
	}
	switch {
	case running > 0 && stopped == 0 && restarting == 0:
		return ApplicationStatus("running:" + string(health))
	case running > 0:
		return StatusDegraded
	case restarting > 0:
		return StatusRestarting
	}
	return StatusExited
}

// Running reports whether the status is one of the running ones.
func (s ApplicationStatus) Running() bool {
	return len(s) > 8 && s[:8] == "running:"
}
