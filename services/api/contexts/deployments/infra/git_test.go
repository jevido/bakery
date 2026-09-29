package infra

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadCommit(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "a"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", "a")
	long := strings.Repeat("x", 300)
	git("-c", "user.name=Jane Doe", "-c", "user.email=jane@example.com", "commit", "-q", "-m", "Fix the login "+long+"\n\nBody text")

	c, err := readCommit(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.SHA) != 40 || c.Author != "Jane Doe" {
		t.Fatalf("commit: %+v", c)
	}
	if !strings.HasPrefix(c.Subject, "Fix the login x") || len([]rune(c.Subject)) != maxSubject || strings.Contains(c.Subject, "Body") {
		t.Fatalf("subject %q (%d runes)", c.Subject, len([]rune(c.Subject)))
	}
}

func TestSSHCommandAndMessages(t *testing.T) {
	cmd := sshCommand("/tmp/bakery-ssh-1/deploy_key", "/tmp/bakery-ssh-1/known_hosts")
	for _, want := range []string{"-F /dev/null", "-i '/tmp/bakery-ssh-1/deploy_key'", "IdentitiesOnly=yes", "BatchMode=yes",
		"UserKnownHostsFile='/tmp/bakery-ssh-1/known_hosts'", "StrictHostKeyChecking=accept-new", "HashKnownHosts=no"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("ssh command lacks %q: %s", want, cmd)
		}
	}
	if got := shellQuote("a'b"); got != `'a'\''b'` {
		t.Errorf("shellQuote: %s", got)
	}
	for raw, want := range map[string]string{
		"git@github.com:me/app.git":           "github.com",
		"ssh://git@127.0.0.1:4952/me/app.git": "127.0.0.1:4952",
	} {
		if got := sshHost(raw); got != want {
			t.Errorf("sshHost(%q) = %q", raw, got)
		}
	}
	if !strings.Contains(sshFailure("git@github.com: Permission denied (publickey)."), "deploy key") {
		t.Error("publickey refusal not recognised")
	}
	if !strings.Contains(sshFailure("Host key verification failed."), "changed") {
		t.Error("host key change not recognised")
	}
	if sshFailure("Cloning into 'x'...") != "" {
		t.Error("ordinary line taken for a failure")
	}
}
