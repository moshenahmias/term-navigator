package tncore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/moshenahmias/term-navigator/internal/backends/local"
	"github.com/moshenahmias/term-navigator/internal/file"
	"github.com/moshenahmias/term-navigator/internal/logbuf"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	_ "embed"

	"charm.land/lipgloss/v2"
)

//go:embed help.txt
var helpText string

var (
	ncBorder = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder())
)

type statusMsg struct {
	text  string
	isErr bool
	d     time.Duration
	next  *statusMsg
}

type clearStatusMsg struct{}

type asyncJobDoneMsg struct {
	msg tea.Msg
}

type progressMsg struct {
	text string
}

type inputMode int

const (
	inputNone inputMode = iota
	inputRename
	inputMkdir
	inputConfirmDelete
	inputConfirmCopy
	inputConfirmMove
	inputChangeDevice
	inputCommand
)

const (
	deleteConfirmationText = "DELETE"
	copyConfirmationText   = "COPY"
	moveConfirmationText   = "MOVE"
	fileViewEditMaxSize    = 1024 * 1024 * 4 // 4 MB
	maxLogHistory          = 300
)

var _, jqErr = exec.LookPath("jq")

var editors = []string{"vi", "nano", "pico"}

var inputText = map[inputMode]string{
	inputRename:        "Rename:",
	inputMkdir:         "New directory name:",
	inputConfirmDelete: fmt.Sprintf("Type %s to confirm:", deleteConfirmationText),
	inputConfirmCopy:   fmt.Sprintf("Type %s to confirm:", copyConfirmationText),
	inputConfirmMove:   fmt.Sprintf("Type %s to confirm:", moveConfirmationText),
	inputChangeDevice:  "Switch to:",
	inputCommand: fmt.Sprintf("Type 'help' for commands (Use %s+↓↑ + TAB for completion):", func() string {
		if runtime.GOOS == "darwin" {
			return "⌥"
		}
		return "ALT"
	}()),
}

var _ tea.Model = (*App)(nil)

type App struct {
	width            int
	height           int
	left             *Pane
	right            *Pane
	focus            int // 0 = left, 1 = right
	textbox          textinput.Model
	inputMode        inputMode
	msg              statusMsg
	ctx              context.Context
	devs             map[string]file.Explorer
	devsHint         string
	asyncJobRunning  bool
	asyncJobCancel   context.CancelFunc
	Send             func(tea.Msg)
	lastProgressSent time.Time
	logger           *slog.Logger
	logBuffer        fmt.Stringer
	commands         map[string]command
}

func NewApp(ctx context.Context, devs map[string]file.Explorer, left, right string, width, height int) (*App, error) {
	var leftExp, rightExp file.Explorer

	if exp, exists := devs[left]; exists {
		leftExp = exp.Copy()
	} else {
		return nil, errors.New("left device not found: " + left)
	}

	if exp, exists := devs[right]; exists {
		rightExp = exp.Copy()
	} else {
		return nil, errors.New("right device not found: " + right)
	}

	leftWidth := width / 2
	rightWidth := width - leftWidth

	ti := textinput.New()
	ti.CharLimit = 256
	ti.SetWidth(75)
	ti.ShowSuggestions = true

	leftPane := NewPane(ctx, left, leftExp, leftWidth, height)
	rightPane := NewPane(ctx, right, rightExp, rightWidth, height)

	leftPane.SetActive(true)

	logBuffer := logbuf.NewLineRingBuffer(maxLogHistory)
	logger := slog.New(slog.NewTextHandler(logBuffer, nil))

	return &App{
		left:      leftPane,
		right:     rightPane,
		focus:     0,
		textbox:   ti,
		ctx:       ctx,
		devs:      devs,
		devsHint:  strings.Join(slices.Collect(maps.Keys(devs)), ", "),
		logger:    logger,
		logBuffer: logBuffer,
		commands:  commands,
	}, nil
}

func (a *App) Init() tea.Cmd { return nil }

func (a *App) runAsyncJob(progressText func(name string, n, total int64) string, job func(context.Context, file.ProgressFunc) tea.Msg) {
	a.lastProgressSent = time.Now()

	progress := func(name string, n, total int64) {
		if time.Since(a.lastProgressSent) < 100*time.Millisecond {
			return
		}
		a.lastProgressSent = time.Now()

		a.Send(progressMsg{
			progressText(name, n, total),
		})
	}

	a.asyncJobRunning = true
	var ctx context.Context
	ctx, a.asyncJobCancel = context.WithCancel(a.ctx)

	go func() {
		a.Send(asyncJobDoneMsg{job(ctx, progress)})
	}()
}

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
				return a, a.applyRename(a.textbox.Value())
			case inputMkdir:
				return a, a.applyMakeDir(a.textbox.Value())
			case inputConfirmDelete:
				return a, a.applyDelete(a.textbox.Value())
			case inputConfirmCopy:
				a.runAsyncJob(func(name string, n, total int64) string {
					if total < 1 {
						return fmt.Sprintf("Copied %s of %q", bytesFormatter(n), name)
					}

					return fmt.Sprintf("Copied %s/%s of %q", bytesFormatter(n), bytesFormatter(total), name)
				}, func(ctx context.Context, progress file.ProgressFunc) tea.Msg {
					return a.applyCopy(ctx, a.textbox.Value(), progress)()
				})

				return a, nil

			case inputConfirmMove:
				a.runAsyncJob(func(name string, n, total int64) string {
					if total < 1 {
						return fmt.Sprintf("Moved %s of %q", bytesFormatter(n), name)
					}
					return fmt.Sprintf("Moved %s/%s of %q", bytesFormatter(n), name)
				}, func(ctx context.Context, progress file.ProgressFunc) tea.Msg {
					return a.applyMove(ctx, a.textbox.Value(), progress)()
				})

				return a, nil
			case inputChangeDevice:
				return a, a.applyChangeDevice(a.textbox.Value())
			case inputCommand:
				return a, a.applyCommand(a.textbox.Value())
			}

			return a, nil

		case "esc":
			a.inputMode = inputNone
			return a, nil
		default:
			a.setLiveSuggestions(a.textbox.Value())
			return a, nil
		}
	}

	return a, cmd
}

func (a *App) setLiveSuggestions(text string) {
	switch a.inputMode {
	case inputCommand:
		a.setCommandSuggestions(text)
	}
}

func (a *App) updateMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	active := a.activePane() // left or right

	switch msg := msg.(type) {

	case progressMsg:
		msg.text = strings.NewReplacer("\n", "", "\r", "").Replace(msg.text)
		a.msg = statusMsg{text: fmt.Sprintf("[ESC] %s", msg.text), isErr: false}
		return a, nil

	case statusMsg:
		msg = splitStatusMsgLines(msg, a.width)
		a.msg = msg

		if msg.text == "" {
			return a, nil
		}

		if msg.isErr {
			a.logger.Error(msg.text)
		} else {
			a.logger.Info(msg.text)
		}

		if msg.d <= 0 {
			return a, nil
		}

		return a, tea.Tick(msg.d, func(time.Time) tea.Msg {
			if msg.next != nil {
				return *msg.next
			}

			return clearStatusMsg{}
		})
	case clearStatusMsg:
		a.msg = statusMsg{} // reset to empty
		return a, nil

	case tea.WindowSizeMsg:
		totalWidth := msg.Width
		totalHeight := msg.Height

		a.width = totalWidth
		a.height = totalHeight

		// subtract 2 columns for each pane border
		paneWidth := (totalWidth / 2)

		// subtract 2 rows for top/bottom border
		paneHeight := totalHeight - 2

		a.left.Resize(paneWidth, paneHeight)
		a.right.Resize(paneWidth, paneHeight)

	case asyncJobDoneMsg:
		a.asyncJobCancel()
		a.asyncJobRunning = false
		return a, func() tea.Msg {
			return msg.msg
		}

	case tea.KeyMsg:
		if a.asyncJobRunning {
			if msg.String() == "esc" {
				a.asyncJobCancel()
			}
			return a, nil
		}

		switch msg.String() {

		case "tab":
			a.focus = 1 - a.focus
			a.left.SetActive(a.focus == 0)
			a.right.SetActive(a.focus == 1)

		case "enter":
			if active.list.FilterState() != list.Filtering {
				info, err := active.Selected()

				if err == nil {
					dst := info.FullPath
					if info.IsDir || info.IsSymlinkToDir {
						if info.Name == parentDirName {
							// Handle parent directory navigation
							if parent, exists := active.explorer.Parent(a.ctx); exists {
								dst = parent
							} else {
								return a, nil // already at root, do nothing
							}
						}
						if err := active.explorer.Chdir(a.ctx, dst); err == nil {
							active.list.SetFilterText("")
							active.list.SetFilterState(list.Unfiltered)
							active.refresh()
						}
					} else {
						// file
						return a.runOpen(active, dst)
					}
				}

			}
		case "backspace":

			if active.list.FilterState() != list.Filtering {
				if parent, exists := active.explorer.Parent(a.ctx); exists {
					if err := active.explorer.Chdir(a.ctx, parent); err == nil {
						active.list.SetFilterText("")
						active.list.SetFilterState(list.Unfiltered)
						active.refresh()
					}
				}

				//return a, nil
			}

		case "f1": // Help
			return a.runHelp()
		case "f2": // rename
			return a.runRename()
		case "f3": // View
			return a.runView()
		case "ctrl+e": // Edit + jq
			return a.runEdit(true)

		case "f4": // Edit / Extract
			return a.runEdit(false)
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
			return a.runSwapDevices()
		case ":":
			return a.runCommand()
		}
	}

	cmd := tea.Cmd(nil)

	if a.focus == 0 {
		a.left, cmd = a.left.Update(msg)
	} else {
		a.right, cmd = a.right.Update(msg)
	}

	return a, cmd
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a.inputMode != inputNone && !a.asyncJobRunning {
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

	return a.msg.text
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
		inputBox := lipgloss.JoinVertical(
			lipgloss.Left,
			panes,
			inputText[a.inputMode],
			a.textbox.View(),
		)
		v := tea.NewView(inputBox)
		v.AltScreen = true
		return v
	}

	// 5. Footer
	footer := a.commandBar()

	// 6. Status bar
	statusBar := a.renderStatus()

	// 7. Compose final layout
	out := lipgloss.JoinVertical(
		lipgloss.Left,
		panes,
		statusBar,
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

func (a *App) panes() (*Pane, *Pane) {
	src := a.activePane()

	// pick destination pane
	dst := a.left
	if src == a.left {
		dst = a.right
	}

	return src, dst
}

var key = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#00afff"))

var greyed = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#555555"))

func (a *App) buildFooter(item *FileItem, itemSelected, extractEnabled, sameDir bool, f []string, format string) string {
	return fmt.Sprintf(
		format,
		key.Render("F1"), f[0],
		func() lipgloss.Style {
			if itemSelected && item.isRenamable() {
				return key
			}

			return greyed
		}().Render("F2"), f[1],
		func() lipgloss.Style {
			if itemSelected && (item.isViewable() || extractEnabled) {
				return key
			}

			return greyed
		}().Render("F3"), f[2],
		func() lipgloss.Style {
			if itemSelected && (item.isEditable() || extractEnabled) {
				return key
			}

			return greyed
		}().Render("F4"), f[3],
		func() lipgloss.Style {
			if itemSelected && item.isCopyable() && !sameDir {
				return key
			}

			return greyed
		}().Render("F5"), f[4],
		func() lipgloss.Style {
			if itemSelected && item.isMoveable() && !sameDir {
				return key
			}

			return greyed
		}().Render("F6"), f[5],
		key.Render("F7"), f[6],
		func() lipgloss.Style {
			if itemSelected && item.isDeleteable() {
				return key
			}

			return greyed
		}().Render("F8"), f[7],
		func() lipgloss.Style {
			if itemSelected && item.hasMetadata() {
				return key
			}

			return greyed
		}().Render("F9"), f[8],
		func() lipgloss.Style {
			if len(a.devs) > 1 {
				return key
			}

			return greyed
		}().Render("F10"), f[9],
		func() lipgloss.Style {
			if len(a.devs) > 1 && a.left.name != a.right.name {
				return key
			}

			return greyed
		}().Render("F12"), f[10],
		key.Render("ESC"), f[11],
	)
}

func (a *App) commandBar() string {
	pane, dst := a.panes()
	item, itemSelected := pane.SelectedItem()
	extractEnabled := itemSelected && item.isArchive() && pane.explorer.Type() == local.Type
	sameDir := pane.explorer.DeviceID(a.ctx) == dst.explorer.DeviceID(a.ctx) && pane.explorer.Cwd(a.ctx) == dst.explorer.Cwd(a.ctx)

	var f [12]string

	f[0] = "Help"
	f[1] = "Rename"
	f[2] = "View"
	f[3] = "Edit"
	f[4] = "Copy →"
	f[5] = "Move →"
	f[6] = "Mkdir"
	f[7] = "Delete"
	f[8] = "Info"
	f[9] = "Device"
	f[10] = "Swap"
	f[11] = "Quit"

	if extractEnabled {
		f[3] = "Extract"
	}

	if a.focus == 1 {
		f[4] = "← Copy"
		f[5] = "← Move"
	}

	format := "%s %s  %s %s  %s %s  %s %s  %s %s  %s %s  %s %s  %s %s  %s %s  %s %s  %s %s  %s %s"
	footer := a.buildFooter(item, itemSelected, extractEnabled, sameDir, f[:], format)

	visibleWidth := lipgloss.Width(footer)

	if visibleWidth > a.width {
		f[0] = "HL"
		f[1] = "RN"
		f[2] = "VW"
		f[3] = "ED"
		f[4] = "CP"
		f[5] = "MV"
		f[6] = "MD"
		f[7] = "DL"
		f[8] = "IN"
		f[9] = "DV"
		f[10] = "SW"
		f[11] = "QT"
		footer = a.buildFooter(item, itemSelected, extractEnabled, sameDir, f[:], format)
	}

	visibleWidth = lipgloss.Width(footer)

	if visibleWidth > a.width {
		f[0] = "H"
		f[1] = "R"
		f[2] = "V"
		f[3] = "E"
		f[4] = "C"
		f[5] = "M"
		f[6] = "F"
		f[7] = "D"
		f[8] = "I"
		f[9] = "/"
		f[10] = "S"
		f[11] = "Q"
		format := "%s %s  %s %s  %s %s  %s %s  %s %s  %s %s  %s %s  %s %s  %s %s  %s %s  %s %s  %s %s"
		footer = a.buildFooter(item, itemSelected, extractEnabled, sameDir, f[:], format)
	}

	footerStyled := lipgloss.NewStyle().
		Background(lipgloss.Color("#222")).
		Foreground(lipgloss.Color("#ccc")).
		Render(footer)

	return lipgloss.NewStyle().
		Width(a.width).
		Align(lipgloss.Center).
		Render(footerStyled)
}

func (a *App) applyRename(text string) tea.Cmd {
	pane := a.activePane()

	fi, ok := pane.SelectedItem()
	if !ok {
		return nil
	}

	if !fi.isRenamable() {
		return nil
	}

	if text == "" || text == fi.Info.Name {
		return nil
	}

	oldPath := fi.Info.FullPath

	return a.applyRenameInner(pane, oldPath, text)
}

func (a *App) applyRenameInner(pane *Pane, oldPath, name string) tea.Cmd {
	exp := pane.explorer

	// Compute new path/key
	newPath := exp.Join(exp.Dir(oldPath), name)

	// Perform backend rename
	if err := exp.Rename(a.ctx, oldPath, newPath); err != nil {
		return check(err)
	}

	pane.lastSelectedPath = newPath

	// Refresh both panes that show this directory
	a.refreshPanesForExplorer(exp)

	return statusf("Renamed %q to %q", oldPath, newPath)
}

func (a *App) applyCopy(ctx context.Context, text string, progress file.ProgressFunc) tea.Cmd {
	src, dst := a.panes()

	item, ok := src.SelectedItem()
	if !ok || !item.isCopyable() {
		return nil
	}

	if text != copyConfirmationText {
		return failure("confirmation text does not match")
	}

	return a.applyCopyInner(ctx, src, dst, item.Info.FullPath, item.Info.Name, progress)
}

func (a *App) applyCopyInner(ctx context.Context, src, dst *Pane, from, name string, progress file.ProgressFunc) tea.Cmd {
	if src.explorer.DeviceID(ctx) == dst.explorer.DeviceID(ctx) && src.explorer.Cwd(ctx) == dst.explorer.Cwd(ctx) {
		return nil
	}

	return func() tea.Msg {
		// 1. Download from source backend
		handle, err := src.explorer.Download(ctx, from, progress)
		if err != nil {
			return check(err)()
		}

		// We will collect ALL errors here
		var errs []string

		name = handle.Dest(name)

		// 2. Upload to destination backend
		dstPath := path.Join(dst.explorer.Cwd(ctx), name)

		if err := dst.explorer.UploadFrom(ctx, handle.Path(), dstPath, progress); err != nil {
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
			return NewLongErrorMsg(errs...)
		}
		return newStatusMsg(fmt.Sprintf("Copied %q to %q", from, dstPath))
	}
}
func (a *App) applyMove(ctx context.Context, text string, progress file.ProgressFunc) tea.Cmd {
	src, dst := a.panes()

	item, ok := src.SelectedItem()
	if !ok || !item.isMoveable() {
		return nil
	}

	if text != moveConfirmationText {
		return failure("confirmation text does not match")
	}

	return a.applyMoveInner(ctx, src, dst, item.Info.FullPath, item.Info.Name, progress)
}

func (a *App) applyMoveInner(ctx context.Context, src, dst *Pane, from, name string, progress file.ProgressFunc) tea.Cmd {
	if src.explorer.DeviceID(ctx) == dst.explorer.DeviceID(ctx) && src.explorer.Cwd(ctx) == dst.explorer.Cwd(ctx) {
		return nil
	}

	return func() tea.Msg {
		// 1. Download from source backend
		handle, err := src.explorer.Download(ctx, from, progress)
		if err != nil {
			return check(err)()
		}

		// We will collect ALL errors here
		var errs []string

		name = handle.Dest(name)

		// 2. Upload to destination backend
		dstPath := path.Join(dst.explorer.Cwd(ctx), name)
		if err := dst.explorer.UploadFrom(ctx, handle.Path(), dstPath, progress); err != nil {
			errs = append(errs, "Move failed: "+err.Error())
		}

		// 3. Always close the handle, even if upload failed
		if err := handle.Close(); err != nil {
			errs = append(errs, "Cleanup failed: "+err.Error())
		}

		// 4. Attempt to delete source (only if download/upload succeeded)
		if len(errs) == 0 {
			if err := src.explorer.Delete(ctx, from); err != nil {
				errs = append(errs, "Delete failed: "+err.Error())
			}
		}

		// 5. Refresh both panes that show the source and destination directories
		a.refreshPanesForExplorer(src.explorer)
		a.refreshPanesForExplorer(dst.explorer)

		// 6. If any errors occurred, show them
		if len(errs) > 0 {
			return NewLongErrorMsg(errs...)
		}

		return newStatusMsg(fmt.Sprintf("Moved %q to %q", from, dstPath))
	}
}

func (a *App) applyMakeDir(text string) tea.Cmd {
	active := a.activePane()
	newDirPath := path.Join(active.explorer.Cwd(a.ctx), text)

	if err := active.explorer.Mkdir(a.ctx, newDirPath); err != nil {
		return check(err)
	}

	// Refresh both panes that show this directory
	active.lastSelectedPath = newDirPath
	a.refreshPanesForExplorer(active.explorer)

	return statusf("Created directory %q", newDirPath)
}

func (a *App) applyDelete(text string) tea.Cmd {
	pane := a.activePane()
	item, ok := pane.SelectedItem()
	if !ok {
		return nil
	}

	if !item.isDeleteable() {
		return nil
	}

	if text != deleteConfirmationText {
		return failure("confirmation text does not match")
	}

	return a.applyDeleteInner(pane, item.Info.FullPath)
}

func (a *App) applyDeleteInner(pane *Pane, target string) tea.Cmd {
	if err := pane.explorer.Delete(a.ctx, target); err != nil {
		return check(err)
	}

	// Refresh both panes that show this directory
	a.refreshPanesForExplorer(pane.explorer)

	return statusf("Deleted %q", target)
}

func (a *App) applyChangeDevice(text string) tea.Cmd {
	pane := a.activePane()

	if text == "" || text == pane.name {
		return nil
	}

	exp, exists := a.devs[text]
	if !exists {
		return failure(fmt.Sprintf("Device %q not found. Available devices: %s", text, a.devsHint))
	}

	pane.explorer = exp.Copy()
	pane.name = text
	pane.lastSelectedPath = ""

	// Refresh both panes that show this directory
	pane.refresh()

	return statusf("Changed device to %q", text)
}

func (a *App) runOpen(pane *Pane, path string) (tea.Model, tea.Cmd) {
	if runtime.GOOS != "darwin" {
		return a, nil
	}

	handle, err := pane.explorer.Download(a.ctx, path, nil)
	if err != nil {
		return a, check(err)
	}

	cmd := exec.Command("open", "-W", handle.Path())

	return a, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return check(errors.Join(err, handle.Close()))()
	})
}

func (a *App) runRename() (tea.Model, tea.Cmd) {
	pane := a.activePane()
	if item, ok := pane.SelectedItem(); ok && item.isRenamable() {
		a.inputMode = inputRename
		a.textbox.SetValue(item.Info.Name)
		a.textbox.Placeholder = "New name"
		a.textbox.SetSuggestions(nil)
		a.textbox.Focus()
	}

	return a, nil
}

func (a *App) runCopy() (tea.Model, tea.Cmd) {
	src, dst := a.panes()

	if src.explorer.DeviceID(a.ctx) == dst.explorer.DeviceID(a.ctx) && src.explorer.Cwd(a.ctx) == dst.explorer.Cwd(a.ctx) {
		return a, nil
	}

	if item, ok := src.SelectedItem(); ok && item.isCopyable() {
		a.inputMode = inputConfirmCopy
		a.textbox.SetValue("")
		a.textbox.Placeholder = copyConfirmationText
		a.textbox.SetSuggestions([]string{copyConfirmationText})
		a.textbox.Focus()
	}

	return a, nil
}

func (a *App) runMove() (tea.Model, tea.Cmd) {
	src, dst := a.panes()

	if src.explorer.DeviceID(a.ctx) == dst.explorer.DeviceID(a.ctx) && src.explorer.Cwd(a.ctx) == dst.explorer.Cwd(a.ctx) {
		return a, nil
	}

	if item, ok := src.SelectedItem(); ok && item.isMoveable() {
		a.inputMode = inputConfirmMove
		a.textbox.SetValue("")
		a.textbox.Placeholder = moveConfirmationText
		a.textbox.SetSuggestions([]string{moveConfirmationText})
		a.textbox.Focus()
	}

	return a, nil
}

func (a *App) runMakeDir() (tea.Model, tea.Cmd) {
	a.inputMode = inputMkdir
	a.textbox.SetValue("")
	a.textbox.Placeholder = "Directory name"
	a.textbox.SetSuggestions(nil)
	a.textbox.Focus()

	return a, nil
}

func (a *App) runDelete() (tea.Model, tea.Cmd) {
	pane := a.activePane()
	if item, ok := pane.SelectedItem(); ok && item.isDeleteable() {
		a.inputMode = inputConfirmDelete
		a.textbox.SetValue("")
		a.textbox.Placeholder = deleteConfirmationText
		a.textbox.SetSuggestions([]string{deleteConfirmationText})
		a.textbox.Focus()
	}

	return a, nil
}
func (a *App) runView() (tea.Model, tea.Cmd) {
	pane := a.activePane()
	item, ok := pane.SelectedItem()
	if !ok || (!item.isViewable() && !item.isArchive()) {
		return a, nil
	}

	return a.runViewInner(pane, item.Info.FullPath)
}

func (a *App) runViewInner(pane *Pane, filename string) (tea.Model, tea.Cmd) {
	handle, err := pane.explorer.Download(a.ctx, filename, nil)
	if err != nil {
		return a, check(err)
	}

	var listArchive func(string) (string, error)

	switch {
	case strings.HasSuffix(filename, ".zip"):
		listArchive = listZip

	case strings.HasSuffix(filename, ".tar"),
		strings.HasSuffix(filename, ".tar.gz"),
		strings.HasSuffix(filename, ".tgz"):
		listArchive = listTarGz
	}

	if listArchive != nil {
		if s, err := listArchive(handle.Path()); err == nil {
			if err := handle.Close(); err != nil {
				return a, check(err)
			}
			return a.viewText(s)
		}
	}

	info, err := os.Stat(handle.Path())

	if err != nil {
		return a, check(errors.Join(err, handle.Close()))
	}

	if info.Size() > fileViewEditMaxSize {
		if err := handle.Close(); err != nil {
			return a, check(err)
		}
		return a, failuref(
			"File %q is too large to view (%s > %s)",
			filename,
			bytesFormatter(info.Size()),
			bytesFormatter(fileViewEditMaxSize),
		)
	}

	var cmd *exec.Cmd

	if jqErr == nil {
		cmd = exec.Command("sh", "-c",
			fmt.Sprintf("(jq . %q 2>/dev/null || cat %q) | less +1", handle.Path(), handle.Path()))
	} else {
		cmd = exec.Command("less", "+1", handle.Path())
	}

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
			return NewLongErrorMsg(errs...)
		}

		return nil
	})
}

func (a *App) runExtract() (tea.Model, tea.Cmd) {
	pane := a.activePane()

	if pane.explorer.Type() != local.Type {
		return a, nil
	}

	item, ok := pane.SelectedItem()
	if !ok || !item.isArchive() {
		return a, nil
	}

	filename := item.Info.FullPath

	pane.lastSelectedPath = filename

	switch {
	case strings.HasSuffix(filename, ".zip"):
		return a, commands["exec"].f(a, "unzip", "-o", filename)
	case strings.HasSuffix(filename, ".tar"):
		return a, commands["exec"].f(a, "tar", "-xf", filename)
	case strings.HasSuffix(filename, ".tgz"), strings.HasSuffix(filename, ".tar.gz"):
		return a, commands["exec"].f(a, "tar", "-xzf", filename)
	}

	return a, nil
}

func (a *App) runEdit(jq bool) (tea.Model, tea.Cmd) {
	pane := a.activePane()
	item, ok := pane.SelectedItem()
	if !ok {
		return a, nil
	}

	switch {
	case item.isArchive():
		return a.runExtract()
	case item.isEditable():
		return a.runEditInner(pane, item.Info.FullPath, jq)
	}

	return a, nil
}

func (a *App) runEditInner(pane *Pane, filename string, jq bool) (tea.Model, tea.Cmd) {
	handle, err := pane.explorer.Download(a.ctx, filename, nil)
	if err != nil {
		return a, check(err)
	}

	info, err := os.Stat(handle.Path())

	if err != nil {
		return a, check(errors.Join(err, handle.Close()))
	}

	if info.Size() > fileViewEditMaxSize {
		if err := handle.Close(); err != nil {
			return a, check(err)
		}
		return a, failuref(
			"File %q is too large to edit (%s > %s)",
			filename,
			bytesFormatter(info.Size()),
			bytesFormatter(fileViewEditMaxSize),
		)
	}

	cmd := execDefaultEditor(handle.Path(), jq)

	return a, tea.ExecProcess(cmd, func(procErr error) tea.Msg {
		var errs []string

		// 1. Editor error
		if procErr != nil {
			errs = append(errs, "Editor failed: "+procErr.Error())
		} else {
			// 2. Upload error (only if editor succeeded)
			if err := pane.explorer.UploadFrom(a.ctx, handle.Path(), filename, nil); err != nil {
				errs = append(errs, "Upload failed: "+err.Error())
			}
		}

		// 3. Cleanup error (always attempt)
		if err := handle.Close(); err != nil {
			errs = append(errs, "Cleanup failed: "+err.Error())
		}

		// 4. Return combined error or nil
		if len(errs) > 0 {
			return NewLongErrorMsg(errs...)
		}

		pane.lastSelectedPath = filename

		// Refresh both panes that show this directory
		a.refreshPanesForExplorer(pane.explorer)

		return nil
	})
}

func (a *App) viewText(text string) (tea.Model, tea.Cmd) {
	cmd := exec.Command("less", "+1")
	cmd.Stdin = strings.NewReader(text)

	return a, tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return check(err)()
		}
		return nil
	})
}

func (a *App) runHelp() (tea.Model, tea.Cmd) {
	return a.viewText(helpText)
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
	if !ok || item.isParentDir() {
		return a, nil
	}

	return a.runMetadataInner(pane, item.Info.FullPath)
}

func (a *App) runMetadataInner(pane *Pane, path string) (tea.Model, tea.Cmd) {
	metadata, err := pane.explorer.Metadata(a.ctx, path)

	if err != nil {
		return a, check(err)
	}

	if len(metadata) == 0 {
		return a, failure("No metadata available for " + path)
	}

	s := formatMetadata(metadata)

	cmd := exec.Command("less", "+1")
	cmd.Stdin = strings.NewReader(s)

	return a, tea.ExecProcess(cmd, execCheck())
}

func (a *App) runChangeDevice() (tea.Model, tea.Cmd) {
	if len(a.devs) > 1 {
		a.inputMode = inputChangeDevice
		a.textbox.SetValue("")
		a.textbox.SetSuggestions(slices.Collect(maps.Keys(a.devs)))
		a.textbox.Placeholder = a.devsHint
		a.textbox.Focus()
	}

	return a, nil
}

func (a *App) runSwapDevices() (tea.Model, tea.Cmd) {
	if len(a.devs) > 1 && a.left.name != a.right.name {
		a.left, a.right = a.right, a.left
		a.focus = 1 - a.focus // switch focus to the other pane

		// Refresh both panes to reflect new devices
		a.refreshPanesForExplorer(a.left.explorer)
		a.refreshPanesForExplorer(a.right.explorer)
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

func execDefaultEditor(path string, jq bool) *exec.Cmd {
	for _, ed := range editors {
		if _, err := exec.LookPath(ed); err == nil {
			// Editor exists
			if jq && jqErr == nil && ed == "vi" {
				// Only vi supports the jq filter command
				return exec.Command("vi", path, "-c", "silent %!jq .")
			}

			// Normal open
			return exec.Command(ed, path)
		}
	}

	// No editor found → return a no-op command
	return exec.Command("true")
}
