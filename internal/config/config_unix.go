//go:build unix || darwin

package config

func defaultConfigName() string {
	return ".termnav"
}

var Default = Config{
	Devices: []DeviceConfig{DefaultDevice},
	Left:    DefaultDevice.Name,
	Right:   DefaultDevice.Name,
}
