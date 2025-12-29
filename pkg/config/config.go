package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AppConfig represents the application configuration
type AppConfig struct {
	CallbackIP string `json:"callback_ip"` // Manual override for GENA listener
}

// GetConfigDir returns the path to the configuration directory (~/.config/homectl)
func GetConfigDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, ".config", "homectl")
	}
	return filepath.Join(configDir, "homectl")
}

// LoadConfig loads the application configuration from config.json
func LoadConfig() AppConfig {
	var cfg AppConfig
	data, err := os.ReadFile(filepath.Join(GetConfigDir(), "config.json"))
	if err == nil {
		json.Unmarshal(data, &cfg)
	}
	return cfg
}

// GetPath returns a path relative to the configuration directory
func GetPath(filename string) string {
	return filepath.Join(GetConfigDir(), filename)
}

// EnsureDir ensures the configuration directory exists
func EnsureDir() error {
	return os.MkdirAll(GetConfigDir(), 0755)
}