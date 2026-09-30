package domain

import "sort"

// ImageRetention is how many finished Deployments per Application keep
// their Image, the running one included, so a Rollback to any of them
// works without a build.
const ImageRetention = 5

// ImagesToPrune returns the Images of an Application's Deployments that
// Image retention lets go: every Image that is not the Image of one of the
// newest ImageRetention finished Deployments, nor of an active Deployment
// (a queued Rollback names the Image it is about to start).
func ImagesToPrune(deployments []Deployment) []string {
	sorted := append([]Deployment(nil), deployments...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID > sorted[j].ID })
	keep := map[string]bool{}
	finished := 0
	for _, d := range sorted {
		if d.Image == "" {
			continue
		}
		if d.Status.Active() {
			keep[d.Image] = true
		}
		if d.Status == Finished && finished < ImageRetention {
			keep[d.Image] = true
			finished++
		}
	}
	var prune []string
	seen := map[string]bool{}
	for _, d := range sorted {
		if d.Image != "" && !keep[d.Image] && !seen[d.Image] {
			seen[d.Image] = true
			prune = append(prune, d.Image)
		}
	}
	return prune
}
