package infra

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

// Git clones with the git binary on the host.
type Git struct{}

// Clone runs a shallow single-branch clone. The URL and branch are
// arguments, never shell text; `--` ends the options so neither can be read
// as one. Only https is allowed, submodules included.
func (Git) Clone(ctx context.Context, url, branch, dir string, out func(stream, line string)) (string, error) {
	cmd := exec.CommandContext(ctx, "git",
		"-c", "protocol.allow=never", "-c", "protocol.https.allow=always",
		"clone", "--depth", "1", "--single-branch", "--branch", branch, "--", url, dir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/bin/false", "GCM_INTERACTIVE=never")
	if err := runStreaming(cmd, out); err != nil {
		return "", err
	}
	rev := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD")
	sha, err := rev.Output()
	if err != nil {
		return "", fmt.Errorf("reading the commit: %w", err)
	}
	return strings.TrimSpace(string(sha)), nil
}

// runStreaming runs cmd, handing stdout and stderr to out line by line, and
// turns a non-zero exit into an error carrying the last stderr line.
func runStreaming(cmd *exec.Cmd, out func(stream, line string)) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	var wg sync.WaitGroup
	var lastErr string
	var mu sync.Mutex
	read := func(r io.Reader, stream string) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			line := sc.Text()
			if stream == domain.StreamErr && strings.TrimSpace(line) != "" {
				mu.Lock()
				lastErr = strings.TrimSpace(line)
				mu.Unlock()
			}
			out(stream, line)
		}
	}
	wg.Add(2)
	go read(stdout, domain.StreamOut)
	go read(stderr, domain.StreamErr)
	wg.Wait()
	if err := cmd.Wait(); err != nil {
		if lastErr != "" {
			return fmt.Errorf("%s", strings.TrimPrefix(lastErr, "fatal: "))
		}
		return err
	}
	return nil
}
