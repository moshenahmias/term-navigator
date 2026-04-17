//go:build windows

package tncore

import (
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func runPager(path string) *exec.Cmd {
	return runEditor(path, false)
}

func runEditor(path string, _ bool) *exec.Cmd {
	return exec.Command("notepad", path)
}

func (a *App) viewText(text string) (tea.Model, tea.Cmd) {
	tmp, _ := os.CreateTemp("", "termnav-*.txt")
	tmp.WriteString(text)
	tmp.Close()

	cmd := exec.Command("notepad.exe", tmp.Name())

	return a, tea.ExecProcess(cmd, func(err error) tea.Msg {
		// Delete immediately after launching
		return NewLongErrorMsgFromErrors(os.Remove(tmp.Name()), err)
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
	case strings.HasSuffix(filename, ".zip"), strings.HasSuffix(filename, ".tgz"), strings.HasSuffix(filename, ".tar"), strings.HasSuffix(filename, ".tar.gz"):
		return a, commands["exec"].f(a, "cmd", "/c", "tar", "-xf", filename)
	}

	return a, nil
}
