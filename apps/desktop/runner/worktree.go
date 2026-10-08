package runner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jevido/bakery/apps/desktop/bakery"
)

// How long a Worktree may go without a Run before the Runner removes it, and
// how often it looks.
var (
	worktreeIdle  = 14 * 24 * time.Hour
	worktreeSweep = 24 * time.Hour
)

// worktrees keeps the git checkouts Runs work in, under home:
// repos/<sha256(repository)[:16]>.git, one bare clone per repository, and
// worktrees/<sha256(address)[:8]>/<issue id>, one Worktree per Bakery and
// Issue, kept between that Issue's Runs. git runs with the person's own
// credentials from the inherited environment; nothing comes from the server.
type worktrees struct {
	home string
	// git is the git binary; empty means `git` on PATH.
	git string

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// lock serialises git on one clone, for two Agents on the same repository.
func (w *worktrees) lock(clone string) func() {
	w.mu.Lock()
	if w.locks == nil {
		w.locks = map[string]*sync.Mutex{}
	}
	l := w.locks[clone]
	if l == nil {
		l = &sync.Mutex{}
		w.locks[clone] = l
	}
	w.mu.Unlock()
	l.Lock()
	return l.Unlock
}

func shortHash(s string, n int) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:n]
}

func (w *worktrees) clonePath(repository string) string {
	return filepath.Join(w.home, "repos", shortHash(repository, 16)+".git")
}

func (w *worktrees) path(address string, issueID uint64) string {
	return filepath.Join(w.home, "worktrees", shortHash(address, 8), strconv.FormatUint(issueID, 10))
}

// prepare answers the Worktree for the Issue issueID of the Bakery at
// address: the repository's clone made or fetched, then the Worktree made
// on the Agent branch (from origin's Agent branch when it exists there,
// else from the base branch) or, when an earlier Run made it, reused as it
// is. Its mtime is touched so prune keeps it.
func (w *worktrees) prepare(ctx context.Context, address string, issueID uint64, ws bakery.RunWorkspace) (string, error) {
	clone := w.clonePath(ws.Repository)
	defer w.lock(clone)()
	if _, err := os.Stat(filepath.Join(clone, "HEAD")); err != nil {
		if err := os.MkdirAll(filepath.Dir(clone), 0o700); err != nil {
			return "", err
		}
		_ = os.RemoveAll(clone)
		if err := w.run(ctx, "", "clone", "--quiet", "--bare", ws.Repository, clone); err != nil {
			_ = os.RemoveAll(clone)
			return "", err
		}
		// A bare clone fetches nothing into refs/remotes; this makes
		// origin/<branch> exist and stay current.
		if err := w.run(ctx, clone, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
			return "", err
		}
	}
	if err := w.run(ctx, clone, "fetch", "--quiet", "--prune", "origin"); err != nil {
		return "", err
	}
	dir := w.path(address, issueID)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		// Not a Worktree (yet): whatever is left of one goes first.
		_ = os.RemoveAll(dir)
		_ = w.run(ctx, clone, "worktree", "prune")
		if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
			return "", err
		}
		from := "origin/" + ws.BaseBranch
		if w.run(ctx, clone, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+ws.Branch) == nil {
			from = "origin/" + ws.Branch
		}
		if err := w.run(ctx, clone, "worktree", "add", "--no-track", "-B", ws.Branch, dir, from); err != nil {
			return "", err
		}
	}
	now := time.Now()
	if err := os.Chtimes(dir, now, now); err != nil {
		return "", err
	}
	return dir, nil
}

// prune removes the Worktrees no Run touched for idle, with their clone's
// `git worktree remove --force` and `git worktree prune`.
func (w *worktrees) prune(ctx context.Context, now time.Time, idle time.Duration) []error {
	dirs, _ := filepath.Glob(filepath.Join(w.home, "worktrees", "*", "*"))
	var errs []error
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() || now.Sub(info.ModTime()) < idle {
			continue
		}
		clone := cloneOf(dir)
		if clone == "" {
			if err := os.RemoveAll(dir); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		unlock := w.lock(clone)
		if err := w.run(ctx, clone, "worktree", "remove", "--force", dir); err != nil {
			errs = append(errs, err)
			_ = os.RemoveAll(dir)
		}
		_ = w.run(ctx, clone, "worktree", "prune")
		unlock()
	}
	return errs
}

// cloneOf is the clone a Worktree belongs to, read from its .git file
// ("gitdir: <clone>/worktrees/<name>"), or "" when it has none.
func cloneOf(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, ".git"))
	if err != nil {
		return ""
	}
	gitdir, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir:")
	if !ok {
		return ""
	}
	return filepath.Dir(filepath.Dir(strings.TrimSpace(gitdir)))
}

// run runs git with args in dir (none when ""), never through a shell and
// never waiting on a credential prompt. A failure carries git's stderr,
// trimmed to 500 characters.
func (w *worktrees) run(ctx context.Context, dir string, args ...string) error {
	git := w.git
	if git == "" {
		git = "git"
	}
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.CommandContext(ctx, git, args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		var exitErr *exec.ExitError
		if msg == "" || !errors.As(err, &exitErr) {
			msg = strings.TrimSpace(err.Error() + " " + msg)
		}
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return fmt.Errorf("git %s: %s", gitVerb(args), msg)
	}
	return nil
}

// gitVerb is git's subcommand in args, for the error: "clone", "worktree add".
func gitVerb(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "-C" {
			i++
			continue
		}
		if args[i] == "worktree" && i+1 < len(args) {
			return "worktree " + args[i+1]
		}
		return args[i]
	}
	return ""
}
