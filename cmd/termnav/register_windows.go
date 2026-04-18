//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
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

			cwd, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("cannot determine current directory: %w", err)
			}

			if len(drives) == 1 && dev.Path == "" {
				path := strings.ToUpper(drives[0]) + ":\\"

				if dev.Path != "" && strings.HasPrefix(dev.Path, path) {
					path = dev.Path
				} else if strings.HasPrefix(cwd, path) {
					path = cwd
				}

				return map[string]file.Explorer{dev.Name: local.NewExplorer(path)}, nil
			}

			explorers := make(map[string]file.Explorer, len(drives))
			for _, drive := range drives {
				drive = strings.ToUpper(drive)
				path := drive + ":\\"
				name := fmt.Sprintf("%s/%s", dev.Name, drive)

				if dev.Path != "" && strings.HasPrefix(dev.Path, path) {
					path = dev.Path
				} else if strings.HasPrefix(cwd, path) {
					path = cwd
				}

				explorers[name] = local.NewExplorer(path)
			}

			return explorers, nil
		}
	}
}
