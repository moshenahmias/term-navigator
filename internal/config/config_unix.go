//go:build unix || darwin

package config

func defaultConfigName() string {
	return ".termnav"
}
