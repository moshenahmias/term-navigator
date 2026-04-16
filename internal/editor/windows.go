//go:build windows

package editor

import (
	"os/exec"
)

func RunPager(path string) *exec.Cmd {
	return exec.Command("cmd", "/c", "type", path+"| more +1")
}

func RunEditor(path string, jq bool) *exec.Cmd {
	return exec.Command("notepad", path)
}

func RunShell() *exec.Cmd {
	return exec.Command("cmd", "/k")
}
