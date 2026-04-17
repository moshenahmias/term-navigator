//go:build unix || darwin

package tncore

import (
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func runPager(path string) *exec.Cmd {
	return exec.Command("less", "+1", path)
}

func runEditor(path string, jq bool) *exec.Cmd {
	if jq {
		return exec.Command("vi", path, "-c", "silent %!jq .")
	}
	return exec.Command("vi", path)
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

func (a *App) runExtract() (tea.Model, tea.Cmd) {
	pane := a.activePane()

	if !isLocal(pane.explorer) {
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