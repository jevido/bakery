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
	got := ImagesToPrune(ds, nil)
	slices.Sort(got)
	// Kept: 6 (twice, counts twice), 7, 5, 4 by the five newest finished
	// (9, 7, 6, 5, 4) and 1 by the queued Rollback.
	want := []string{"localhost/bakery/app:2", "localhost/bakery/app:3", "localhost/bakery/app:8"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if len(ImagesToPrune(ds[:5], nil)) != 0 {
		t.Fatal("pruned within retention")
	}
}

func TestImagesToPruneWithPreviews(t *testing.T) {
	ds := []Deployment{
		{ID: 1, Status: Finished, Image: "app:1"},
		{ID: 2, Preview: 7, Status: Finished, Image: "app:2"},
		{ID: 3, Preview: 7, Status: Finished, Image: "app:3"},
		{ID: 4, Preview: 8, Status: Finished, Image: "app:4"},
		{ID: 5, Preview: 7, Status: Building, Image: "app:5"},
		{ID: 6, Status: Finished, Image: "app:6"},
	}
	got := ImagesToPrune(ds, map[int]bool{7: true, 8: false})
	slices.Sort(got)
	// Production keeps both (well under five), open #7 its newest finished
	// and the active one, closed #8 nothing.
	if want := []string{"app:2", "app:4"}; !slices.Equal(got, want) {
		t.Fatalf("pruned %v, want %v", got, want)
	}
}
