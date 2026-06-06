package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
	cfg := Config{
		Country:   "US",
		ServerIP:  "169.254.141.5",
		APBootIP:  "169.254.141.6",
		APNetmask: "255.255.255.0",
	}
	switch runtime.GOOS {
	case "windows":
		cfg.SerialPort = "COM3"
		cfg.Interface = "Ethernet"
	case "darwin":
		cfg.SerialPort = "/dev/cu.usbserial-A9JPRHL1"
		cfg.Interface = "en7"
	default:
		cfg.SerialPort = "/dev/ttyUSB0"
		cfg.Interface = "eth0"
	}
	return cfg
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
