package ui

import (
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/moshenahmias/term-navigator/internal/explorer"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"charm.land/lipgloss/v2"
)

var (
	ncBorder = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()) // simple border
)

type statusMsg struct {
	text  string
	isErr bool
}

type clearStatusMsg struct{}

type inputMode int

const (
	inputNone inputMode = iota
	inputRename
	inputMkdir
	inputConfirmDelete
	inputConfirmCopy
)

type inputSettings struct {
	text        string
	placeholder string
}

var inputText = map[inputMode]inputSettings{
	inputRename:        {text: "Rename:", placeholder: "New name"},
	inputMkdir:         {text: "New directory name:", placeholder: "Directory name"},
	inputConfirmDelete: {text: "Type DELETE to confirm:", placeholder: "DELETE"},
	inputConfirmCopy:   {text: "Type COPY to confirm:", placeholder: "COPY"},
}

var _ tea.Model = (*App)(nil)

type App struct {
	left      Pane
	right     Pane
	focus     int // 0 = left, 1 = right
	textbox   textinput.Model
	inputMode inputMode

	msg statusMsg
}

func NewApp(leftExp, rightExp explorer.FileExplorer, width, height int) *App {
	half := width / 2
	ti := textinput.New()
	ti.CharLimit = 256
	ti.SetWidth(40)

	left := NewPane(leftExp, half, height)
	right := NewPane(rightExp, half, height)

	left.SetActive(true)

	return &App{
		left:    left,
		right:   right,
		focus:   0,
		textbox: ti,
	}
}

func (a *App) Init() tea.Cmd { return nil }

func (a *App) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.textbox, cmd = a.textbox.Update(msg)

	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "enter":
			currentInput := a.inputMode
			a.inputMode = inputNone

			switch currentInput {
			case inputRename:
				return a, a.applyRename()
			case inputMkdir:
				return a, a.applyMakeDir()
			case inputConfirmDelete:
				return a, a.applyDelete()
			case inputConfirmCopy:
				return a, a.applyCopy()
			}

			return a, nil

		case "esc":
			a.inputMode = inputNone
			return a, nil
		}
	}

	// 🔥 IMPORTANT: return here so pane does NOT update
	return a, cmd
}

func (a *App) updateMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case statusMsg:
		a.msg = msg
		return a, tea.Tick(time.Second*2, func(time.Time) tea.Msg {
			return clearStatusMsg{}
		})
	case clearStatusMsg:
		a.msg = statusMsg{} // reset to empty
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
		case "f5":
			return a.runCopy()
		case "f7":
			return a.runMakeDir()
		case "f8":
			return a.runDelete()
		case "f10":
			return a, tea.Quit

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
	if a.inputMode != inputNone {
		return a.updateInput(msg)
	}

	return a.updateMain(msg)
}

var errorStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#FF0000")).
	Foreground(lipgloss.Color("#FFFFFF"))

var successStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#00AA00")).
	Foreground(lipgloss.Color("#FFFFFF"))

func (a *App) renderStatus() string {
	if a.msg.text == "" {
		return ""
	}

	if a.msg.isErr {
		return errorStyle.Render(a.msg.text)
	}
	return successStyle.Render(a.msg.text)
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

	// 4. Input mode
	if a.inputMode != inputNone {
		a.textbox.Placeholder = inputText[a.inputMode].placeholder
		inputBox := lipgloss.JoinVertical(
			lipgloss.Left,
			panes,
			inputText[a.inputMode].text,
			a.textbox.View(),
		)
		v := tea.NewView(inputBox)
		v.AltScreen = true
		return v
	}

	// 5. Footer
	footer := a.commandBar()

	// 🔥 6. Status bar
	statusBar := a.renderStatus()

	// 7. Compose final layout
	out := lipgloss.JoinVertical(
		lipgloss.Left,
		panes,
		statusBar, // 🔥 inserted here
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

func (a *App) applyRename() tea.Cmd {
	pane := a.activePane()

	fi, ok := pane.SelectedItem()
	if !ok {
		return nil
	}

	if !fi.isRenamable() {
		return nil
	}

	newName := a.textbox.Value()
	if newName == "" || newName == fi.Info.Name {
		return nil
	}

	// Compute new path/key
	oldPath := fi.Info.FullPath
	newPath := path.Join(path.Dir(oldPath), newName)

	// Perform backend rename
	if err := pane.explorer.Rename(oldPath, newPath); err != nil {
		return func() tea.Msg {
			return a.newErrorMsg("Rename failed: " + err.Error())
		}
	}

	pane.lastSelectedPath = newPath

	// Refresh both panes that show this directory
	a.refreshPanesForPath(filepath.Dir(oldPath))

	return func() tea.Msg {
		return a.newStatusMsg(fmt.Sprintf("Renamed %q to %q", oldPath, newPath))
	}
}

func (a *App) applyCopy() tea.Cmd {
	src := a.activePane()

	// pick destination pane
	dst := &a.left
	if src == &a.left {
		dst = &a.right
	}

	if src.explorer.Cwd() == dst.explorer.Cwd() {
		return func() tea.Msg {
			return a.newErrorMsg("Source and destination are the same")
		}
	}

	item, ok := src.SelectedItem()
	if !ok || (item.Info.IsDir && item.Info.Name == "..") || item.Info.IsSymlinkToDir {
		return nil
	}

	if !item.isCopyable() {
		return nil
	}

	if a.textbox.Value() != "COPY" {
		return func() tea.Msg {
			return a.newErrorMsg("confirmation text does not match")
		}
	}

	return func() tea.Msg {
		// 1. Download from source backend
		handle, err := src.explorer.Download(item.Info.FullPath)
		if err != nil {
			return a.newErrorMsg("Copy failed: " + err.Error())
		}

		// We will collect ALL errors here
		var errs []string

		// 2. Upload to destination backend
		dstPath := path.Join(dst.explorer.Cwd(), item.Info.Name)
		if err := dst.explorer.UploadFrom(handle.Path(), dstPath); err != nil {
			errs = append(errs, "Copy failed: "+err.Error())
		}

		// 3. Always close the handle, even if upload failed
		if err := handle.Close(); err != nil {
			errs = append(errs, "Cleanup failed: "+err.Error())
		}

		// 4. Refresh destination pane
		dst.refresh()

		// 5. If any errors occurred, show them
		if len(errs) > 0 {
			return a.newErrorMsg(strings.Join(errs, " | "))
		}

		return a.newStatusMsg(fmt.Sprintf("Copied %q to %q", item.Info.FullPath, dstPath))
	}
}

func (a *App) applyMakeDir() tea.Cmd {
	active := a.activePane()
	newDirPath := path.Join(active.explorer.Cwd(), a.textbox.Value())

	if err := active.explorer.Mkdir(newDirPath); err != nil {
		return func() tea.Msg {
			return a.newErrorMsg("Mkdir failed: " + err.Error())
		}
	}

	// Refresh both panes that show this directory
	a.refreshPanesForPath(active.explorer.Cwd())

	return func() tea.Msg {
		return a.newStatusMsg(fmt.Sprintf("Created directory %q", newDirPath))
	}
}

func (a *App) applyDelete() tea.Cmd {
	pane := a.activePane()
	item, ok := pane.SelectedItem()
	if !ok {
		return nil
	}

	if !item.isDeleteable() {
		return nil
	}

	if a.textbox.Value() != "DELETE" {
		return func() tea.Msg {
			return a.newErrorMsg("confirmation text does not match")
		}
	}

	if err := pane.explorer.Delete(item.Info.FullPath); err != nil {
		return func() tea.Msg {
			return a.newErrorMsg("Delete failed: " + err.Error())
		}
	}

	// Refresh both panes that show this directory
	a.refreshPanesForPath(pane.explorer.Cwd())

	return func() tea.Msg {
		return a.newStatusMsg(fmt.Sprintf("Deleted %q", item.Info.FullPath))
	}
}

func (a *App) newErrorMsg(text string) tea.Msg {
	return statusMsg{text: text, isErr: true}
}

func (a *App) newStatusMsg(text string) tea.Msg {
	return statusMsg{text: text, isErr: false}
}

func (a *App) runRename() (tea.Model, tea.Cmd) {
	pane := a.activePane()
	if item, ok := pane.SelectedItem(); ok && item.isRenamable() {
		a.inputMode = inputRename
		a.textbox.SetValue(item.Info.Name)
		a.textbox.Focus()
	}

	return a, nil
}

func (a *App) runCopy() (tea.Model, tea.Cmd) {
	pane := a.activePane()
	if item, ok := pane.SelectedItem(); ok && item.isCopyable() {
		a.inputMode = inputConfirmCopy
		a.textbox.SetValue("")
		a.textbox.Focus()
	}

	return a, nil
}

func (a *App) runMakeDir() (tea.Model, tea.Cmd) {
	a.inputMode = inputMkdir
	a.textbox.SetValue("New Folder")
	a.textbox.Focus()

	return a, nil
}

func (a *App) runDelete() (tea.Model, tea.Cmd) {
	pane := a.activePane()
	if item, ok := pane.SelectedItem(); ok && item.isDeleteable() {
		a.inputMode = inputConfirmDelete
		a.textbox.SetValue("")
		a.textbox.Focus()
	}

	return a, nil
}

func (a *App) runView() (tea.Model, tea.Cmd) {
	pane := a.activePane()
	item, ok := pane.SelectedItem()
	if !ok || !item.isViewable() {
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
	if !ok || !item.isEditable() {
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
