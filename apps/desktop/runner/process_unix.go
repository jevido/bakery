//go:build unix

package runner

import (
	"os/exec"
	"syscall"
)

// ownGroup starts claude in its own process group, so a cancel stops the
// tools it started too.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminate asks claude's process group to stop.
func terminate(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
}

// kill stops claude's process group at once.
func kill(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
