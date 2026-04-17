//go:build windows

package config

func defaultConfigName() string {
	return ".termnav"
}

var Default = Config{
	Devices: []DeviceConfig{DefaultDevice},
	Left:    DefaultDevice.Name + "/C",
	Right:   DefaultDevice.Name + "/C",
}
