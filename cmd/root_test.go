package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ghchinoy/homectl/modules/core"
	"github.com/ghchinoy/homectl/modules/sonos"
	"github.com/ghchinoy/homectl/pkg/config"
	"github.com/ghchinoy/homectl/pkg/version"
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
		{[]string{"sonos", "seek"}},
		{[]string{"sonos", "now-playing"}},
		{[]string{"sonos", "details"}},
		{[]string{"sonos", "volume"}},
		{[]string{"sonos", "favorites"}},
		{[]string{"sonos", "play-favorite"}},
		{[]string{"sonos", "play-stream"}},
		{[]string{"sonos", "queue-add"}},
		{[]string{"sonos", "queue"}},
		{[]string{"sonos", "queue-remove"}},
		{[]string{"sonos", "queue-clear"}},
		{[]string{"sonos", "queue-reorder"}},
		{[]string{"sonos", "services"}},
		{[]string{"sonos", "join"}},
		{[]string{"sonos", "leave"}},
		{[]string{"qolsys"}},
		{[]string{"qolsys", "monitor"}},
		{[]string{"version"}},
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
			t.Fatalf("rootCmd.Find(sonos, play) error = %v, want nil", err)
		}
		if err := cmd.Args(cmd, []string{}); err == nil {
			t.Error("cmd.Args(cmd, []) = nil, want error")
		}
	})

	t.Run("sonos join requires 2 args", func(t *testing.T) {
		cmd, _, err := rootCmd.Find([]string{"sonos", "join"})
		if err != nil || cmd == nil {
			t.Fatalf("could not find sonos join: %v", err)
		}
		if err := cmd.Args(cmd, []string{"192.168.1.10"}); err == nil {
			t.Error("expected error when sonos join called with 1 arg, got nil")
		}
	})

	t.Run("sonos leave requires 1 arg", func(t *testing.T) {
		cmd, _, err := rootCmd.Find([]string{"sonos", "leave"})
		if err != nil || cmd == nil {
			t.Fatalf("could not find sonos leave: %v", err)
		}
		if err := cmd.Args(cmd, []string{}); err == nil {
			t.Error("expected error when sonos leave called with 0 args, got nil")
		}
	})

	t.Run("lutron set level requires 2 args", func(t *testing.T) {
		cmd, _, err := rootCmd.Find([]string{"lutron", "set", "level"})
		if err != nil || cmd == nil {
			t.Fatalf("rootCmd.Find(lutron, set, level) error = %v, want nil", err)
		}
		if err := cmd.Args(cmd, []string{"/zone/1"}); err == nil {
			t.Error("cmd.Args(cmd, [/zone/1]) = nil, want error")
		}
	})
}

func TestResolveLutronBridgePrecedence(t *testing.T) {
	// 1. Explicit flag takes precedence
	addr, err := ResolveLutronBridge("10.0.0.99")
	if err != nil || addr != "10.0.0.99" {
		t.Errorf("ResolveLutronBridge(%q) = %q, want %q (err: %v)", "10.0.0.99", addr, "10.0.0.99", err)
	}

	// 2. Environment variable
	t.Setenv("HOMECTL_LUTRON_BRIDGE", "10.0.0.50")
	addr, err = ResolveLutronBridge("")
	if err != nil || addr != "10.0.0.50" {
		t.Errorf("ResolveLutronBridge(%q) with env = %q, want %q (err: %v)", "", addr, "10.0.0.50", err)
	}

	// Explicit flag still beats env var
	addr, err = ResolveLutronBridge("10.0.0.99")
	if err != nil || addr != "10.0.0.99" {
		t.Errorf("ResolveLutronBridge(%q) with override = %q, want %q (err: %v)", "10.0.0.99", addr, "10.0.0.99", err)
	}
}

func TestServeCommandFlags(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"serve"})
	if err != nil || cmd == nil {
		t.Fatalf("could not find serve command: %v", err)
	}
	hostFlag := cmd.Flags().Lookup("host")
	if hostFlag == nil {
		t.Fatal("expected --host flag on serve command")
	}
	if hostFlag.Shorthand != "H" {
		t.Errorf("expected shorthand 'H', got %q", hostFlag.Shorthand)
	}
	portFlag := cmd.Flags().Lookup("port")
	if portFlag == nil || portFlag.Shorthand != "p" {
		t.Error("expected --port flag with shorthand 'p'")
	}
}

func TestResolveAPIHostPrecedence(t *testing.T) {
	tempDir := t.TempDir()
	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tempDir)
	defer os.Setenv("XDG_CONFIG_HOME", origConfigHome)

	// 1. Explicit CLI flag takes precedence over everything
	t.Setenv("HOMECTL_API_HOST", "192.168.1.50")
	if host := ResolveAPIHost("127.0.0.1"); host != "127.0.0.1" {
		t.Errorf("expected flag '127.0.0.1', got %q", host)
	}

	// 2. HOMECTL_API_HOST environment variable takes precedence over config.json
	_ = config.EnsureDir()
	configJSON := `{"api_host": "100.85.12.34"}`
	_ = os.WriteFile(config.GetPath("config.json"), []byte(configJSON), 0644)

	t.Setenv("HOMECTL_API_HOST", "192.168.1.50")
	if host := ResolveAPIHost(""); host != "192.168.1.50" {
		t.Errorf("expected env '192.168.1.50', got %q", host)
	}

	// 3. config.json api_host takes precedence over default
	t.Setenv("HOMECTL_API_HOST", "")
	if host := ResolveAPIHost(""); host != "100.85.12.34" {
		t.Errorf("expected config.json '100.85.12.34', got %q", host)
	}

	// 4. Default fallback returns "0.0.0.0"
	_ = os.Remove(config.GetPath("config.json"))
	if host := ResolveAPIHost(""); host != "0.0.0.0" {
		t.Errorf("expected default '0.0.0.0', got %q", host)
	}
}

func TestTailscaleAndLoopbackDetection(t *testing.T) {
	tailscaleTests := []struct {
		host string
		want bool
	}{
		{"100.64.0.1", true},
		{"100.85.12.34", true},
		{"100.127.255.254", true},
		{"100.128.0.1", false},
		{"192.168.1.1", false},
		{"127.0.0.1", false},
		{"fd7a:115c:a1e0::1", true},
		{"2001:db8::1", false},
		{"", false},
		{"localhost", false},
	}
	for _, tc := range tailscaleTests {
		got := IsTailscaleIP(tc.host)
		if got != tc.want {
			t.Errorf("IsTailscaleIP(%q) = %v; want %v", tc.host, got, tc.want)
		}
	}

	loopbackTests := []struct {
		host string
		want bool
	}{
		{"127.0.0.1", true},
		{"localhost", true},
		{"LocalHost", true},
		{"::1", true},
		{"0.0.0.0", false},
		{"192.168.1.100", false},
		{"100.85.12.34", false},
		{"", false},
	}
	for _, tc := range loopbackTests {
		got := IsLoopbackHost(tc.host)
		if got != tc.want {
			t.Errorf("IsLoopbackHost(%q) = %v; want %v", tc.host, got, tc.want)
		}
	}
}

func captureStdout(f func() error) (string, error) {
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

func TestDryRunCommands(t *testing.T) {
	captureOutput := captureStdout

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

	t.Run("sonos seek dry-run track json", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "seek"})
		_ = rootCmd.PersistentFlags().Set("dry-run", "true")
		_ = rootCmd.PersistentFlags().Set("json", "true")
		defer rootCmd.PersistentFlags().Set("dry-run", "false")
		defer rootCmd.PersistentFlags().Set("json", "false")
		_ = cmd.Flags().Set("track", "5")
		defer cmd.Flags().Set("track", "0")

		out, err := captureOutput(func() error {
			return cmd.RunE(cmd, []string{"192.168.1.100"})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var res DryRunResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v, raw: %q", err, out)
		}
		if !res.DryRun || res.Command != "sonos seek" || res.Planned["track"] != float64(5) {
			t.Errorf("unexpected dry run result: %+v", res)
		}
	})

	t.Run("sonos seek dry-run time json", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "seek"})
		_ = rootCmd.PersistentFlags().Set("dry-run", "true")
		_ = rootCmd.PersistentFlags().Set("json", "true")
		defer rootCmd.PersistentFlags().Set("dry-run", "false")
		defer rootCmd.PersistentFlags().Set("json", "false")
		_ = cmd.Flags().Set("time", "1:30")
		defer cmd.Flags().Set("time", "")

		out, err := captureOutput(func() error {
			return cmd.RunE(cmd, []string{"192.168.1.100"})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var res DryRunResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v, raw: %q", err, out)
		}
		if !res.DryRun || res.Command != "sonos seek" || res.Planned["time"] != "1:30" {
			t.Errorf("unexpected dry run result: %+v", res)
		}
	})

	t.Run("sonos queue-remove dry-run json", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "queue-remove"})
		_ = rootCmd.PersistentFlags().Set("dry-run", "true")
		_ = rootCmd.PersistentFlags().Set("json", "true")
		defer rootCmd.PersistentFlags().Set("dry-run", "false")
		defer rootCmd.PersistentFlags().Set("json", "false")
		_ = cmd.Flags().Set("track", "3")
		_ = cmd.Flags().Set("count", "2")
		defer cmd.Flags().Set("track", "0")
		defer cmd.Flags().Set("count", "1")

		out, err := captureOutput(func() error {
			return cmd.RunE(cmd, []string{"192.168.1.100"})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var res DryRunResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v, raw: %q", err, out)
		}
		if !res.DryRun || res.Command != "sonos queue-remove" || res.Planned["track"] != float64(3) || res.Planned["count"] != float64(2) {
			t.Errorf("unexpected dry run result: %+v", res)
		}
	})

	t.Run("sonos queue-clear dry-run json", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "queue-clear"})
		_ = rootCmd.PersistentFlags().Set("dry-run", "true")
		_ = rootCmd.PersistentFlags().Set("json", "true")
		defer rootCmd.PersistentFlags().Set("dry-run", "false")
		defer rootCmd.PersistentFlags().Set("json", "false")

		out, err := captureOutput(func() error {
			return cmd.RunE(cmd, []string{"192.168.1.100"})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var res DryRunResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v, raw: %q", err, out)
		}
		if !res.DryRun || res.Command != "sonos queue-clear" {
			t.Errorf("unexpected dry run result: %+v", res)
		}
	})

	t.Run("sonos queue-reorder dry-run json", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "queue-reorder"})
		_ = rootCmd.PersistentFlags().Set("dry-run", "true")
		_ = rootCmd.PersistentFlags().Set("json", "true")
		defer rootCmd.PersistentFlags().Set("dry-run", "false")
		defer rootCmd.PersistentFlags().Set("json", "false")
		_ = cmd.Flags().Set("track", "5")
		_ = cmd.Flags().Set("insert-before", "2")
		defer cmd.Flags().Set("track", "0")
		defer cmd.Flags().Set("insert-before", "0")

		out, err := captureOutput(func() error {
			return cmd.RunE(cmd, []string{"192.168.1.100"})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var res DryRunResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v, raw: %q", err, out)
		}
		if !res.DryRun || res.Command != "sonos queue-reorder" || res.Planned["track"] != float64(5) || res.Planned["insert_before"] != float64(2) {
			t.Errorf("unexpected dry run result: %+v", res)
		}
	})

	t.Run("sonos queue-mode dry-run json", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "queue-mode"})
		_ = rootCmd.PersistentFlags().Set("dry-run", "true")
		_ = rootCmd.PersistentFlags().Set("json", "true")
		defer rootCmd.PersistentFlags().Set("dry-run", "false")
		defer rootCmd.PersistentFlags().Set("json", "false")

		_ = cmd.Flags().Set("shuffle", "on")
		_ = cmd.Flags().Set("repeat", "all")
		_ = cmd.Flags().Set("crossfade", "off")
		defer func() {
			_ = cmd.Flags().Set("shuffle", "")
			_ = cmd.Flags().Set("repeat", "")
			_ = cmd.Flags().Set("crossfade", "")
		}()

		out, err := captureOutput(func() error {
			return cmd.RunE(cmd, []string{"192.168.1.100"})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var res DryRunResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v, raw: %q", err, out)
		}
		if !res.DryRun || res.Command != "sonos queue-mode" {
			t.Errorf("unexpected dry run result: %+v", res)
		}
		if res.Planned["shuffle"] != true || res.Planned["repeat"] != "all" || res.Planned["crossfade"] != false {
			t.Errorf("unexpected planned values: %+v", res.Planned)
		}
	})
}

func TestValidationRanges(t *testing.T) {
	t.Run("set level invalid range", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"lutron", "set", "level"})
		if err := cmd.RunE(cmd, []string{"/zone/1", "150"}); err == nil {
			t.Error("cmd.RunE(/zone/1, 150) = nil, want error")
		}
		if err := cmd.RunE(cmd, []string{"/zone/1", "-10"}); err == nil {
			t.Error("cmd.RunE(/zone/1, -10) = nil, want error")
		}
	})

	t.Run("set all invalid range", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"lutron", "set", "all"})
		if err := cmd.RunE(cmd, []string{"105"}); err == nil {
			t.Error("cmd.RunE(105) = nil, want error")
		}
	})

	t.Run("sonos volume invalid range", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "volume"})
		if err := cmd.RunE(cmd, []string{"192.168.1.1", "101"}); err == nil {
			t.Error("cmd.RunE(192.168.1.1, 101) = nil, want error")
		}
	})

	t.Run("sonos play-stream invalid scheme", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "play-stream"})
		if err := cmd.RunE(cmd, []string{"192.168.1.1", "ftp://example.com/audio.mp3"}); err == nil {
			t.Error("cmd.RunE(ftp://...) = nil, want error")
		}
	})

	t.Run("sonos seek invalid args", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "seek"})
		// Neither flag
		if err := cmd.RunE(cmd, []string{"192.168.1.1"}); err == nil {
			t.Error("cmd.RunE without --track or --time = nil, want error")
		}
		// Both flags
		_ = cmd.Flags().Set("track", "2")
		_ = cmd.Flags().Set("time", "1:00")
		if err := cmd.RunE(cmd, []string{"192.168.1.1"}); err == nil {
			t.Error("cmd.RunE with both --track and --time = nil, want error")
		}
		_ = cmd.Flags().Set("track", "0")
		_ = cmd.Flags().Set("time", "")
	})

	t.Run("sonos queue-remove invalid args", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "queue-remove"})
		// Missing track
		_ = cmd.Flags().Set("track", "0")
		if err := cmd.RunE(cmd, []string{"192.168.1.1"}); err == nil {
			t.Error("cmd.RunE with --track 0 = nil, want error")
		}
	})

	t.Run("sonos queue-reorder invalid args", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "queue-reorder"})
		// Missing track
		_ = cmd.Flags().Set("track", "0")
		_ = cmd.Flags().Set("insert-before", "2")
		if err := cmd.RunE(cmd, []string{"192.168.1.1"}); err == nil {
			t.Error("cmd.RunE with --track 0 = nil, want error")
		}
		// Neither insert-before nor as-next
		_ = cmd.Flags().Set("track", "3")
		_ = cmd.Flags().Set("insert-before", "0")
		_ = cmd.Flags().Set("as-next", "false")
		if err := cmd.RunE(cmd, []string{"192.168.1.1"}); err == nil {
			t.Error("cmd.RunE without --insert-before or --as-next = nil, want error")
		}
		// Both insert-before and as-next
		_ = cmd.Flags().Set("insert-before", "2")
		_ = cmd.Flags().Set("as-next", "true")
		if err := cmd.RunE(cmd, []string{"192.168.1.1"}); err == nil {
			t.Error("cmd.RunE with both --insert-before and --as-next = nil, want error")
		}
		_ = cmd.Flags().Set("track", "0")
		_ = cmd.Flags().Set("insert-before", "0")
		_ = cmd.Flags().Set("as-next", "false")
	})

	t.Run("sonos queue-mode invalid args", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "queue-mode"})
		// Neither flag specified
		if err := cmd.RunE(cmd, []string{"192.168.1.1"}); err == nil {
			t.Error("cmd.RunE without mode flags = nil, want error")
		}
		// Invalid shuffle flag
		_ = cmd.Flags().Set("shuffle", "invalid")
		if err := cmd.RunE(cmd, []string{"192.168.1.1"}); err == nil {
			t.Error("cmd.RunE with invalid --shuffle = nil, want error")
		}
		_ = cmd.Flags().Set("shuffle", "")

		// Invalid repeat flag
		_ = cmd.Flags().Set("repeat", "invalid")
		if err := cmd.RunE(cmd, []string{"192.168.1.1"}); err == nil {
			t.Error("cmd.RunE with invalid --repeat = nil, want error")
		}
		_ = cmd.Flags().Set("repeat", "")

		// Invalid crossfade flag
		_ = cmd.Flags().Set("crossfade", "invalid")
		if err := cmd.RunE(cmd, []string{"192.168.1.1"}); err == nil {
			t.Error("cmd.RunE with invalid --crossfade = nil, want error")
		}
		_ = cmd.Flags().Set("crossfade", "")
	})
}

func TestSonosGenerationAndRendererValidation(t *testing.T) {
	memStorage := core.NewMemoryStorage()
	sonos.SetDefaultStorage(memStorage)
	defer sonos.SetDefaultStorage(nil)

	devices := []sonos.Device{
		{Name: "Play:5 Gen 1", IP: "192.168.1.10", RinconID: "RINCON_S1", Generation: "S1", IsRenderer: true},
		{Name: "Sonos One", IP: "192.168.1.20", RinconID: "RINCON_S2", Generation: "S2", IsRenderer: true},
		{Name: "Bridge", IP: "192.168.1.50", RinconID: "RINCON_BRIDGE", ModelName: "Sonos Bridge", Generation: "S1", IsRenderer: false},
	}
	_ = sonos.SaveCache(devices)

	t.Run("play rejected on non-renderer bridge", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "play"})
		err := cmd.RunE(cmd, []string{"192.168.1.50"})
		if err == nil || !strings.Contains(err.Error(), "non-rendering device") {
			t.Errorf("expected non-rendering device error, got %v", err)
		}
	})

	t.Run("volume rejected on non-renderer bridge", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "volume"})
		err := cmd.RunE(cmd, []string{"192.168.1.50", "25"})
		if err == nil || !strings.Contains(err.Error(), "non-rendering device") {
			t.Errorf("expected non-rendering device error, got %v", err)
		}
	})

	t.Run("join rejected when source is non-renderer", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "join"})
		err := cmd.RunE(cmd, []string{"192.168.1.50", "192.168.1.10"})
		if err == nil || !strings.Contains(err.Error(), "non-rendering device") {
			t.Errorf("expected non-rendering device error, got %v", err)
		}
	})

	t.Run("join rejected when target is non-renderer", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "join"})
		err := cmd.RunE(cmd, []string{"192.168.1.10", "192.168.1.50"})
		if err == nil || !strings.Contains(err.Error(), "non-rendering device") {
			t.Errorf("expected non-rendering device error, got %v", err)
		}
	})

	t.Run("join rejected across generations S1 and S2", func(t *testing.T) {
		cmd, _, _ := rootCmd.Find([]string{"sonos", "join"})
		err := cmd.RunE(cmd, []string{"192.168.1.10", "192.168.1.20"})
		if err == nil || !strings.Contains(err.Error(), "cannot group S1 speaker") {
			t.Errorf("expected cross-generation error, got %v", err)
		}
	})
}

func TestVersionCommand(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"version"})
	if err != nil || cmd == nil {
		t.Fatalf("rootCmd.Find(version) = %v, want nil", err)
	}

	t.Run("default text output", func(t *testing.T) {
		out, err := captureStdout(func() error {
			return cmd.RunE(cmd, []string{})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "homectl v"+version.Version) {
			t.Errorf("expected output to contain %q, got %q", "homectl v"+version.Version, out)
		}
	})

	t.Run("json output", func(t *testing.T) {
		_ = cmd.Flags().Set("json", "true")
		defer cmd.Flags().Set("json", "false")

		out, err := captureStdout(func() error {
			return cmd.RunE(cmd, []string{})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var info version.Info
		if err := json.Unmarshal([]byte(out), &info); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v, raw: %q", err, out)
		}
		if info.Version != version.Version {
			t.Errorf("info.Version = %q, want %q", info.Version, version.Version)
		}
	})
}
