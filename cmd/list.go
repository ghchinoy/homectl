
package cmd

import (
	"fmt"
	"log"

	"github.com/ghchinoy/control/pkg/leap"
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

var listDevicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List all devices",
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient(cmd)
		defer client.Close()

		devices, err := client.GetDevices()
		if err != nil {
			log.Fatalf("Failed to get devices: %v", err)
		}

		fmt.Printf("% -30s % -20s % -15s % -10s\n", "NAME", "TYPE", "MODEL", "HREF")
		fmt.Println("------------------------------------------------------------------------------------------")
		for _, d := range devices {
			fmt.Printf("% -30s % -20s % -15s % -10s\n", d.Name, d.DeviceType, d.ModelNumber, d.Href)
		}
	},
}

func getClient(cmd *cobra.Command) *leap.Client {
	addr, _ := cmd.Flags().GetString("bridge")
	// For now, we assume certs are in the root directory
	client, err := leap.NewClient(addr+ ":8081", "lutron_client.crt", "lutron_client.key", "lutron_ca.crt")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	return client
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.AddCommand(listAreasCmd)
	listCmd.AddCommand(listDevicesCmd)
}
