package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const defaultConfigName = ".termnav"

type Config struct {
	Devices []DeviceConfig `json:"devices"`
	Left    string         `json:"left,omitempty"`
	Right   string         `json:"right,omitempty"`
}

type DeviceConfig struct {
	Name string `json:"name"`
	Type string `json:"type"`

	// Local
	Path string `json:"path,omitempty"`

	// S3
	Bucket   string `json:"bucket,omitempty"`
	Region   string `json:"region,omitempty"`
	Prefix   string `json:"prefix,omitempty"`
	Key      string `json:"key,omitempty"`
	Secret   string `json:"secret,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
}

var Default = Config{
	Devices: []DeviceConfig{
		{
			Name: "default",
			Type: "local",
		},
	},
	Left:  "default",
	Right: "default",
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	path := filepath.Join(home, defaultConfigName)
	return path, nil
}

func Load() (*Config, error) {
	path, err := Path()

	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
