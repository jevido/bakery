package infra

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jevido/bakery/services/api/contexts/deployments/app"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

// Git clones with the git binary where the API runs. SSH Git repositories are
// cloned with the Application's Deploy key and the Known hosts from KnownHosts.
type Git struct {
	KnownHosts app.KnownHosts
}

// Clone runs a shallow single-branch clone. The URL and branch are
// arguments, never shell text; `--` ends the options so neither can be read
// as one. Only https is allowed for an https URL and only ssh for an SSH
// URL, submodules included.
func (g Git) Clone(ctx context.Context, req app.CloneRequest, out func(stream, line string)) (app.Commit, error) {
	protocol := "https"
	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/bin/false", "GCM_INTERACTIVE=never")
	var knownHostsFile, storedLines string
	if req.DeployKey != "" {
		protocol = "ssh"
		dir, err := os.MkdirTemp("", "bakery-ssh-")
		if err != nil {
			return app.Commit{}, err
		}
		defer os.RemoveAll(dir)
		keyFile := filepath.Join(dir, "deploy_key")
		knownHostsFile = filepath.Join(dir, "known_hosts")
		if storedLines, err = g.KnownHosts.Lines(ctx); err != nil {
			return app.Commit{}, fmt.Errorf("reading the known hosts: %w", err)
		}
		key := req.DeployKey
		if !strings.HasSuffix(key, "\n") {
			key += "\n" // ssh refuses a key file without a final newline
		}
		if err := os.WriteFile(keyFile, []byte(key), 0o600); err != nil {
			return app.Commit{}, err
		}
		if err := os.WriteFile(knownHostsFile, []byte(storedLines), 0o600); err != nil {
			return app.Commit{}, err
		}
		env = append(env, "GIT_SSH_COMMAND="+sshCommand(keyFile, knownHostsFile), "GIT_SSH_VARIANT=ssh")
	}
	cmd := exec.CommandContext(ctx, "git",
		"-c", "protocol.allow=never", "-c", "protocol."+protocol+".allow=always",
		"clone", "--depth", "1", "--single-branch", "--branch", req.Branch, "--", req.URL, req.Dir)
	cmd.Env = env
	var sshProblem string
	err := runStreaming(cmd, func(stream, line string) {
		if p := sshFailure(line); p != "" && sshProblem == "" {
			sshProblem = p
		}
		out(stream, line)
	})
	if req.DeployKey != "" && err == nil {
		// accept-new wrote the keys of a host seen for the first time.
		if b, rerr := os.ReadFile(knownHostsFile); rerr == nil && string(b) != storedLines {
			if rerr := g.KnownHosts.Remember(ctx, string(b)); rerr != nil {
				return app.Commit{}, fmt.Errorf("remembering the host key: %w", rerr)
			}
			out(domain.StreamInfo, "Trusted the SSH host key of "+sshHost(req.URL)+" on first use")
		}
	}
	if err != nil {
		if sshProblem != "" {
			return app.Commit{}, errors.New(strings.ReplaceAll(sshProblem, "<host>", sshHost(req.URL)))
		}
		return app.Commit{}, err
	}
	return readCommit(ctx, req.Dir)
}

// sshCommand is run by git through a shell, hence the quoting. -F /dev/null
// keeps the system and user ssh config (agents, other keys, hashed
// known_hosts) out of it; accept-new trusts an unknown host once and refuses
// a changed key.
func sshCommand(keyFile, knownHostsFile string) string {
	return strings.Join([]string{
		"ssh", "-F", "/dev/null", "-i", shellQuote(keyFile),
		"-o", "IdentitiesOnly=yes", "-o", "IdentityAgent=none", "-o", "BatchMode=yes",
		"-o", "UserKnownHostsFile=" + shellQuote(knownHostsFile), "-o", "GlobalKnownHostsFile=/dev/null",
		"-o", "StrictHostKeyChecking=accept-new", "-o", "HashKnownHosts=no", "-o", "ConnectTimeout=15",
	}, " ")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// sshFailure turns the ssh line that explains a failed clone into a reason
// the Owner can act on, or returns "".
func sshFailure(line string) string {
	switch {
	case strings.Contains(line, "REMOTE HOST IDENTIFICATION HAS CHANGED"), strings.Contains(line, "Host key verification failed"):
		return "the SSH host key of <host> changed since the first clone; if that is expected, forget the host in The Bakery's settings and deploy again"
	case strings.Contains(line, "Permission denied (publickey"):
		return "the repository refused the deploy key; add the public key shown on the application page to the repository's deploy keys"
	}
	return ""
}

// sshHost is the host part of an SSH URL, for messages.
func sshHost(raw string) string {
	if strings.HasPrefix(raw, "ssh://") {
		if u, err := url.Parse(raw); err == nil {
			return u.Host
		}
	}
	if _, rest, ok := strings.Cut(raw, "@"); ok {
		host, _, _ := strings.Cut(rest, ":")
		return host
	}
	return raw
}

// maxSubject caps the stored commit subject; the full message stays in git.
const maxSubject = 200

// readCommit reads HEAD's SHA, author and subject. The subject comes last:
// it is the only one that may be empty.
func readCommit(ctx context.Context, dir string) (app.Commit, error) {
	b, err := exec.CommandContext(ctx, "git", "-C", dir, "log", "-1", "--format=%H%n%an%n%s").Output()
	if err != nil {
		return app.Commit{}, fmt.Errorf("reading the commit: %w", err)
	}
	parts := strings.SplitN(strings.TrimRight(string(b), "\n"), "\n", 3)
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	subject := strings.TrimSpace(parts[2])
	if r := []rune(subject); len(r) > maxSubject {
		subject = string(r[:maxSubject-1]) + "…"
	}
	return app.Commit{SHA: strings.TrimSpace(parts[0]), Author: strings.TrimSpace(parts[1]), Subject: subject}, nil
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
