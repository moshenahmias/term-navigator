//go:build unix || darwin

package tncore

import (
	"os/exec"
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
