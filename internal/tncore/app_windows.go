//go:build windows

package tncore

import "os/exec"

func runPager(path string) *exec.Cmd {
	return exec.Command("cmd", "/c", "type", path+"| more +1")
}

func runEditor(path string, jq bool) *exec.Cmd {
	return exec.Command("notepad", path)
}
