//go:build windows

package tncore

import (
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"
	"github.com/moshenahmias/term-navigator/internal/backends/local"
)

func init() {
	commands["logs"] = command{f: func(a *App, args ...string) tea.Cmd {
		if len(args) != 0 {
			return failure("Usage: logs")
		}
		tmp, _ := os.CreateTemp("", "logs-*.txt")
		tmp.WriteString(a.logBuffer.String())
		tmp.Close()

		cmd := exec.Command("notepad.exe", tmp.Name())

		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			// Delete immediately after launching
			os.Remove(tmp.Name())
			return execCheck()(err)
		})
	}}

	commands["cmd"] = command{f: func(a *App, args ...string) tea.Cmd {
		if len(args) != 0 {
			return func() tea.Msg {
				return newErrorMsg("Usage: cmd")
			}
		}

		cmd := exec.Command("cmd", "/k")

		pane := a.activePane()

		if pane.explorer.Type() == local.Type {
			cmd.Dir = a.activePane().explorer.Cwd(a.ctx)
		}

		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return tea.ExecProcess(cmd, execResolve("Returned from cmd"))
	}}
}
