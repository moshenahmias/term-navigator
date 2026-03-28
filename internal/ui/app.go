package ui

import (
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

	out := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftBox,
		rightBox,
	)

	return tea.NewView(out)
}
