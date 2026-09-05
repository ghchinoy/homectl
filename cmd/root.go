// Package cmd provides the Cobra CLI commands for homectl.
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/ghchinoy/homectl/pkg/config"
	"github.com/ghchinoy/homectl/pkg/leap"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "homectl",
	Short: "homectl is a CLI for local smart home management",
	Long:  `A modern, Go-powered toolkit for local smart home management (Lutron, Sonos, Google Cast, and RTSP cameras).`,
}

// Execute runs the root command and exits on error.
func Execute() {
	_ = config.EnsureDir()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ResolveLutronBridge resolves the Lutron Bridge address according to strict precedence:
// 1. Explicit CLI flag (--bridge)
// 2. HOMECTL_LUTRON_BRIDGE environment variable
// 3. config.json "lutron_bridge" setting
// 4. Cached discovery in ~/.config/homectl/lutron_cache.json
// 5. Dynamic mDNS discovery
func ResolveLutronBridge(flagAddr string) (string, error) {
	if flagAddr != "" {
		return flagAddr, nil
	}

	if envAddr := os.Getenv("HOMECTL_LUTRON_BRIDGE"); envAddr != "" {
		return envAddr, nil
	}

	cfg := config.LoadConfig()
	if cfg.LutronBridge != "" {
		return cfg.LutronBridge, nil
	}

	cached, _ := leap.LoadCache()
	if len(cached) > 0 && cached[0].IP != "" {
		return cached[0].IP, nil
	}

	fmt.Fprintln(os.Stderr, "Discovering Lutron bridge via mDNS...")
	bridges, err := leap.Discover(2 * time.Second)
	if err == nil && len(bridges) > 0 && bridges[0].IP != "" {
		_ = leap.SaveCache(bridges)
		return bridges[0].IP, nil
	}

	return "", fmt.Errorf("no Lutron bridge specified or discovered; pass --bridge, set HOMECTL_LUTRON_BRIDGE, or set lutron_bridge in config.json")
}

func getClient(cmd *cobra.Command) (*leap.Client, error) {
	flagAddr, _ := cmd.Flags().GetString("bridge")
	addr, err := ResolveLutronBridge(flagAddr)
	if err != nil {
		return nil, err
	}

	// Certs are in the config directory
	client, err := leap.NewClient(
		addr+":8081",
		config.GetPath("lutron_client.crt"),
		config.GetPath("lutron_client.key"),
		config.GetPath("lutron_ca.crt"),
	)
	if err != nil {
		return nil, fmt.Errorf("create Lutron client: %w", err)
	}
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("connect to Lutron bridge at %s: %w", addr, err)
	}
	return client, nil
}

// DryRunResult formats simulated mutations for structured JSON output.
type DryRunResult struct {
	DryRun  bool           `json:"dry_run"`
	Command string         `json:"command"`
	Planned map[string]any `json:"planned"`
	Message string         `json:"message"`
}

func isJSON(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if f := cmd.Flag("json"); f != nil {
		return f.Value.String() == "true"
	}
	return false
}

func isDryRun(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if f := cmd.Flag("dry-run"); f != nil {
		return f.Value.String() == "true"
	}
	return false
}

func init() {
	rootCmd.PersistentFlags().String("bridge", "", "Lutron Bridge IP address (defaults to auto-discovery or cache)")
	rootCmd.PersistentFlags().Bool("json", false, "Output results as machine-readable JSON")
	rootCmd.PersistentFlags().Bool("dry-run", false, "Simulate operation without sending network mutations")
}
