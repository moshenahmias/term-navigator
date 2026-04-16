//go:build unix || darwin

package platform

import (
	"context"
	"os/exec"
	"syscall"
)

func SetupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func KillProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		pgid, _ := syscall.Getpgid(cmd.Process.Pid)
		syscall.Kill(-pgid, syscall.SIGKILL)
	}
}

func StopOnContext(ctx context.Context, cmd *exec.Cmd) context.CancelFunc {
	_, cancel := context.WithCancel(ctx)
	go func() {
		<-ctx.Done()
		KillProcess(cmd)
	}()
	return cancel
}
