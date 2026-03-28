package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/moshenahmias/term-navigator/internal/explorer"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type ncDelegate struct {
	normalStyle   lipgloss.Style
	selectedStyle lipgloss.Style
}

func (d ncDelegate) Height() int  { return 1 }
func (d ncDelegate) Spacing() int { return 0 }
func (d ncDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func padToWidth(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func (d ncDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	fi, ok := item.(FileItem)
	if !ok {
		return
	}

	selected := index == m.Index()

	var style lipgloss.Style
	if selected {
		style = d.selectedStyle
	} else {
		style = d.normalStyle
	}

	line := fi.Title()

	if fi.Info.IsSymlink {
		line += " ↪"
	}

	line = padToWidth(line, m.Width())

	fmt.Fprint(w, style.Render(line))
}

type FileItem struct {
	Info explorer.FileInfo
}

func (f FileItem) Title() string       { return f.Info.Name }
func (f FileItem) Description() string { return "" }
func (f FileItem) FilterValue() string { return f.Info.Name }

type Pane struct {
	explorer explorer.FileExplorer
	list     list.Model
	width    int
	height   int
}

func NewPane(exp explorer.FileExplorer, width, height int) Pane {
	// Create delegate with NC styles
	d := ncDelegate{
		normalStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")),

		selectedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")). // black text
			Background(lipgloss.Color("#FFFFFF")). // white background
			Bold(true),
	}

	// Create list
	l := list.New([]list.Item{}, d, width, height)

	l.SetShowTitle(false)
	//l.SetShowStatusBar(false)
	//l.SetFilteringEnabled(false)
	//l.SetShowHelp(false)

	styles := list.DefaultStyles(true)

	l.Styles = styles
	l.Title = exp.Cwd()

	// Build pane
	p := Pane{
		explorer: exp,
		list:     l,
		width:    width,
		height:   height,
	}

	p.refresh()
	return p
}

func (p *Pane) refresh() {
	items, err := p.explorer.List()
	if err != nil {
		p.list.SetItems([]list.Item{})
		return
	}

	li := make([]list.Item, 0, len(items)+1)

	// Add ".." only if not at filesystem root
	if p.explorer.Cwd() != "/" {
		li = append(li, FileItem{
			Info: explorer.FileInfo{
				Name:     "..",
				FullPath: "..",
				IsDir:    true,
			},
		})
	}

	for _, fi := range items {
		li = append(li, FileItem{Info: fi})
	}

	p.list.SetItems(li)
	p.list.Title = p.explorer.Cwd()
}

func (p Pane) Init() tea.Cmd { return nil }

func (p Pane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func (p Pane) View() string {
	header := padToWidth(p.explorer.Cwd(), p.width)
	header = lipgloss.NewStyle().
		Bold(true).
		Render(header)

	body := p.list.View()

	return header + "\n" + body
}

// Selected returns the FileInfo of the currently selected item.
func (p Pane) Selected() (explorer.FileInfo, error) {
	item, ok := p.list.SelectedItem().(FileItem)
	if !ok {
		return explorer.FileInfo{}, fmt.Errorf("no selection")
	}
	return item.Info, nil
}

func (p *Pane) Resize(width, height int) {
	p.width = width
	p.height = height

	// list must be smaller than pane so border has room
	p.list.SetSize(width, height-2)
}
