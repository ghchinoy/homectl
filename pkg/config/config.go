package config

import (
	"os"
	"path/filepath"
)

// GetConfigDir returns the path to the configuration directory (~/.config/homectl)
func GetConfigDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, ".config", "homectl")
	}
	return filepath.Join(configDir, "homectl")
}

// GetPath returns a path relative to the configuration directory
func GetPath(filename string) string {
	return filepath.Join(GetConfigDir(), filename)
}

// EnsureDir ensures the configuration directory exists
func EnsureDir() error {
	return os.MkdirAll(GetConfigDir(), 0755)
}
