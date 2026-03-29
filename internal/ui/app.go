package ui

import (
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

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
	lastError   string
}

func NewApp(leftExp, rightExp explorer.FileExplorer, width, height int) *App {
	half := width / 2
	ti := textinput.New()
	ti.Placeholder = "New name"
	ti.CharLimit = 256
	ti.SetWidth(40)

	left := NewPane(leftExp, half, height)
	right := NewPane(rightExp, half, height)

	left.SetActive(true)

	return &App{
		left:        left,
		right:       right,
		focus:       0,
		renameInput: ti,
	}
}

func (a *App) Init() tea.Cmd { return nil }

func (a *App) updateRename(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.renameInput, cmd = a.renameInput.Update(msg)

	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "enter":
			a.renaming = false
			if err := a.applyRename(); err != nil {
				return a, func() tea.Msg {
					return a.newErrorMsg("Rename failed: " + err.Error())
				}
			}

			return a, nil

		case "esc":
			a.renaming = false
			return a, nil
		}
	}

	// 🔥 IMPORTANT: return here so pane does NOT update
	return a, cmd
}

func (a *App) updateMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case ErrorMsg:
		a.lastError = msg.Text
		return a, nil

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
			return a.runRename()

		case "f3": // View
			return a.runView()

		case "f4": // Edit
			return a.runEdit()

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

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a.renaming {
		return a.updateRename(msg)
	}

	return a.updateMain(msg)
}

func (a *App) View() tea.View {
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
		v.AltScreen = true
		return v
	}

	// 5. Footer
	footer := a.commandBar()

	// 🔥 6. Optional error bar
	var errorBar string
	if a.lastError != "" {
		errorBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")). // red
			Padding(0, 1).
			Render("Error: " + a.lastError)
	}

	// 7. Compose final layout
	out := lipgloss.JoinVertical(
		lipgloss.Left,
		panes,
		errorBar, // 🔥 inserted here
		footer,
	)

	v := tea.NewView(out)
	v.AltScreen = true

	return v
}

func (a *App) activePane() *Pane {
	if a.focus == 0 {
		return &a.left
	}
	return &a.right
}

func (a *App) commandBar() string {
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

func (a *App) applyRename() error {
	pane := a.activePane()

	fi, ok := pane.SelectedItem()
	if !ok {
		return nil
	}

	newName := a.renameInput.Value()
	if newName == "" || newName == fi.Info.Name {
		return nil
	}

	// Compute new path/key
	oldPath := fi.Info.FullPath
	newPath := path.Join(path.Dir(oldPath), newName)

	// Perform backend rename
	if err := pane.explorer.Rename(oldPath, newPath); err != nil {
		return err
	}

	pane.lastSelectedPath = newPath

	// Refresh both panes that show this directory
	a.refreshPanesForPath(filepath.Dir(oldPath))

	return nil
}

type ErrorMsg struct {
	Text string
}

func (a *App) newErrorMsg(text string) ErrorMsg {
	return ErrorMsg{Text: text}
}

func (a *App) runRename() (tea.Model, tea.Cmd) {
	pane := a.activePane()
	if item, ok := pane.SelectedItem(); ok {
		a.renaming = true
		a.renameInput.SetValue(item.Info.Name)
		a.renameInput.Focus()

	}

	return a, nil
}

func (a *App) runView() (tea.Model, tea.Cmd) {
	pane := a.activePane()
	item, ok := pane.SelectedItem()
	if !ok || item.Info.IsDir {
		return a, nil
	}

	handle, err := pane.explorer.Download(item.Info.FullPath)
	if err != nil {
		return a, func() tea.Msg {
			return a.newErrorMsg("Download failed: " + err.Error())
		}
	}

	cmd := exec.Command("less", handle.Path())

	return a, tea.ExecProcess(cmd, func(procErr error) tea.Msg {
		var errs []string

		// 1. less error
		if procErr != nil {
			errs = append(errs, "Viewer failed: "+procErr.Error())
		}

		// 2. cleanup error
		if err := handle.Close(); err != nil {
			errs = append(errs, "Cleanup failed: "+err.Error())
		}

		// 3. return combined error or nil
		if len(errs) > 0 {
			return a.newErrorMsg(strings.Join(errs, " | "))
		}

		return nil
	})
}

func (a *App) runEdit() (tea.Model, tea.Cmd) {
	pane := a.activePane()
	item, ok := pane.SelectedItem()
	if !ok || item.Info.IsDir {
		return a, nil
	}

	handle, err := pane.explorer.Download(item.Info.FullPath)
	if err != nil {
		return a, func() tea.Msg {
			return a.newErrorMsg("Download failed: " + err.Error())
		}
	}

	cmd := exec.Command("vim", handle.Path())

	return a, tea.ExecProcess(cmd, func(procErr error) tea.Msg {
		var errs []string

		// 1. Editor error
		if procErr != nil {
			errs = append(errs, "Editor failed: "+procErr.Error())
		} else {
			// 2. Upload error (only if editor succeeded)
			if err := pane.explorer.UploadFrom(handle.Path(), item.Info.FullPath); err != nil {
				errs = append(errs, "Upload failed: "+err.Error())
			}
		}

		// 3. Cleanup error (always attempt)
		if err := handle.Close(); err != nil {
			errs = append(errs, "Cleanup failed: "+err.Error())
		}

		// 4. Return combined error or nil
		if len(errs) > 0 {
			return a.newErrorMsg(strings.Join(errs, " | "))
		}

		return nil
	})
}

func sameDir(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func (a *App) refreshPanesForPath(path string) {
	clean := filepath.Clean(path)

	if sameDir(a.left.explorer.Cwd(), clean) {
		a.left.refresh()
	}
	if sameDir(a.right.explorer.Cwd(), clean) {
		a.right.refresh()
	}
}
