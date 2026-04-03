package ui

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/moshenahmias/term-navigator/internal/file"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type ncDelegate struct {
	normalStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	active        bool
}

func (d ncDelegate) Height() int  { return 1 }
func (d ncDelegate) Spacing() int { return 0 }
func (d ncDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d ncDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	fi, ok := item.(*FileItem)
	if !ok {
		return
	}

	selected := index == m.Index()

	var style lipgloss.Style
	if selected && d.active {
		style = d.selectedStyle
	} else {
		style = d.normalStyle
	}

	title := fi.Title()
	desc := fi.Description()

	if fi.Info.IsSymlink {
		title += " ↪"
	}

	// REAL usable width for charm.land/bubbles/v2
	innerWidth := m.Width() - 2
	if innerWidth < 5 { // safety for tiny widths during resize
		innerWidth = 5
	}

	// Measure original widths
	titleW := lipgloss.Width(title)
	descW := lipgloss.Width(desc)

	// If everything fits, no truncation needed
	if titleW+1+descW <= innerWidth {
		spaces := innerWidth - titleW - descW
		gap := strings.Repeat(" ", spaces)
		dimDesc := style.Foreground(lipgloss.Color("8")).Render(desc)
		fmt.Fprint(w, style.Render(title+gap+dimDesc))
		return
	}

	// Not enough space → shrink the longer one first
	available := innerWidth - 1 // reserve 1 space gap

	if titleW > descW {
		// shrink title first
		title = truncate(title, available-descW)
	} else {
		// shrink desc first
		desc = truncate(desc, available-titleW)
	}

	// Recompute widths
	titleW = lipgloss.Width(title)
	descW = lipgloss.Width(desc)

	// If still too long, truncate both proportionally
	if titleW+1+descW > innerWidth {
		titleMax := innerWidth / 2
		descMax := innerWidth - titleMax - 1

		title = truncate(title, titleMax)
		desc = truncate(desc, descMax)

		titleW = lipgloss.Width(title)
		descW = lipgloss.Width(desc)
	}

	// Final spacing
	spaces := innerWidth - titleW - descW
	if spaces < 1 {
		spaces = 1
	}
	gap := strings.Repeat(" ", spaces)

	dimDesc := style.Foreground(lipgloss.Color("8")).Render(desc)
	line := title + gap + dimDesc

	fmt.Fprint(w, style.Render(line))
}

type FileItem struct {
	Info file.Info
}

func (f *FileItem) isDeleteable() bool {
	return !f.Info.IsDir || f.Info.Name != ".."
}

func (f *FileItem) isRenamable() bool {
	return !f.Info.IsDir || f.Info.Name != ".."
}

func (f *FileItem) isCopyable() bool {
	return !f.Info.IsDir || f.Info.Name != ".."
}

func (f *FileItem) isMoveable() bool {
	return f.isCopyable() && f.isDeleteable()
}

func (f *FileItem) isViewable() bool {
	return !f.Info.IsDir && !f.Info.IsSymlink
}

func (f *FileItem) isEditable() bool {
	return !f.Info.IsDir && !f.Info.IsSymlink
}

func (f *FileItem) hasMetadata() bool {
	return !f.Info.IsDir || f.Info.Name != ".."
}

func (f *FileItem) TitleNoIcons() string {
	name := f.Info.Name

	if f.Info.IsDir && name != ".." {
		return name + "/"
	}

	return name
}

func (f *FileItem) Title() string {
	name := f.Info.Name

	var icon string
	switch {
	case f.Info.IsSymlink:
		icon = "🔗"
	case f.Info.IsDir:
		icon = "📁"
	default:
		icon = "📄"
	}

	// Force icon to stable width
	icon = lipgloss.NewStyle().
		Width(2).     // always 2 columns
		Inline(true). // prevent reflow
		Render(icon)

	// Add slash for directories
	if f.Info.IsDir && name != ".." {
		name += "/"
	}

	return icon + " " + name
}

func (f *FileItem) Description() string {
	t := f.Info.Modified.Local()

	if f.Info.IsDir && f.Info.Size == 0 {
		if t.IsZero() {
			return ""
		}

		return t.Format("2006-01-02 15:04") + " "
	}

	if t.IsZero() {
		return bytesFormatter(f.Info.Size)
	}

	return fmt.Sprintf("%s • %s ",
		bytesFormatter(f.Info.Size),
		t.Format("2006-01-02 15:04"))
}

func (f *FileItem) FilterValue() string { return f.Info.Name }

type Pane struct {
	explorer         file.Explorer
	list             list.Model
	width            int
	height           int
	lastSelectedPath string
	delegate         *ncDelegate
	ctx              context.Context
	name             string
}

func NewPane(ctx context.Context, name string, exp file.Explorer, width, height int) *Pane {
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

	styles := list.DefaultStyles(true)

	l.Styles = styles
	l.Title = exp.PrintableCwd(ctx)

	// Build pane
	p := &Pane{
		explorer: exp,
		list:     l,
		width:    width,
		height:   height,
		delegate: &d,
		ctx:      ctx,
		name:     name,
	}

	p.refresh()
	return p
}

func (p *Pane) refresh() {
	items, err := p.explorer.List(p.ctx)
	if err != nil {
		p.list.SetItems([]list.Item{})
		return
	}

	sort.Slice(items, func(i, j int) bool {
		a := items[i]
		b := items[j]

		// Directories first
		if a.IsDir != b.IsDir {
			return a.IsDir
		}

		// Then alphabetical
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})

	li := make([]list.Item, 0, len(items)+1)

	selectedIndex := 0 // default to first item

	// Add ".." only if not at filesystem root
	if !p.explorer.IsRoot(p.ctx) {
		upItem := &FileItem{
			Info: file.Info{
				Name:     "..",
				FullPath: "..",
				IsDir:    true,
			},
		}

		li = append(li, upItem)

		// If renamed item is ".." (unlikely), match here
		if p.lastSelectedPath == ".." {
			selectedIndex = 0
		}
	}

	// Add real items
	for i, fi := range items {
		item := &FileItem{Info: fi}
		li = append(li, item)

		// 🔥 If this item matches the renamed path, remember its index
		if p.lastSelectedPath != "" && fi.FullPath == p.lastSelectedPath {
			if p.explorer.IsRoot(p.ctx) {
				selectedIndex = i
			} else {
				selectedIndex = i + 1 // +1 because of ".."
			}
		}
	}

	p.list.SetItems(li)
	p.list.Select(selectedIndex)

	// Clear remembered path
	p.lastSelectedPath = ""
}

func (p *Pane) Init() tea.Cmd { return nil }

func (p *Pane) Update(msg tea.Msg) (*Pane, tea.Cmd) {
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func truncate(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	return string(r[:width-1]) + "…"
}

func (p *Pane) View() string {
	cwd := p.name + " " + p.explorer.PrintableCwd(p.ctx)
	cwd = truncate(cwd, p.width-2) // account for borders

	header := lipgloss.NewStyle().
		Bold(true).
		Inline(true).Faint(!p.delegate.active).
		Render(cwd)

	body := p.list.View()

	return header + "\n" + body
}

// Selected returns the FileInfo of the currently selected item.
func (p *Pane) Selected() (file.Info, error) {
	item, ok := p.list.SelectedItem().(*FileItem)
	if !ok {
		return file.Info{}, fmt.Errorf("no selection")
	}
	return item.Info, nil
}

func (p *Pane) Resize(width, height int) {
	p.width = width
	p.height = height

	// list must be smaller than pane so border has room
	p.list.SetSize(width, height-3)
	p.list.SetShowHelp(p.width > 46) // hide help on very narrow widths
}

func (p *Pane) SelectedItem() (*FileItem, bool) {
	item := p.list.SelectedItem()
	if item == nil {
		return nil, false
	}

	fi, ok := item.(*FileItem)
	return fi, ok
}

func (p *Pane) SetActive(active bool) {
	p.delegate.active = active
	p.list.SetDelegate(*p.delegate)

	// 🔥 Re-apply styles so help/status/title don't disappear
	styles := list.DefaultStyles(true)
	p.list.Styles = styles
}
