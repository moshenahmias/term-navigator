package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

var (
	DefaultConfigName = ".termnav"
	LocalType         = "local"
	DefaultType       = LocalType
	Types             = []string{LocalType, "s3"}
)

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

	// TLS
	InsecureSkipVerify bool   `json:"insecure,omitempty"`
	CAFile             string `json:"ca_file,omitempty"`
	ExpectedCertName   string `json:"expected_cert_name,omitempty"`
}

var DefaultDevice = DeviceConfig{
	Name: DefaultType,
	Type: DefaultType,
}

var Default = Config{
	Devices: []DeviceConfig{DefaultDevice},
	Left:    DefaultDevice.Name,
	Right:   DefaultDevice.Name,
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	path := filepath.Join(home, DefaultConfigName)
	return path, nil
}

func Load(path string) (*Config, error) {
	if path == "" {
		return &Default, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Default, nil
		}
		return &Default, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &Default, err
	}

	var found bool

	for _, d := range cfg.Devices {
		if found = d.Type == DefaultType; found {
			break
		}
	}

	if !found {
		cfg.Devices = append(cfg.Devices, DefaultDevice)
	}

	if cfg.Left == "" {
		cfg.Left = cfg.Devices[0].Name
	}

	if cfg.Right == "" {
		cfg.Right = cfg.Devices[0].Name
	}

	return &cfg, nil
}
