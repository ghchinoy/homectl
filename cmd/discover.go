package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ghchinoy/homectl/pkg/camera"
	"github.com/ghchinoy/homectl/pkg/cast"
	"github.com/ghchinoy/homectl/pkg/discovery"
	"github.com/ghchinoy/homectl/pkg/leap"
	"github.com/ghchinoy/homectl/pkg/miio"
	"github.com/ghchinoy/homectl/pkg/onvif"
	"github.com/ghchinoy/homectl/modules/sonos"
	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover all smart home devices on the network",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut := isJSON(cmd)
		if !jsonOut {
			fmt.Println("Starting network-wide discovery...")
		} else {
			fmt.Fprintln(os.Stderr, "Starting network-wide discovery...")
		}

		manager := discovery.NewManager()
		manager.AddProvider(&leap.DiscoveryProvider{})
		manager.AddProvider(&sonos.DiscoveryProvider{})
		manager.AddProvider(&miio.DiscoveryProvider{})
		manager.AddProvider(&cast.DiscoveryProvider{})
		manager.AddProvider(&onvif.DiscoveryProvider{})
		manager.AddProvider(&camera.DiscoveryProvider{})

		devices := manager.DiscoverAll(5 * time.Second)

		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(devices)
		}

		if len(devices) == 0 {
			fmt.Println("No devices found.")
			return nil
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
		return nil
	},
}

func init() {
	rootCmd.AddCommand(discoverCmd)
}
