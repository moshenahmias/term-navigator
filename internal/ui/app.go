package ui

import (
	"fmt"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/moshenahmias/term-navigator/internal/explorer"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"charm.land/lipgloss/v2"
)

var (
	paneStyle      = lipgloss.NewStyle() // no background, no foreground
	separatorStyle = lipgloss.NewStyle().Render("│")

	ncPaneStyle = lipgloss.NewStyle() // no forced colors

	ncBorder = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()) // simple border

)

type App struct {
	left        Pane
	right       Pane
	focus       int // 0 = left, 1 = right
	renaming    bool
	renameInput textinput.Model
}

func NewApp(leftExp, rightExp explorer.FileExplorer, width, height int) App {
	half := width / 2
	ti := textinput.New()
	ti.Placeholder = "New name"
	ti.CharLimit = 256
	ti.SetWidth(40)

	left := NewPane(leftExp, half, height)
	right := NewPane(rightExp, half, height)

	left.SetActive(true)

	return App{
		left:        left,
		right:       right,
		focus:       0,
		renameInput: ti,
	}
}

func (a App) Init() tea.Cmd { return nil }

func (a App) updateRename(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.renameInput, cmd = a.renameInput.Update(msg)

	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "enter":
			a.applyRename()
			a.renaming = false
			return a, nil

		case "esc":
			a.renaming = false
			return a, nil
		}
	}

	// 🔥 IMPORTANT: return here so pane does NOT update
	return a, cmd
}

func (a App) updateMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		totalWidth := msg.Width
		totalHeight := msg.Height

		// subtract 2 columns for each pane border
		paneWidth := (totalWidth / 2)

		// subtract 2 rows for top/bottom border
		paneHeight := totalHeight - 2

		a.left.Resize(paneWidth, paneHeight)
		a.right.Resize(paneWidth, paneHeight)

		return a, nil

	case tea.KeyMsg:
		switch msg.String() {

		case "tab":
			a.focus = 1 - a.focus
			a.left.SetActive(a.focus == 0)
			a.right.SetActive(a.focus == 1)
			return a, nil

		case "enter":
			pane := &a.left
			if a.focus == 1 {
				pane = &a.right
			}

			info, err := pane.Selected()
			if err == nil && (info.IsDir || info.IsSymlinkToDir) {
				pane.explorer.Chdir(info.FullPath)
				pane.refresh()
			}

			return a, nil
		case "backspace":
			active := a.activePane() // left or right
			cwd := active.explorer.Cwd()

			parent := filepath.Dir(cwd)

			if err := active.explorer.Chdir(parent); err == nil {
				active.refresh()
			}
		case "f2": // rename
			pane := a.activePane()
			item, ok := pane.SelectedItem()
			if !ok {
				return a, nil
			}

			a.renaming = true
			a.renameInput.SetValue(item.Info.Name)
			a.renameInput.Focus()

			return a, nil
		case "f3": // View
			pane := a.activePane()
			item, ok := pane.SelectedItem()
			if !ok || item.Info.IsDir {
				return a, nil
			}

			cmd := exec.Command("less", item.Info.FullPath)
			return a, tea.ExecProcess(cmd, nil)

		case "f4": // Edit
			pane := a.activePane()
			item, ok := pane.SelectedItem()
			if !ok || item.Info.IsDir {
				return a, nil
			}

			cmd := exec.Command("vim", item.Info.FullPath)
			return a, tea.ExecProcess(cmd, nil)

		}
	}

	// Update focused pane
	if a.focus == 0 {
		var cmd tea.Cmd
		a.left, cmd = a.left.Update(msg)
		return a, cmd
	}

	var cmd tea.Cmd
	a.right, cmd = a.right.Update(msg)
	return a, cmd
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a.renaming {
		return a.updateRename(msg)
	}

	return a.updateMain(msg)
}

func (a App) View() tea.View {
	// 1. Render pane content with height constraint (subtract borders)
	leftContent := lipgloss.NewStyle().
		MaxHeight(a.left.height - 2).
		Render(a.left.View())

	rightContent := lipgloss.NewStyle().
		MaxHeight(a.right.height - 2).
		Render(a.right.View())

	// 2. Wrap content in border
	leftBox := ncBorder.
		Width(a.left.width).
		Height(a.left.height).
		Render(leftContent)

	rightBox := ncBorder.
		Width(a.right.width).
		Height(a.right.height).
		Render(rightContent)

	// 3. Join horizontally
	panes := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftBox,
		rightBox,
	)

	// 4. Rename mode
	if a.renaming {
		renameBox := lipgloss.JoinVertical(
			lipgloss.Left,
			panes,
			"Rename:",
			a.renameInput.View(),
		)
		v := tea.NewView(renameBox)
		v.AltScreen = true // 🔥 use alternate screen
		return v
	}

	// 5. Footer
	footer := a.commandBar()

	out := lipgloss.JoinVertical(
		lipgloss.Left,
		panes,
		footer,
	)

	v := tea.NewView(out)
	v.AltScreen = true // 🔥 use alternate screen
	return v
}

func (a *App) activePane() *Pane {
	if a.focus == 0 {
		return &a.left
	}
	return &a.right
}

func (a App) commandBar() string {
	key := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00afff"))

	copyTarget := "Right"
	moveTarget := "Right"
	if a.focus == 1 {
		copyTarget = "Left"
		moveTarget = "Left"
	}

	footer := fmt.Sprintf(
		"%s Help   %s Rename   %s View   %s Edit   %s Copy→%s   %s Move→%s   %s Mkdir   %s Delete   %s Quit",
		key.Render("F1"),
		key.Render("F2"),
		key.Render("F3"),
		key.Render("F4"),
		key.Render("F5"), copyTarget,
		key.Render("F6"), moveTarget,
		key.Render("F7"),
		key.Render("F8"),
		key.Render("F10"),
	)

	return lipgloss.NewStyle().
		Background(lipgloss.Color("#222")).
		Foreground(lipgloss.Color("#ccc")).
		Padding(0, 1).
		Render(footer)
}

func (a *App) applyRename() {
	pane := a.activePane()

	fi, ok := pane.SelectedItem()
	if !ok {
		return
	}

	newName := a.renameInput.Value()
	if newName == "" || newName == fi.Info.Name {
		return
	}

	// Compute new path/key
	oldPath := fi.Info.FullPath
	newPath := path.Join(path.Dir(oldPath), newName)

	// Perform backend rename
	if err := pane.explorer.Rename(oldPath, newPath); err != nil {
		// TODO: show error in status bar
		return
	}

	pane.lastSelectedPath = newPath

	// Refresh pane contents
	pane.refresh()
}
