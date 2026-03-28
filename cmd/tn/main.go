package main

import (
	"github.com/moshenahmias/term-navigator/internal/backends/local"
	"github.com/moshenahmias/term-navigator/internal/ui"

	tea "charm.land/bubbletea/v2"
)

func main() {
	left := local.NewLocalExplorer(".")
	right := local.NewLocalExplorer(".")

	p := tea.NewProgram(ui.NewApp(left, right, 120, 30))

	if _, err := p.Run(); err != nil {
		panic(err)
	}

}
