package ui

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os/exec"
	"path"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/moshenahmias/term-navigator/internal/file"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	_ "embed"

	"charm.land/lipgloss/v2"
)

//go:embed help.txt
var helpText string

const statusMsgDuration = time.Second * 5

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
	inputConfirmMove
	inputChangeDevice
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
	inputConfirmMove:   {text: "Type MOVE to confirm:", placeholder: "MOVE"},
	inputChangeDevice:  {text: "Enter device name:"},
}

var _ tea.Model = (*App)(nil)

type App struct {
	left      *Pane
	right     *Pane
	focus     int // 0 = left, 1 = right
	textbox   textinput.Model
	inputMode inputMode
	msg       statusMsg
	ctx       context.Context
	devs      map[string]file.Explorer
	devsHint  string
}

func NewApp(ctx context.Context, devs map[string]file.Explorer, left, right string, width, height int) (*App, error) {
	var leftExp, rightExp file.Explorer

	if exp, exists := devs[left]; exists {
		leftExp = exp
	} else {
		return nil, errors.New("left device not found: " + left)
	}

	if exp, exists := devs[right]; exists {
		rightExp = exp
	} else {
		return nil, errors.New("right device not found: " + right)
	}

	leftWidth := width / 2
	rightWidth := width - leftWidth

	ti := textinput.New()
	ti.CharLimit = 256
	ti.SetWidth(40)

	leftPane := NewPane(ctx, left, leftExp, leftWidth, height)
	rightPane := NewPane(ctx, right, rightExp, rightWidth, height)

	leftPane.SetActive(true)

	return &App{
		left:     leftPane,
		right:    rightPane,
		focus:    0,
		textbox:  ti,
		ctx:      ctx,
		devs:     devs,
		devsHint: strings.Join(slices.Collect(maps.Keys(devs)), ", "),
	}, nil
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

			if a.textbox.Value() == "" {
				return a, nil
			}

			switch currentInput {
			case inputRename:
				return a, a.applyRename()
			case inputMkdir:
				return a, a.applyMakeDir()
			case inputConfirmDelete:
				return a, a.applyDelete()
			case inputConfirmCopy:
				return a, a.applyCopy()
			case inputConfirmMove:
				return a, a.applyMove()
			case inputChangeDevice:
				return a, a.applyChangeDevice()
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
		return a, tea.Tick(statusMsgDuration, func(time.Time) tea.Msg {
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

		//return a, nil

	case tea.KeyMsg:
		switch msg.String() {

		case "tab":
			a.focus = 1 - a.focus
			a.left.SetActive(a.focus == 0)
			a.right.SetActive(a.focus == 1)
			//return a, nil

		case "enter":
			active := a.activePane() // left or right

			info, err := active.Selected()
			if err == nil && (info.IsDir || info.IsSymlinkToDir) {
				dst := info.FullPath
				if info.Name == ".." {
					// Handle parent directory navigation
					if parent, exists := active.explorer.Parent(a.ctx); exists {
						dst = parent
					} else {
						return a, nil // already at root, do nothing
					}
				}
				active.explorer.Chdir(a.ctx, dst)
				active.refresh()
			}

			//return a, nil
		case "backspace":
			active := a.activePane() // left or right

			if parent, exists := active.explorer.Parent(a.ctx); exists {
				if err := active.explorer.Chdir(a.ctx, parent); err == nil {
					active.refresh()
				}
			}

		case "f1": // Help
			return a.runHelp()
		case "f2": // rename
			return a.runRename()

		case "f3": // View
			return a.runView()

		case "f4": // Edit
			return a.runEdit()
		case "f5":
			return a.runCopy()
		case "f6":
			return a.runMove()
		case "f7":
			return a.runMakeDir()
		case "f8":
			return a.runDelete()
		case "f9":
			return a.runMetadata()
		case "f10":
			return a.runChangeDevice()
		case "f12":
			return a, tea.Quit

		}
	}

	leftCmd := tea.Cmd(nil)
	rightCmd := tea.Cmd(nil)

	a.left, leftCmd = a.left.Update(msg)
	a.right, rightCmd = a.right.Update(msg)

	return a, tea.Batch(leftCmd, rightCmd)

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
		if placeholder := inputText[a.inputMode].placeholder; placeholder != "" {
			a.textbox.Placeholder = placeholder
		}

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
		return a.left
	}
	return a.right
}

func (a *App) commandBar() string {
	key := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00afff"))

	greyed := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#555555"))

	copyTarget := "Copy→Right"
	moveTarget := "Move→Right"
	if a.focus == 1 {
		copyTarget = "Left←Copy"
		moveTarget = "Left←Move"
	}

	item, itemSelected := a.activePane().SelectedItem()

	footer := fmt.Sprintf(
		"%s Help   %s Rename   %s View   %s Edit   %s %s   %s %s   %s Mkdir   %s Delete   %s Info   %s Device   %s Quit",
		key.Render("F1"),
		func() lipgloss.Style {
			if itemSelected && item.isRenamable() {
				return key
			}

			return greyed
		}().Render("F2"),
		func() lipgloss.Style {
			if itemSelected && item.isViewable() {
				return key
			}

			return greyed
		}().Render("F3"),
		func() lipgloss.Style {
			if itemSelected && item.isEditable() {
				return key
			}

			return greyed
		}().Render("F4"),
		func() lipgloss.Style {
			if itemSelected && item.isCopyable() {
				return key
			}

			return greyed
		}().Render("F5"), copyTarget,
		func() lipgloss.Style {
			if itemSelected && item.isMoveable() {
				return key
			}

			return greyed
		}().Render("F6"), moveTarget,
		key.Render("F7"),
		func() lipgloss.Style {
			if itemSelected && item.isDeleteable() {
				return key
			}

			return greyed
		}().Render("F8"),
		func() lipgloss.Style {
			if itemSelected && item.hasMetadata() {
				return key
			}

			return greyed
		}().Render("F9"),
		func() lipgloss.Style {
			if len(a.devs) > 1 {
				return key
			}

			return greyed
		}().Render("F10"),
		key.Render("F12"),
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

	exp := pane.explorer

	// Compute new path/key
	oldPath := fi.Info.FullPath
	newPath := exp.Join(exp.Dir(oldPath), newName)

	// Perform backend rename
	if err := pane.explorer.Rename(a.ctx, oldPath, newPath); err != nil {
		return func() tea.Msg {
			return a.newErrorMsg("Rename failed: " + err.Error())
		}
	}

	pane.lastSelectedPath = newPath

	// Refresh both panes that show this directory
	a.refreshPanesForExplorer(pane.explorer)

	return func() tea.Msg {
		return a.newStatusMsg(fmt.Sprintf("Renamed %q to %q", oldPath, newPath))
	}
}

func (a *App) applyCopy() tea.Cmd {
	src := a.activePane()

	// pick destination pane
	dst := a.left
	if src == a.left {
		dst = a.right
	}

	if src.explorer.Cwd(a.ctx) == dst.explorer.Cwd(a.ctx) {
		return func() tea.Msg {
			return a.newErrorMsg("Source and destination are the same")
		}
	}

	item, ok := src.SelectedItem()
	if !ok || !item.isCopyable() {
		return nil
	}

	if a.textbox.Value() != "COPY" {
		return func() tea.Msg {
			return a.newErrorMsg("confirmation text does not match")
		}
	}

	return func() tea.Msg {
		// 1. Download from source backend
		handle, err := src.explorer.Download(a.ctx, item.Info.FullPath)
		if err != nil {
			return a.newErrorMsg("Copy failed: " + err.Error())
		}

		// We will collect ALL errors here
		var errs []string

		// 2. Upload to destination backend
		dstPath := path.Join(dst.explorer.Cwd(a.ctx), item.Info.Name)
		if err := dst.explorer.UploadFrom(a.ctx, handle.Path(), dstPath); err != nil {
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

func (a *App) applyMove() tea.Cmd {
	src := a.activePane()

	// pick destination pane
	dst := a.left
	if src == a.left {
		dst = a.right
	}

	if src.explorer.Cwd(a.ctx) == dst.explorer.Cwd(a.ctx) {
		return func() tea.Msg {
			return a.newErrorMsg("Source and destination are the same")
		}
	}

	item, ok := src.SelectedItem()
	if !ok || !item.isMoveable() {
		return nil
	}

	if a.textbox.Value() != "MOVE" {
		return func() tea.Msg {
			return a.newErrorMsg("confirmation text does not match")
		}
	}

	return func() tea.Msg {
		// 1. Download from source backend
		handle, err := src.explorer.Download(a.ctx, item.Info.FullPath)
		if err != nil {
			return a.newErrorMsg("Move failed: " + err.Error())
		}

		// We will collect ALL errors here
		var errs []string

		// 2. Upload to destination backend
		dstPath := path.Join(dst.explorer.Cwd(a.ctx), item.Info.Name)
		if err := dst.explorer.UploadFrom(a.ctx, handle.Path(), dstPath); err != nil {
			errs = append(errs, "Move failed: "+err.Error())
		}

		// 3. Always close the handle, even if upload failed
		if err := handle.Close(); err != nil {
			errs = append(errs, "Cleanup failed: "+err.Error())
		}

		// 4. Attempt to delete source (only if download/upload succeeded)
		if len(errs) == 0 {
			if err := src.explorer.Delete(a.ctx, item.Info.FullPath); err != nil {
				errs = append(errs, "Delete failed: "+err.Error())
			}
		}

		// 5. Refresh both panes that show the source and destination directories
		a.refreshPanesForExplorer(src.explorer)
		a.refreshPanesForExplorer(dst.explorer)

		// 6. If any errors occurred, show them
		if len(errs) > 0 {
			return a.newErrorMsg(strings.Join(errs, " | "))
		}

		return a.newStatusMsg(fmt.Sprintf("Moved %q to %q", item.Info.FullPath, dstPath))
	}
}

func (a *App) applyMakeDir() tea.Cmd {
	active := a.activePane()
	newDirPath := path.Join(active.explorer.Cwd(a.ctx), a.textbox.Value())

	if err := active.explorer.Mkdir(a.ctx, newDirPath); err != nil {
		return func() tea.Msg {
			return a.newErrorMsg("Mkdir failed: " + err.Error())
		}
	}

	// Refresh both panes that show this directory
	active.lastSelectedPath = newDirPath
	a.refreshPanesForExplorer(active.explorer)

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

	if err := pane.explorer.Delete(a.ctx, item.Info.FullPath); err != nil {
		return func() tea.Msg {
			return a.newErrorMsg("Delete failed: " + err.Error())
		}
	}

	// Refresh both panes that show this directory
	a.refreshPanesForExplorer(pane.explorer)

	return func() tea.Msg {
		return a.newStatusMsg(fmt.Sprintf("Deleted %q", item.Info.FullPath))
	}
}

func (a *App) applyChangeDevice() tea.Cmd {
	pane := a.activePane()

	value := a.textbox.Value()

	if value == "" || value == pane.name {
		return nil
	}

	exp, exists := a.devs[value]
	if !exists {
		return func() tea.Msg {
			return a.newErrorMsg(fmt.Sprintf("Device %q not found. Available devices: %s", value, a.devsHint))
		}
	}

	pane.explorer = exp
	pane.name = value
	pane.lastSelectedPath = ""

	// Refresh both panes that show this directory
	pane.refresh()

	return func() tea.Msg {
		return a.newStatusMsg(fmt.Sprintf("Changed device to %q", value))
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

func (a *App) runMove() (tea.Model, tea.Cmd) {
	pane := a.activePane()
	if item, ok := pane.SelectedItem(); ok && item.isMoveable() {
		a.inputMode = inputConfirmMove
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

	handle, err := pane.explorer.Download(a.ctx, item.Info.FullPath)
	if err != nil {
		return a, func() tea.Msg {
			return a.newErrorMsg("Download failed: " + err.Error())
		}
	}

	cmd := exec.Command("less", "+1", handle.Path())

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

	handle, err := pane.explorer.Download(a.ctx, item.Info.FullPath)
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
			if err := pane.explorer.UploadFrom(a.ctx, handle.Path(), item.Info.FullPath); err != nil {
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

		pane.lastSelectedPath = item.Info.FullPath

		// Refresh both panes that show this directory
		a.refreshPanesForExplorer(pane.explorer)

		return nil
	})
}

func (a *App) runHelp() (tea.Model, tea.Cmd) {
	cmd := exec.Command("less", "+1")
	cmd.Stdin = strings.NewReader(helpText)

	return a, tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return a.newErrorMsg("Help failed: " + err.Error())
		}
		return nil
	})
}

func formatMetadata(meta map[string]string) string {
	var b strings.Builder
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Fprintf(&b, "%s: %s\n", k, meta[k])
	}

	return b.String()
}

func (a *App) runMetadata() (tea.Model, tea.Cmd) {
	pane := a.activePane()
	item, ok := pane.SelectedItem()
	if !ok {
		return a, nil
	}

	metadata, err := pane.explorer.Metadata(a.ctx, item.Info.FullPath)

	if err != nil {
		return a, func() tea.Msg {
			return a.newErrorMsg("Metadata failed: " + err.Error())
		}
	}

	if len(metadata) == 0 {
		return a, func() tea.Msg {
			return a.newStatusMsg("No metadata available for " + item.Info.FullPath)
		}
	}

	s := formatMetadata(metadata)

	cmd := exec.Command("less", "+1")
	cmd.Stdin = strings.NewReader(s)

	return a, tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return a.newErrorMsg("Viewer failed: " + err.Error())
		}
		return nil
	})
}

func (a *App) runChangeDevice() (tea.Model, tea.Cmd) {
	if len(a.devs) > 1 {
		a.inputMode = inputChangeDevice
		a.textbox.SetValue("")
		a.textbox.Placeholder = a.devsHint
		a.textbox.Focus()
	}

	return a, nil
}

func sameDirSameDevice(a, b file.Explorer, ctx context.Context) bool {
	return a.DeviceID(ctx) == b.DeviceID(ctx) &&
		a.Cwd(ctx) == b.Cwd(ctx)
}

func (a *App) refreshPanesForExplorer(active file.Explorer) {
	left := a.left.explorer
	right := a.right.explorer

	// Refresh left if needed
	if left == active || sameDirSameDevice(left, active, a.ctx) {
		a.left.refresh()
	}

	// Refresh right if needed
	if right == active || sameDirSameDevice(right, active, a.ctx) {
		a.right.refresh()
	}
}
