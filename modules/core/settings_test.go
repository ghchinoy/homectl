package core

import "testing"

func TestStaticSettings(t *testing.T) {
	s := NewStaticSettings("192.168.1.50", "admin:secret")

	if s.CallbackIP() != "192.168.1.50" {
		t.Fatalf("expected callback IP 192.168.1.50, got %s", s.CallbackIP())
	}
	if s.CameraAuth() != "admin:secret" {
		t.Fatalf("expected camera auth admin:secret, got %s", s.CameraAuth())
	}

	s.SetCallbackIP("10.0.0.1")
	if s.CallbackIP() != "10.0.0.1" {
		t.Fatalf("expected updated callback IP 10.0.0.1, got %s", s.CallbackIP())
	}

	s.Set("custom_key", "custom_val")
	if s.Get("custom_key") != "custom_val" {
		t.Fatalf("expected custom_val, got %s", s.Get("custom_key"))
	}
}
