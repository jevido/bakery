//go:build podman

package podman

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// build builds a one-layer image from busybox writing content, tagged tag.
func buildImage(t *testing.T, c *Client, tag, content string, labels map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	cf := "FROM docker.io/library/busybox:1.36\nRUN echo " + content + " > /content\n"
	if err := os.WriteFile(filepath.Join(dir, "Containerfile"), []byte(cf), 0o644); err != nil {
		t.Fatal(err)
	}
	id, err := c.Build(context.Background(), TarDir(dir), BuildOptions{Tag: tag, Dockerfile: "Containerfile", Labels: labels}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestPruneOnlyLabelledDanglingImages(t *testing.T) {
	c := client(t)
	ctx := context.Background()
	labels := map[string]string{"bakery.managed": "true", "bakery.test": "true"}
	// Building the same tag twice leaves the first image dangling.
	labelled := buildImage(t, c, "localhost/bakery-test/prune:labelled", "one-"+t.Name(), labels)
	buildImage(t, c, "localhost/bakery-test/prune:labelled", "two-"+t.Name(), labels)
	unlabelled := buildImage(t, c, "localhost/bakery-test/prune:unlabelled", "three-"+t.Name(), nil)
	buildImage(t, c, "localhost/bakery-test/prune:unlabelled", "four-"+t.Name(), nil)
	t.Cleanup(func() {
		for _, ref := range []string{"localhost/bakery-test/prune:labelled", "localhost/bakery-test/prune:unlabelled", unlabelled, labelled} {
			_ = c.RemoveImage(ctx, ref)
		}
	})

	if _, err := c.PruneDanglingImages(ctx, map[string]string{"bakery.test": "true"}); err != nil {
		t.Fatal(err)
	}
	if ok, _ := c.ImageExists(ctx, labelled); ok {
		t.Error("the dangling bakery image survived")
	}
	if ok, _ := c.ImageExists(ctx, unlabelled); !ok {
		t.Error("an unlabelled dangling image was pruned")
	}
	if ok, _ := c.ImageExists(ctx, "localhost/bakery-test/prune:labelled"); !ok {
		t.Error("a tagged bakery image was pruned")
	}
}

func TestRemoveUnusedImageKeepsImagesInUse(t *testing.T) {
	c := client(t)
	ctx := context.Background()
	tag := "localhost/bakery-test/in-use:1"
	buildImage(t, c, tag, "in-use-"+t.Name(), map[string]string{"bakery.managed": "true", "bakery.test": "true"})
	id, err := c.CreateContainer(ctx, ContainerSpec{Name: "bakery-test-in-use", Image: tag, Command: []string{"true"}, Labels: map[string]string{"bakery.test": "true"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = c.RemoveContainer(ctx, id)
		_ = c.RemoveImage(ctx, tag)
	})
	if _, removed, err := c.RemoveUnusedImage(ctx, tag); err != nil || removed {
		t.Fatalf("in use: removed=%v err=%v", removed, err)
	}
	if ok, _ := c.ImageExists(ctx, tag); !ok {
		t.Fatal("an image a container uses was removed")
	}
	if err := c.RemoveContainer(ctx, id); err != nil {
		t.Fatal(err)
	}
	reclaimed, removed, err := c.RemoveUnusedImage(ctx, tag)
	if err != nil || !removed || reclaimed <= 0 {
		t.Fatalf("unused: reclaimed=%d removed=%v err=%v", reclaimed, removed, err)
	}
	if ok, _ := c.ImageExists(ctx, tag); ok {
		t.Fatal("unused image survived")
	}
}
