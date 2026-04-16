//go:build unix || darwin

package editor

import (
	"os/exec"
)

func RunPager(path string) *exec.Cmd {
	return exec.Command("less", "+1", path)
}

func RunEditor(path string, jq bool) *exec.Cmd {
	if jq {
		return exec.Command("vi", path, "-c", "silent %!jq .")
	}
	return exec.Command("vi", path)
}

func RunShell() *exec.Cmd {
	return exec.Command("sh")
}
