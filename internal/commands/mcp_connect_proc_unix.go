//go:build !windows

package commands

import (
	"os/exec"
	"syscall"
)

// startInOwnGroup puts the client CLI in its own process group, so stopping
// it also stops anything it launched (e.g. an npm-installed codex is a Node
// wrapper around the native binary).
func startInOwnGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killGroup kills the client CLI and everything in its process group.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
