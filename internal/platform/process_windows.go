//go:build windows

package platform

import (
	"context"
	"os/exec"
)

func SetupProcessGroup(cmd *exec.Cmd) {
	// No process group setup needed on Windows
}

func KillProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		cmd.Process.Kill()
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
