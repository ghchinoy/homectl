
package cmd

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ghchinoy/homectl/pkg/config"
	"github.com/ghchinoy/homectl/pkg/leap"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "homectl",
	Short: "homectl is a CLI for Lutron Caseta and RA2 Select",
	Long:  `A modern, Go-powered toolkit for local smart home management.`,
}

func Execute() {
	config.EnsureDir()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func getClient(cmd *cobra.Command) *leap.Client {
	addr, _ := cmd.Flags().GetString("bridge")
	
	// If the default IP is used, try discovery/cache first
	if addr == "192.168.4.90" {
		cached, _ := leap.LoadCache()
		if len(cached) > 0 {
			addr = cached[0].IP
		} else {
			fmt.Println("Discovering Lutron bridge...")
			bridges, _ := leap.Discover(2 * time.Second)
			if len(bridges) > 0 {
				addr = bridges[0].IP
				leap.SaveCache(bridges)
			}
		}
	}

	// Certs are now in the config directory
	client, err := leap.NewClient(
		addr+":8081",
		config.GetPath("lutron_client.crt"),
		config.GetPath("lutron_client.key"),
		config.GetPath("lutron_ca.crt"),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect to Lutron bridge at %s: %v", addr, err)
	}
	return client
}

func init() {
	// Global flags can be added here
	rootCmd.PersistentFlags().String("bridge", "192.168.4.90", "Lutron Bridge IP address")
}
