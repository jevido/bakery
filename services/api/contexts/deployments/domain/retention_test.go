package domain

import (
	"fmt"
	"slices"
	"testing"
)

func TestImagesToPrune(t *testing.T) {
	var ds []Deployment
	for i := uint64(1); i <= 7; i++ {
		ds = append(ds, Deployment{ID: i, Status: Finished, Image: fmt.Sprintf("localhost/bakery/app:%d", i)})
	}
	// A failed build that tagged an Image, a queued Rollback to deployment
	// 1, and a Rollback that finished reusing deployment 6's Image.
	ds = append(ds,
		Deployment{ID: 8, Status: Failed, Image: "localhost/bakery/app:8"},
		Deployment{ID: 9, Status: Finished, Image: "localhost/bakery/app:6"},
		Deployment{ID: 10, Status: Queued, Image: "localhost/bakery/app:1"},
		Deployment{ID: 11, Status: Cancelled},
	)
	got := ImagesToPrune(ds)
	slices.Sort(got)
	// Kept: 6 (twice, counts twice), 7, 5, 4 by the five newest finished
	// (9, 7, 6, 5, 4) and 1 by the queued Rollback.
	want := []string{"localhost/bakery/app:2", "localhost/bakery/app:3", "localhost/bakery/app:8"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if len(ImagesToPrune(ds[:5])) != 0 {
		t.Fatal("pruned within retention")
	}
}
