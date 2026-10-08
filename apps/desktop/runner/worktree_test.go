package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jevido/bakery/apps/desktop/bakery"
)

// gitIn runs git in dir and answers its trimmed output, failing the test
// when git fails.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// isolateGit keeps the person's own git config out of the tests and gives
// git an identity to commit with.
func isolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Tester")
	t.Setenv("GIT_AUTHOR_EMAIL", "tester@example.test")
	t.Setenv("GIT_COMMITTER_NAME", "Tester")
	t.Setenv("GIT_COMMITTER_EMAIL", "tester@example.test")
}

// originRepo is a bare repository on main with one commit (README.md), and
// its file:// URL.
func originRepo(t *testing.T) (dir, url string) {
	t.Helper()
	isolateGit(t)
	root := t.TempDir()
	dir = filepath.Join(root, "origin.git")
	gitIn(t, root, "init", "--bare", "-b", "main", dir)
	seed := filepath.Join(root, "seed")
	gitIn(t, root, "clone", dir, seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("# App\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, seed, "checkout", "-b", "main")
	gitIn(t, seed, "add", "README.md")
	gitIn(t, seed, "commit", "-m", "first")
	gitIn(t, seed, "push", "origin", "main")
	return dir, "file://" + dir
}

func workspace(url, branch string) bakery.RunWorkspace {
	return bakery.RunWorkspace{Application: bakery.Named{ID: 5, Name: "web"}, Repository: url, BaseBranch: "main", Branch: branch}
}

func TestWorktreeIsMadeAndReusedForTheIssue(t *testing.T) {
	_, url := originRepo(t)
	w := &worktrees{home: t.TempDir()}
	ctx := context.Background()
	dir, err := w.prepare(ctx, "https://bakery.test", 1, workspace(url, "bakery/def-1"))
	if err != nil {
		t.Fatal(err)
	}
	if got := gitIn(t, dir, "branch", "--show-current"); got != "bakery/def-1" {
		t.Fatalf("on %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); err != nil {
		t.Fatal("the base branch's README is not there:", err)
	}

	// The first Run commits and pushes, and leaves something uncommitted.
	if err := os.WriteFile(filepath.Join(dir, "fix.txt"), []byte("fixed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "fix.txt")
	gitIn(t, dir, "commit", "-m", "fix")
	gitIn(t, dir, "push", "origin", "bakery/def-1")
	if err := os.WriteFile(filepath.Join(dir, "wip.txt"), []byte("half\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	again, err := w.prepare(ctx, "https://bakery.test", 1, workspace(url, "bakery/def-1"))
	if err != nil {
		t.Fatal(err)
	}
	if again != dir {
		t.Fatalf("second Run in %s, first in %s", again, dir)
	}
	for _, f := range []string{"fix.txt", "wip.txt"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("%s is gone on the second Run: %v", f, err)
		}
	}
	if got := gitIn(t, dir, "log", "-1", "--format=%s", "origin/bakery/def-1"); got != "fix" {
		t.Fatalf("origin/bakery/def-1 is at %q", got)
	}

	// Another Bakery's Issue 1 is another Worktree.
	other, err := w.prepare(ctx, "https://other.test", 1, workspace(url, "bakery/xyz-1"))
	if err != nil {
		t.Fatal(err)
	}
	if other == dir {
		t.Fatal("two Bakeries share a Worktree")
	}
}

func TestWorktreeStartsFromTheAgentBranchOnOrigin(t *testing.T) {
	origin, url := originRepo(t)
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	gitIn(t, filepath.Dir(elsewhere), "clone", origin, elsewhere)
	gitIn(t, elsewhere, "checkout", "-b", "bakery/def-2")
	if err := os.WriteFile(filepath.Join(elsewhere, "earlier.txt"), []byte("from an earlier laptop\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, elsewhere, "add", "earlier.txt")
	gitIn(t, elsewhere, "commit", "-m", "earlier")
	gitIn(t, elsewhere, "push", "origin", "bakery/def-2")

	w := &worktrees{home: t.TempDir()}
	dir, err := w.prepare(context.Background(), "https://bakery.test", 2, workspace(url, "bakery/def-2"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "earlier.txt")); err != nil {
		t.Fatal("the Agent branch's commit is not there:", err)
	}
}

func TestWorktreeOfAnUnreachableRepositoryFails(t *testing.T) {
	isolateGit(t)
	w := &worktrees{home: t.TempDir()}
	missing := "file://" + filepath.Join(t.TempDir(), "missing.git")
	_, err := w.prepare(context.Background(), "https://bakery.test", 3, workspace(missing, "bakery/def-3"))
	if err == nil || !strings.HasPrefix(err.Error(), "git clone: ") || len(err.Error()) <= len("git clone: ") {
		t.Fatalf("err %v", err)
	}
	t.Log(err)
	if _, err := os.Stat(w.clonePath(missing)); !os.IsNotExist(err) {
		t.Fatal("a failed clone was left behind")
	}
}

func TestPruneRemovesIdleWorktrees(t *testing.T) {
	_, url := originRepo(t)
	w := &worktrees{home: t.TempDir()}
	ctx := context.Background()
	idle, err := w.prepare(ctx, "https://bakery.test", 4, workspace(url, "bakery/def-4"))
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := w.prepare(ctx, "https://bakery.test", 5, workspace(url, "bakery/def-5"))
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-15 * 24 * time.Hour)
	if err := os.Chtimes(idle, old, old); err != nil {
		t.Fatal(err)
	}
	if errs := w.prune(ctx, time.Now(), 14*24*time.Hour); len(errs) > 0 {
		t.Fatal(errs)
	}
	if _, err := os.Stat(idle); !os.IsNotExist(err) {
		t.Fatal("the idle Worktree is still there:", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatal("the fresh Worktree is gone:", err)
	}
	list := gitIn(t, w.clonePath(url), "worktree", "list")
	if strings.Contains(list, idle) || !strings.Contains(list, fresh) {
		t.Fatalf("worktree list:\n%s", list)
	}
	// The idle Issue's next Run makes it again.
	if _, err := w.prepare(ctx, "https://bakery.test", 4, workspace(url, "bakery/def-4")); err != nil {
		t.Fatal(err)
	}
}
