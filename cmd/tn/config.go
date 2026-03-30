package main

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

var defaultConfig = Config{
	Devices: []DeviceConfig{
		{
			Name: "default",
			Type: "local",
		},
	},
	Left:  "default",
	Right: "default",
}

func LoadConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(home, defaultConfigName)

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
