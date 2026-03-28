package ui

import (
	"fmt"
	"path/filepath"

	"github.com/moshenahmias/term-navigator/internal/explorer"

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
	left  Pane
	right Pane
	focus int // 0 = left, 1 = right
}

func NewApp(leftExp, rightExp explorer.FileExplorer, width, height int) App {
	half := width / 2
	return App{
		left:  NewPane(leftExp, half, height),
		right: NewPane(rightExp, half, height),
		focus: 0,
	}
}

func (a App) Init() tea.Cmd { return nil }

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		totalWidth := msg.Width
		totalHeight := msg.Height

		// subtract 2 columns for each pane border
		paneWidth := (totalWidth / 2) - 2

		// subtract 2 rows for top/bottom border
		paneHeight := totalHeight - 2

		a.left.Resize(paneWidth, paneHeight)
		a.right.Resize(paneWidth, paneHeight)

		return a, nil

	case tea.KeyMsg:
		switch msg.String() {

		case "tab":
			a.focus = 1 - a.focus
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

func (a App) View() tea.View {
	left := ncPaneStyle.
		Width(a.left.width).
		Height(a.left.height).
		MaxHeight(a.left.height).
		Padding(0).
		Align(lipgloss.Top).
		Render(a.left.View())

	right := ncPaneStyle.
		Width(a.right.width).
		Height(a.right.height).
		MaxHeight(a.right.height).
		Padding(0).
		Align(lipgloss.Top).
		Render(a.right.View())

	leftBox := ncBorder.Render(left)
	rightBox := ncBorder.Render(right)

	panes := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftBox,
		rightBox,
	)

	footer := a.commandBar()

	// Join vertically: panes above, footer below
	out := lipgloss.JoinVertical(
		lipgloss.Left,
		panes,
		footer,
	)

	return tea.NewView(out)
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
