package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type memPreviews struct {
	mu    sync.Mutex
	items []domain.Preview
}

func (m *memPreviews) Save(_ context.Context, p domain.Preview) (domain.Preview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, q := range m.items {
		if q.ApplicationID == p.ApplicationID && q.Number == p.Number {
			p.ID = q.ID
			m.items[i] = p
			return p, nil
		}
	}
	p.ID = uint64(len(m.items) + 1)
	m.items = append(m.items, p)
	return p, nil
}

func (m *memPreviews) ByNumber(_ context.Context, applicationID uint64, number int) (domain.Preview, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.items {
		if p.ApplicationID == applicationID && p.Number == number {
			return p, true, nil
		}
	}
	return domain.Preview{}, false, nil
}

func (m *memPreviews) ByApplication(_ context.Context, applicationID uint64) ([]domain.Preview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Preview
	for _, p := range m.items {
		if p.ApplicationID == applicationID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *memPreviews) DeleteForApplication(_ context.Context, applicationID uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var keep []domain.Preview
	for _, p := range m.items {
		if p.ApplicationID != applicationID {
			keep = append(keep, p)
		}
	}
	m.items = keep
	return nil
}

// recordingSource remembers the branch each clone asked for.
type recordingSource struct {
	fakeSource
	branches *[]string
}

func (r recordingSource) Clone(ctx context.Context, req CloneRequest, out func(string, string)) (Commit, error) {
	*r.branches = append(*r.branches, req.Branch)
	return r.fakeSource.Clone(ctx, req, out)
}

func TestPreviewDeploymentRunsBesideTheApplication(t *testing.T) {
	ctx := context.Background()
	var branches []string
	s := newSetup(t, recordingSource{branches: &branches})
	s.app = func(a *Application) { a.Storages = []Storage{{Name: "data", MountPath: "/data"}} }
	s.runtime.running["bakery-app-1-0"] = true // production
	s.previews.Save(ctx, domain.Preview{ApplicationID: 1, Number: 7, Branch: "feature", State: domain.PreviewOpen})

	// Production and the Preview may both wait in the queue.
	if _, err := s.service.Deploy(ctx, 1); err != nil {
		t.Fatal(err)
	}
	d, err := s.service.DeployPreview(ctx, 1, 7, domain.TriggerWebhook)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.service.DeployPreview(ctx, 1, 7, domain.TriggerWebhook); !errors.Is(err, domain.ErrAlreadyQueued) {
		t.Fatalf("second preview deploy while queued: %v", err)
	}
	s.worker.RunOnce(ctx) // production, replaces bakery-app-1-0
	s.worker.RunOnce(ctx) // the preview

	got, _ := s.service.Deployment(ctx, d.ID)
	if got.Status != domain.Finished || got.Branch != "feature" || got.Container != "bakery-app-1-pr7-2" {
		t.Fatalf("preview deployment %+v\n%s", got, s.logs.text())
	}
	if branches[1] != "feature" {
		t.Errorf("cloned %v", branches)
	}
	if s.routes["pr-7.whoami.localhost"] != "bakery-app-1-pr7-2" || s.routes["whoami.localhost"] != "bakery-app-1-1" {
		t.Fatalf("routes %v", s.routes)
	}
	if !s.runtime.running["bakery-app-1-1"] || !s.runtime.running["bakery-app-1-pr7-2"] {
		t.Fatalf("containers %v", s.runtime.running)
	}
	spec := s.runtime.specs[1]
	if spec.Preview != 7 || spec.Mounts[0].Volume != "bakery-app-1-pr7-data" {
		t.Errorf("preview spec %+v", spec)
	}
	if b := s.runtime.builds[1]; b.Labels["bakery.preview"] != "7" {
		t.Errorf("build labels %v", b.Labels)
	}
	if !strings.Contains(s.logs.text(), "Deploying the preview of pull request #7 (feature)") {
		t.Errorf("log:\n%s", s.logs.text())
	}

	// A production redeploy leaves the Preview alone, and the reverse.
	s.service.Deploy(ctx, 1)
	s.worker.RunOnce(ctx)
	if !s.runtime.running["bakery-app-1-pr7-2"] || s.runtime.running["bakery-app-1-1"] {
		t.Fatalf("after production redeploy %v", s.runtime.running)
	}
	s.service.DeployPreview(ctx, 1, 7, domain.TriggerManual)
	s.worker.RunOnce(ctx)
	if s.runtime.running["bakery-app-1-pr7-2"] || !s.runtime.running["bakery-app-1-3"] || !s.runtime.running["bakery-app-1-pr7-4"] {
		t.Fatalf("after preview redeploy %v", s.runtime.running)
	}

	// No Rollback to a Preview Deployment.
	if _, err := s.service.Rollback(ctx, d.ID); !errors.Is(err, domain.ErrPreviewRollback) {
		t.Fatalf("rollback to a preview deployment: %v", err)
	}
}

func TestClosedPreviewFailsItsDeployment(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{})
	s.previews.Save(ctx, domain.Preview{ApplicationID: 1, Number: 3, Branch: "x", State: domain.PreviewOpen})
	d, err := s.service.DeployPreview(ctx, 1, 3, domain.TriggerWebhook)
	if err != nil {
		t.Fatal(err)
	}
	p, _, _ := s.previews.ByNumber(ctx, 1, 3)
	p.State = domain.PreviewClosed
	s.previews.Save(ctx, p)
	if _, err := s.service.DeployPreview(ctx, 1, 3, domain.TriggerManual); !errors.Is(err, domain.ErrPreviewClosed) {
		t.Fatalf("deploying a closed preview: %v", err)
	}
	s.worker.RunOnce(ctx)
	got, _ := s.service.Deployment(ctx, d.ID)
	if got.Status != domain.Failed || !strings.Contains(got.Error, "closed") {
		t.Fatalf("deployment of a closed preview: %+v", got)
	}
	if len(s.runtime.specs) != 0 {
		t.Fatalf("started %v", s.runtime.specs)
	}
	if _, err := s.service.DeployPreview(ctx, 1, 99, domain.TriggerManual); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown preview: %v", err)
	}
}

func TestImageApplicationsHaveNoPreviews(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{})
	s.app = func(a *Application) { a.BuildPack = BuildPackImage }
	s.previews.Save(ctx, domain.Preview{ApplicationID: 1, Number: 3, Branch: "x", State: domain.PreviewOpen})
	if _, err := s.service.DeployPreview(ctx, 1, 3, domain.TriggerManual); !errors.Is(err, ErrNoPreviews) {
		t.Fatalf("preview of an image application: %v", err)
	}
}

func TestClosePreviewRemovesWhatItRan(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{})
	var dropped []int
	s.service.DropPreviewRoute = func(_ context.Context, _ uint64, n int) error {
		dropped = append(dropped, n)
		return nil
	}
	s.previews.Save(ctx, domain.Preview{ApplicationID: 1, Number: 7, Branch: "f", State: domain.PreviewOpen})
	s.service.Deploy(ctx, 1)
	s.worker.RunOnce(ctx)
	first, _ := s.service.DeployPreview(ctx, 1, 7, domain.TriggerWebhook)
	s.worker.RunOnce(ctx)
	second, _ := s.service.DeployPreview(ctx, 1, 7, domain.TriggerWebhook) // left queued

	if err := s.service.ClosePreview(ctx, 1, 7); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.service.Deployment(ctx, second.ID); got.Status != domain.Cancelled {
		t.Fatalf("queued preview deployment: %s", got.Status)
	}
	if p, _, _ := s.previews.ByNumber(ctx, 1, 7); p.State != domain.PreviewClosed {
		t.Fatalf("preview %+v", p)
	}
	if len(dropped) != 1 || dropped[0] != 7 {
		t.Fatalf("dropped routes %v", dropped)
	}
	if s.runtime.running["bakery-app-1-pr7-2"] || !s.runtime.running["bakery-app-1-1"] {
		t.Fatalf("containers %v", s.runtime.running)
	}
	img, _ := s.service.Deployment(ctx, first.ID)
	if !s.runtime.gone[img.Image] {
		t.Fatalf("image %s of the preview kept", img.Image)
	}
	if prod, _ := s.service.Deployment(ctx, 1); s.runtime.gone[prod.Image] {
		t.Fatal("the application's own image was removed")
	}
	// Twice is harmless.
	if err := s.service.ClosePreview(ctx, 1, 7); err != nil {
		t.Fatal(err)
	}
	if err := s.service.ClosePreview(ctx, 1, 99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown preview: %v", err)
	}
}
