package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ghchinoy/homectl/modules/sonos"
	"github.com/ghchinoy/homectl/pkg/camera"
	"github.com/ghchinoy/homectl/pkg/cast"
	"github.com/ghchinoy/homectl/pkg/discovery"
	"github.com/ghchinoy/homectl/pkg/leap"
	"github.com/ghchinoy/homectl/pkg/miio"
	"github.com/ghchinoy/homectl/pkg/onvif"
	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:     "discover",
	GroupID: "devices",
	Short:   "Discover all smart home devices on the local network",
	Long:    `Scans the local network concurrently via mDNS, UPnP SSDP, and RTSP port probing to discover Lutron bridges, Sonos speakers, Google Cast devices, ONVIF/RTSP cameras, and MiIO appliances.`,
	Example: `  # Discover all devices on the LAN
  homectl discover

  # Output discovered devices as JSON (capped at 5 results)
  homectl discover --json --limit 5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut := isJSON(cmd)
		statusWriter := os.Stdout
		if jsonOut {
			statusWriter = os.Stderr
		}
		fmt.Fprintln(statusWriter, "Starting network-wide discovery...")

		manager := discovery.NewManager()
		manager.AddProvider(&leap.DiscoveryProvider{})
		manager.AddProvider(&sonos.DiscoveryProvider{})
		manager.AddProvider(&miio.DiscoveryProvider{})
		manager.AddProvider(&cast.DiscoveryProvider{})
		manager.AddProvider(&onvif.DiscoveryProvider{})
		manager.AddProvider(&camera.DiscoveryProvider{})

		devices := applyLimit(cmd, manager.DiscoverAllContext(cmd.Context(), 5*time.Second))

		if jsonOut {
			if devices == nil {
				devices = []discovery.Device{}
			}
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
