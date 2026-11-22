// Package config provides configuration directory and nickname management conforming to the XDG specification.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// RoborockCertPair configures a PEM certificate/key pair for a Roborock SNI hostname.
type RoborockCertPair struct {
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
}

// RoborockTURNConfig configures the optional WebRTC TURN/STUN relay for camera-equipped vacuums.
type RoborockTURNConfig struct {
	Mode     string `json:"mode,omitempty"`      // disabled | embedded | external
	BindHost string `json:"bind_host,omitempty"` // default 0.0.0.0
	Host     string `json:"host,omitempty"`      // reachable hostname or IP
	PublicIP string `json:"public_ip,omitempty"` // optional IPv4 override for embedded relay
	Port     int    `json:"port,omitempty"`      // default 3478
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Realm    string `json:"realm,omitempty"`
}

// RoborockConfig stores local Roborock cloud emulator and fleet instrumentation settings.
type RoborockConfig struct {
	StackFQDN      string                      `json:"stack_fqdn,omitempty"`      // e.g. "api-r.example.com"
	BindHost       string                      `json:"bind_host,omitempty"`       // default "0.0.0.0"
	HTTPSPort      int                         `json:"https_port,omitempty"`      // default 443
	AdvertisedPort int                         `json:"advertised_port,omitempty"` // optional external HTTPS port
	MQTTTLSPort    int                         `json:"mqtt_tls_port,omitempty"`   // default 8881
	Region         string                      `json:"region,omitempty"`          // default "us"
	Timezone       string                      `json:"timezone,omitempty"`        // e.g. "America/Denver"
	CameraDomain   string                      `json:"camera_domain,omitempty"`   // <=14-char hostname for Saros 10R/20 createca
	CertsBySNI     map[string]RoborockCertPair `json:"certs_by_sni,omitempty"`    // SNI hostname or "*" -> cert/key
	AllowLegacyTLS bool                        `json:"allow_legacy_tls,omitempty"`
	KeyRecoverCmd  string                      `json:"key_recover_cmd,omitempty"`
	TraceJSONL     bool                        `json:"trace_jsonl,omitempty"`
	ControlAPIURL  string                      `json:"control_api_url,omitempty"` // e.g. "http://127.0.0.1:8889"
	TURN           RoborockTURNConfig          `json:"turn,omitempty"`
}

// AppConfig represents the application configuration
type AppConfig struct {
	CallbackIP          string          `json:"callback_ip"`            // Manual override for GENA listener
	CameraAuth          string          `json:"camera_auth"`            // global user:pass for cameras
	LutronBridge        string          `json:"lutron_bridge"`          // Lutron Caseta / RA2 Select bridge IP address
	SonosDefaultService string          `json:"sonos_default_service"`  // Default music service (e.g. Spotify, Apple Music)
	APIHost             string          `json:"api_host,omitempty"`     // Host/IP address for API server to bind to (e.g. 127.0.0.1, 0.0.0.0, or Tailscale IP)
	QolsysHost          string          `json:"qolsys_host,omitempty"`  // Qolsys IQ Panel IP address
	QolsysToken         string          `json:"qolsys_token,omitempty"` // Qolsys IQ Panel 6-digit access token
	Roborock            *RoborockConfig `json:"roborock,omitempty"`     // Local Roborock server & fleet configuration
}

// ConfigDir returns the path to the configuration directory, honoring XDG_CONFIG_HOME
// across all platforms (including macOS) before falling back to os.UserConfigDir().
func ConfigDir() string {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "homectl")
	}
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
	baseDir := ConfigDir()
	cleanName := filepath.Base(filepath.Clean(filename))
	candidate := filepath.Join(baseDir, cleanName)
	if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
		if resolvedBase, baseErr := filepath.EvalSymlinks(baseDir); baseErr == nil {
			if rel, relErr := filepath.Rel(resolvedBase, resolved); relErr == nil && len(rel) >= 2 && rel[:2] == ".." {
				return filepath.Join(baseDir, filepath.Base(cleanName))
			}
		}
	}
	return candidate
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
	if err := EnsureDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(nicknames, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(GetPath("nicknames.json"), data, 0600)
}
