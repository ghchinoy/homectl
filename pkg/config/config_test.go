package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetPath(t *testing.T) {
	p := Path("test.json")
	if got := filepath.Base(p); got != "test.json" {
		t.Errorf("Path(%q) base = %q, want %q", "test.json", got, "test.json")
	}
}

func TestNicknamesSaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	if err := EnsureDir(); err != nil {
		t.Fatalf("EnsureDir() = %v, want nil", err)
	}

	testNicknames := map[string]string{
		"192.168.1.100": "Living Room Sonos",
		"/zone/1":       "Kitchen Pendant",
	}

	if err := SaveNicknames(testNicknames); err != nil {
		t.Fatalf("SaveNicknames(%v) = %v, want nil", testNicknames, err)
	}

	loaded := LoadNicknames()
	if len(loaded) != 2 {
		t.Fatalf("len(LoadNicknames()) = %d, want 2", len(loaded))
	}

	if got := loaded["192.168.1.100"]; got != "Living Room Sonos" {
		t.Errorf("LoadNicknames()[192.168.1.100] = %q, want %q", got, "Living Room Sonos")
	}
	if got := loaded["/zone/1"]; got != "Kitchen Pendant" {
		t.Errorf("LoadNicknames()[/zone/1] = %q, want %q", got, "Kitchen Pendant")
	}
}

func TestLoadConfigWithAPIHost(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	if err := EnsureDir(); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	configJSON := `{
		"callback_ip": "192.168.1.100",
		"camera_auth": "user:pass",
		"api_host": "100.85.12.34"
	}`
	if err := os.WriteFile(Path("config.json"), []byte(configJSON), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg := LoadConfig()
	if cfg.APIHost != "100.85.12.34" {
		t.Errorf("expected APIHost to be '100.85.12.34', got %q", cfg.APIHost)
	}
	if cfg.CallbackIP != "192.168.1.100" {
		t.Errorf("expected CallbackIP to be '192.168.1.100', got %q", cfg.CallbackIP)
	}
}
