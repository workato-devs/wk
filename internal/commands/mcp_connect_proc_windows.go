//go:build windows

package commands

import "os/exec"

// startInOwnGroup is a no-op on Windows; see the Unix version.
func startInOwnGroup(cmd *exec.Cmd) {}

// killGroup kills the client CLI. Windows has no process groups to signal, so
// a child it launched may outlive it; codexAdd closes its end of the output
// pipe afterwards, so a surviving child can't keep wk waiting.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
