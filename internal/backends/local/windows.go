//go:build windows

package local

import (
	"os"
	"strings"
	"syscall"
	"unsafe"
)

func platformNormalizePath(path string) string {
	path = strings.ToUpper(path)
	if len(path) == 2 && path[1] == ':' {
		return path + "\\"
	}
	return path
}

func platformIsRoot(path string) bool {
	if len(path) == 3 && path[1] == ':' {
		c := path[0]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			return path[2] == '\\'
		}
	}
	return path == "\\"
}

func platformAddMetadata(meta map[string]string, info os.FileInfo) {
}

func PlatformDetectDrives() []string {
	var drives []string
	for i := 0; i < 26; i++ {
		letter := rune('A' + i)
		path := string(letter) + ":\\"

		kernel32 := syscall.MustLoadDLL("kernel32.dll")
		getDriveType, err := kernel32.FindProc("GetDriveTypeW")
		if err != nil {
			continue
		}

		pathPtr, _ := syscall.UTF16PtrFromString(path)
		dt, _, _ := getDriveType.Call(uintptr(unsafe.Pointer(pathPtr)))

		// 2 = DRIVE_REMOVABLE, 3 = DRIVE_FIXED, 4 = DRIVE_REMOTE, 5 = DRIVE_CDROM
		if dt >= 3 && dt <= 5 {
			drives = append(drives, string(letter))
		}
	}
	return drives
}
