//go:build windows

package rl

import "os/exec"

// applyProcGroup is a no-op on Windows — process groups work differently
// (job objects / CREATE_NEW_PROCESS_GROUP) and we don't need them for the
// single-process subprocesses hermind's RL manager spawns.
func applyProcGroup(_ *exec.Cmd) {}

// Windows has no SIGTERM; Kill is the only portable stop mechanism. We use
// it for both the graceful and forced paths.
func terminateGroup(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}

func killGroup(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
