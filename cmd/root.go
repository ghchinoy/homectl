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

func getClient(cmd *cobra.Command) (*leap.Client, error) {
	addr, _ := cmd.Flags().GetString("bridge")

	// If empty or default fallback, try cache then discovery
	if addr == "" || addr == "192.168.4.90" {
		cached, _ := leap.LoadCache()
		if len(cached) > 0 {
			addr = cached[0].IP
		} else {
			fmt.Println("Discovering Lutron bridge...")
			bridges, _ := leap.Discover(2 * time.Second)
			if len(bridges) > 0 {
				addr = bridges[0].IP
				_ = leap.SaveCache(bridges)
			}
		}
	}

	if addr == "" {
		return nil, fmt.Errorf("no Lutron bridge specified or discovered; pass --bridge <ip>")
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

func init() {
	rootCmd.PersistentFlags().String("bridge", "", "Lutron Bridge IP address (defaults to auto-discovery or cache)")
}
