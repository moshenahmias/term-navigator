//go:build windows

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/moshenahmias/term-navigator/internal/backends/local"
	appcfg "github.com/moshenahmias/term-navigator/internal/config"
	"github.com/moshenahmias/term-navigator/internal/file"
)

func init() {
	constructors["local"] = func(dev *appcfg.DeviceConfig) file.ExplorerConstructor {
		return func(ctx context.Context) (map[string]file.Explorer, error) {
			drives := dev.Drives
			if len(drives) == 0 {
				drives = local.PlatformDetectDrives()
			}

			if len(drives) == 0 {
				return nil, fmt.Errorf("no drives found")
			}

			if len(drives) == 1 && dev.Path == "" {
				path := strings.ToUpper(drives[0]) + ":\\"
				return map[string]file.Explorer{dev.Name: local.NewExplorer(path)}, nil
			}

			explorers := make(map[string]file.Explorer, len(drives))
			for _, drive := range drives {
				drive = strings.ToUpper(drive)
				path := drive + ":\\"
				name := fmt.Sprintf("%s/%s", dev.Name, drive)
				explorers[name] = local.NewExplorer(path)
			}

			return explorers, nil
		}
	}
}
