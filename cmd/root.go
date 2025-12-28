
package cmd

import (
	"fmt"
	"os"

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

func init() {
	// Global flags can be added here
	rootCmd.PersistentFlags().String("bridge", "192.168.4.90", "Lutron Bridge IP address")
}
