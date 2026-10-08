//go:build !unix

package runner

import "os/exec"

// Without process groups, claude itself is stopped; Windows is not
// packaged yet.
func ownGroup(*exec.Cmd) {}

func terminate(cmd *exec.Cmd) { kill(cmd) }

func kill(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
