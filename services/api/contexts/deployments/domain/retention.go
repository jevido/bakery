package domain

import "sort"

// ImageRetention is how many finished Deployments per Application keep
// their Image, the running one included, so a Rollback to any of them
// works without a build.
const ImageRetention = 5

// PreviewImageRetention is how many finished Deployments of an open Preview
// keep their Image: the one it runs. Previews cannot be rolled back.
const PreviewImageRetention = 1

// ImagesToPrune returns the Images of an Application's Deployments that
// Image retention lets go: every Image that is not the Image of one of the
// newest ImageRetention finished Deployments of the Application itself, of
// the newest finished Deployment of each Preview in openPreviews (a closed
// Preview keeps none), nor of an active Deployment (a queued Rollback names
// the Image it is about to start).
func ImagesToPrune(deployments []Deployment, openPreviews map[int]bool) []string {
	sorted := append([]Deployment(nil), deployments...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID > sorted[j].ID })
	keep := map[string]bool{}
	finished := map[int]int{}
	for _, d := range sorted {
		if d.Image == "" {
			continue
		}
		if d.Status.Active() {
			keep[d.Image] = true
		}
		limit := ImageRetention
		if d.Preview != 0 {
			limit = 0
			if openPreviews[d.Preview] {
				limit = PreviewImageRetention
			}
		}
		if d.Status == Finished && finished[d.Preview] < limit {
			keep[d.Image] = true
			finished[d.Preview]++
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
