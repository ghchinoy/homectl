package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
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
		{[]string{"sonos", "favorites"}},
		{[]string{"sonos", "play-favorite"}},
		{[]string{"sonos", "play-stream"}},
		{[]string{"sonos", "queue-add"}},
		{[]string{"sonos", "services"}},
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

func TestDryRunCommands(t *testing.T) {
	captureOutput := func(f func() error) (string, error) {
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := f()

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		return buf.String(), err
	}

	t.Run("lutron set level dry-run json", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"lutron", "set", "level"})
		_ = rootCmd.PersistentFlags().Set("dry-run", "true")
		_ = rootCmd.PersistentFlags().Set("json", "true")
		defer rootCmd.PersistentFlags().Set("dry-run", "false")
		defer rootCmd.PersistentFlags().Set("json", "false")

		out, err := captureOutput(func() error {
			return cmd.RunE(cmd, []string{"/zone/1", "45"})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var res DryRunResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v, raw: %q", err, out)
		}
		if !res.DryRun || res.Command != "set level" || res.Planned["level"] != float64(45) {
			t.Errorf("unexpected dry run result: %+v", res)
		}
	})

	t.Run("lutron set all dry-run json", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"lutron", "set", "all"})
		_ = rootCmd.PersistentFlags().Set("dry-run", "true")
		_ = rootCmd.PersistentFlags().Set("json", "true")
		defer rootCmd.PersistentFlags().Set("dry-run", "false")
		defer rootCmd.PersistentFlags().Set("json", "false")

		out, err := captureOutput(func() error {
			return cmd.RunE(cmd, []string{"75"})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var res DryRunResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v, raw: %q", err, out)
		}
		if !res.DryRun || res.Command != "set all" || res.Planned["level"] != float64(75) {
			t.Errorf("unexpected dry run result: %+v", res)
		}
	})

	t.Run("sonos volume dry-run json", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "volume"})
		_ = rootCmd.PersistentFlags().Set("dry-run", "true")
		_ = rootCmd.PersistentFlags().Set("json", "true")
		defer rootCmd.PersistentFlags().Set("dry-run", "false")
		defer rootCmd.PersistentFlags().Set("json", "false")

		out, err := captureOutput(func() error {
			return cmd.RunE(cmd, []string{"192.168.1.100", "30"})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var res DryRunResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v, raw: %q", err, out)
		}
		if !res.DryRun || res.Command != "sonos volume" || res.Planned["volume"] != float64(30) {
			t.Errorf("unexpected dry run result: %+v", res)
		}
	})

	t.Run("sonos play-favorite dry-run json", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "play-favorite"})
		_ = rootCmd.PersistentFlags().Set("dry-run", "true")
		_ = rootCmd.PersistentFlags().Set("json", "true")
		defer rootCmd.PersistentFlags().Set("dry-run", "false")
		defer rootCmd.PersistentFlags().Set("json", "false")

		out, err := captureOutput(func() error {
			return cmd.RunE(cmd, []string{"192.168.1.100", "FV:2/1"})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var res DryRunResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v, raw: %q", err, out)
		}
		if !res.DryRun || res.Command != "sonos play-favorite" || res.Planned["favorite_id"] != "FV:2/1" {
			t.Errorf("unexpected dry run result: %+v", res)
		}
	})

	t.Run("sonos play-stream dry-run json", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "play-stream"})
		_ = rootCmd.PersistentFlags().Set("dry-run", "true")
		_ = rootCmd.PersistentFlags().Set("json", "true")
		defer rootCmd.PersistentFlags().Set("dry-run", "false")
		defer rootCmd.PersistentFlags().Set("json", "false")

		out, err := captureOutput(func() error {
			return cmd.RunE(cmd, []string{"192.168.1.100", "https://stream.example.com/live.mp3"})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var res DryRunResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v, raw: %q", err, out)
		}
		if !res.DryRun || res.Command != "sonos play-stream" || res.Planned["url"] != "https://stream.example.com/live.mp3" {
			t.Errorf("unexpected dry run result: %+v", res)
		}
	})

	t.Run("sonos queue-add dry-run json", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "queue-add"})
		_ = rootCmd.PersistentFlags().Set("dry-run", "true")
		_ = rootCmd.PersistentFlags().Set("json", "true")
		defer rootCmd.PersistentFlags().Set("dry-run", "false")
		defer rootCmd.PersistentFlags().Set("json", "false")

		out, err := captureOutput(func() error {
			return cmd.RunE(cmd, []string{"192.168.1.100", "x-file-cifs://nas/track.flac"})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var res DryRunResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v, raw: %q", err, out)
		}
		if !res.DryRun || res.Command != "sonos queue-add" || res.Planned["uri"] != "x-file-cifs://nas/track.flac" {
			t.Errorf("unexpected dry run result: %+v", res)
		}
	})
}

func TestValidationRanges(t *testing.T) {
	t.Run("set level invalid range", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"lutron", "set", "level"})
		if err := cmd.RunE(cmd, []string{"/zone/1", "150"}); err == nil {
			t.Error("expected error for level 150, got nil")
		}
		if err := cmd.RunE(cmd, []string{"/zone/1", "-10"}); err == nil {
			t.Error("expected error for level -10, got nil")
		}
	})

	t.Run("set all invalid range", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"lutron", "set", "all"})
		if err := cmd.RunE(cmd, []string{"105"}); err == nil {
			t.Error("expected error for level 105, got nil")
		}
	})

	t.Run("sonos volume invalid range", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "volume"})
		if err := cmd.RunE(cmd, []string{"192.168.1.1", "101"}); err == nil {
			t.Error("expected error for volume 101, got nil")
		}
	})

	t.Run("sonos play-stream invalid scheme", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "play-stream"})
		if err := cmd.RunE(cmd, []string{"192.168.1.1", "ftp://example.com/audio.mp3"}); err == nil {
			t.Error("expected error for ftp scheme, got nil")
		}
	})
}
