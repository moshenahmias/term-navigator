//go:build unix || darwin

package config

import (
	"os"
)

func defaultConfigName() string {
	return ".termnav"
}

var Default = Config{
	Devices: []DeviceConfig{DefaultDevice},
	Left:    DefaultDevice.Name,
	Right:   DefaultDevice.Name,
}

func init() {
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		DefaultDevice.Path = cwd
		Default.Devices[0] = DefaultDevice
	}
}
