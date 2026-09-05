package cmd

import (
	"testing"
)

func TestCommandRegistration(t *testing.T) {
	expectedSubcommands := []struct {
		path []string
	}{
		{[]string{"discover"}},
		{[]string{"serve"}},
		{[]string{"ui"}},
		{[]string{"lutron"}},
		{[]string{"lutron", "list"}},
		{[]string{"lutron", "list", "devices"}},
		{[]string{"lutron", "list", "zones"}},
		{[]string{"lutron", "list", "areas"}},
		{[]string{"lutron", "set"}},
		{[]string{"lutron", "set", "level"}},
		{[]string{"lutron", "set", "all"}},
		{[]string{"sonos"}},
		{[]string{"sonos", "list"}},
		{[]string{"sonos", "play"}},
		{[]string{"sonos", "pause"}},
		{[]string{"sonos", "stop"}},
		{[]string{"sonos", "next"}},
		{[]string{"sonos", "prev"}},
		{[]string{"sonos", "now-playing"}},
		{[]string{"sonos", "details"}},
		{[]string{"sonos", "volume"}},
		{[]string{"qolsys"}},
		{[]string{"qolsys", "monitor"}},
	}

	for _, tc := range expectedSubcommands {
		cmd, _, err := rootCmd.Find(tc.path)
		if err != nil {
			t.Errorf("rootCmd.Find(%v) returned error: %v", tc.path, err)
			continue
		}
		if cmd == nil || cmd == rootCmd {
			t.Errorf("command %v not registered under rootCmd", tc.path)
		}
	}
}

func TestSubcommandArgValidation(t *testing.T) {
	t.Run("sonos play requires 1 arg", func(t *testing.T) {
		cmd, _, err := rootCmd.Find([]string{"sonos", "play"})
		if err != nil || cmd == nil {
			t.Fatalf("could not find sonos play: %v", err)
		}
		if err := cmd.Args(cmd, []string{}); err == nil {
			t.Error("expected error when sonos play called with 0 args, got nil")
		}
	})

	t.Run("lutron set level requires 2 args", func(t *testing.T) {
		cmd, _, err := rootCmd.Find([]string{"lutron", "set", "level"})
		if err != nil || cmd == nil {
			t.Fatalf("could not find lutron set level: %v", err)
		}
		if err := cmd.Args(cmd, []string{"/zone/1"}); err == nil {
			t.Error("expected error when lutron set level called with 1 arg, got nil")
		}
	})
}

func TestResolveLutronBridgePrecedence(t *testing.T) {
	// 1. Explicit flag takes precedence
	addr, err := ResolveLutronBridge("10.0.0.99")
	if err != nil || addr != "10.0.0.99" {
		t.Errorf("expected explicit flag '10.0.0.99', got %q, err: %v", addr, err)
	}

	// 2. Environment variable
	t.Setenv("HOMECTL_LUTRON_BRIDGE", "10.0.0.50")
	addr, err = ResolveLutronBridge("")
	if err != nil || addr != "10.0.0.50" {
		t.Errorf("expected env var '10.0.0.50', got %q, err: %v", addr, err)
	}

	// Explicit flag still beats env var
	addr, err = ResolveLutronBridge("10.0.0.99")
	if err != nil || addr != "10.0.0.99" {
		t.Errorf("expected flag '10.0.0.99' to override env, got %q", addr)
	}
}
