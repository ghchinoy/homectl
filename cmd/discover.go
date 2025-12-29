package cmd

import (
	"fmt"
	"time"

	"github.com/ghchinoy/homectl/pkg/discovery"
	"github.com/ghchinoy/homectl/pkg/leap"
	"github.com/ghchinoy/homectl/pkg/sonos"
	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover all smart home devices on the network",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting network-wide discovery...")
		
		manager := discovery.NewManager()
		manager.AddProvider(&leap.DiscoveryProvider{})
		manager.AddProvider(&sonos.DiscoveryProvider{})

	
devices := manager.DiscoverAll(5 * time.Second)

		if len(devices) == 0 {
			fmt.Println("No devices found.")
			return
		}

		fmt.Printf("\n%-10s %-20s %-15s %-30s\n", "PROVIDER", "NAME", "IP", "MODEL/ID")
		fmt.Println("--------------------------------------------------------------------------------")
		for _, d := range devices {
			model := d.Model
			if model == "" {
				model = d.ID
			}
			fmt.Printf("%-10s %-20s %-15s %-30s\n", d.Provider, d.Name, d.IP, model)
		}
		fmt.Printf("\nTotal devices found: %d\n", len(devices))
	},
}

func init() {
	rootCmd.AddCommand(discoverCmd)
}
