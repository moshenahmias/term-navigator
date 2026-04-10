package tncore

import (
	"bufio"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (a *App) applyBatch() tea.Cmd {
	return a.runBatchInner(toBatch...)
}

func (a *App) runBatch(path string) tea.Cmd {
	f, err := os.Open(path)

	if err != nil {
		return check(err)
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)

	var lines []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return check(err)
	}

	return a.runBatchInner(lines...)
}

func (a *App) showBatch(lines ...string) tea.Cmd {
	cmd := exec.Command("less")
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n"))
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return check(err)()
		}

		return batchMsg{lines}
	})
}

func (a *App) runBatchInner(lines ...string) tea.Cmd {
	var cmds []tea.Cmd

	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue // skip empty lines and comments
		}

		args := strings.Fields(line)

		if len(args) == 0 {
			continue
		}

		if args[0] == "batch" {
			return failure("nested batching is not supported")
		}

		cmd, exists := a.commands[args[0]]

		if !exists {
			return failuref("unknown command %s", args[0])
		}

		argsCopy := append([]string(nil), args[1:]...)
		cmdCopy := cmd

		cmds = append(cmds, func() tea.Msg {
			c := cmdCopy.f(a, argsCopy...)

			if c == nil {
				return nil
			}

			return c()
		})
	}

	if len(cmds) > 0 {
		cmds = append(cmds, func() tea.Msg {
			a.left.refresh()
			a.right.refresh()
			return nil
		})
	}

	return tea.Sequence(cmds...)
}
