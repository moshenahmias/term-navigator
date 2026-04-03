package ui

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/moshenahmias/term-navigator/internal/config"
	"github.com/moshenahmias/term-navigator/internal/file"
)

type command struct {
	f       func(a *App, args []string) tea.Cmd
	aliases []string
}

var (
	commands = map[string]command{
		"help": {f: func(a *App, args []string) tea.Cmd {
			if len(args) != 0 {
				return failure("Usage: help")
			}
			_, cmd := a.runHelp()
			return cmd
		}},
		"rename": {f: func(a *App, args []string) tea.Cmd {
			if len(args) != 2 {
				return failure("Usage: rename <old> <new>")
			}

			active := a.activePane()

			return a.applyRenameInner(active, args[0], args[1])
		}},
		"view": {f: func(a *App, args []string) tea.Cmd {
			if len(args) != 1 {
				return failure("Usage: view <filename>")
			}

			active := a.activePane()
			_, cmd := a.runViewInner(active, args[0])
			return cmd
		}},
		"edit": {f: func(a *App, args []string) tea.Cmd {
			if len(args) != 1 {
				return failure("Usage: edit <filename>")
			}

			active := a.activePane()
			_, cmd := a.runEditInner(active, args[0])
			return cmd
		}},
		"copy": {f: func(a *App, args []string) tea.Cmd {
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
				return fmt.Sprintf("Copied %s/%s of %q", bytesFormatter(n), bytesFormatter(total), name)
			}, func(ctx context.Context, progress file.ProgressFunc) tea.Msg {
				return a.applyCopyInner(a.ctx, src, dst, from, args[1], progress)()
			})

			return nil
		}, aliases: []string{"cp"}},
		"move": {f: func(a *App, args []string) tea.Cmd {
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
				return fmt.Sprintf("Moved %s/%s of %q", bytesFormatter(n), bytesFormatter(total), name)
			}, func(ctx context.Context, progress file.ProgressFunc) tea.Msg {
				return a.applyMoveInner(ctx, src, dst, from, args[1], progress)()
			})

			return nil
		}, aliases: []string{"mv"}},
		"mkdir": {f: func(a *App, args []string) tea.Cmd {
			if len(args) != 1 {
				return failure("Usage: mkdir <name>")
			}

			return a.applyMakeDir(args[0])
		}},
		"delete": {f: func(a *App, args []string) tea.Cmd {
			if len(args) != 1 {
				return failure("Usage: delete <name>")
			}

			return a.applyDeleteInner(a.activePane(), args[0])
		}, aliases: []string{"del"}},
		"info": {f: func(a *App, args []string) tea.Cmd {
			if len(args) != 1 {
				return failure("Usage: info <filename>")
			}

			active := a.activePane()
			_, cmd := a.runMetadataInner(active, args[0])
			return cmd
		}},
		"device": {f: func(a *App, args []string) tea.Cmd {
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
		}, aliases: []string{"dev"}},
		"swap": {f: func(a *App, args []string) tea.Cmd {
			if len(args) != 0 {
				return failure("Usage: swap")
			}

			_, cmd := a.runSwapDevices()
			return cmd
		}},
		"exit": {f: func(a *App, args []string) tea.Cmd {
			if len(args) != 0 {
				return failure("Usage: exit")
			}

			return tea.Quit
		}, aliases: []string{"quit", "bye"}},
		"config": {f: func(a *App, args []string) tea.Cmd {
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
		"exec": {f: func(a *App, args []string) tea.Cmd {
			if len(args) == 0 {
				return failure("Usage: exec <command>")
			}

			cmd := exec.Command(args[0], args[1:]...)
			var out bytes.Buffer
			cmd.Stdout = &out

			return tea.ExecProcess(cmd, func(err error) tea.Msg {
				if err != nil {
					return check(err)
				}
				msg := strings.ReplaceAll(out.String(), "\r\n", " ")
				msg = strings.ReplaceAll(msg, "\r", " ")
				msg = strings.ReplaceAll(msg, "\n", " ")
				return newStatusMsg(msg)
			})
		}},
		"refresh": {f: func(a *App, args []string) tea.Cmd {
			a.left.refresh()
			a.right.refresh()
			return nil
		}},
		"cd": {f: func(a *App, args []string) tea.Cmd {
			if len(args) != 1 {
				return failure("Usage: cd <folder>")
			}

			active := a.activePane()

			var path string

			if args[0] == ".." {
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
		}},
		"shell": {f: func(a *App, args []string) tea.Cmd {
			if len(args) != 1 {
				return func() tea.Msg {
					return newErrorMsg("Usage: shell")
				}
			}

			shell := os.Getenv("SHELL")
			if shell == "" {
				shell = "/bin/sh"
			}

			cmd := exec.Command(shell)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			return tea.ExecProcess(cmd, execResolve("Returned from shell"))
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

	suggestions := slices.Clone(slices.Collect(maps.Keys(commands)))

	if active := a.activePane(); active != nil {
		for cmd := range commands {
			for _, item := range active.list.Items() {
				name := item.(*FileItem).Info.Name
				if name != ".." {
					suggestions = append(suggestions, fmt.Sprintf("%s %s", cmd, name))
				}
			}
		}
	}

	a.textbox.Placeholder = ""
	a.textbox.SetSuggestions(suggestions)
	a.textbox.Focus()

	return a, nil
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
		return cmd.f(a, args[1:])
	}

	return failure(fmt.Sprintf("Unknown command: %q", args[0]))
}
