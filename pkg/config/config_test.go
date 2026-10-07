package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetPath(t *testing.T) {
	p := GetPath("test.json")
	if filepath.Base(p) != "test.json" {
		t.Errorf("expected base name test.json, got %s", filepath.Base(p))
	}
}

func TestNicknamesSaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tempDir)
	defer os.Setenv("XDG_CONFIG_HOME", origConfigHome)

	if err := EnsureDir(); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	testNicknames := map[string]string{
		"192.168.1.100": "Living Room Sonos",
		"/zone/1":       "Kitchen Pendant",
	}

	if err := SaveNicknames(testNicknames); err != nil {
		t.Fatalf("SaveNicknames failed: %v", err)
	}

	loaded := LoadNicknames()
	if len(loaded) != 2 {
		t.Fatalf("expected 2 nicknames, got %d", len(loaded))
	}

	if loaded["192.168.1.100"] != "Living Room Sonos" {
		t.Errorf("expected 'Living Room Sonos', got '%s'", loaded["192.168.1.100"])
	}
	if loaded["/zone/1"] != "Kitchen Pendant" {
		t.Errorf("expected 'Kitchen Pendant', got '%s'", loaded["/zone/1"])
	}
}

func TestLoadConfigWithAPIHost(t *testing.T) {
	tempDir := t.TempDir()
	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tempDir)
	defer os.Setenv("XDG_CONFIG_HOME", origConfigHome)

	if err := EnsureDir(); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	configJSON := `{
		"callback_ip": "192.168.1.100",
		"camera_auth": "user:pass",
		"api_host": "100.85.12.34"
	}`
	if err := os.WriteFile(GetPath("config.json"), []byte(configJSON), 0644); err != nil {
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
