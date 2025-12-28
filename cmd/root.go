
package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/ghchinoy/control/pkg/leap"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "control",
	Short: "Control is a CLI for Lutron Caseta and RA2 Select",
	Long:  `A modern, Go-powered toolkit for local Lutron smart home management.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func getClient(cmd *cobra.Command) *leap.Client {
	addr, _ := cmd.Flags().GetString("bridge")
	// Certs are now in the secrets/ directory
	client, err := leap.NewClient(
		addr+":8081",
		"secrets/lutron_client.crt",
		"secrets/lutron_client.key",
		"secrets/lutron_ca.crt",
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	return client
}

func init() {
	// Global flags can be added here
	rootCmd.PersistentFlags().String("bridge", "192.168.4.90", "Lutron Bridge IP address")
}
