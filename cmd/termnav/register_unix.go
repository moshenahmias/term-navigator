//go:build unix || darwin

package main

import (
	"context"

	"github.com/moshenahmias/term-navigator/internal/backends/local"
	appcfg "github.com/moshenahmias/term-navigator/internal/config"
	"github.com/moshenahmias/term-navigator/internal/file"
)

func init() {
	constructors["local"] = func(dev *appcfg.DeviceConfig) file.ExplorerConstructor {
		return func(ctx context.Context) (map[string]file.Explorer, error) {
			path := dev.Path
			if path == "" {
				path = "."
			}
			return map[string]file.Explorer{dev.Name: local.NewExplorer(path)}, nil
		}
	}
}
