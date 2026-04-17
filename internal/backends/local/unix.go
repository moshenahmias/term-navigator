//go:build unix || darwin

package local

import (
	"context"
	"fmt"
	"os"
	"syscall"
)

func platformNormalizePath(path string) string {
	return path
}

func platformIsRoot(path string) bool {
	return path == "/"
}

func platformAddMetadata(meta map[string]string, info os.FileInfo) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		meta["UID"] = fmt.Sprintf("%d", stat.Uid)
		meta["GID"] = fmt.Sprintf("%d", stat.Gid)
	}
}

func (l *explorer) DeviceID(context.Context) string {
	return Type
}
