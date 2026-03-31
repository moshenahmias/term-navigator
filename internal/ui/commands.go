package ui

import (
	"bytes"
	"fmt"
	"maps"
	"os/exec"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/moshenahmias/term-navigator/internal/config"
)

var (
	commands = map[string]func(*App, []string) tea.Cmd{
		"help": func(a *App, args []string) tea.Cmd {
			if len(args) != 0 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: help")
				}
			}
			_, cmd := a.runHelp()
			return cmd
		},
		"rename": func(a *App, args []string) tea.Cmd {
			if len(args) != 2 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: rename <old> <new>")
				}
			}

			active := a.activePane()

			return a.applyRenameInner(active, args[0], args[1])
		},
		"view": func(a *App, args []string) tea.Cmd {
			if len(args) != 1 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: view <filename>")
				}
			}

			active := a.activePane()
			_, cmd := a.runViewInner(active, args[0])
			return cmd
		},
		"edit": func(a *App, args []string) tea.Cmd {
			if len(args) != 1 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: edit <filename>")
				}
			}

			active := a.activePane()
			_, cmd := a.runEditInner(active, args[0])
			return cmd
		},
		"copy": func(a *App, args []string) tea.Cmd {
			if len(args) != 2 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: copy <src> <dest>")
				}
			}

			src := a.activePane()
			dst := a.left

			if src == a.left {
				dst = a.right
			}

			from := src.explorer.Join(src.explorer.Cwd(a.ctx), args[0])

			return a.applyCopyInner(src, dst, from, args[1])
		},
		"move": func(a *App, args []string) tea.Cmd {
			if len(args) != 2 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: move <src> <dest>")
				}
			}

			src := a.activePane()
			dst := a.left

			if src == a.left {
				dst = a.right
			}

			from := src.explorer.Join(src.explorer.Cwd(a.ctx), args[0])

			return a.applyMoveInner(src, dst, from, args[1])
		},
		"mkdir": func(a *App, args []string) tea.Cmd {
			if len(args) != 1 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: mkdir <name>")
				}
			}

			return a.applyMakeDir(args[0])
		},
		"delete": func(a *App, args []string) tea.Cmd {
			if len(args) != 1 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: delete <name>")
				}
			}

			return a.applyDeleteInner(a.activePane(), args[0])
		},
		"info": func(a *App, args []string) tea.Cmd {
			if len(args) != 1 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: info <filename>")
				}
			}

			active := a.activePane()
			_, cmd := a.runMetadataInner(active, args[0])
			return cmd
		},
		"device": func(a *App, args []string) tea.Cmd {
			if len(a.devs) < 2 {
				return nil
			}

			if len(args) != 1 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: device <name>")
				}
			}

			if a.activePane().name == args[0] {
				return nil
			}

			return a.applyChangeDevice(args[0])
		},
		"swap": func(a *App, args []string) tea.Cmd {
			if len(args) != 0 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: swap")
				}
			}

			_, cmd := a.runSwapDevices()
			return cmd
		},
		"exit": func(a *App, args []string) tea.Cmd {
			if len(args) != 0 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: exit")
				}
			}

			return tea.Quit
		},
		"config": func(a *App, args []string) tea.Cmd {
			if len(args) != 0 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: config")
				}
			}

			path, err := config.Path()

			if err != nil {
				return func() tea.Msg {
					return a.newErrorMsg("Faild to load config file")
				}
			}

			cmd := exec.Command("vim", path)

			return tea.ExecProcess(cmd, func(procErr error) tea.Msg {
				if procErr != nil {
					return a.newErrorMsg(procErr.Error())
				}

				return nil
			})
		},
		"exec": func(a *App, args []string) tea.Cmd {
			if len(args) == 0 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: exec <command>")
				}
			}

			cmd := exec.Command(args[0], args[1:]...)
			var out bytes.Buffer
			cmd.Stdout = &out

			return tea.ExecProcess(cmd, func(procErr error) tea.Msg {
				if procErr != nil {
					return a.newErrorMsg(procErr.Error())
				}
				msg := strings.ReplaceAll(out.String(), "\r\n", " ")
				msg = strings.ReplaceAll(msg, "\r", " ")
				msg = strings.ReplaceAll(msg, "\n", " ")
				return a.newStatusMsg(msg)
			})
		},
		"refresh": func(a *App, args []string) tea.Cmd {
			a.left.refresh()
			a.right.refresh()
			return nil
		},
		"cd": func(a *App, args []string) tea.Cmd {
			if len(args) != 1 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: cd <folder>")
				}
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
				return func() tea.Msg {
					return a.newErrorMsg(err.Error())
				}
			}

			active.refresh()

			return nil
		},
	}
)

func (a *App) runCommand() (tea.Model, tea.Cmd) {
	a.inputMode = inputCommand
	a.textbox.SetValue("")

	suggestions := slices.Clone(slices.Collect(maps.Keys(commands)))

	if active := a.activePane(); active != nil {
		for cmd := range commands {
			for _, item := range active.list.Items() {
				suggestions = append(suggestions, fmt.Sprintf("%s %s", cmd, item.(*FileItem).Info.Name))
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

	if cmd, exists := commands[args[0]]; exists {
		return cmd(a, args[1:])
	}
	return func() tea.Msg {
		return a.newErrorMsg(fmt.Sprintf("Unknown command: %q", args[0]))
	}
}
