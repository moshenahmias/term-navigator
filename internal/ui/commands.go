package ui

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
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
					return a.newErrorMsg("Usage: rename <old_name> <new_name>")
				}
			}

			active := a.activePane()

			return a.applyRenameInner(active, args[0], args[1])
		},
		"view": func(a *App, args []string) tea.Cmd {
			if len(args) != 1 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: view [filename]")
				}
			}

			active := a.activePane()
			_, cmd := a.runViewInner(active, args[0])
			return cmd
		},
		"edit": func(a *App, args []string) tea.Cmd {
			if len(args) != 1 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: edit [filename]")
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

			from := src.explorer.Join(src.explorer.Cwd(src.ctx), args[0])

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

			from := src.explorer.Join(src.explorer.Cwd(src.ctx), args[0])

			return a.applyMoveInner(src, dst, from, args[1])
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
