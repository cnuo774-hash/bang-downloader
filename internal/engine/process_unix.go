//go:build !windows

package engine

import (
	"errors"
	"os/exec"
	"syscall"
)

type processGuard struct{}

func configureProcess(cmd *exec.Cmd) (*processGuard, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &processGuard{}, nil
}

func (g *processGuard) attach(cmd *exec.Cmd) error { return nil }

func (g *processGuard) kill(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
