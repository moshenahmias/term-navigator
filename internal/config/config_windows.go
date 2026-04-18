//go:build windows

package config

import (
	"fmt"
	"os"
)

func defaultConfigName() string {
	return ".termnav"
}

var Default = Config{
	Devices: []DeviceConfig{DefaultDevice},
	Left:    DefaultDevice.Name + "/C",
	Right:   DefaultDevice.Name + "/C",
}

func init() {
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		DefaultDevice.Path = cwd
		Default.Devices[0] = DefaultDevice
		Default.Left = fmt.Sprintf("%s/%s", DefaultDevice.Name, cwd[0:1])
		Default.Right = Default.Left
	}
}
