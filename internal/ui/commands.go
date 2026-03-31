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
			_, cmd := a.runHelp()
			return cmd
		},
		"rename": func(a *App, args []string) tea.Cmd {
			if len(args) > 2 || len(args) == 0 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: rename <new_name> (or rename <old_name> <new_name>)")
				}
			}

			active := a.activePane()

			if len(args) == 1 {
				if item, ok := active.SelectedItem(); ok && item.isRenamable() {
					return a.applyRename(args[0])
				}
				return func() tea.Msg {
					return a.newErrorMsg("No renamable item selected")
				}
			}

			return a.applyRenameInner(active, args[0], args[1])
		},
		"view": func(a *App, args []string) tea.Cmd {
			if len(args) > 1 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: view [filename]")
				}
			}

			if len(args) == 0 {
				_, cmd := a.runView()
				return cmd
			}

			active := a.activePane()
			_, cmd := a.runViewInner(active, args[0])
			return cmd
		},
		"edit": func(a *App, args []string) tea.Cmd {
			if len(args) > 1 {
				return func() tea.Msg {
					return a.newErrorMsg("Usage: edit [filename]")
				}
			}

			if len(args) == 0 {
				_, cmd := a.runEdit()
				return cmd
			}

			active := a.activePane()
			_, cmd := a.runEditInner(active, args[0])
			return cmd
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
