package core

import "testing"

func TestStaticSettings(t *testing.T) {
	s := NewStaticSettings("192.168.1.50", "admin:secret")

	if got := s.CallbackIP(); got != "192.168.1.50" {
		t.Fatalf("CallbackIP() = %q, want %q", got, "192.168.1.50")
	}
	if got := s.CameraAuth(); got != "admin:secret" {
		t.Fatalf("CameraAuth() = %q, want %q", got, "admin:secret")
	}

	s.SetCallbackIP("10.0.0.1")
	if got := s.CallbackIP(); got != "10.0.0.1" {
		t.Fatalf("CallbackIP() = %q, want %q", got, "10.0.0.1")
	}

	s.Set("custom_key", "custom_val")
	if got := s.Get("custom_key"); got != "custom_val" {
		t.Fatalf("Get(%q) = %q, want %q", "custom_key", got, "custom_val")
	}
}
