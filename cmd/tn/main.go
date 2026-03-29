package main

import (
	"context"

	"github.com/moshenahmias/term-navigator/internal/backends/local"
	"github.com/moshenahmias/term-navigator/internal/ui"

	tea "charm.land/bubbletea/v2"
)

func main() {
	ctx := context.Background()
	left := local.NewExplorer(".")
	right := local.NewExplorer(".")

	p := tea.NewProgram(ui.NewApp(ctx, left, right, 120, 30))

	if _, err := p.Run(); err != nil {
		panic(err)
	}

}
