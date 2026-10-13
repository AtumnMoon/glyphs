//go:build !windows

package app

import (
	"os"
	"os/exec"
	"syscall"
)

// detach makes cmd outlive the parent shell/terminal.
func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

// terminate asks the process to stop.
func terminate(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGTERM)
}
