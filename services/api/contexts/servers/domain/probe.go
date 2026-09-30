package domain

// HealthChange is what a Server probe found changed about a Server.
type HealthChange string

const (
	BecameUnreachable HealthChange = "unreachable"
	BecameReachable   HealthChange = "reachable"
	BecameAlmostFull  HealthChange = "disk_almost_full"
)

const (
	// FailedProbesForUnreachable is how many probes in a row must fail
	// before a Reachable Server counts as Unreachable.
	FailedProbesForUnreachable = 2
	// DiskAlmostFullPercent sets Disk almost full; it is cleared below
	// DiskClearPercent, so a disk hovering at the mark does not flap.
	DiskAlmostFullPercent = 90
	DiskClearPercent      = 85
)

// Probed says whether the Server probe checks the Server: one whose latest
// Validation passed. A Server that failed its Validation stays as that
// Validation left it until it is validated again; an unvalidated one has
// nothing to compare with.
func (s Server) Probed() bool {
	return s.Validation.Passed() && (s.Status == Reachable || s.Status == Unreachable)
}

// RecordProbe records one Server probe: whether it reached the Server and,
// when it did, how full its disk is. It returns what changed, which is
// what is worth telling anyone.
func (s *Server) RecordProbe(reached bool, diskUsed, diskTotal int64) []HealthChange {
	if !s.Probed() {
		return nil
	}
	var changes []HealthChange
	if !reached {
		s.FailedProbes++
		if s.Status == Reachable && s.FailedProbes >= FailedProbesForUnreachable {
			s.Status = Unreachable
			changes = append(changes, BecameUnreachable)
		}
		return changes
	}
	s.FailedProbes = 0
	if s.Status == Unreachable {
		s.Status = Reachable
		changes = append(changes, BecameReachable)
	}
	if diskTotal > 0 {
		used := diskUsed * 100 / diskTotal
		switch {
		case !s.DiskAlmostFull && used >= DiskAlmostFullPercent:
			s.DiskAlmostFull = true
			changes = append(changes, BecameAlmostFull)
		case s.DiskAlmostFull && used < DiskClearPercent:
			s.DiskAlmostFull = false
		}
	}
	return changes
}
