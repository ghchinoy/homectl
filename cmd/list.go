package cmd

import (
	"fmt"
	"log"
	"sync"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List resources from the Lutron bridge",
}

var listAreasCmd = &cobra.Command{
	Use:   "areas",
	Short: "List all areas",
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient(cmd)
		defer client.Close()

		areas, err := client.GetAreas()
		if err != nil {
			log.Fatalf("Failed to get areas: %v", err)
		}

		fmt.Printf("% -20s % -20s\n", "NAME", "HREF")
		fmt.Println("------------------------------------------")
		for _, a := range areas {
			fmt.Printf("% -20s % -20s\n", a.Name, a.Href)
		}
	},
}

var listZonesCmd = &cobra.Command{
	Use:   "zones",
	Short: "List all zones",
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient(cmd)
		defer client.Close()

		zones, err := client.GetZones()
		if err != nil {
			log.Fatalf("Failed to get zones: %v", err)
		}

		fmt.Printf("% -30s % -15s % -10s\n", "NAME", "TYPE", "HREF")
		fmt.Println("-----------------------------------------------------------")
		for _, z := range zones {
			fmt.Printf("% -30s % -15s % -10s\n", z.Name, z.ControlType, z.Href)
		}
	},
}

var listDevicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List all devices with status",
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient(cmd)
		defer client.Close()

		devices, err := client.GetDevices()
		if err != nil {
			log.Fatalf("Failed to get devices: %v", err)
		}

		zones, _ := client.GetZones()
		zoneNames := make(map[string]string)
		for _, z := range zones {
			zoneNames[z.Href] = z.Name
		}

		fmt.Printf("% -30s % -20s % -10s % -20s % -10s\n", "NAME", "TYPE", "STATUS", "ZONE NAME", "HREF")
		fmt.Println("-------------------------------------------------------------------------------------------------------------")
		
		var wg sync.WaitGroup
		type devStatus struct {
			index int
			status string
		}
		statusChan := make(chan devStatus, len(devices))

		for i, d := range devices {
			if len(d.LocalZones) > 0 {
				wg.Add(1)
				go func(idx int, zoneHref string) {
					defer wg.Done()
					status, err := client.GetZoneStatus(zoneHref)
					if err != nil {
						statusChan <- devStatus{idx, "Error"}
						return
					}
					statusStr := "OFF"
					if status.Level > 0 {
						statusStr = fmt.Sprintf("%%.0f%%", status.Level)
					}
					statusChan <- devStatus{idx, statusStr}
				}(i, d.LocalZones[0].Href)
			} else {
				statusChan <- devStatus{i, "-"}
			}
		}

		go func() {
			wg.Wait()
			close(statusChan)
		}()

		results := make([]string, len(devices))
		for s := range statusChan {
			results[s.index] = s.status
		}

		for i, d := range devices {
			zn := "-"
			if len(d.LocalZones) > 0 {
				zn = zoneNames[d.LocalZones[0].Href]
			}
			fmt.Printf("% -30s % -20s % -10s % -20s % -10s\n", d.Name, d.DeviceType, results[i], zn, d.Href)
		}
	},
}

func init() {
	lutronCmd.AddCommand(listCmd)
	listCmd.AddCommand(listAreasCmd)
	listCmd.AddCommand(listZonesCmd)
	listCmd.AddCommand(listDevicesCmd)
}