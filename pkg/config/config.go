// Package config provides configuration directory and nickname management conforming to the XDG specification.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AppConfig represents the application configuration
type AppConfig struct {
	CallbackIP          string `json:"callback_ip"`           // Manual override for GENA listener
	CameraAuth          string `json:"camera_auth"`           // global user:pass for cameras
	LutronBridge        string `json:"lutron_bridge"`         // Lutron Caseta / RA2 Select bridge IP address
	SonosDefaultService string `json:"sonos_default_service"` // Default music service (e.g. Spotify, Apple Music)
	APIHost             string `json:"api_host,omitempty"`    // Host/IP address for API server to bind to (e.g. 127.0.0.1, 0.0.0.0, or Tailscale IP)
}

// ConfigDir returns the path to the configuration directory (~/.config/homectl).
func ConfigDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, ".config", "homectl")
	}
	return filepath.Join(configDir, "homectl")
}

// GetConfigDir returns the path to the configuration directory (~/.config/homectl).
// Deprecated: use ConfigDir instead.
func GetConfigDir() string {
	return ConfigDir()
}

// LoadConfig loads the application configuration from config.json.
func LoadConfig() AppConfig {
	var cfg AppConfig
	data, err := os.ReadFile(filepath.Join(ConfigDir(), "config.json"))
	if err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	return cfg
}

// Path returns a path relative to the configuration directory.
func Path(filename string) string {
	return filepath.Join(ConfigDir(), filename)
}

// GetPath returns a path relative to the configuration directory.
// Deprecated: use Path instead.
func GetPath(filename string) string {
	return Path(filename)
}

// EnsureDir ensures the configuration directory exists.
func EnsureDir() error {
	return os.MkdirAll(ConfigDir(), 0700)
}

// LoadNicknames loads the device nicknames from nicknames.json.
func LoadNicknames() map[string]string {
	data, err := os.ReadFile(GetPath("nicknames.json"))
	if err != nil {
		return make(map[string]string)
	}
	var nicknames map[string]string
	if err := json.Unmarshal(data, &nicknames); err != nil || nicknames == nil {
		return make(map[string]string)
	}
	return nicknames
}

// SaveNicknames saves the device nicknames to nicknames.json.
func SaveNicknames(nicknames map[string]string) error {
	data, err := json.MarshalIndent(nicknames, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(GetPath("nicknames.json"), data, 0644)
}
