package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	FirmwarePath string `json:"firmware_path"`
	SerialPort   string `json:"serial_port"`
	Interface    string `json:"interface"`
	Country      string `json:"country"`
	ServerIP     string `json:"server_ip"`
	APBootIP     string `json:"apboot_ip"`
	APNetmask    string `json:"ap_netmask"`
}

func Defaults() Config {
	return Config{
		SerialPort: "/dev/cu.usbserial-A9JPRHL1",
		Interface:  "en7",
		Country:    "US",
		ServerIP:   "169.254.141.5",
		APBootIP:   "169.254.141.6",
		APNetmask:  "255.255.255.0",
	}
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "iap325-converter", "config.json"), nil
}

func Load() Config {
	cfg := Defaults()
	path, err := Path()
	if err != nil {
		return cfg
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	return cfg
}

func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}
