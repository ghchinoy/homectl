package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// DeviceSummary represents a Lutron device with zone status for JSON output.
type DeviceSummary struct {
	Href       string  `json:"href"`
	Name       string  `json:"name"`
	DeviceType string  `json:"device_type"`
	ZoneName   string  `json:"zone_name,omitempty"`
	ZoneHref   string  `json:"zone_href,omitempty"`
	Status     string  `json:"status"`
	Level      float64 `json:"level,omitempty"`
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List resources from the Lutron bridge",
}

var listAreasCmd = &cobra.Command{
	Use:   "areas",
	Short: "List all areas",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient(cmd)
		if err != nil {
			return err
		}
		defer client.Close()

		areas, err := client.GetAreas()
		if err != nil {
			return fmt.Errorf("get areas: %w", err)
		}

		if isJSON(cmd) {
			return json.NewEncoder(os.Stdout).Encode(areas)
		}

		fmt.Printf("% -20s % -20s\n", "NAME", "HREF")
		fmt.Println("------------------------------------------")
		for _, a := range areas {
			fmt.Printf("% -20s % -20s\n", a.Name, a.Href)
		}
		return nil
	},
}

var listZonesCmd = &cobra.Command{
	Use:   "zones",
	Short: "List all zones",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient(cmd)
		if err != nil {
			return err
		}
		defer client.Close()

		zones, err := client.GetZones()
		if err != nil {
			return fmt.Errorf("get zones: %w", err)
		}

		if isJSON(cmd) {
			return json.NewEncoder(os.Stdout).Encode(zones)
		}

		fmt.Printf("% -30s % -15s % -10s\n", "NAME", "TYPE", "HREF")
		fmt.Println("-----------------------------------------------------------")
		for _, z := range zones {
			fmt.Printf("% -30s % -15s % -10s\n", z.Name, z.ControlType, z.Href)
		}
		return nil
	},
}

var listDevicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List all devices with status",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient(cmd)
		if err != nil {
			return err
		}
		defer client.Close()

		devices, err := client.GetDevices()
		if err != nil {
			return fmt.Errorf("get devices: %w", err)
		}

		zones, _ := client.GetZones()
		zoneNames := make(map[string]string)
		for _, z := range zones {
			zoneNames[z.Href] = z.Name
		}

		// Batch fetch all zone statuses in one request
		zoneStatusMap := make(map[string]float64)
		if allStatuses, err := client.GetAllZoneStatuses(); err == nil {
			for _, zs := range allStatuses {
				cleanHref := strings.TrimSuffix(zs.Href, "/status")
				zoneStatusMap[cleanHref] = zs.Level
			}
		}

		if isJSON(cmd) {
			var summaries []DeviceSummary
			for _, d := range devices {
				summary := DeviceSummary{
					Href:       d.Href,
					Name:       d.Name,
					DeviceType: d.DeviceType,
					Status:     "-",
				}
				if len(d.LocalZones) > 0 {
					zHref := d.LocalZones[0].Href
					summary.ZoneHref = zHref
					summary.ZoneName = zoneNames[zHref]
					if lvl, ok := zoneStatusMap[zHref]; ok {
						summary.Level = lvl
						if lvl > 0 {
							summary.Status = fmt.Sprintf("%.0f%%", lvl)
						} else {
							summary.Status = "OFF"
						}
					}
				}
				summaries = append(summaries, summary)
			}
			return json.NewEncoder(os.Stdout).Encode(summaries)
		}

		fmt.Printf("% -30s % -20s % -10s % -20s % -10s\n", "NAME", "TYPE", "STATUS", "ZONE NAME", "HREF")
		fmt.Println("-------------------------------------------------------------------------------------------------------------")

		for _, d := range devices {
			statusStr := "-"
			zn := "-"
			if len(d.LocalZones) > 0 {
				zHref := d.LocalZones[0].Href
				zn = zoneNames[zHref]
				if lvl, ok := zoneStatusMap[zHref]; ok {
					if lvl > 0 {
						statusStr = fmt.Sprintf("%.0f%%", lvl)
					} else {
						statusStr = "OFF"
					}
				}
			}
			fmt.Printf("% -30s % -20s % -10s % -20s % -10s\n", d.Name, d.DeviceType, statusStr, zn, d.Href)
		}
		return nil
	},
}

func init() {
	lutronCmd.AddCommand(listCmd)
	listCmd.AddCommand(listAreasCmd)
	listCmd.AddCommand(listZonesCmd)
	listCmd.AddCommand(listDevicesCmd)
}
