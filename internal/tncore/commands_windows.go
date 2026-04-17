//go:build windows

package tncore

import (
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/moshenahmias/term-navigator/internal/backends/local"
)

func init() {
	commands["logs"] = command{f: func(a *App, args ...string) tea.Cmd {
		if len(args) != 0 {
			return failure("Usage: logs")
		}

		cmd := exec.Command("notepad")
		cmd.Stdin = strings.NewReader(a.logBuffer.String())

		return tea.ExecProcess(cmd, execCheck())
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
