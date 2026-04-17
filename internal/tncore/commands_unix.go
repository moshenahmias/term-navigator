//go:build unix || darwin

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

		cmd := exec.Command("less", "+G")
		cmd.Stdin = strings.NewReader(a.logBuffer.String())

		return tea.ExecProcess(cmd, execCheck())
	}}

	commands["shell"] = command{f: func(a *App, args ...string) tea.Cmd {
		if len(args) != 0 {
			return func() tea.Msg {
				return newErrorMsg("Usage: shell")
			}
		}

		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}

		cmd := exec.Command(shell)

		pane := a.activePane()

		if pane.explorer.Type() == local.Type {
			cmd.Dir = a.activePane().explorer.Cwd(a.ctx)
		}

		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return tea.ExecProcess(cmd, execResolve("Returned from shell"))
	}}
}
