package ui

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/moshenahmias/term-navigator/internal/backends/local"
	"github.com/moshenahmias/term-navigator/internal/config"
	"github.com/moshenahmias/term-navigator/internal/file"
)

var (
	allButParentDirItemSuggestionsFilter = func(fi *FileItem) bool {
		return !fi.isParentDir()
	}

	dirsOnlyItemSuggestionsFilter = func(fi *FileItem) bool {
		return fi.Info.IsDir
	}

	filesOnlyItemSuggestionsFilter = func(fi *FileItem) bool {
		return !fi.Info.IsDir
	}
)

type command struct {
	f           func(a *App, args ...string) tea.Cmd
	aliases     []string
	suggestions func(*App, string) []string
}

var (
	commands = map[string]command{
		"switch": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) != 0 {
				return failure("Usage: switch")
			}

			a.focus = 1 - a.focus
			a.left.SetActive(a.focus == 0)
			a.right.SetActive(a.focus == 1)

			return nil
		}},
		"logs": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) != 0 {
				return failure("Usage: logs")
			}

			cmd := exec.Command("less")
			cmd.Stdin = strings.NewReader(a.logBuffer.String())

			return tea.ExecProcess(cmd, execCheck())
		}},
		"help": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) != 0 {
				return failure("Usage: help")
			}
			_, cmd := a.runHelp()
			return cmd
		}},
		"rename": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) != 2 {
				return failure("Usage: rename <old> <new>")
			}

			active := a.activePane()

			return a.applyRenameInner(active, args[0], args[1])
		}, suggestions: func(a *App, s string) []string {
			return a.generateItemSuggestions(s, allButParentDirItemSuggestionsFilter)
		}},
		"view": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) != 1 {
				return failure("Usage: view <filename>")
			}

			active := a.activePane()
			_, cmd := a.runViewInner(active, args[0])
			return cmd
		}, suggestions: func(a *App, s string) []string {
			return a.generateItemSuggestions(s, filesOnlyItemSuggestionsFilter)
		}},
		"edit": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) != 1 {
				return failure("Usage: edit <filename>")
			}

			active := a.activePane()
			_, cmd := a.runEditInner(active, args[0])
			return cmd
		}, suggestions: func(a *App, s string) []string {
			return a.generateItemSuggestions(s, filesOnlyItemSuggestionsFilter)
		}},
		"copy": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) != 2 {
				return failure("Usage: copy <src> <dest>")
			}

			src := a.activePane()
			dst := a.left

			if src == a.left {
				dst = a.right
			}

			from := src.explorer.Join(src.explorer.Cwd(a.ctx), args[0])

			a.runAsyncJob(func(name string, n, total int64) string {
				if total < 1 {
					return fmt.Sprintf("Copied %s of %q", bytesFormatter(n), name)
				}
				return fmt.Sprintf("Copied %s/%s of %q", bytesFormatter(n), bytesFormatter(total), name)
			}, func(ctx context.Context, progress file.ProgressFunc) tea.Msg {
				return a.applyCopyInner(a.ctx, src, dst, from, args[1], progress)()
			})

			return nil
		}, aliases: []string{"cp"}, suggestions: func(a *App, s string) []string {
			return a.generateItemSuggestions(s, allButParentDirItemSuggestionsFilter)
		}},
		"move": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) != 2 {
				return failure("Usage: move <src> <dest>")
			}

			src := a.activePane()
			dst := a.left

			if src == a.left {
				dst = a.right
			}

			from := src.explorer.Join(src.explorer.Cwd(a.ctx), args[0])

			a.runAsyncJob(func(name string, n, total int64) string {
				if total < 1 {
					return fmt.Sprintf("Moved %s of %q", bytesFormatter(n), name)
				}
				return fmt.Sprintf("Moved %s/%s of %q", bytesFormatter(n), bytesFormatter(total), name)
			}, func(ctx context.Context, progress file.ProgressFunc) tea.Msg {
				return a.applyMoveInner(ctx, src, dst, from, args[1], progress)()
			})

			return nil
		}, aliases: []string{"mv"}, suggestions: func(a *App, s string) []string {
			return a.generateItemSuggestions(s, allButParentDirItemSuggestionsFilter)
		}},
		"mkdir": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) != 1 {
				return failure("Usage: mkdir <name>")
			}

			return a.applyMakeDir(args[0])
		}},
		"delete": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) != 1 {
				return failure("Usage: delete <name>")
			}

			return a.applyDeleteInner(a.activePane(), args[0])
		}, aliases: []string{"del"}, suggestions: func(a *App, s string) []string {
			return a.generateItemSuggestions(s, allButParentDirItemSuggestionsFilter)
		}},
		"info": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) != 1 {
				return failure("Usage: info <filename>")
			}

			active := a.activePane()
			_, cmd := a.runMetadataInner(active, args[0])
			return cmd
		}, suggestions: func(a *App, s string) []string {
			return a.generateItemSuggestions(s, allButParentDirItemSuggestionsFilter)
		}},
		"device": {f: func(a *App, args ...string) tea.Cmd {
			if len(a.devs) < 2 {
				return nil
			}

			if len(args) != 1 {
				return failure("Usage: device <name>")
			}

			if a.activePane().name == args[0] {
				return nil
			}

			return a.applyChangeDevice(args[0])
		}, aliases: []string{"dev"}, suggestions: func(a *App, s string) (sugg []string) {
			for d := range a.devs {
				sugg = append(sugg, fmt.Sprintf("%s %s", s, d))
			}
			return
		}},
		"swap": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) != 0 {
				return failure("Usage: swap")
			}

			_, cmd := a.runSwapDevices()
			return cmd
		}},
		"exit": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) != 0 {
				return failure("Usage: exit")
			}

			return tea.Quit
		}, aliases: []string{"quit", "bye"}},
		"config": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) != 0 {
				return failure("Usage: config")
			}

			path, err := config.Path()
			if err != nil {
				return check(err)
			}

			cmd := execDefaultEditor(path, true)

			return tea.ExecProcess(cmd, execResolve("Restart required for changes to take effect"))
		}, aliases: []string{"cfg"}},
		"exec": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) == 0 {
				return failure("Usage: exec <command>")
			}

			pane := a.activePane()

			if pane.explorer.Type() != local.Type {
				return failuref("exec works only for local devices")
			}

			cmd := exec.Command(args[0], args[1:]...)
			cmd.Dir = pane.explorer.Cwd(a.ctx)
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out

			return tea.ExecProcess(cmd, func(err error) tea.Msg {
				if err != nil {
					return check(err)()
				}

				lines := strings.Split(out.String(), "\n")

				for i := range lines {
					lines[i] = strings.TrimSpace(lines[i])
				}

				a.refreshPanesForExplorer(pane.explorer)

				return NewLongStatusMsg(lines...)
			})
		}},
		"refresh": {f: func(a *App, args ...string) tea.Cmd {
			a.left.refresh()
			a.right.refresh()
			return nil
		}},
		"cd": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) != 1 {
				return failure("Usage: cd <folder>")
			}

			active := a.activePane()

			var path string

			if args[0] == parentDirName {
				if parent, ok := active.explorer.Parent(a.ctx); ok {
					path = parent
				} else {
					return nil
				}
			} else {
				path = active.explorer.Join(active.explorer.Cwd(a.ctx), args[0])
			}

			if err := active.explorer.Chdir(a.ctx, path); err != nil {
				return check(err)
			}

			active.refresh()

			return nil
		}, suggestions: func(a *App, s string) []string {
			return a.generateItemSuggestions(s, dirsOnlyItemSuggestionsFilter)
		}},
		"shell": {f: func(a *App, args ...string) tea.Cmd {
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
		}},
		"batch": {f: func(a *App, args ...string) tea.Cmd {
			if len(args) != 1 {
				return func() tea.Msg {
					return newErrorMsg("Usage: batch <file>")
				}
			}

			pane := a.activePane()
			from := pane.explorer.Join(pane.explorer.Cwd(a.ctx), args[0])

			handle, err := pane.explorer.Download(a.ctx, from, nil)
			if err != nil {
				return check(err)
			}

			return func() tea.Msg {
				m := a.runBatch(handle.Path())()

				if err := handle.Close(); err != nil {
					return check(err)()
				}

				return m
			}
		}, suggestions: func(a *App, s string) []string {
			return a.generateItemSuggestions(s, filesOnlyItemSuggestionsFilter)
		}},
	}
	commandAlias = make(map[string]string)
)

func init() {
	for name, cmd := range commands {
		for _, alias := range cmd.aliases {
			commandAlias[alias] = name
		}
	}
}

func (a *App) runCommand() (tea.Model, tea.Cmd) {
	a.inputMode = inputCommand
	a.textbox.SetValue("")
	a.setCommandSuggestions("")
	a.textbox.Placeholder = ""
	a.textbox.Focus()

	return a, nil
}

func (a *App) generateItemSuggestions(text string, filter func(*FileItem) bool) []string {
	var suggestions []string

	if active := a.activePane(); active != nil {
		for _, item := range active.list.Items() {
			fi := item.(*FileItem)
			if filter == nil || filter(fi) {
				suggestions = append(suggestions, fmt.Sprintf("%s %s", text, fi.Info.Name))
			}
		}
	}

	return suggestions
}

func (a *App) setCommandSuggestions(text string) {
	if text == "" {
		var suggestions []string

		for name, cmd := range commands {
			suggestions = append(suggestions, name)
			for _, alias := range cmd.aliases {
				suggestions = append(suggestions, alias)
			}
		}

		a.textbox.SetSuggestions(suggestions)
	} else {
		name := text

		if s, exists := commandAlias[text]; exists {
			name = s
		}

		if cmd, ok := commands[name]; ok {
			if cmd.suggestions != nil {
				a.textbox.SetSuggestions(cmd.suggestions(a, text))
			}
		}
	}
}

func (a *App) applyCommand(text string) tea.Cmd {
	if text == "" {
		return nil
	}

	args := strings.Fields(text)

	if len(args) == 0 {
		return nil
	}

	name := args[0]

	if s, exists := commandAlias[name]; exists {
		name = s
	}

	if cmd, exists := commands[name]; exists {
		return cmd.f(a, args[1:]...)
	}

	return failure(fmt.Sprintf("Unknown command: %q", args[0]))
}

func (a *App) runBatch(path string) tea.Cmd {
	f, err := os.Open(path)

	if err != nil {
		return check(err)
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)

	var cmds []tea.Cmd

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
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

	if err := scanner.Err(); err != nil {
		return check(err)
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
