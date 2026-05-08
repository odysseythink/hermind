//go:build !windows

package rl

import (
	"os/exec"
	"syscall"
)

// applyProcGroup puts the child in its own process group so we can signal
// the whole group later. Required because `cmd` often forks further (e.g.,
// uv or python wrappers) and a naked SIGTERM on the parent would orphan
// descendants.
func applyProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateGroup(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
}

func killGroup(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
